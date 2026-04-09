# Redis Queue

Internal queue layer that decouples event ingestion from downstream delivery workers.

## Design

Two independent Redis List keys act as work queues:

| Key | Producer | Consumer | Purpose |
|-----|---------|---------|---------|
| `notif:ws` | `InAppDispatcher` | `worker-dispatch` | In-app WebSocket delivery |
| `notif:push` | `PushDispatcher` | `worker-push` | Mobile / browser push delivery |

**Protocol**: `LPUSH` to enqueue, `BRPOP` (blocking pop) to consume. This gives FIFO ordering and zero-polling idle workers.

```
Ingest  ──LPUSH notif:ws──▶  Redis List
                                  │
                             BRPOP (blocks)
                                  │
                                  ▼
                           worker-dispatch
```

## Message Types

### WSMessage — for `notif:ws`

Sent by `InAppDispatcher`. Consumed by `worker-dispatch`.

```go
// pkg/event/event.go
type WSMessage struct {
    RoomID  string         `json:"room_id"`   // e.g. "user:abc123"
    Event   string         `json:"event"`     // identity, e.g. "content_published"
    Payload map[string]any `json:"payload"`   // notification data
}
```

**Example JSON** on the queue:
```json
{
  "room_id": "user:abc123",
  "event":   "content_published",
  "payload": {
    "notification_id": "notif-xyz",
    "title": "New post from Alice",
    "body":  "Check this article out",
    "link":  "/posts/post-1",
    "actor_id":    "user-1",
    "content_id":  "post-1",
    "content_type": "post"
  }
}
```

### PushMessage — for `notif:push`

Sent by `PushDispatcher`. Consumed by `worker-push`.

```go
// pkg/event/event.go
type PushMessage struct {
    DeviceTokens  []string              `json:"device_tokens"`
    Subscriptions []WebPushSubscription `json:"subscriptions,omitempty"`
    Title         string                `json:"title"`
    Body          string                `json:"body"`
    Link          string                `json:"link"`
}

type WebPushSubscription struct {
    Endpoint string            `json:"endpoint"`
    Keys     map[string]string `json:"keys"`  // p256dh, auth
}
```

**Example JSON** on the queue:
```json
{
  "device_tokens": ["fcm-token-user-abc", "fcm-token-user-abc-tablet"],
  "subscriptions": [
    {
      "endpoint": "https://fcm.googleapis.com/fcm/send/...",
      "keys": { "p256dh": "...", "auth": "..." }
    }
  ],
  "title": "New post from Alice",
  "body":  "Check this article out",
  "link":  "/posts/post-1"
}
```

## Queue Implementation

```go
// internal/queue/queue.go
type Queue struct { rdb *redis.Client }

func (q *Queue) Enqueue(ctx, key string, v any) error {
    b, _ := json.Marshal(v)
    return q.rdb.LPush(ctx, key, b).Err()
}

func (q *Queue) Consume(ctx, key string, timeout time.Duration, fn func([]byte) error) error {
    for {
        res, err := q.rdb.BRPop(ctx, timeout, key).Result()
        if err == redis.Nil { continue }
        if err != nil { return err }
        fn([]byte(res[1]))  // res[0] is key name, res[1] is value
    }
}
```

`BRPOP` with a timeout means workers sleep cheaply when the queue is empty and wake immediately when a message arrives. The `timeout` is used to re-check context cancellation.

## Worker Dispatch

`cmd/worker-dispatch` loops over `notif:ws`:

```
BRPOP notif:ws
  → Unmarshal WSMessage
  → channel = ChannelFromRoomID(msg.RoomID)   // "notifications:user:abc123"
  → Centrifugo.Publish(channel, msg.Event, msg.Payload)
      POST http://centrifugo:8000/api
      { "method": "publish", "params": { "channel": "...", "data": {...} } }
```

Worker health server listens on `:7070` with `/healthz` and `/readyz`.

## Worker Push

`cmd/worker-push` loops over `notif:push`:

```
BRPOP notif:push
  → Unmarshal PushMessage
  → Log payload (current stub)
  → [Future] Send FCM batch notification
  → [Future] Send Web Push with VAPID keys
```

Worker health server listens on `:7071`.

### FCM Integration (planned)

When integrating FCM, `worker-push` should:

1. Load `fcm.credentials_path` service account JSON
2. For each `DeviceToken`: send `messaging.Message{Token, Notification{Title, Body}, Webpush{Link}}`
3. Handle expired/invalid tokens by calling `DELETE /v1/noti-services/users/{userId}/subscribers/{token}`

### Web Push Integration (planned)

For `Subscriptions` (Web Push):
1. Load VAPID keys
2. For each `WebPushSubscription`: `webpush.SendNotification(payload, sub, options)`

## Configuration

Queue keys are configured in `config/config.yaml`:

```yaml
queue:
  ws_key:   "notif:ws"
  push_key: "notif:push"
```

Keys must be consistent across ingest and workers. If you change them, redeploy all three services together.

## Monitoring

Key Redis metrics to watch:

| Metric | Command | Alert if |
|--------|---------|---------|
| Queue depth | `LLEN notif:ws` | > 1000 (worker falling behind) |
| Queue depth | `LLEN notif:push` | > 1000 |
| Consumer lag | compare enqueue vs consume rate | growing consistently |

```bash
# Check queue depth
redis-cli LLEN notif:ws
redis-cli LLEN notif:push

# Peek at next item without consuming
redis-cli LINDEX notif:ws -1
```
