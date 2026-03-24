# Centrifugo Deploy Notes

This note documents how to self-host Centrifugo (open-source), run with Docker, and expose admin UI safely.

## 1) Is Centrifugo self-hosted and open-source?

Yes.

- Centrifugo is open-source and can be self-hosted.
- You can run it as:
  - Docker container
  - Kubernetes deployment
  - binary process
- It provides an admin web interface (UI). In production, keep it private and protected.

## 2) Integration model in this project

- App services publish realtime events to Centrifugo API.
- Clients connect to Centrifugo using WebSocket/SSE.
- Proxy endpoints in this project validate connect/subscribe/publish requests.

Related code:
- `internal/gateway/handler.go`
- `pkg/centrifugo/client.go`
- `pkg/centrifugo/proxy.go`

## 3) Docker quick start

Use the compose file:

- `deploy/docker-compose.centrifugo.yml`

Start:

```bash
docker compose -f deploy/docker-compose.centrifugo.yml up -d
```

Default exposed ports:
- `8000`: client/public API endpoint
- `10000`: admin endpoint (restrict access in production)

## 4) Environment variables and config

Minimal variables:

- `CENTRIFUGO_TOKEN_HMAC_SECRET_KEY`: JWT HMAC key for client tokens.
- `CENTRIFUGO_API_KEY`: server API key used by backend publisher.
- `CENTRIFUGO_ADMIN_PASSWORD`: admin UI password.
- `CENTRIFUGO_ADMIN_SECRET`: admin API secret.
- `CENTRIFUGO_ALLOWED_ORIGINS`: CORS/origin allowlist (never `*` in production).

Config file used by compose:
- `deploy/centrifugo/config.json`

## 5) Does Centrifugo have UI?

Yes, admin web UI is available when admin is enabled.

Production guidance:
- Do not expose admin endpoint publicly.
- Put admin behind private network/VPN/IP allowlist.
- Rotate admin/API/token secrets.
- Enable TLS at ingress/load balancer.

## 6) Production hardening checklist

- Network:
  - Keep admin port internal only.
  - Add ingress auth for admin route.
- Security:
  - No default credentials.
  - Keep `allowed_origins` strict.
  - Rotate keys regularly.
- Availability:
  - Configure liveness/readiness probes.
  - Set CPU/memory requests and limits.
  - Use multiple replicas behind load balancer (if needed).
- Observability:
  - Track connection count, subscribe/publish errors, disconnect spikes.
  - Alert on unusual reconnect storms.

## 7) Common pitfalls

- Using wildcard origins in production.
- Exposing admin UI to internet.
- Reusing same secret across environments.
- Missing proxy auth checks for channel access.

