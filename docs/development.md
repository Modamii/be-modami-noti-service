# Development Guide

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.25+ | https://go.dev/dl |
| swag | latest | `go install github.com/swaggo/swag/cmd/swag@latest` |
| Docker | any | https://docs.docker.com/get-docker |
| MongoDB | 6+ | Docker or local |
| Redis | 7+ | Docker or local |
| Centrifugo | 5+ | Docker or local |

## Local Setup

### 1. Clone and install dependencies

```bash
git clone <repo>
cd be-modami-noti-service
make deps
```

### 2. Start infrastructure

```bash
# MongoDB + Redis via Docker
docker run -d --name mongo -p 27017:27017 mongo:6
docker run -d --name redis -p 6379:6379 redis:7

# Centrifugo (local config)
docker run -d --name centrifugo -p 8000:8000 -p 10000:10000 \
  -v $(pwd)/deploy/centrifugo/config.local.json:/centrifugo/config.json \
  centrifugo/centrifugo:latest centrifugo --config=config.json
```

Or use Docker Compose:
```bash
docker compose up -d
```

### 3. Run services

Each service runs independently. Open separate terminals:

```bash
make run-api              # REST API          → localhost:7073
make run-ingest           # Kafka + webhook   → localhost:7074
make run-ws-gateway       # Centrifugo proxy  → localhost:7072
make run-worker-dispatch  # WS fanout worker  → :7073 (health)
make run-worker-push      # Push worker       → :7074 (health)
```

## Make Commands

```
make help                 Show all targets

make deps                 Download Go modules
make tidy                 go mod tidy
make fmt                  Format all Go files
make fmt-check            Check formatting (CI)
make vet                  Run go vet
make test                 Run all tests
make ci                   deps + fmt-check + vet + test + build-all

make swagger              Regenerate Swagger docs from godoc annotations

make run-api              Run API server
make run-ingest           Run ingest service
make run-ws-gateway       Run WebSocket gateway
make run-worker-dispatch  Run dispatch worker
make run-worker-push      Run push worker

make build-api            Build API binary → bin/api
make build-ingest         Build ingest binary → bin/ingest
make build-worker-dispatch
make build-worker-push
make build-all            Build all binaries

make docker-build-api     Build API Docker image
make docker-build-all     Build all Docker images
```

## Swagger / API Docs

Swagger is auto-generated from godoc annotations on handler methods.

```bash
make swagger
# Writes: docs/docs.go, docs/swagger.json, docs/swagger.yaml
```

Browse at: `http://localhost:7073/swagger/`

### Annotation format

```go
// MyHandler godoc
// @Summary     Short description
// @Description Longer description
// @Tags        notifications
// @Produce     json
// @Param       userId path   string true "User ID"
// @Param       page   query  int    false "Page number" default(1)
// @Success     200 {object} httputil.Response{data=domain.Notification}
// @Failure     400 {object} httputil.Response
// @Router      /users/{userId}/notifications [get]
func (h *NotificationHandler) MyHandler(w http.ResponseWriter, r *http.Request) {
```

Base path and service info are set in `cmd/api/main.go` header comments:
```go
// @title       Modami Notification Service API
// @version     1.0
// @BasePath    /v1/noti-services
```

## Testing

### Run all tests

```bash
make test
```

### Test webhook manually

```bash
# content_published
curl -X POST http://localhost:7074/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "identity": "content_published",
    "payload": {
      "actor": { "id": "user-1", "type": "user", "data": { "fullname": "Alice" } },
      "do": [{
        "id": "post-1",
        "type": "post",
        "data": { "title": "Hello", "body": "World", "audience_ids": ["user-2"] }
      }]
    },
    "extra": { "to": ["user-2"] }
  }'

# comment_created
curl -X POST http://localhost:7074/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "identity": "comment_created",
    "payload": {
      "actor": { "id": "user-2", "type": "user" },
      "do": [{ "id": "c-1", "type": "comment", "data": { "content": "Nice!" } }],
      "io": [{ "id": "post-1", "type": "post" }]
    },
    "extra": { "to": ["user-1"] }
  }'
```

### Test REST API

```bash
# Get Centrifugo token
curl -X POST http://localhost:7073/v1/noti-services/auth/centrifugo-token \
  -H "Content-Type: application/json" \
  -d '{ "user_id": "user-2" }'

# List notifications
curl http://localhost:7073/v1/noti-services/users/user-2/notifications

# Unread count
curl http://localhost:7073/v1/noti-services/users/user-2/notifications/unread-count

# Mark all read
curl -X PATCH http://localhost:7073/v1/noti-services/users/user-2/notifications/read-all

# Get/set preferences
curl http://localhost:7073/v1/noti-services/users/user-2/preferences

curl -X PUT http://localhost:7073/v1/noti-services/users/user-2/preferences \
  -H "Content-Type: application/json" \
  -d '{ "in_app_enabled": true, "push_enabled": false }'

# Register device
curl -X POST http://localhost:7073/v1/noti-services/users/user-2/subscribers \
  -H "Content-Type: application/json" \
  -d '{ "device_token": "fcm-abc123", "platform": "android" }'
```

### Check health

```bash
curl http://localhost:7073/healthz
curl http://localhost:7074/healthz
curl http://localhost:7073/healthz
curl http://localhost:7074/healthz
```

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
           // 1. Extract fields from evt.Payload
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

6. **Update docs** — add to [contract.md](contract.md)

## Project Layout

```
.
├── cmd/
│   ├── api/              REST API server
│   ├── ingest/           Kafka consumer + HTTP webhook
│   ├── ws-gateway/       Centrifugo proxy callbacks
│   ├── worker-dispatch/  WebSocket fanout worker
│   └── worker-push/      Push notification worker
├── config/
│   └── config.yaml       Application configuration
├── deploy/
│   ├── centrifugo/       Centrifugo config files
│   └── k8s/              Kubernetes manifests
├── docs/                 Documentation (this folder)
├── internal/
│   ├── api/              HTTP handlers + route registration
│   ├── domain/           Data models (Notification, Preference, Subscriber)
│   ├── gateway/          Centrifugo proxy handler + rate limiter
│   ├── handlers/         Notification event handlers
│   ├── queue/            Redis queue wrapper
│   ├── service/          Business logic (NotificationService, dispatchers)
│   └── store/            Repository interfaces + MongoDB implementations
└── pkg/
    ├── centrifugo/       Token generation, HTTP client, channel naming
    ├── contract/         Event envelope, identity constants, topic mapping
    ├── event/            Queue message types (WSMessage, PushMessage)
    ├── health/           Health check handlers
    ├── httputil/         Response helpers, middleware
    ├── kafka/            KafkaService (producer + consumer)
    └── storage/          MongoDB + Redis client wrappers
```
