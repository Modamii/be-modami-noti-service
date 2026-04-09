# Architecture

## Overview

The notification service is a **distributed, event-driven system** composed of five independent Go binaries. Each binary has a single responsibility and communicates through Kafka, Redis queues, or HTTP.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         EVENT SOURCES                                   │
│  Kafka Topics                    HTTP Webhook                           │
│  (producer services)             POST /webhook                          │
└────────────────┬─────────────────────────┬──────────────────────────────┘
                 │                         │
                 ▼                         ▼
┌────────────────────────────────────────────────────────────────────────┐
│                      cmd/ingest  (:7071)                               │
│  • Consumes Kafka topics                                               │
│  • Accepts HTTP webhook                                                │
│  • Validates + dispatches NotificationEvent via handler registry       │
│  • Persists to MongoDB                                                 │
│  • Enqueues to Redis (notif:ws, notif:push)                            │
└────────────┬──────────────────────────────────────┬────────────────────┘
             │                                      │
      notif:ws (Redis)                     notif:push (Redis)
             │                                      │
             ▼                                      ▼
┌─────────────────────────┐          ┌──────────────────────────┐
│  cmd/worker-dispatch    │          │   cmd/worker-push        │
│  (:7070 health)         │          │   (:7071 health)         │
│  BRPOP notif:ws         │          │   BRPOP notif:push       │
│  → Publish to Centrifugo│          │   → FCM / Web Push       │
└───────────┬─────────────┘          └──────────────────────────┘
            │
            ▼
┌───────────────────────────────────────────────────────────────┐
│                   Centrifugo  (:8000)                         │
│  WebSocket server                                             │
│  Proxies auth requests → ws-gateway                           │
└───────────────────────┬───────────────────────────────────────┘
            ▲           │ proxy callbacks
            │           ▼
            │  ┌─────────────────────────┐
            │  │  cmd/ws-gateway (:7072) │
            │  │  POST /centrifugo/connect│
            │  │  POST /centrifugo/subscribe
            │  │  POST /centrifugo/publish│
            │  └─────────────────────────┘
            │
     WebSocket connections
            │
┌───────────┴───────────┐
│   Browser / Mobile    │
│   clients             │
└───────────────────────┘

┌───────────────────────────────────────────────────────────────┐
│                    cmd/api  (:7070)                           │
│  REST API — read-only (for clients)                           │
│  • GET/PATCH/DELETE notifications                             │
│  • GET/PUT preferences                                        │
│  • POST/DELETE subscribers                                    │
│  • POST auth/centrifugo-token                                 │
└───────────────────────────────────────────────────────────────┘
```

## Components

### cmd/api — REST API Server

Serves all client-facing HTTP endpoints. Stateless — connects only to MongoDB.

- Routes registered under `/v1/noti-services/`
- Swagger UI at `/swagger/`
- Health probes at `/healthz`, `/readyz`

### cmd/ingest — Event Ingestion

The ingestion gateway. Receives events from two sources in parallel:

1. **Kafka consumer** — `KafkaService` polls configured topics, maps topic → identity, validates and dispatches via handler registry.
2. **HTTP webhook** — `POST /webhook` accepts raw `NotificationEvent` JSON for direct integration.

After handler processing, `NotificationService.Process()` runs the full pipeline: persist → filter → enrich → enqueue.

### cmd/ws-gateway — Centrifugo Proxy

Handles the three Centrifugo proxy callbacks. Centrifugo calls these endpoints before allowing client actions.

- **connect** — Validates the client JWT, extracts `userID`, auto-subscribes to personal channel.
- **subscribe** — Enforces channel access policy.
- **publish** — Rate-limits client-side publishes.

### cmd/worker-dispatch — WebSocket Fanout

Blocking Redis consumer (`BRPOP notif:ws`). For each message:
1. Deserialize `WSMessage`
2. Derive Centrifugo channel via `ChannelFromRoomID(msg.RoomID)`
3. `POST /api/publish` to Centrifugo HTTP API

### cmd/worker-push — Push Notification Worker

Blocking Redis consumer (`BRPOP notif:push`). Currently logs payload; designed for FCM/Web Push integration.

## Persistence

| Store | Technology | Collections |
|-------|-----------|-------------|
| Notifications | MongoDB | `notifications` |
| Preferences | MongoDB | `preferences` |
| Subscribers | MongoDB | `subscribers` |
| WS queue | Redis List | `notif:ws` |
| Push queue | Redis List | `notif:push` |

## Design Patterns

| Pattern | Where |
|---------|-------|
| **Strategy** | `ChannelDispatcher` interface — pluggable delivery per channel |
| **Registry** | `handlers.Registry` — maps identity string → handler function |
| **Repository** | `store.NotificationStore` / `PreferenceStore` / `SubscriberStore` interfaces |
| **Middleware chain** | `httputil.Chain()` — Recovery → RequestID → RequestLogging → CORS |
| **Token bucket** | `gateway.RateLimiter` — per-user publish rate limiting |
| **Event envelope** | `contract.NotificationEvent` — RDF-like actor/do/io/po grammar |

## Service Ports Summary

| Service | Port | Protocol |
|---------|------|---------|
| api | `:7070` | HTTP |
| ingest | `:7071` | HTTP |
| ws-gateway | `:7072` | HTTP (called by Centrifugo) |
| worker-dispatch | `:7070` | HTTP (health only) |
| worker-push | `:7071` | HTTP (health only) |
| Centrifugo | `:8000` | HTTP + WebSocket |
| MongoDB | `:27017` | TCP |
| Redis | `:6379` | TCP |
| Kafka | `:7072` | TCP |
