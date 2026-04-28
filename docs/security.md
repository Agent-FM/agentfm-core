# Security Model

Zero-trust threat model. Every remote peer is treated as potentially slow, faulty, or malicious.

## Defense layers

| Layer | Defense |
|---|---|
| **Transport** | End-to-end encrypted libp2p streams (Noise / TLS). |
| **Peer identity** | Peer IDs are Ed25519 public keys; identities persist via `relay_identity.key` (mode `0600`). |
| **HTTP gateway auth** | `AGENTFM_API_KEYS` enables bearer-token auth on `/api/*` and `/v1/*`; constant-time comparison; per-IP rate limiting on failed attempts. Loopback bind is the default; non-loopback bind without keys refuses to start unless `AGENTFM_ALLOW_UNAUTH_PUBLIC=1`. See [Authentication](auth.md). |
| **Private networks** | `-swarmkey` enables PSK; non-key-holders dropped before any protocol byte. See [Private Swarms](private-swarms.md). |
| **Execution** | Every task runs in a fresh Podman container with `--rm --network host`. SIGKILLed the instant the stream dies. **Caveat:** `--network host` gives the agent container full access to the worker's loopback (Ollama, internal admin endpoints, cloud metadata at `169.254.169.254`). Treat agent images as **trusted code**; review their Dockerfiles before running. The worker prints a startup warning to this effect. |
| **DoS / Slow-loris** | Every libp2p stream has explicit deadlines. HTTP server has full timeout matrix. Payloads capped with `io.LimitReader`. Async submissions capped at 256 in-flight. |
| **Webhook SSRF** | Webhook URLs validated against private/loopback/link-local; resolves at validation AND re-validates each candidate IP at dial-time (TOCTOU safe). Set `AGENTFM_WEBHOOK_ALLOW_PRIVATE=1` to opt back in. |
| **Webhook integrity** | Set `AGENTFM_WEBHOOK_SECRET` to enable HMAC-SHA256 signing (`X-AgentFM-Signature` header). Python `WebhookReceiver(secret=...)` verifies in constant time. |
| **Webhook DoS** | Response body capped at 64 KiB; redirects not followed; 30 s connection timeout. |
| **Artifact safety** | Zip extraction defends against zip-slip + zip-bomb (per-entry chunked copy, 1 GiB total budget, partial files unlinked on overflow). TaskIDs are alphabet-validated before being joined into a path. |
| **Resource budgets** | Workers reject tasks past `-maxcpu`, `-maxgpu`, `-maxtasks`. |

## What AgentFM does NOT protect against

- **A malicious worker you've voluntarily dispatched to with sensitive prompt data.** On the public mesh, treat your prompt as "published." If confidentiality matters, **use a private swarm.**
- **Container-level escape.** AgentFM uses Podman's default isolation (rootless + cgroup limits). Hardware-isolated runtimes (gVisor, Kata, Firecracker) are out of scope; if your threat model needs them, run AgentFM workers inside those.
- **Side-channel attacks.** Workers run on shared host hardware. Spectre/Meltdown-style cross-tenant leakage is not addressed.
- **Network metadata.** A network observer can see which IP-to-IP libp2p connections exist and roughly how much data flows between them, even though the stream payloads are encrypted. Use Tor or a VPN if metadata privacy matters.

## Operator hardening checklist

- [ ] `AGENTFM_API_KEYS` set on every off-host gateway. (`--api-bind 0.0.0.0` without it fails fast — but check.)
- [ ] `swarm.key` is `0600` and never committed. AgentFM warns at load time but doesn't refuse.
- [ ] Reverse proxy in front of the gateway? Set `AGENTFM_ALLOW_UNAUTH_PUBLIC=1` and rely on the proxy for auth + rate-limiting (the in-built per-IP limiter shares quota across all clients behind a proxy — see [Authentication](auth.md)).
- [ ] Workers running in production use a real container runtime / VM if the agent images aren't trusted.
- [ ] Prometheus alerts wired on `agentfm_auth_attempts_total{outcome="invalid_token"}` and `{outcome="rate_limited"}`. See [Observability](observability.md).
- [ ] `AGENTFM_WEBHOOK_SECRET` set whenever async tasks fan out to operator-controlled URLs.

## Related

- [Authentication](auth.md) — bearer-token specifics
- [Private Swarms](private-swarms.md) — PSK-gated darknet mode
- [Architecture](architecture.md) — wire-protocol surfaces being defended
- [Observability](observability.md) — alerting on attack signal
