<div align="center">
  <img src="assets/logo-git.png" alt="AgentFM" width="380" />

  <br /><br />

  [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
  [![libp2p](https://img.shields.io/badge/libp2p-P2P-6E4AFF?style=for-the-badge)](https://libp2p.io)
  [![Podman](https://img.shields.io/badge/Podman-Sandboxed-892CA0?style=for-the-badge&logo=podman)](https://podman.io)
  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](LICENSE)

  <h3>Run your AI agents on machines you already own, as one private network.</h3>
  <p>Package an agent as a container, run it on any Mac or Linux box with spare capacity, and send it work from a desktop app, a <code>curl</code> one-liner, or any OpenAI SDK. Traffic is end-to-end encrypted, there is no cloud account, and on a private swarm your prompts and files only travel between machines in your swarm.</p>

  <p>
    <a href="#-desktop-app"><b>Desktop App</b></a> ·
    <a href="#-what-is-agentfm"><b>What is it</b></a> ·
    <a href="#-who-its-for"><b>Who it's for</b></a> ·
    <a href="#-how-it-compares"><b>Compare</b></a> ·
    <a href="#-quick-start"><b>Quick Start</b></a> ·
    <a href="#-python-sdk"><b>Python SDK</b></a> ·
    <a href="#-documentation">Docs</a> ·
    <a href="https://agentfm.net">agentfm.net</a>
  </p>
</div>

---

## 🖥️ Desktop App

The easiest way in. A calm, native mesh console for **Mac & Linux** (Windows in progress) — see every agent on your mesh, dispatch a task, watch it stream, rate the result. No terminal required. **It bundles the full mesh backend, so installing it gives you a complete node.**

<table>
  <tr>
    <td width="50%"><img src="assets/screenshots/radar.png" alt="Mesh Radar — live view of every agent on the mesh" /><br/><sub><b>Mesh Radar</b> — every agent, live CPU/GPU/RAM/queue, honesty stars, one-click dispatch</sub></td>
    <td width="50%"><img src="assets/screenshots/dispatch.png" alt="Dispatch — live-streamed task output with a trust badge" /><br/><sub><b>Dispatch</b> — live-streamed output with a trust badge (<code>+0.67 · Allowed</code>)</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="assets/screenshots/chat.png" alt="Chat — pin an agent and stream a conversation" /><br/><sub><b>Chat</b> — pin an agent, stream responses, rate each reply</sub></td>
    <td width="50%"><img src="assets/screenshots/history.png" alt="Agent profile — reputation, signed ratings, telemetry" /><br/><sub><b>Agent profile</b> — reputation, signed ratings, Merkle-backed history</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="assets/screenshots/assets.png" alt="Artifacts — files produced by agents, by project" /><br/><sub><b>Artifacts</b> — every file an agent produced, organized by project</sub></td>
    <td width="50%"><img src="assets/screenshots/status.png" alt="Status — node health and mesh connectivity" /><br/><sub><b>Status</b> — node health, relay connectivity, mesh reachability</sub></td>
  </tr>
</table>

**Download:** [**Releases** → `AgentFM-*.dmg` (macOS, arm64 + x64) · `AgentFM-*.AppImage` (Linux)](https://github.com/Agent-FM/agentfm-core/releases)

> Prefer the terminal? The [CLI](#-quick-start) and [SDK](#-python-sdk) drive the exact same mesh. Full tour: **[docs/DESKTOP.md](docs/DESKTOP.md)**.

---

## 🌐 What is AgentFM

AgentFM connects the computers you already have (office Macs, a gaming PC with a GPU, a spare Linux server) into one peer-to-peer mesh. Each machine runs an agent in a sandbox and advertises its live hardware; you send a task to the mesh and it goes to a machine that can take it. **Three cooperating roles, one binary** — pick the role with `-mode`.

```mermaid
flowchart LR
    subgraph Boss["Boss / Desktop"]
        B["Dispatch + orchestrate<br/>(TUI · HTTP/OpenAI · Desktop)"]
    end
    subgraph Relay["Relay (lighthouse)"]
        R["Circuit Relay v2 + DHT<br/>archive ledger + comment bodies"]
    end
    subgraph Worker["Worker"]
        W["Podman sandbox<br/>runs your agent"]
    end

    W -. "telemetry (CPU/GPU/RAM/queue)" .-> B
    B -- "task stream" --> W
    W -- "artifact zip" --> B
    B -- "signed rating + comment" --> R
    W <-. "discovery / NAT relay" .-> R
    B <-. "discovery / ledger catch-up" .-> R
```

- **Worker** — runs your agent in a fresh Podman sandbox (`podman run --rm`) and broadcasts live hardware over a libp2p mesh.
- **Boss** — orchestrates and dispatches tasks: the desktop app, an interactive TUI, or a headless HTTP gateway.
- **Relay** — a permanent lighthouse that helps peers discover each other, punches through NAT, and persists the trust ledger + comment bodies so a fresh Boss can recover full history from the relay alone.

**Why it's interesting:**

1. **OpenAI-compatible** — point any OpenAI SDK at your local mesh and it just works.
2. **Hardware-aware** — workers broadcast CPU/GPU/RAM/queue every ~2 s; the matcher routes each task to the least-loaded peer.
3. **Trust without a middleman** — every rating and comment is Ed25519-signed, with the pubkey derived *from the rater's peer id*, so identity and signature are cryptographically bound. Ratings land on a per-peer append-only Merkle log (RFC 6962 inclusion proofs); reputation is EigenTrust-lite; equivocators are caught by witnesses and floored mesh-wide. No allow-lists, no central authority, no blockchain. → [Trust & Verification](docs/trust.md)

---

## 👥 Who it's for

- **Small teams with idle hardware** who want AI agents running on their own machines: no per-token bill, and no prompts sent to an AI provider.
- **Developers already using the OpenAI SDK** who want to point existing code at their own hardware by changing the base URL.
- **Agent builders** who want to publish an agent on the public mesh and let its reputation grow from signed, verifiable ratings.

### What people use it for

| Use case | How it works with AgentFM |
|---|---|
| **A team assistant on one shared machine** | A Mac running Ollama hosts an HR or docs agent. Colleagues send requests from the desktop app and get files back. See [`sick-leave-generator`](agent-example/sick-leave-generator). |
| **Batch work across every idle machine** | `client.tasks.scatter` spreads hundreds of prompts over all available workers with failover and retry. See [`02_batch_scatter_gather.py`](agentfm-python/examples/02_batch_scatter_gather.py). |
| **Sharing the one GPU in the office** | One machine runs an image-generation agent; everyone else dispatches to it. See [`image-generator`](agent-example/image-generator). |
| **Existing apps, new hardware** | Anything that speaks the OpenAI API can call the gateway at `/v1/chat/completions`. See [OpenAI-compatible API](docs/openai.md). |

---

## ⚖️ How it compares

| | Cloud AI API | A model server on one machine (e.g. Ollama) | **AgentFM** |
|---|---|---|---|
| Where it runs | The provider's servers | One machine | Any machines you add to the mesh |
| Where your prompts go | To the provider | Stay on that machine | Private swarm: only machines in your swarm. Public mesh: the worker that runs the task |
| Unit of work | A model call | A model call | A whole agent in a container: its own code and tools, with files returned |
| OpenAI SDK compatible | Yes | Yes | Yes |
| Shares hardware across people | — | No | Yes, routed by live CPU/GPU/RAM load |
| Cost | Per token | Your hardware | Your hardware |

AgentFM does not replace a model server: a worker usually runs one (Ollama, vLLM) next to its agent.

**What AgentFM is not:**
- **Not a way to split one model across machines.** Each task runs on a single worker; the mesh spreads *many* tasks, not one big one.
- **Not a paid compute marketplace.** Nobody pays anybody; reputation is the only currency.
- **Not private on the public mesh.** A task's prompt is visible to the worker that runs it. For confidential work, use a [private swarm](docs/private-swarms.md).

---

## 🚀 Quick Start

### Desktop (recommended)

1. Download the installer from [**Releases**](https://github.com/Agent-FM/agentfm-core/releases) — `.dmg` (macOS) or `.AppImage` (Linux).
2. Launch it. The bundled node connects to the public mesh automatically.
3. Open **Mesh Radar**, pick an agent, and hit **Dispatch**. Watch it stream. Rate it. Done.

> Installers are **unsigned in v1**, so macOS blocks the first launch. Allow it once via **System Settings → Privacy & Security → Open Anyway**, or clear the flag with `xattr -d com.apple.quarantine ~/Downloads/AgentFM-*.dmg` before mounting.

### CLI

Boot a worker running local **Llama 3.2**, then dispatch to it over the OpenAI-compatible gateway.

```bash
# 1. Prereqs (macOS shown; use apt on Ubuntu)
brew install podman && podman machine init && podman machine start
curl -fsSL https://ollama.com/install.sh | sh && ollama run llama3.2

# 2. Install AgentFM (or grab a binary from Releases)
curl -fsSL https://api.agentfm.net/install.sh | bash

# 3. Start a worker
agentfm -mode worker -agentdir ./agent-example/sick-leave-generator/agent \
  -image agentfm-sick-leave:v1 -model llama3.2 -agent "HR Assistant" -maxtasks 10

# 4. In another terminal, start the API gateway and hit it with any OpenAI client
agentfm -mode api -apiport 8080 &
curl http://127.0.0.1:8080/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"Draft a sick-leave email"}]}'
```

Files the agent drops in `/tmp/output` come back zipped to `./agentfm_artifacts/<task_id>.zip`.

> **Want the interactive TUI instead of curl?** Run `agentfm -mode boss`.

---

## 🔀 How a dispatch flows

Every dispatch passes the **trust gate** before a stream is opened. Equivocators are blocked; peers below `--reputation-floor` are refused with `403`; missing fields fail fast with `400`.

```mermaid
sequenceDiagram
    participant C as Client (SDK / curl / Desktop)
    participant B as Boss (api)
    participant W as Worker
    participant P as Podman sandbox

    C->>B: POST /api/execute {worker_id, prompt}
    B->>B: validate fields (400 if missing)
    B->>B: trust gate — reputation + equivocation (403 if refused)
    B->>W: open task stream (prompt)
    W->>P: podman run --rm (mount /tmp/output)
    P-->>W: stdout streamed live
    W-->>B: stdout + artifact zip
    B-->>C: streamed output + trust badge
    C->>B: POST /v1/peers/{id}/comments/self (rating -1..+1)
    B->>B: sign + append to Merkle log, gossip to mesh
```

---

## 🐍 Python SDK

```bash
pip install agentfm-sdk
```

```python
from agentfm import AgentFMClient

with AgentFMClient(gateway_url="http://127.0.0.1:8080") as client:
    workers = client.workers.list(model="llama3.2", available_only=True)
    result = client.tasks.run(worker_id=workers[0].peer_id, prompt="Draft a leave policy.")
    print(result.text, result.artifacts)   # artifacts: list[Path], auto-extracted
```

Typed sync + async clients, the full OpenAI-compatible namespace, `scatter`/`scatter_by_model` batch dispatch (never raises — failures come back as `ScatterResult(status="failed", ...)`), and signed webhook callbacks. Full guide: [**Python SDK**](agentfm-python/README.md) · every endpoint: [**HTTP API reference**](docs/http-api.md).

---

## 📡 Join the public mesh

No allow-list. Push your image anywhere, point a worker at the public lighthouse, and you're in — reputation accrues from honest behavior over time.

```bash
podman build -t ghcr.io/you/myagent:v1 ./my-agent && podman push ghcr.io/you/myagent:v1
agentfm -mode worker -agentdir ./my-agent -image ghcr.io/you/myagent:v1 \
  -agent "My Agent" -capability "research-assistant" -model llama3.2
```

The public lighthouse is baked in:
`/ip4/78.47.21.107/tcp/4001/p2p/12D3KooWQHw8mVQkx17kLTNiRTbYckU2cAGcAwFFLzVJhhmBs5zL`

Tighten the dispatch gate with `--reputation-floor=-0.3`, or run a fully isolated darknet with `--swarmkey`. → [Private Swarms](docs/private-swarms.md)

---

## 🧩 Modes — one binary, `-mode` picks the role

Every role ships in the same `agentfm` binary, each with its own flags (`agentfm --help`). Add `-swarmkey` + `-bootstrap` to make any of them join a private swarm.

| Mode | Role | Example |
|---|---|---|
| `worker` | Run your agent in a Podman sandbox | `agentfm -mode worker -agentdir ./a -image a:v1 -agent "My Agent" -model llama3.2 -maxtasks 4` |
| `api` | Headless HTTP + OpenAI gateway (**bundled by the desktop app**) | `agentfm -mode api -apiport 8080 -reputation-floor -0.3` |
| `boss` | Interactive TUI dispatcher | `agentfm -mode boss` |
| `relay` | Lighthouse — Circuit Relay v2 + DHT + archive ledger | `agentfm -mode relay -port 4001 -swarmkey ./swarm.key` |
| `witness` | Ledger-only replica that catches equivocation | `agentfm -mode witness` |
| `genkey` | Generate a private-swarm key | `agentfm -mode genkey` |

Full per-flag reference: [docs/cli.md](docs/cli.md).

---

## 📚 Documentation

| | |
|---|---|
| 🖥️ **Desktop app** guide | [docs/DESKTOP.md](docs/DESKTOP.md) |
| 📦 Install the binaries | [docs/install.md](docs/install.md) |
| 🏃 Run a worker | [docs/worker.md](docs/worker.md) |
| 🌐 **HTTP API** — every endpoint | [docs/http-api.md](docs/http-api.md) |
| 🤖 OpenAI-compatible API | [docs/openai.md](docs/openai.md) |
| 🐍 Python SDK | [agentfm-python/README.md](agentfm-python/README.md) |
| 🔐 Authentication | [docs/auth.md](docs/auth.md) |
| 🛡️ Trust & verification | [docs/trust.md](docs/trust.md) |
| 🔒 Security model | [docs/security.md](docs/security.md) |
| 🕸️ Private swarms | [docs/private-swarms.md](docs/private-swarms.md) |
| 🏗️ Architecture & wire protocols | [docs/architecture.md](docs/architecture.md) |
| 📊 Observability | [docs/observability.md](docs/observability.md) |
| ⚙️ CLI reference | [docs/cli.md](docs/cli.md) |
| 🧑‍💻 Build from source / contribute | [docs/development.md](docs/development.md) · [CONTRIBUTING.md](CONTRIBUTING.md) |

---

## 💬 Community

- **Questions, ideas, show-and-tell:** [GitHub Discussions](https://github.com/Agent-FM/agentfm-core/discussions)
- **Bugs and feature requests:** [Issues](https://github.com/Agent-FM/agentfm-core/issues)
- **Security reports:** please report privately as described in [SECURITY.md](SECURITY.md), not in a public issue.
- **Contributing:** start with [CONTRIBUTING.md](CONTRIBUTING.md).

---

<div align="center">
  <sub>Built with Go, libp2p, and a belief that compute should belong to everyone.</sub>
  <br/><br/>
  <b>⭐ Star the repo if a distributed agent mesh sounds useful — it helps a lot.</b>
</div>
