# Installation

## Prerequisites

- **Podman** for worker nodes (boss / api / relay don't need it)
- **Go 1.25+** only if building from source

## One-line install (recommended)

```bash
curl -fsSL https://api.agentfm.net/install.sh | bash
```

Drops the single `agentfm` binary into `/usr/local/bin` (it covers every role via `-mode`).

## From source

```bash
git clone https://github.com/Agent-FM/agentfm-core.git
cd agentfm-core/agentfm-go
make build && make install
```

`make build-all` cross-compiles for macOS / Linux / Windows on amd64 + arm64.

## Python SDK

```bash
pip install agentfm-sdk
```

Full SDK docs: [agentfm-python/README.md](../agentfm-python/README.md).

## Verify install

```bash
agentfm -mode genkey   # → writes ./swarm.key
agentfm --help         # → all modes + flags
```

## Related

- [CLI Reference](cli.md) — full flag list once installed
- [Run a Worker](worker.md) — first-time worker setup
- [Development](development.md) — building from source for contributors
