<div align="center">
  <img src="assets/logo-git.png" alt="AgentFM" width="380" />

  <br /><br />

  [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
  [![libp2p](https://img.shields.io/badge/libp2p-P2P-6E4AFF?style=for-the-badge)](https://libp2p.io)
  [![Podman](https://img.shields.io/badge/Podman-Sandboxed-892CA0?style=for-the-badge&logo=podman)](https://podman.io)
  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](LICENSE)

  <h3>Run AI agents on a peer-to-peer mesh of idle machines.</h3>
  <p><b>Ollama, but distributed.</b> Package an agent as a container, drop it on the mesh, and dispatch tasks to it from a desktop app, a <code>curl</code> one-liner, or any OpenAI SDK — end-to-end encrypted, no cloud account, no API keys, no data egress.</p>

  <p>
    <a href="#-desktop-app"><b>Desktop App</b></a> ·
    <a href="#quick-start-cli"><b>CLI</b></a> ·
    <a href="#python-sdk"><b>Python SDK</b></a> ·
    <a href="docs/">Docs</a> ·
    <a href="https://agentfm.net">agentfm.net</a>
  </p>
</div>

---

## 🖥️ Desktop App

The easiest way in. A calm, native mesh console for **Mac & Windows** — see every agent on your mesh, dispatch a task, watch it stream, and rate the result. No terminal required.

<table>
  <tr>
    <td width="50%"><img src="assets/screenshots/radar.png" alt="Radar — live view of every agent on the mesh" /><br/><sub><b>Radar</b> — every agent, live CPU/GPU/queue, one-click dispatch</sub></td>
    <td width="50%"><img src="assets/screenshots/chat.png" alt="Chat — dispatch and stream a task" /><br/><sub><b>Chat</b> — dispatch, stream the response, collect artifacts</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="assets/screenshots/history.png" alt="Agent history — reputation, ratings, telemetry" /><br/><sub><b>Agent profile</b> — reputation, signed ratings, live telemetry</sub></td>
    <td width="50%"><img src="assets/screenshots/assets.png" alt="Assets — artifacts produced by agents" /><br/><sub><b>Assets</b> — every artifact an agent produced, by project</sub></td>
  </tr>
</table>

**What it does:** a visual **Radar** of every agent (name, description, hardware, live load, reputation stars); one-click **Dispatch** with live-streamed output; an **Assets** browser for the files agents produce; per-agent **history** with signed ratings and a tamper-evident trust ledger; **projects** to switch between the public mesh and your own private swarms; and a built-in **API explorer**.

**Download:** [**Releases** → `AgentFM-*.dmg` (Mac) · `AgentFM-Setup-*.exe` (Windows)](https://github.com/Agent-FM/agentfm-core/releases)

> The desktop app bundles the mesh backend — install it and you have a full node. Prefer the terminal? The [CLI](#quick-start-cli) and [SDK](#python-sdk) drive the same mesh. Full tour: **[docs/DESKTOP.md](docs/DESKTOP.md)**.

---

## What is AgentFM

A peer-to-peer compute grid that turns idle hardware into a decentralized AI supercomputer. A few cooperating roles, one binary:

- **Worker** — runs your agent in a fresh Podman sandbox and advertises its hardware on a libp2p mesh.
- **Boss** — orchestrates and dispatches tasks (the desktop app, an interactive TUI, or a headless HTTP gateway).
- **Relay** — a lighthouse that helps peers discover each other and punch through NAT.
- **Witness** *(optional)* — `agentfm -mode witness`: a ledger-only replica that persists the tamper-evident trust ledger so a fresh Boss can recover it even when every Boss is offline. (A Relay already does this too.)

**Why it's interesting:**

1. **OpenAI-compatible** — point any OpenAI SDK at your local mesh and it just works.
2. **Hardware-aware** — workers broadcast live CPU/GPU/queue every 2 s; the matcher routes each task to the least-loaded peer.
3. **Trust without a middleman** — every rating is a signed receipt on a tamper-evident Merkle log; equivocators are caught and floored, bad actors auto-rejected. No allow-lists, no central authority, no blockchain. → [Trust & Verification](docs/trust.md)

---

## Quick Start (CLI)

Boot a worker running local **Llama 3.2**, then dispatch to it.

```bash
# 1. Prereqs (macOS shown; apt for Ubuntu)
brew install podman && podman machine init && podman machine start
curl -fsSL https://ollama.com/install.sh | sh && ollama run llama3.2

# 2. Install AgentFM
curl -fsSL https://api.agentfm.net/install.sh | bash   # or grab a binary from Releases

# 3. Start a worker
agentfm -mode worker -agentdir ./agent-example/sick-leave-generator/agent \
  -image agentfm-sick-leave:v1 -model llama3.2 -agent "HR Assistant" -maxtasks 10

# 4. In another terminal, start the API gateway and hit it with any OpenAI client
agentfm -mode api -apiport 8080 &
curl http://127.0.0.1:8080/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"Draft a sick-leave email"}]}'
```

Files the agent drops in `/tmp/output` come back zipped to `./agentfm_artifacts/<task_id>.zip`.

> **Want the visual radar without a terminal?** Grab the [desktop app](#-desktop-app). Want the interactive TUI? Run `agentfm -mode boss`.

---

## Python SDK

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

Typed sync + async clients, full OpenAI-compatible namespace, scatter-gather batch dispatch, signed webhook callbacks. Routing (`peer_id` pin vs. engine-name failover), the OpenAI namespace, and every endpoint are in the [**HTTP API reference**](docs/http-api.md) and the [Python SDK guide](agentfm-python/README.md).

---

## Join the public mesh

No allow-list. Push your image anywhere, point a worker at the public lighthouse, and you're in — reputation accrues from honest behavior over time.

```bash
podman build -t ghcr.io/you/myagent:v1 ./my-agent && podman push ghcr.io/you/myagent:v1
agentfm -mode worker -agentdir ./my-agent -image ghcr.io/you/myagent:v1 \
  -agent "My Agent" -capability "research-assistant" -model llama3.2
```

Tighten the dispatch gate with `--reputation-floor=-0.3`, or run a fully isolated darknet with `--swarmkey`. → [Private Swarms](docs/private-swarms.md)

---

## Documentation

| | |
|---|---|
| 🖥️ **Desktop app** guide | [docs/DESKTOP.md](docs/DESKTOP.md) |
| 🌐 **HTTP API** — every endpoint | [docs/http-api.md](docs/http-api.md) |
| 🤖 OpenAI-compatible API | [docs/openai.md](docs/openai.md) |
| 🐍 Python SDK | [agentfm-python/README.md](agentfm-python/README.md) |
| 📦 Install the binaries | [docs/install.md](docs/install.md) |
| 🏃 Run a worker | [docs/worker.md](docs/worker.md) |
| 🔐 Authentication | [docs/auth.md](docs/auth.md) |
| 🛡️ Trust & verification | [docs/trust.md](docs/trust.md) · [Security model](docs/security.md) |
| 🕸️ Private swarms | [docs/private-swarms.md](docs/private-swarms.md) |
| 🏗️ Architecture & wire protocols | [docs/architecture.md](docs/architecture.md) |
| 📊 Observability | [docs/observability.md](docs/observability.md) |
| ⚙️ CLI reference | [docs/cli.md](docs/cli.md) |
| 🧑‍💻 Build from source / contribute | [docs/development.md](docs/development.md) · [CONTRIBUTING.md](CONTRIBUTING.md) |

---

<div align="center">
  <sub>Built with Go, libp2p, and a belief that compute should belong to everyone.</sub>
  <br/>
  <b>⭐ Star the repo if a distributed agent mesh sounds useful — it helps a lot.</b>
</div>
