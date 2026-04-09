# Configuration Reference

Config file: `config/config.yaml`

Loaded by `config/config.go` using Viper. Environment variables override any field using `UPPER_SNAKE_CASE` with `_` replacing `.` (e.g. `MONGODB_URI` overrides `mongodb.uri`).

An environment-specific override file is merged if it exists:
```
config/config.{app.environment}.yaml
```

---

## app

General application metadata and HTTP server settings.

```yaml
app:
  name: "Modami chat"
  version: "1.0.0"
  environment: "development"   # development | staging | production
  debug: true
  port: 8087                   # informational; actual port set in servers.api_addr
  host: "localhost"
  shutdown_timeout: 30s        # graceful shutdown window
  read_timeout: 30s            # HTTP server read timeout
  write_timeout: 30s           # HTTP server write timeout
  idle_timeout: 120s           # HTTP keep-alive idle timeout
  cors_origins:
    - "*"                      # restrict in production
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `modami-notification-service` | Service name (used in logs) |
| `version` | string | `1.0.0` | Service version |
| `environment` | string | `development` | Affects log format, topic prefixes |
| `debug` | bool | `false` | Verbose debug output |
| `port` | int | `8080` | Informational port |
| `host` | string | `localhost` | Informational host |
| `shutdown_timeout` | duration | `30s` | Graceful shutdown timeout |
| `read_timeout` | duration | `30s` | HTTP read timeout |
| `write_timeout` | duration | `30s` | HTTP write timeout |
| `idle_timeout` | duration | `120s` | HTTP idle connection timeout |
| `cors_origins` | []string | `["*"]` | Allowed CORS origins |

---

## mongodb

```yaml
mongodb:
  uri: "mongodb://localhost:27017/?directConnection=true"
  database: "modami-core-service"
  timeout: 10s
  max_pool_size: 100
  min_pool_size: 0
  max_idle_time: 900s
  retry_writes: true
  read_concern: "majority"
  write_concern: "majority"
  enable_logging: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `uri` | string | `mongodb://localhost:27017` | MongoDB connection string |
| `database` | string | `notifications` | Database name |
| `timeout` | duration | `10s` | Connection/operation timeout |
| `max_pool_size` | uint64 | `100` | Max connections in pool |
| `min_pool_size` | uint64 | `0` | Min idle connections |
| `max_idle_time` | duration | — | Max time a connection can be idle |
| `retry_writes` | bool | `true` | Retry transient write errors |
| `read_concern` | string | — | `majority` \| `local` \| `linearizable` |
| `write_concern` | string | — | `majority` \| `1` etc. |
| `enable_logging` | bool | `false` | Enable MongoDB driver debug logs |

---

## redis

```yaml
redis:
  host: "localhost"
  port: 6379
  database: 0
  rate_limit_database: 5
  ttl: 259200s
  pool_size: 100
  pass: ""
  user_name: ""
  write_timeout: 600s
  read_timeout: 600s
  dial_timeout: 600s
  tls_config:
    insecure_skip_verify: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `localhost` | Redis host |
| `port` | int | `6379` | Redis port |
| `database` | int | `0` | Redis DB index for queues |
| `rate_limit_database` | int | `5` | Redis DB index for rate limiter |
| `ttl` | duration | — | Default key TTL (informational) |
| `pool_size` | int | `10` | Connection pool size |
| `pass` | string | — | Redis password |
| `user_name` | string | — | Redis ACL username |
| `write_timeout` | duration | `600s` | Write operation timeout |
| `read_timeout` | duration | `600s` | Read operation timeout |
| `dial_timeout` | duration | `600s` | Connection dial timeout |
| `tls_config.insecure_skip_verify` | bool | `true` | Skip TLS cert verification (disable in prod) |

`RedisAddr()` builds the connection address as `host:port`.

---

## kafka

```yaml
kafka:
  broker_list: "localhost:7072"
  enable: true
  tls_enable: false
  partition: 1
  partitioner: "random"
  sasl_producer_username: ""
  sasl_producer_password: ""
  sasl_consumer_username: ""
  sasl_consumer_password: ""
  user_activated_topic_name: "modami.auth.user.activated"
  consumer_group: "user-service-group"
  client_id: "user-service"
  env: "local"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `broker_list` | string | — | Comma-separated broker addresses |
| `enable` | bool | `false` | Whether to start Kafka consumer |
| `tls_enable` | bool | `false` | Enable TLS for Kafka |
| `partition` | int | `1` | Default partition count for new topics |
| `partitioner` | string | `random` | Producer partitioner strategy |
| `sasl_producer_username` | string | — | SASL credentials for producer |
| `sasl_producer_password` | string | — | SASL credentials for producer |
| `sasl_consumer_username` | string | — | SASL credentials for consumer |
| `sasl_consumer_password` | string | — | SASL credentials for consumer |
| `user_activated_topic_name` | string | — | Topic name to consume |
| `consumer_group` | string | — | Kafka consumer group ID |
| `client_id` | string | — | Kafka client identifier |
| `env` | string | `local` | Environment prefix for topic names |

`KafkaConfig.Brokers()` splits `broker_list` by `,` and trims whitespace.

---

## queue

```yaml
queue:
  ws_key:   "notif:ws"
  push_key: "notif:push"
```

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `ws_key` | string | `notif:ws` | yes | Redis list key for WebSocket jobs |
| `push_key` | string | `notif:push` | yes | Redis list key for push jobs |

Both keys are **required**. Startup will fail if either is empty.

---

## servers

```yaml
servers:
  api_addr:     ":7070"
  ingest_addr:  ":7071"
  gateway_addr: ":7072"   # set by default, not in config.yaml
```

| Field | Default | Service |
|-------|---------|---------|
| `api_addr` | `:7070` | cmd/api |
| `ingest_addr` | `:7071` | cmd/ingest |
| `gateway_addr` | `:7072` | cmd/ws-gateway |

---

## centrifugo

```yaml
centrifugo:
  api_url:     "http://localhost:8000/api"
  api_key:     "centrifugo-api-key"
  hmac_secret: "centrifugo-hmac-secret"
  token_ttl:   3600
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `api_url` | string | `http://localhost:8000/api` | Centrifugo HTTP API endpoint |
| `api_key` | string | — | Server API key (set in Centrifugo config as `api_key`) |
| `hmac_secret` | string | — | JWT signing secret (must match Centrifugo `token_hmac_secret_key`) |
| `token_ttl` | int | `3600` | Connection token lifetime in seconds |

---

## fcm

```yaml
fcm:
  credentials_path: ""
```

| Field | Description |
|-------|-------------|
| `credentials_path` | Path to Firebase service account JSON file |

---

## observability

```yaml
observability:
  service_name:    "core-service"
  service_version: "1.0.0"
  environment:     "development"
  log_level:       "info"
  otlp_endpoint:   ""
  otlp_insecure:   true
```

| Field | Default | Description |
|-------|---------|-------------|
| `service_name` | `modami-notification-service` | Reported service name in logs/traces |
| `service_version` | `1.0.0` | Reported service version |
| `environment` | `development` | Environment label in logs |
| `log_level` | `info` | Log level: `debug` \| `info` \| `warn` \| `error` |
| `otlp_endpoint` | — | OpenTelemetry collector endpoint (empty = disabled) |
| `otlp_insecure` | `true` | Skip TLS for OTLP exporter |

Used by `cfg.ToLoggingConfig()` to initialize the logger.

---

## Environment Variable Overrides

Any field can be overridden with an environment variable. Replace `.` with `_` and uppercase:

```bash
MONGODB_URI=mongodb://prod-host:27017
REDIS_HOST=redis-prod
REDIS_PASS=secret
CENTRIFUGO_API_KEY=prod-api-key
CENTRIFUGO_HMAC_SECRET=prod-hmac-secret
KAFKA_BROKER_LIST=kafka-1:7072,kafka-2:7072
KAFKA_ENABLE=true
QUEUE_WS_KEY=notif:ws
QUEUE_PUSH_KEY=notif:push
OBSERVABILITY_LOG_LEVEL=warn
```
