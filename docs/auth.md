# Authentication

The gateway runs in **solo-dev mode** by default: bound to `127.0.0.1`, no API keys, no auth. Off-host exposure requires both flags.

## Enable bearer auth

```bash
# Generate a token (any opaque string ≥ 16 chars works; prefer 32+ random bytes)
TOKEN=$(openssl rand -hex 32)

# Start the gateway with bearer auth + off-host bind
AGENTFM_API_KEYS="$TOKEN" agentfm -mode api -api-bind 0.0.0.0 -apiport 8080
```

Multiple keys are accepted (comma-separated): `AGENTFM_API_KEYS="key1,key2,key3"` — each is a valid bearer.

## Safe-by-default invariants

**Startup refusal.** Setting `--api-bind 0.0.0.0` (or any non-loopback) without `AGENTFM_API_KEYS` aborts with a non-zero exit code and a clear error message. To intentionally expose an unauthenticated gateway (private network, behind a reverse proxy with its own auth), set `AGENTFM_ALLOW_UNAUTH_PUBLIC=1`.

**Open routes.** `/health` and `/metrics` are intentionally unauthenticated — load-balancer probes and Prometheus scrapers don't carry bearer tokens.

**Constant-time comparison.** Token check uses `crypto/subtle.ConstantTimeCompare` over every configured token (no first-match short-circuit). The number of valid tokens does not leak through timing.

**Per-IP rate limiting.** Failed auth attempts are throttled per remote IP (token bucket, 30/min) with a bounded LRU map. Successful requests are not rate-limited.

> **Behind a reverse proxy.** The rate limiter keys on `RemoteAddr`, which is the proxy's IP when AgentFM sits behind nginx / Caddy / Cloudflare. In that topology the per-IP throttle becomes a shared quota across all clients coming through the proxy. Either keep the gateway as the edge (loopback bind + `AGENTFM_API_KEYS` for off-host clients), or rely on the proxy's own rate limiting upstream of AgentFM.

**Async submission cap.** `/api/execute/async` is bounded by `MaxInflightAsyncTasks` (256). When saturated, returns `503` with `Retry-After: 5` and an OpenAI-shaped envelope (`code: async_capacity_exhausted`).

**Operator monitoring.** Alert on the `agentfm_auth_attempts_total{outcome="invalid_token"}` and `{outcome="rate_limited"}` Prometheus counters rather than scraping logs — the per-failure log line at WARN can be noisy during legitimate token rotation.

## From the Python SDK

```python
from agentfm import AgentFMClient, AsyncAgentFMClient, AuthenticationError

# Three modes:
client = AgentFMClient(api_key="your-key")          # explicit
client = AgentFMClient()                             # falls back to AGENTFM_API_KEY env var
client = AgentFMClient(api_key=None)                 # explicit no-auth (skips env fallback)

# Override on a derived client:
short_lived = client.with_options(api_key="other-key")
no_auth = client.with_options(api_key=None)

# 401 envelopes raise:
try:
    client.workers.list()
except AuthenticationError as e:
    print(e.code, e.status, e.message)  # "invalid_api_key" 401 "..."
```

> `client.api_key` is read-after-construction. Mutating it after the client exists does NOT update the underlying `httpx.Client.headers["Authorization"]`. Use `with_options(api_key=...)` to derive a new client.

## From the OpenAI SDK

```python
from openai import OpenAI
client = OpenAI(base_url="http://gateway:8080/v1", api_key="your-key")
```

The OpenAI SDK forwards `api_key` as `Authorization: Bearer <key>` — drop-in compatible with AgentFM's auth.

## Error envelope (401)

```json
{
  "error": {
    "message": "missing or malformed Bearer token",
    "type": "invalid_request_error",
    "code": "unauthorized"
  }
}
```

`code` is one of `unauthorized` (no/malformed bearer), `invalid_api_key` (wrong token), or `rate_limited` (per-IP throttle exhausted).

## Related

- [OpenAI-Compatible API](openai.md) — gateway routes that this auth protects
- [Security Model](security.md) — full threat model
- [Python SDK](../agentfm-python/README.md) — `AuthenticationError` exception
