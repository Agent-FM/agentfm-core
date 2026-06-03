# Observability

Every role exposes Prometheus `/metrics` and structured `slog` logs. A single Prometheus + Grafana stack watches the entire grid.

## Metric endpoints

| Role | Endpoint | Bind | Override |
|---|---|---|---|
| **Boss (api mode)** | `:8080/metrics` | same as `-apiport` | n/a — shares the API port |
| **Worker** | `:9090/metrics` | `127.0.0.1` (loopback) | `-prom-listen 0.0.0.0:9090`; `-prom-listen -` to disable |
| **Relay** | `:9091/metrics` | `127.0.0.1` (loopback) | same `-prom-listen` flag |
| **Witness** | `:9092/metrics` | `127.0.0.1` (loopback) | same `-prom-listen` flag |

## Metric families

| Name | Type | Labels |
|---|---|---|
| `agentfm_tasks_total` | Counter | `status` ∈ `{ok, error, rejected, timeout}` |
| `agentfm_task_duration_seconds` | Histogram | (none) — buckets tuned for AI workloads (1s → 30 min) |
| `agentfm_workers_online` | Gauge | (none) |
| `agentfm_artifact_bytes_sent_total` | Counter | (none) |
| `agentfm_stream_errors_total` | Counter | `protocol` ∈ `{task, artifacts, feedback}`, `reason` ∈ enum |
| `agentfm_auth_attempts_total` | Counter | `outcome` ∈ `{success, missing_bearer, malformed_bearer, invalid_token, rate_limited}` |
| `agentfm_dht_queries_total` | Counter | `op` ∈ `{find_peer, provide, get_value}` (relay only) |

Plus the standard `process_*` and `go_*` runtime collectors. **All label tuples are pre-warmed at zero** so dashboards don't false-alert on a fresh node.

### Cardinality discipline

Labels are deliberately bounded enums. Per-peer / per-task / per-key labels are explicitly avoided — they would explode Prometheus' time-series store on a busy mesh. If you need per-peer attribution, use the structured logs (which carry `peer_id` / `task_id` fields) and aggregate them in your log shipper.

## Recommended alerts

- `rate(agentfm_auth_attempts_total{outcome="invalid_token"}[5m]) > 1` — sustained bad-token attempts (could be brute-force or a stale client after rotation)
- `rate(agentfm_auth_attempts_total{outcome="rate_limited"}[5m]) > 0` — per-IP throttle is firing
- `agentfm_workers_online == 0 for 5m` — gateway has lost all workers
- `histogram_quantile(0.95, rate(agentfm_task_duration_seconds_bucket[5m])) > 600` — p95 task latency above 10 min
- `rate(agentfm_stream_errors_total{protocol="artifacts"}[5m]) > 0` — artifact transfers failing

## Structured logs

Every binary accepts `-log-format` (`json`/`console`/`auto`) and `-log-level` (`debug`/`info`/`warn`/`error`). `auto` (default) uses `console` on a TTY and `json` everywhere else. JSON entries always include `time`, `level`, `msg`, `component`, plus `err`/`task_id`/`peer_id`/`protocol` when applicable.

```json
{"time":"2026-04-26T18:42:11Z","level":"ERROR","msg":"worker stream","component":"api","err":"context deadline exceeded","task_id":"task_abc","protocol":"task"}
```

### Component tags

The `component` field is set from the `-mode` flag:
- `api` — boss running as HTTP gateway
- `boss` — boss running interactive TUI
- `worker` — worker node
- `relay` — relay / lighthouse node

This lets log shippers (Loki, ELK, Datadog) split a single mesh's logs into per-role streams without parsing the message body.

## Related

- [Authentication](auth.md) — `agentfm_auth_attempts_total` outcome labels
- [Architecture](architecture.md) — protocol surfaces that emit metrics
- [CLI Reference](cli.md) — `-prom-listen`, `-log-format`, `-log-level` flags
