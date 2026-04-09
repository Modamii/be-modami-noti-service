# REST API Reference

Base URL: `/v1/noti-services`

Swagger UI: `GET /swagger/` (when API server is running)

## Response Envelope

All endpoints return a consistent JSON envelope:

```json
{
  "data":   <any>,
  "meta":   <pagination meta | null>,
  "errors": [{ "code": "string", "field": "string", "message": "string" }]
}
```

Success responses include `data`. Error responses include `errors`.

---

## Auth

### Generate Centrifugo Token

```
POST /v1/noti-services/auth/centrifugo-token
```

Generates a JWT for the client to connect to Centrifugo WebSocket. See [websocket-centrifugo.md](websocket-centrifugo.md) for the full flow.

**Request**
```json
{ "user_id": "abc123" }
```

**Response 200**
```json
{
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**Response 400** — missing `user_id`
```json
{ "errors": [{ "message": "user_id is required" }] }
```

---

## Notifications

### List user notifications

```
GET /v1/noti-services/users/{userId}/notifications
```

Returns paginated notifications for a user, newest first.

**Path params**

| Param | Type | Description |
|-------|------|-------------|
| `userId` | string | User ID |

**Query params**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | int | `1` | Page number (1-based) |
| `per_page` | int | `20` | Items per page |
| `unread_only` | bool | `false` | Filter to unread only (`true` or `1`) |

**Response 200**
```json
{
  "data": [
    {
      "id":         "notif-xyz",
      "user_id":    "abc123",
      "event_type": "content_published",
      "title":      "New post from Alice",
      "body":       "Check this out",
      "link":       "/posts/post-1",
      "read":       false,
      "extra": {
        "actor_id":    "user-1",
        "content_id":  "post-1",
        "content_type": "post"
      },
      "created_at": "2026-04-09T10:00:00Z"
    }
  ],
  "meta": {
    "total":       42,
    "page":        1,
    "per_page":    20,
    "total_pages": 3,
    "has_more":    true
  }
}
```

---

### Count unread notifications

```
GET /v1/noti-services/users/{userId}/notifications/unread-count
```

**Response 200**
```json
{
  "data": { "count": 7 }
}
```

---

### Get notification by ID

```
GET /v1/noti-services/notifications/{id}
```

**Response 200**
```json
{
  "data": {
    "id":         "notif-xyz",
    "user_id":    "abc123",
    "event_type": "comment_created",
    "title":      "New comment on your post",
    "body":       "Great post!",
    "link":       "/posts/post-1#comment-comment-1",
    "read":       false,
    "extra":      { "actor_id": "user-2", "comment_id": "comment-1" },
    "created_at": "2026-04-09T10:05:00Z"
  }
}
```

**Response 404**
```json
{ "errors": [{ "message": "notification not found" }] }
```

---

### Mark notification as read

```
PATCH /v1/noti-services/notifications/{id}/read
```

**Response 204** — No Content

---

### Mark all notifications as read

```
PATCH /v1/noti-services/users/{userId}/notifications/read-all
```

**Response 200**
```json
{
  "data": { "updated": 7 }
}
```

---

### Delete notification

```
DELETE /v1/noti-services/notifications/{id}
```

**Response 204** — No Content

---

## Preferences

User preferences control which delivery channels are active.

### Get preferences

```
GET /v1/noti-services/users/{userId}/preferences
```

**Response 200**
```json
{
  "data": {
    "user_id":       "abc123",
    "in_app_enabled": true,
    "push_enabled":   true
  }
}
```

If no preference document exists for the user, defaults are returned (`true` for both).

---

### Update preferences

```
PUT /v1/noti-services/users/{userId}/preferences
```

**Request**
```json
{
  "in_app_enabled": true,
  "push_enabled":   false
}
```

**Response 204** — No Content

Preference updates take effect on the next notification dispatch. The `user_id` field in the body is ignored; the path param is used.

---

## Subscribers (Device Registration)

Register device tokens or Web Push subscriptions for push notification delivery.

### Register a subscriber

```
POST /v1/noti-services/users/{userId}/subscribers
```

Upserts a device subscription. If a subscriber with the same `(user_id, device_token)` already exists it is updated; otherwise a new document is created.

**Request — FCM (iOS/Android)**
```json
{
  "device_token": "fcm-token-abc",
  "platform":     "android"
}
```

**Request — Web Push**
```json
{
  "device_token":      "web-push-unique-id",
  "platform":          "web",
  "web_push_endpoint": "https://fcm.googleapis.com/fcm/send/...",
  "web_push_keys": {
    "p256dh": "BNcR...",
    "auth":   "tBHIJ..."
  }
}
```

**Platform values**: `ios`, `android`, `web`

**Response 201** — Created (no body)

---

### Delete a subscriber

```
DELETE /v1/noti-services/users/{userId}/subscribers/{token}
```

Unregisters a device token. Call this when the user logs out or the FCM token is invalidated.

| Param | Description |
|-------|-------------|
| `userId` | User ID |
| `token` | Device token (URL-encoded if it contains special characters) |

**Response 204** — No Content

---

## Health Probes

Not under the `/v1/noti-services` prefix. Available on all services.

```
GET /healthz    — Liveness probe
GET /readyz     — Readiness probe (checks dependencies)
```

**Response 200**
```json
{ "status": "ok" }
```

**Response 503** — Dependency unavailable
```json
{ "status": "unavailable", "details": { "mongodb": "connection refused" } }
```

---

## Error Codes

| HTTP | Scenario |
|------|---------|
| 400 | Missing or invalid request body / path param |
| 404 | Resource not found |
| 500 | Internal server error (check service logs) |

All errors follow the envelope format:
```json
{
  "errors": [{ "message": "description of the error" }]
}
```
