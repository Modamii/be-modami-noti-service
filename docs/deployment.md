# Deployment Guide

## Docker Compose (Local / Development)

### Start Centrifugo

```bash
docker compose -f docker-compose.yml up -d centrifugo
```

Centrifugo config used: `deploy/centrifugo/config.local.json`

Ports exposed:
- `8000` — WebSocket + API endpoint
- `10000` — Admin UI (http://localhost:10000)

### Start all services

```bash
docker compose up -d
```

---

## Centrifugo Configuration

### Production (`deploy/centrifugo/config.json`)

Key settings:

```json
{
  "token_hmac_secret_key": "${CENTRIFUGO_TOKEN_HMAC_SECRET_KEY}",
  "api_key":               "${CENTRIFUGO_API_KEY}",
  "admin":                 true,
  "admin_password":        "${CENTRIFUGO_ADMIN_PASSWORD}",
  "admin_secret":          "${CENTRIFUGO_ADMIN_SECRET}",
  "allowed_origins":       ["https://app.modami.com"],

  "proxy_connect_endpoint":   "http://ws-gateway:7072/centrifugo/connect",
  "proxy_subscribe_endpoint": "http://ws-gateway:7072/centrifugo/subscribe",
  "proxy_publish_endpoint":   "http://ws-gateway:7072/centrifugo/publish",
  "proxy_connect_timeout":    "5s",
  "proxy_subscribe_timeout":  "3s",
  "proxy_publish_timeout":    "3s",

  "engine": "redis",
  "redis_address": "redis:6379",

  "namespaces": [{
    "name": "notifications",
    "presence": true,
    "history_size": 100,
    "history_ttl": "300s",
    "recover": true
  }]
}
```

### Local (`deploy/centrifugo/config.local.json`)

```json
{
  "token_hmac_secret_key": "centrifugo-hmac-secret",
  "api_key": "centrifugo-api-key",
  "admin": true,
  "allowed_origins": ["http://localhost:3000", "http://localhost:5173"],
  "log_level": "debug"
}
```

### Required Environment Variables

| Variable | Description |
|----------|-------------|
| `CENTRIFUGO_TOKEN_HMAC_SECRET_KEY` | JWT signing secret — **must match** `centrifugo.hmac_secret` in service config |
| `CENTRIFUGO_API_KEY` | Server API key — **must match** `centrifugo.api_key` in service config |
| `CENTRIFUGO_ADMIN_PASSWORD` | Admin UI login password |
| `CENTRIFUGO_ADMIN_SECRET` | Admin API secret |
| `CENTRIFUGO_ALLOWED_ORIGINS` | Comma-separated WebSocket origins (never `*` in production) |

---

## Kubernetes

Manifests are in `deploy/k8s/`.

```bash
# Create namespace
kubectl apply -f deploy/k8s/namespace.yaml

# Secrets (edit values first)
kubectl apply -f deploy/k8s/secret.yaml

# ConfigMap
kubectl apply -f deploy/k8s/configmap.yaml

# Deploy services
kubectl apply -f deploy/k8s/api-deployment.yaml
kubectl apply -f deploy/k8s/ingest-deployment.yaml
kubectl apply -f deploy/k8s/worker-dispatch-deployment.yaml
kubectl apply -f deploy/k8s/worker-push-deployment.yaml
kubectl apply -f deploy/k8s/centrifugo-deployment.yaml

# Autoscaling
kubectl apply -f deploy/k8s/hpa.yaml
```

### Health Probes

All services expose:
- `GET /healthz` — liveness
- `GET /readyz` — readiness

Configure in K8s deployment spec:
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 15

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

---

## Service Secrets

Never commit secrets to git. Use environment variables or a secrets manager.

| Secret | Services | Description |
|--------|---------|-------------|
| `MONGODB_URI` | api, ingest | MongoDB connection string with credentials |
| `REDIS_PASS` | ingest, worker-dispatch, worker-push | Redis password |
| `CENTRIFUGO_HMAC_SECRET` | api, ws-gateway | JWT signing secret |
| `CENTRIFUGO_API_KEY` | worker-dispatch | Centrifugo HTTP API key |
| `FCM_CREDENTIALS_PATH` / file | worker-push | Firebase service account |

---

## Docker Images

Built with multi-stage Dockerfiles in `build/docker/`:

| Image | Dockerfile | Entrypoint |
|-------|-----------|-----------|
| api | `Dockerfile.api` | `cmd/api` |
| ingest | `Dockerfile.ingest` | `cmd/ingest` |
| worker-dispatch | `Dockerfile.worker-dispatch` | `cmd/worker-dispatch` |
| worker-push | `Dockerfile.worker-push` | `cmd/worker-push` |

```bash
# Build all
make docker-build-all

# Build individual
make docker-build-api
make docker-build-ingest
make docker-build-worker-dispatch
make docker-build-worker-push

# Push (set REGISTRY and TAG)
REGISTRY=myregistry TAG=v1.2.3 make docker-build-all
```

---

## Production Hardening Checklist

### Security

- [ ] `centrifugo.hmac_secret` is a strong random secret (≥ 32 bytes)
- [ ] `centrifugo.api_key` is a strong random secret
- [ ] `allowed_origins` is set to exact frontend domains (not `*`)
- [ ] Centrifugo admin port (`:10000`) is not publicly exposed
- [ ] MongoDB uses authenticated connection string
- [ ] Redis uses password authentication
- [ ] `redis.tls_config.insecure_skip_verify` is `false`
- [ ] Secrets are injected via environment variables or secrets manager (not hardcoded)
- [ ] Different secrets per environment

### Availability

- [ ] At least 2 replicas of each service
- [ ] MongoDB replica set or Atlas cluster
- [ ] Redis with persistence (AOF) or Redis Sentinel/Cluster
- [ ] Centrifugo with Redis engine (for multi-replica state sharing)
- [ ] HPA configured for ingest and worker-dispatch

### Observability

- [ ] `observability.log_level` set to `warn` or `info` in production
- [ ] `observability.otlp_endpoint` configured for distributed tracing
- [ ] Alert on `LLEN notif:ws > 1000` (worker-dispatch falling behind)
- [ ] Alert on `LLEN notif:push > 1000`
- [ ] Alert on Centrifugo disconnect spikes
- [ ] MongoDB slow query logging enabled

### Operational

- [ ] MongoDB indexes created at startup (`mongostore.EnsureIndexes`)
- [ ] Kafka topic retention matches expected event volume
- [ ] `kafka.consumer_group` is unique per environment
- [ ] `queue.ws_key` and `queue.push_key` are consistent across ingest and workers
- [ ] Centrifugo proxy URLs point to correct ws-gateway service
