# Raw HTTP API

For non-Python clients (Next.js, n8n, curl, Slack bots). The OpenAI-compatible `/v1/*` endpoints are documented separately — see [OpenAI-Compatible API](openai.md).

## Routes

| Route | Method | Description |
|---|---|---|
| `GET /api/workers` | GET | Live list of every worker on the mesh with telemetry. |
| `POST /api/execute` | POST | Sync streaming task dispatch. Body: `{"worker_id":..., "prompt":..., "task_id":...}`. Streams worker stdout back chunked. |
| `POST /api/execute/async` | POST | Fire-and-forget. Returns `202 {"task_id":...}` immediately. POSTs to `webhook_url` on completion (signed if `AGENTFM_WEBHOOK_SECRET` is set). The background task spawns *before* the 202 ack is written: a 202 means the task is being executed, even if the client hangs up before reading the body. |
| `GET /health` | GET | Unauthenticated liveness probe. Returns `{"status":"ok","online_workers":N}`. |
| `GET /metrics` | GET | Prometheus scrape endpoint (see [Observability](observability.md)). |

## `GET /api/workers`

```bash
curl http://127.0.0.1:8080/api/workers \
  -H 'Authorization: Bearer YOUR_KEY'
```

Response:

```json
{
  "agents": [
    {
      "peer_id": "12D3KooW...",
      "author": "alice",
      "name": "research-agent",
      "status": "AVAILABLE",
      "hardware": "llama3.2 (CPU: 12 Cores)",
      "description": "...",
      "cpu_usage_pct": 14.2,
      "ram_free_gb": 12.5,
      "current_tasks": 1,
      "max_tasks": 10,
      "has_gpu": false,
      "gpu_used_gb": 0.0,
      "gpu_total_gb": 0.0,
      "gpu_usage_pct": 0.0
    }
  ]
}
```

## `POST /api/execute`

Streaming task dispatch. The response is plain text streamed live as the worker writes to stdout.

```bash
curl -N http://127.0.0.1:8080/api/execute \
  -H 'Authorization: Bearer YOUR_KEY' \
  -H 'Content-Type: application/json' \
  -d '{"worker_id":"12D3KooW...","prompt":"Write a haiku","task_id":"task_abc123"}'
```

If `task_id` is omitted, the gateway generates one and returns it via `X-AgentFM-Task-Id` header.

## `POST /api/execute/async`

```bash
curl http://127.0.0.1:8080/api/execute/async \
  -H 'Authorization: Bearer YOUR_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "worker_id": "12D3KooW...",
    "prompt": "Long-running batch job",
    "webhook_url": "https://my-host.example.com:8000/cb"
  }'
# → 202 {"task_id":"task_abc123","status":"queued","message":"..."}
```

Webhook POSTs are bounded by a 30 s timeout, do not follow redirects, and the response body is capped at 64 KiB. URL is validated against private/loopback/link-local addresses (set `AGENTFM_WEBHOOK_ALLOW_PRIVATE=1` to opt back in for trusted private deploys).

When `AGENTFM_WEBHOOK_SECRET` is set, every webhook POST carries an HMAC-SHA256 signature in `X-AgentFM-Signature`. Receivers should verify in constant time (the Python `WebhookReceiver` does this).

### Capacity cap

`/api/execute/async` is bounded by `MaxInflightAsyncTasks` (256). When saturated, returns `503` with `Retry-After: 5` and an OpenAI-shaped envelope:

```json
{
  "error": {
    "message": "too many async tasks in flight; retry shortly",
    "type": "server_error",
    "code": "async_capacity_exhausted"
  }
}
```

## Error envelopes

All errors use OpenAI's standard envelope:

```json
{
  "error": {
    "message": "...",
    "type": "...",
    "code": "..."
  }
}
```

Common codes: `unauthorized`, `invalid_api_key`, `model_not_found`, `mesh_overloaded`, `worker_unreachable`, `worker_stream_failed`, `async_capacity_exhausted`, `internal_error`.

## Related

- [OpenAI-Compatible API](openai.md) — `/v1/*` routes
- [Authentication](auth.md) — bearer-token setup
- [Python SDK](../agentfm-python/README.md) — typed client
- [Observability](observability.md) — `/metrics` + `/health`
