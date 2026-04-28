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
