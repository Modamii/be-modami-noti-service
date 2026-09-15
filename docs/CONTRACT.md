# Event Contract

Defines the Kafka message format and webhook format that external services use to trigger notifications.

The notification service consumes these events, maps each `identity` to a handler, and dispatches to the appropriate delivery channels (in-app WebSocket, push, email).

---

## Event Envelope

Every event — whether delivered via Kafka or HTTP webhook — shares the same JSON envelope:

```json
{
  "identity": "content_published",
  "payload":  { "actor": {...}, "do": [...], "io": [...], "po": [...] },
  "metadata": { "x-request-id": "req-abc" },
  "extra":    { "to": ["user-2"] }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `identity` | string | yes | Event type; determines handler and channels |
| `payload` | BaseEvent | yes | RDF-like actor/do/io/po structure |
| `metadata` | object | no | Tracing headers, request IDs |
| `extra` | object | no | `to` (recipient IDs), `ignore`, `old_data`, custom fields |

Go type: `pkg/contract/event.go` → `NotificationEvent`

---

## Payload — BaseEvent

The `payload` field uses an RDF-inspired grammar with four roles:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `actor` | EventObject | yes | Who performed the action (user or system) |
| `do` | EventObject[] | yes | Main objects: post, comment, article, … |
| `io` | EventObject[] | no | Indirect objects; e.g. the post containing a comment |
| `po` | EventObject[] | no | Prepositional objects; e.g. the group the post belongs to |
| `context` | any | no | Per-identity extra data |

### EventObject

Every entity in `actor`, `do`, `io`, `po` follows this shape:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Entity ID |
| `type` | string | yes | `user` \| `system` \| `post` \| `article` \| `comment` \| `group` \| … |
| `data` | object | no | Type-specific fields (title, body, content, audience_ids, …) |

---

## Identities and Channels

An *identity* names the event type. The notification service maps each identity to a set of delivery channels.

### techinsight / modami

Copy is composed here, by a bespoke handler per identity.

| Identity | Description | Channels |
|----------|-------------|----------|
| `content_published` | A post or article was published | `in_app`, `push` |
| `comment_created` | A comment was added to content | `in_app`, `push` |

### lingocast

All lingocast identities share one handler (`internal/handlers/lingocast.go`).
The producer owns the wording and supplies `title`, `body` and `link` in
`do[0].data`; everything else in `do[0].data` is passed through to the client.
Adding a lingocast notification type needs an identity constant and a channel
mapping here — no new handler.

| Identity | Description | Channels |
|----------|-------------|----------|
| `video_ready` | Video processed, transcript available | `in_app`, `push` |
| `video_failed` | Pipeline failed at some step | `in_app`, `push` |
| `video_progress` | Pipeline step changed | `in_app` |
| `flashcard_due` | Cards are due for review | `in_app`, `push` |
| `streak_at_risk` | Streak will be lost today | `push` |
| `achievement_unlocked` | New achievement earned | `in_app`, `push` |
| `challenge_leaderboard` | Challenge standings changed | `in_app` (broadcast) |
| `challenge_ended` | Challenge finished, results ready | `in_app`, `push` |

`video_progress` and `challenge_leaderboard` are in-app only — they are useful
while the screen is open and would be noise as a push. `streak_at_risk` is push
only, since the streak counter is already visible in-app.

Channel values: `in_app` (WebSocket via Centrifugo), `push` (FCM / Web Push), `email` (future).

Mapping lives in: `pkg/contract/channels.go` → `IdentityChannels`

---

## WebSocket Channels and Access

| Channel | Who may subscribe |
|---------|-------------------|
| `noti:user:{userID}` | The owner only |
| `noti:topic:{topic}` | Any authenticated user — public broadcast |
| `noti:challenge:{id}` | Requires a subscription token minted for exactly this user and channel |

Enforced in one place: `internal/gateway/policy.go` → `ChannelPolicy.Allow`.
A challenge channel is shared between participants, so the noti service cannot
judge membership itself — the service that owns the challenge mints a
subscription token (`centrifugo.GenerateSubscriptionToken`) after checking the
user is a participant. Token-gated channels are never client-publishable.

### In-app message payload

`InAppDispatcher` sends everything the client needs to render without a
follow-up API call:

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Matches the stored notification; used to dedupe on reconnect |
| `event_type` | string | The identity |
| `title`, `body` | string | Render directly |
| `link` | string | Deep-link target; omitted when empty |
| `unread_count` | int | The recipient's unread total after this notification |
| `created_at` | RFC3339 | |

Plus every key from `do[0].data`. `device_tokens` is stripped — it is a push
delivery detail and never reaches a client. Broadcast messages (challenge
channels) carry no `id`, `unread_count` or `created_at`, since they are not
per-recipient.

---

## Kafka Transport

### Topic → Identity Mapping

```
pkg/contract/topic_mapping.go → TopicToIdentity
```

| Kafka Topic | Identity |
|-------------|----------|
| `modami.content.published` | `content_published` |
| `modami.comment.created` | `comment_created` |

### Message Format

- **Key**: `payload.do[0].id` (for partition locality)
- **Value**: JSON-encoded envelope (see above)
- **Headers**: optional; `x-request-id` for tracing

### Consumer

`cmd/ingest` subscribes via `KafkaService.Subscribe(topics, handler)`. The ingest service:

1. Deserializes the Kafka value into `NotificationEvent`
2. Looks up the handler from the registry by `identity`
3. Calls `handler(ctx, event)`

Config: `kafka.broker_list`, `kafka.consumer_group`, `kafka.enable`

---

## Webhook Transport

For services that cannot use Kafka, the ingest service also accepts HTTP webhooks:

```
POST http://ingest:8082/webhook
Content-Type: application/json

{ same envelope JSON }
```

The webhook endpoint performs the same dispatch as the Kafka path. Useful for:
- External services outside the Kafka cluster
- Local development and manual testing

---

## Example Messages

### `content_published`

```json
{
  "identity": "content_published",
  "payload": {
    "actor": {
      "id": "user-1",
      "type": "user",
      "data": { "fullname": "Alice" }
    },
    "do": [{
      "id": "post-1",
      "type": "post",
      "data": {
        "title": "Hello World",
        "body":  "Check this out",
        "audience_ids": ["user-2", "user-3"]
      }
    }]
  },
  "metadata": { "x-request-id": "req-123" },
  "extra": { "to": ["user-2", "user-3"] }
}
```

Handler behaviour:
- Creates a `Notification` for each recipient in `extra.to`
- Title: `"New post from <actor.data.fullname>"`
- Link: `/posts/<do[0].id>`

### `comment_created`

```json
{
  "identity": "comment_created",
  "payload": {
    "actor": { "id": "user-2", "type": "user" },
    "do": [{
      "id": "comment-1",
      "type": "comment",
      "data": { "content": "Nice post!" }
    }],
    "io": [{ "id": "post-1", "type": "post" }]
  },
  "extra": { "to": ["user-1"] }
}
```

Handler behaviour:
- Creates a `Notification` for each recipient in `extra.to`
- Title: `"New comment on your post"`
- Link: `/posts/<io[0].id>#comment-<do[0].id>`

---

## `extra.to` Field

`extra.to` is the authoritative list of recipient user IDs. The handler reads it to know who should receive the notification.

- Producers are responsible for populating `extra.to` with the correct recipients.
- If `extra.to` is empty the handler must derive recipients from the payload (e.g. `do[0].data.audience_ids`).

---

## Adding a New Notification Type

1. **Add identity constant** — `pkg/contract/identity.go`
   ```go
   const ReactionAdded = "reaction_added"
   var AllIdentities = []string{..., ReactionAdded}
   ```

2. **Map channels** — `pkg/contract/channels.go`
   ```go
   ReactionAdded: {ChannelInApp, ChannelPush},
   ```

3. **Map Kafka topic** — `pkg/contract/topic_mapping.go`
   ```go
   "modami.reaction.added": ReactionAdded,
   ```

4. **Implement handler** — `internal/handlers/reaction_added.go`
   ```go
   func ReactionAdded(svc *service.NotificationService) handlers.Handler {
       return func(ctx context.Context, evt *contract.NotificationEvent) error {
           // 1. Extract actor, do[0], io[0] from evt.Payload
           // 2. Build service.NotifyParams
           // 3. Call svc.Process(ctx, params)
           return nil
       }
   }
   ```

5. **Register handler** — `cmd/ingest/main.go`
   ```go
   reg.Register(contract.ReactionAdded, handlers.ReactionAdded(notifSvc))
   ```

6. **Document** — add the identity to the tables above with example JSON

---

## Go Types Reference

```go
// pkg/contract/event.go
type NotificationEvent struct {
    Identity string         `json:"identity"`
    Payload  BaseEvent      `json:"payload"`
    Metadata map[string]any `json:"metadata,omitempty"`
    Extra    ExtraData      `json:"extra,omitempty"`
}

type BaseEvent struct {
    Actor   EventObject   `json:"actor"`
    Do      []EventObject `json:"do"`
    IO      []EventObject `json:"io,omitempty"`
    PO      []EventObject `json:"po,omitempty"`
    Context any           `json:"context,omitempty"`
}

type EventObject struct {
    ID   string         `json:"id"`
    Type string         `json:"type"`
    Data map[string]any `json:"data,omitempty"`
}

type ExtraData struct {
    To      []string       `json:"to,omitempty"`
    Ignore  []string       `json:"ignore,omitempty"`
    OldData map[string]any `json:"old_data,omitempty"`
}

// pkg/contract/identity.go — see the file for the full lingocast set
const (
    ContentPublished = "content_published"
    CommentCreated   = "comment_created"
    VideoReady       = "video_ready"
    // …
)

// pkg/contract/channels.go
var IdentityChannels = map[string][]string{
    ContentPublished: {ChannelInApp, ChannelPush},
    VideoProgress:    {ChannelInApp},
    StreakAtRisk:     {ChannelPush},
    // …
}
```
