# Observability

Each worker exposes Prometheus metrics on its existing health port at `/metrics`.

| Service | Endpoint |
|---------|----------|
| worker-dispatch | `:7073/metrics` |
| worker-push | `:7074/metrics` |

The exposition is hand-rolled (`pkg/metrics`) rather than using the Prometheus
client library — a few counters and one gauge do not justify that dependency
tree. Swap it in if histograms or exemplars become necessary.

## Metrics

| Metric | Type | Meaning |
|--------|------|---------|
| `notif_queue_depth{queue="ws"}` | gauge | Messages waiting in `notif:ws` |
| `notif_queue_depth{queue="push"}` | gauge | Messages waiting in `notif:push` |
| `notif_dispatch_total` | counter | WebSocket messages published to Centrifugo |
| `notif_dispatch_failed_total` | counter | Messages dropped after a publish failure |
| `notif_push_sent_total` | counter | Pushes accepted by FCM |
| `notif_push_failed_total` | counter | Pushes FCM rejected for a retryable reason |
| `notif_push_pruned_total` | counter | Device tokens deleted as unregistered |

A gauge that cannot be sampled — Redis unreachable — is **omitted from the
scrape rather than reported as zero**. Zero would read as a healthy empty queue,
which is exactly the state an alert needs to distinguish it from.

## Alerts

**Queue depth is the earliest signal that a worker has stalled.** If a consumer
dies, its Redis list grows while `/healthz` and `/readyz` both keep answering —
the process is alive, it just is not consuming.

```yaml
- alert: NotificationQueueBacklog
  expr: notif_queue_depth > 500
  for: 5m
  annotations:
    summary: "{{ $labels.queue }} queue is backing up — is the worker consuming?"

- alert: NotificationQueueUnscrapable
  # The gauge disappearing means Redis could not be reached, not that it is empty.
  expr: absent(notif_queue_depth)
  for: 5m

- alert: NotificationDispatchDropping
  # Dispatch failures lose the message: it is already off the queue when the
  # publish is attempted. Any sustained rate needs a look.
  expr: rate(notif_dispatch_failed_total[5m]) > 0
  for: 10m

- alert: PushFailureRateHigh
  expr: >
    rate(notif_push_failed_total[10m])
      / clamp_min(rate(notif_push_sent_total[10m]) + rate(notif_push_failed_total[10m]), 1)
      > 0.1
  for: 15m
  annotations:
    summary: "Over 10% of pushes are failing — check FCM credentials and quota"
```

`notif_push_pruned_total` needs no alert: pruning is healthy behaviour. Watch it
as a rate instead — a sudden spike usually means a bad app release invalidated a
large number of tokens at once.

## Not covered

- **Centrifugo connection count.** Centrifugo exposes its own Prometheus
  endpoint; scrape that directly rather than proxying it through this service.
- **Event-to-client latency.** Measuring it end to end needs a timestamp that
  survives from producer to browser, which no component carries today.
