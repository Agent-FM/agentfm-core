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
| `GET /ui/peer/{peer_id}` | GET | v1.3 — single-page reputation viewer. Unauthenticated; reads via the routes below. |
| `GET /v1/peers/{peer_id}/reputation` | GET | v1.3 — fetch scores + equivocator status + agent info for a peer. |
| `GET /v1/peers/{peer_id}/log?limit=M&offset=N` | GET | v1.3 — paginated entries + signed head from this Boss's local ledger. Each entry carries `entry_hash` (hex leaf hash) — feed it straight to `/proof?entry=`. |
| `GET /v1/peers/{peer_id}/proof?entry={hex_hash}` | GET | v1.3 — RFC 6962 inclusion proof for an entry (the `entry_hash` from `/log`). |
| `GET /v1/peers/{peer_id}` | GET | v1.3 — one-shot peer summary (name, capability, score, equivocator flag, last-seen). |
| `POST /v1/peers/{peer_id}/comments` | POST | v1.3 — caller-signed free-text comment. |
| `POST /v1/peers/{peer_id}/comments/self` | POST | v1.3 — self-signed comment + optional `rating` (`[-1,+1]`); the Boss signs it with its own identity. Body: `{"text":..., "rating":0.8, "language":"en"}`. This is what the desktop "rate" button calls. |
| `GET /v1/peers/{peer_id}/comments/{cid}` | GET | Comment body as `text/plain`. |
| `GET /v1/peers/{peer_id}/comments/{cid}.json` | GET | Comment body as JSON: `{cid, body, language}`. |
| `POST /api/relay/test` | POST | Probe a candidate relay multiaddr from this Boss (the desktop "Test connection" button). Body: `{"multiaddr":"/ip4/.../p2p/12D3KooW..."}` → `{"ok":bool, "peer_id":..., "error":...}`. Refuses private/link-local ranges. |
| `GET /v1/about` | GET | Boss/relay identity + mesh info (peer id, relay multiaddr, ledger tree size, version, uptime). |
| `GET /v1/events` | GET | Server-Sent-Events stream of mesh events (`worker_online`, `worker_offline`, `entry_appended`). Powers the desktop's live refresh. |
| `POST /v1/chat/completions`, `/v1/completions`, `GET /v1/models` | — | OpenAI-compatible — see [openai.md](openai.md). |

## v1.3 Verifiable agent mesh endpoints

See [Trust & Verification](trust.md) for the underlying threat model and CLI / SDK equivalents.

### `GET /v1/peers/{peer_id}/reputation`

```bash
curl http://127.0.0.1:8080/v1/peers/12D3KooW.../reputation \
  -H 'Authorization: Bearer YOUR_KEY'
```

Response:
```json
{
  "peer_id": "12D3KooW...",
  "scores": {"honesty": 0.42},
  "rating_count": 7,
  "last_updated": "2026-05-16T08:12:33Z",
  "is_equivocator": false,
  "agent_image_ref": "ghcr.io/agentfm/sick-leave-generator:v1",
  "agent_image_digest": "sha256:abc...",
  "agent_capability": "hr-specialist"
}
```

Equivocators always have `is_equivocator: true` and `scores.honesty: -1.0` regardless of other ratings — the marker is permanent (manual rehab via CLI / private API in v1.4).

### `GET /v1/peers/{peer_id}/log`

Returns ledger entries plus the current signed head. Query params:

- `from=N` (default `1`, 1-based, inclusive)
- `limit=M` (default `100`, max `1000`)

### `GET /v1/peers/{peer_id}/proof?entry={hex}`

Returns an RFC 6962 inclusion proof. Caller verifies offline by:
1. Hashing the entry with `HashLeaf` (`SHA-256(0x00 || canonical_bytes)`).
2. Walking the `audit_path`, combining with `HashChildren` (`SHA-256(0x01 || left || right)`) at each level.
3. Comparing the derived root to `head.root_hash`.

### `POST /v1/peers/{peer_id}/comments`

Submits a signed free-text comment about `peer_id`. v1.3 only supports SELF-submission (the rater must be the Boss's own libp2p identity). External-submitter delegation lands in v1.4.

Body:
```json
{
  "rater_peer_id": "12D3KooW...",
  "text": "Worked great for our use case",
  "language": "en",
  "attached_rating_hash": "ff...",
  "signature": "<base64 Ed25519 signature over the canonical comment digest>"
}
```

Response (201 Created):
```json
{
  "cid": "1220abc...",
  "ledger_hash": "deadbeef..."
}
```

Errors:
- `400 bad_request` — missing/invalid field, malformed peer ID
- `400 body_too_large` — text exceeds 10 KiB
- `401 bad_signature` — signature didn't verify against rater's libp2p key
- `403 non_self_submitter` — rater isn't this gateway's own identity
- `503 ledger_unavailable` — the boss wasn't constructed with a ledger handle

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
