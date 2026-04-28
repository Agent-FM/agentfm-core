# CLI Reference

The `agentfm` binary is multi-mode; the `agentfm-relay` binary is a dedicated lighthouse.

## `agentfm`

| Flag | Default | Description |
|---|:---:|---|
| `-mode` | *required* | `boss`, `worker`, `relay`, `api`, `test`, `genkey` |
| `-agentdir` | (none) | Path to agent dir (must contain `Dockerfile`/`Containerfile`) |
| `-image` | (none) | Podman image tag |
| `-agent` | (none) | Advertised agent name (max 20 chars) |
| `-model` | `llama3.2` | Advertised model engine (max 40 chars) |
| `-desc` | (none) | Agent description (max 1000 chars) |
| `-author` | `Anonymous` | Operator handle (max 50 chars) |
| `-maxtasks` | `1` | Max concurrent tasks (1-1000) |
| `-maxcpu` | `80.0` | Reject tasks above this CPU % (0-99) |
| `-maxgpu` | `80.0` | Reject tasks above this GPU VRAM % (0-99) |
| `-apiport` | `8080` | Port for `-mode api` HTTP gateway |
| `-api-bind` | `127.0.0.1` | Bind host for `-mode api`. Loopback by default; pass `0.0.0.0` to expose off-host (also requires `AGENTFM_API_KEYS` or `AGENTFM_ALLOW_UNAUTH_PUBLIC=1`) |
| `-prom-listen` | mode default | Prometheus `/metrics` bind. `-` disables |
| `-log-format` | `auto` | `json`, `console`, or `auto` (TTY → console, else json) |
| `-log-level` | `info` | `debug` / `info` / `warn` / `error` |
| `-swarmkey` | (none) | Path to `swarm.key` for private mesh |
| `-bootstrap` | *public lighthouse* | Custom relay multiaddr |
| `-port` | `0` | Listen port (0 = random; relays should use 4001) |
| `-prompt` | (none) | One-shot prompt for `-mode test` |

### Modes

| Mode | Purpose |
|---|---|
| `worker` | Run an agent container; advertise capabilities; accept task streams. |
| `boss` | Interactive pterm TUI for browsing the mesh and dispatching tasks. |
| `api` | Headless HTTP gateway exposing `/api/*` and OpenAI-compatible `/v1/*`. |
| `relay` | Persistent lighthouse + circuit-relay; same role as the dedicated `agentfm-relay` binary. |
| `test` | Local Podman-only sandbox dry-run; bypasses libp2p entirely. |
| `genkey` | Generate a 256-bit `swarm.key` for private-mesh PSK. |

## `agentfm-relay`

A dedicated relay binary for permanent lighthouse deploys (e.g. a $5/mo VPS). Identity persists in `relay_identity.key` so the multiaddr stays stable across restarts.

| Flag | Default | Description |
|---|:---:|---|
| `-port` | `4001` | TCP listen port |
| `-swarmkey` | (none) | Optional PSK for private-mesh mode |
| `-identity` | `relay_identity.key` | Path to persistent Ed25519 identity file |
| `-prom-listen` | `127.0.0.1:9091` | Prometheus `/metrics` bind. `-` disables |
| `-log-format` | `auto` | Same as `agentfm` |
| `-log-level` | `info` | Same as `agentfm` |

## Environment variables

| Variable | Purpose |
|---|---|
| `AGENTFM_API_KEYS` | Comma-separated bearer tokens for the HTTP gateway. See [Authentication](auth.md). |
| `AGENTFM_API_KEY` | Single bearer token for the Python SDK. |
| `AGENTFM_ALLOW_UNAUTH_PUBLIC` | `1` allows non-loopback `--api-bind` without `AGENTFM_API_KEYS`. |
| `AGENTFM_WEBHOOK_SECRET` | HMAC-SHA256 signing secret for outbound async-task webhooks. |
| `AGENTFM_WEBHOOK_ALLOW_PRIVATE` | `1` allows webhook URLs pointing at private/loopback/link-local addresses (default rejected as SSRF defense). |

## Examples

```bash
# Worker for a public-mesh image-gen agent
agentfm -mode worker -agentdir ./flux-agent -image flux-agent:v1 \
  -agent "FLUX 4k" -model "FLUX.2 [dev]" -maxtasks 4 -maxgpu 95

# Headless API gateway, off-host with auth
AGENTFM_API_KEYS=$(openssl rand -hex 32) agentfm -mode api \
  -api-bind 0.0.0.0 -apiport 8080

# Local test (no libp2p)
agentfm -mode test -agentdir ./my-agent -image my-agent:v1 \
  -agent "My Bot" -model "llama3.2" -prompt "Write a haiku."

# Generate private-mesh PSK
agentfm -mode genkey  # → ./swarm.key
```

## Related

- [Run a Worker](worker.md) — full worker setup walkthrough
- [Authentication](auth.md) — `AGENTFM_API_KEYS` + `--api-bind` interactions
- [Private Swarms](private-swarms.md) — `-swarmkey` + `-bootstrap` usage
- [Installation](install.md) — getting the binaries
