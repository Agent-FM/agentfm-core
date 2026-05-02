<div align="center">
  <img src="assets/logo-git.png" alt="AgentFM Logo" width="400" />

  <br />
  <br />

  [![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
  [![libp2p](https://img.shields.io/badge/libp2p-v0.47-6E4AFF?style=for-the-badge)](https://libp2p.io)
  [![Podman](https://img.shields.io/badge/Podman-Sandboxed-892CA0?style=for-the-badge&logo=podman)](https://podman.io)
  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](LICENSE)
  [![Status](https://img.shields.io/badge/Status-v1.0.0-brightgreen?style=for-the-badge)](#)

  <h3>SETI@Home, but for AI. A peer-to-peer compute grid for your containerized agents.</h3>
  <p><i>Zero-config P2P networking. Hardware-aware routing. OpenAI-compatible API. Live artifact streaming.</i></p>
  <p><strong><a href="https://agentfm.net">agentfm.net</a></strong></p>

  <h4>One-Line Install (macOS &amp; Linux)</h4>

  ```bash
  curl -fsSL https://api.agentfm.net/install.sh | bash
  ```
</div>

---

## What is AgentFM

A peer-to-peer compute grid that turns idle hardware into a decentralized AI supercomputer. Package your agent as a Podman container, advertise it on a libp2p mesh, and any client (your Next.js app, a LangChain script, a `curl` one-liner) can dispatch tasks over an end-to-end encrypted tunnel. **No cloud accounts, no API keys, no data egress.**

**Three roles:** a *Worker* runs your agent in a Podman sandbox; a *Boss* orchestrates and dispatches tasks (TUI or HTTP gateway); a *Relay* helps peers discover each other and punch through NAT. All you need to start is a laptop with Podman.

**Two things make it interesting:**

1. **OpenAI-compatible** — point any OpenAI SDK at your local mesh and it just works.
2. **Hardware-aware** — workers broadcast live CPU / GPU / queue state; the matcher picks the least-loaded peer for every request.

---

## Hello World

Boot a worker that runs a local **Llama 3.2** model, then dispatch tasks to it.

```bash
# 1. Prereqs (macOS shown; apt for Ubuntu)
brew install podman && podman machine init && podman machine start
curl -fsSL https://ollama.com/install.sh | sh
ollama run llama3.2

# 2. Clone and start a worker
git clone https://github.com/Agent-FM/agentfm-core.git && cd agentfm-core
agentfm -mode worker \
  -agentdir "./agent-example/sick-leave-generator/agent" \
  -image "agentfm-sick-leave:v1" \
  -model "llama3.2" -agent "HR Specialist" \
  -maxtasks 10 -maxcpu 60 -maxgpu 70

# 3. In another terminal, start the API gateway and hit it
agentfm -mode api -apiport 8080 &
curl http://127.0.0.1:8080/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"Draft a sick-leave email"}]}'
```

That's it. Files the agent drops into `/tmp/output` get zipped and shipped back to `./agentfm_artifacts/<task_id>.zip`.

> **Want the interactive radar?** Skip step 3 and run `agentfm -mode boss` for the live TUI.

---

## Features

- **OpenAI-compatible API** on `/v1/chat/completions`, `/v1/completions`, `/v1/models`. Drop-in for LangChain, LlamaIndex, LiteLLM, Continue, Open WebUI, the official OpenAI SDKs, anything.
- **Hardware-aware routing.** Workers broadcast live CPU / GPU / RAM / queue every 2s. The matcher picks the least-loaded peer per request. No central scheduler.
- **End-to-end encrypted P2P.** libp2p Noise streams between Boss and Worker. The Relay sees discovery metadata only; never prompt content.
- **Bearer-token auth.** `AGENTFM_API_KEYS` enables per-request bearer validation, constant-time comparison, per-IP rate limiting on failed attempts. Refuses to start with public bind + no keys.
- **Public mesh, private swarms, or solo-dev.** Same binary. Toggle PSK mode for fully isolated darknet meshes invisible to the public network.
- **Container sandboxing.** Every task runs in a fresh Podman container. SIGKILL'd the instant the stream dies. Resource budgets stop a noisy task from hurting its operator.
- **Live artifact streaming.** Anything an agent writes to `/tmp/output` is auto-zipped, transferred, and extracted client-side. Zip-slip + zip-bomb defense baked in.
- **Observability built in.** Prometheus metrics on every node (`/metrics`), structured slog JSON logs ready for Loki / ELK / Datadog, `/health` endpoint for load balancers.
- **Async + webhook callbacks.** Fire-and-forget submission with HMAC-signed webhook delivery on completion. SSRF-guarded against private network attacks.
- **Cross-platform.** Single statically-linked binary for Linux, macOS, Windows, FreeBSD across amd64, arm64, armv7, 386, and RISC-V.

---

## Python SDK

```bash
pip install agentfm-sdk
```

Typed sync and async clients with full OpenAI-compatible namespace, scatter-gather batch dispatch, signed webhook callbacks, and strict mypy compliance.

```python
from agentfm import AgentFMClient

with AgentFMClient(gateway_url="http://127.0.0.1:8080") as client:
    # workers.list(model=...) is a discovery FILTER: "show me workers
    # whose advertised engine string equals 'llama3.2'"
    workers = client.workers.list(model="llama3.2", available_only=True)

    # tasks.run dispatches to a specific machine by its cryptographic peer_id
    result = client.tasks.run(worker_id=workers[0].peer_id, prompt="Draft a leave policy.")
    print(result.text)
    print(result.artifacts)   # list[Path] auto-extracted
```

### A note on the word "model"

The SDK uses `model` for two different things:

- **`workers.list(model=...)`** is a **discovery filter** — exact-match against what each worker advertised at startup (`-model llama3.2`).
- **`openai.chat.completions.create(model=...)`** is a **routing identifier** — the gateway accepts three kinds of values here, matched in priority order: a `peer_id` (most specific, cryptographically verifiable), an agent name, or an engine name.

Both forms work for the OpenAI namespace:

```python
# Option 1: OpenAI-native shape — route to ANY worker advertising "llama3.2".
# Familiar if you're coming from the cloud OpenAI SDK.
resp = client.openai.chat.completions.create(
    model="llama3.2",
    messages=[{"role": "user", "content": "hi"}],
)

# Option 2: pin to a SPECIFIC machine by peer_id.
# Recommended for production: peer_id is the only cryptographically
# verifiable identifier, so you know exactly which worker served the request.
resp = client.openai.chat.completions.create(
    model=workers[0].peer_id,
    messages=[{"role": "user", "content": "hi"}],
)
```

Both calls work; pick the one that matches your trust model. In a federated mesh, anyone can advertise themselves as running `llama3.2`, so production code that cares about provenance should pin by `peer_id`.

#### What actually happens for each form

The two forms behave very differently when worker state changes. **Engine-name routing trades guaranteed placement for automatic fallback. PeerID pinning trades fallback for guaranteed placement.**

| Worker state | `model="12D3KooW..."` (pin) | `model="llama3.2"` (engine) |
|---|---|---|
| Worker online with capacity | 200 — served by that exact machine | 200 — served by least-loaded `llama3.2` worker |
| Worker online but at `max_tasks` | **503 `mesh_overloaded`** (no fallback) | 200 — served by another `llama3.2` worker |
| Worker offline / wrong peer_id | **404 `model_not_found`** | 200 — any other `llama3.2` worker |
| All matching workers at capacity | n/a | 503 `mesh_overloaded` |

For engine-name routing, the gateway picks the worker with the lowest `current_tasks/max_tasks` ratio (CPU usage as tiebreaker). The OpenAI response doesn't tell you which peer served the request — if you need that, list-and-pick yourself, or use the AgentFM-native `client.tasks.run()` which returns `result.worker_id`.

**Pick by peer_id** when you need provenance, reproducibility, or a specific machine (custom weights, fine-tunes, hardware benchmarking). **Use engine-name routing** when all matching workers are equivalent and trusted, and you want automatic load balancing and failover.

Async mirror via `AsyncAgentFMClient`. Full SDK docs: [agentfm-python/README.md](agentfm-python/README.md). PyPI: [pypi.org/project/agentfm-sdk](https://pypi.org/project/agentfm-sdk/).

---

## Documentation

| Topic | Doc |
|---|---|
| Get the binaries | [Installation](docs/install.md) |
| Run an agent on the mesh | [Run a Worker](docs/worker.md) |
| Use OpenAI SDKs against your mesh | [OpenAI-Compatible API](docs/openai.md) |
| Lock down off-host gateways | [Authentication](docs/auth.md) |
| Raw HTTP for non-Python clients | [Raw HTTP API](docs/http-api.md) |
| Typed Python client | [Python SDK](agentfm-python/README.md) |
| Prometheus + structured logs | [Observability](docs/observability.md) |
| Stand up a private darknet mesh | [Private Swarms](docs/private-swarms.md) |
| Wire protocols + system topology | [Architecture](docs/architecture.md) |
| Threat model + hardening checklist | [Security Model](docs/security.md) |
| Every flag, every env var | [CLI Reference](docs/cli.md) |
| Build from source, run tests | [Development](docs/development.md) |
| Branching + PR conventions | [Contributing](CONTRIBUTING.md) |

---

<div align="center">

**Built with Go, libp2p, and a belief that compute should belong to everyone.**

</div>
