# Base Notification / WS / Push (Golang)

Event → Handler (build + enqueue) → Redis ws/push queue → Worker → Delivery. No workflow engine; Redis List + go-redis only.

## Cấu trúc

- **cmd/ingest** — Kafka (copy config từ `config/kafka`) hoặc HTTP webhook → handler → enqueue
- **cmd/api** — REST: feeds, preferences, subscribers (wire store từ `config/mongo`)
- **cmd/ws-gateway** — WebSocket (gorilla/websocket), subscribe Redis PubSub, broadcast theo room
- **cmd/worker-ws** — BRPOP `notif:ws` → Publish Redis PubSub → gateway broadcast
- **cmd/worker-push** — BRPOP `notif:push` → stub log (FCM/Web Push sau)

## Config (env)

| Env | Mặc định |
|-----|----------|
| REDIS_URL | redis://localhost:6379 |
| NOTIF_QUEUE_WS | notif:ws |
| NOTIF_QUEUE_PUSH | notif:push |
| NOTIF_PUBSUB_WS | notif:ws:dispatch |
| MONGO_URI | mongodb://localhost:27017 |
| MONGO_DB | notifications |
| API_ADDR | :7070 |
| WS_GATEWAY_ADDR | :8081 |
| INGEST_ADDR | :7071 |

Copy cấu hình Kafka/Mongo/Redis của bạn vào:

- `config/kafka/` — consumer/producer
- `config/mongo/` — client
- `config/redis/` — (tùy chọn) wrapper; mặc định dùng REDIS_URL + go-redis

## Chạy

```bash
go mod tidy

# Terminal 1: API
go run ./cmd/api

# Terminal 2: WS Gateway (subscribe PubSub, phục vụ WS)
go run ./cmd/ws-gateway

# Terminal 3: Worker WS (BRPOP → PubSub)
go run ./cmd/worker-ws

# Terminal 4: Worker Push (BRPOP → stub log)
go run ./cmd/worker-push

# Terminal 5: Ingest (webhook hoặc Kafka)
go run ./cmd/ingest
```

Test webhook:

```bash
curl -X POST http://localhost:7074/webhook -H "Content-Type: application/json" -d '{"type":"post.published","payload":{"post_id":"p1","author_id":"a1","title":"Hi","body":"Body","audience_ids":["u1","u2"]}}'
```

WS client: kết nối tới `ws://localhost:8081/ws`, gửi đầu tiên: `{"action":"subscribe","user_id":"u1","topic":""}`.
