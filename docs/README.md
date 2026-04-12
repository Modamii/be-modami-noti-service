# Modami Notification Service — Documentation

Real-time notification service built on Go, Kafka, Redis, MongoDB, and Centrifugo.

## Documents

| File | Description |
|------|-------------|
| [services.md](services.md) | Chức năng từng service, full flow FE↔WS↔Kafka |
| [architecture.md](architecture.md) | System overview, components, ports, design patterns |
| [notification-flow.md](notification-flow.md) | Event ingestion → processing → dispatch pipeline |
| [websocket-centrifugo.md](websocket-centrifugo.md) | WebSocket auth, Centrifugo proxy, channel model |
| [queue.md](queue.md) | Redis queue internals, message contracts, workers |
| [contract.md](contract.md) | Kafka event contract, identities, payload examples |
| [api.md](api.md) | REST API reference with request/response examples |
| [configuration.md](configuration.md) | Full config reference (`config/config.yaml`) |
| [deployment.md](deployment.md) | Docker, Kubernetes, Centrifugo deploy guide |
| [development.md](development.md) | Local setup, make commands, adding new features |

## Quick Start

```bash
# Install tools
go install github.com/swaggo/swag/cmd/swag@latest

# Run locally (requires MongoDB, Redis, Centrifugo)
make run-api
make run-ingest
make run-ws-gateway
make run-worker-dispatch
make run-worker-push

# Regenerate Swagger docs
make swagger
```

## Services and Ports

| Service | Default Port | Purpose |
|---------|-------------|---------|
| api | `:7070` | REST API |
| ingest | `:7071` | Kafka consumer + HTTP webhook |
| ws-gateway | `:7072` | Centrifugo proxy callbacks |
| worker-dispatch | `:7073` | WebSocket fanout worker |
| worker-push | `:7074` | Push notification worker |
| Centrifugo | `:8000` | WebSocket server |
