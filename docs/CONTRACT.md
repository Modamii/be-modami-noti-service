# Notification Event Contract

Services that need to trigger notifications send events to Kafka using this contract. The notification service consumes messages, dispatches by **identity**, and enqueues to in_app / push / email queues.

## 1. Event envelope (message sent to Kafka)

Each Kafka message **value** must be JSON of the following shape:

| Field    | Type       | Required | Description |
| -------- | ---------- | -------- | ----------- |
| identity | string     | yes      | Event type; determines handler and channels (e.g. `content_published`, `comment_created`) |
| payload  | BaseEvent  | yes      | RDF-like payload: actor, do, io?, po? |
| metadata | object     | no       | Tracing (e.g. `x-request-id`) |
| extra    | object     | no       | Optional: `to` (recipients), `ignore`, `old_data`, custom fields |

**Kafka message format:**

- **key**: `payload.do[0].id` (or join of `payload.do[].id`) for partitioning.
- **value**: `JSON.stringify({ identity, payload, metadata?, extra? })`. Do not nest metadata inside payload.
- **headers**: Optional; e.g. request-id in headers if your Kafka client supports it.

## 2. Payload – BaseEvent (RDF-like)

`payload` must follow this structure:

| Field   | Type          | Required | Description |
| ------- | ------------- | -------- | ----------- |
| actor  | EventObject   | yes      | Who performed the action (user or system) |
| do     | EventObject[] | yes      | Main object(s): post, comment, article, ... |
| io     | EventObject[] | no       | Indirect object(s); e.g. post that contains the comment |
| po     | EventObject[] | no       | Prepositional; e.g. group |
| context| any           | no       | Extra per-identity data |

## 3. Object – EventObject

Every entity in `actor`, `do`, `io`, `po` uses this format:

| Field | Type     | Required | Description |
| ----- | -------- | -------- | ----------- |
| id    | string   | yes      | Entity ID |
| type  | string   | yes      | Object type: `user`, `system`, `post`, `article`, `comment`, `group`, ... |
| data  | object   | no       | Type-specific fields (title, body, content, audience_ids, ...) |

## 4. Identity and channels

| Identity             | Description                    | Channels   |
| -------------------- | ------------------------------ | ---------- |
| content_published    | Content (post/article) published | in_app, push |
| comment_created      | Comment created                | in_app, push |

Channels: `in_app` (WebSocket), `push` (FCM/Web Push), `email` (future). The notification service uses this mapping to enqueue to the correct queues.

## 5. Example Kafka message

**content_published**

```json
{
  "identity": "content_published",
  "payload": {
    "actor": { "id": "user-1", "type": "user", "data": { "fullname": "Alice" } },
    "do": [
      { "id": "post-1", "type": "post", "data": { "title": "Hello", "body": "World", "audience_ids": ["user-2", "user-3"] } }
    ]
  },
  "metadata": { "x-request-id": "req-123" },
  "extra": { "to": ["user-2", "user-3"] }
}
```

**comment_created**

```json
{
  "identity": "comment_created",
  "payload": {
    "actor": { "id": "user-2", "type": "user" },
    "do": [ { "id": "comment-1", "type": "comment", "data": { "content": "Nice post!", "parent_id": "post-1" } } ],
    "io": [ { "id": "post-1", "type": "post" } ]
  },
  "extra": { "to": ["user-1"] }
}
```

**Kafka key**: Use `payload.do[0].id` (e.g. `post-1` or `comment-1`) for partitioning.

## 6. Adding a new feature

1. **Identity**: Add a new constant (e.g. `reaction_added`) in `pkg/contract/identity.go` and document it here.
2. **Payload type**: Define the payload shape (BaseEvent with appropriate do/io/po). Optionally add a typed struct in `pkg/contract/payloads.go`.
3. **Channels**: Add the identity to `IdentityChannels` in `pkg/contract/channels.go` (e.g. `[in_app, push]`).
4. **Handler**: Implement a handler in `internal/handlers/` that reads envelope payload and extra, resolves recipients, and enqueues to ws/push/email queues. Register the handler in `cmd/ingest/main.go` with `reg.Register(contract.NewIdentity, handlers.NewHandler(...))`.
5. **Documentation**: Update this file with the new identity and an example message.

Producers (other services) can import `pkg/contract` (Go) or follow this document and the example JSON to produce correctly formatted messages.
