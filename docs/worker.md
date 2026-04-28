# Run a Worker

A worker exposes one Podman-containerized agent to the mesh. The agent itself can call any model (Ollama, vLLM, llama.cpp, OpenAI API, custom) — the mesh doesn't care.

## Boot a worker

```bash
agentfm -mode worker \
  -agentdir "./my-agent" -image "my-agent:v1" \
  -agent "My Bot" -model "llama3.2" -author "you" \
  -maxtasks 10 -maxcpu 60 -maxgpu 70
```

The agent directory must contain a `Dockerfile` or `Containerfile`. On startup the worker builds the image, then advertises capabilities via GossipSub every 2 s.

**Circuit breakers** auto-reject tasks and flip status to BUSY when any of `-maxtasks`, `-maxcpu`, `-maxgpu` is exceeded. A node serving the public mesh can't be DoS'd into hurting its operator.

## Authoring agents — the three streaming rules

Because AgentFM pipes your container's stdout directly over a libp2p stream, *how* you write to stdout is the single most important UX decision in your agent.

```python
# 1. Always flush
print("Analyzing the CSV file...", flush=True)

# 2. Set PYTHONUNBUFFERED=1 in your Dockerfile

# 3. Trap noisy framework chatter into a StringIO so the Boss sees clean output:
import io, sys
from contextlib import redirect_stdout, redirect_stderr

boss_stream = sys.stdout
trap = io.StringIO()
with redirect_stdout(trap), redirect_stderr(trap):
    result = run_heavy_pipeline()
print(str(result), file=boss_stream, flush=True)
```

Anything you write to `/tmp/output` gets zipped and streamed back to the Boss automatically. No SDK, no decorators, no callbacks.

```dockerfile
RUN mkdir -p /tmp/output && chmod 777 /tmp/output
ENV PYTHONUNBUFFERED=1
```

## Container security caveat

Workers launch containers with `--network host` so the agent can reach a local Ollama at `127.0.0.1:11434`, vLLM, or other loopback services. The flip side: **the agent container has full access to the worker host's network namespace**, including loopback (Ollama, internal admin endpoints, cloud metadata at `169.254.169.254`).

Treat agent images as **trusted code**; review their Dockerfiles before running. The worker prints a startup warning to this effect.

If you need a stricter sandbox, run the worker inside a VM or a hardware-isolated container runtime.

## Local sandbox testing

Test offline before broadcasting to the mesh:

```bash
agentfm -mode test -agentdir "./my-agent" -image "my-agent:v1" \
  -agent "My Bot" -model "llama3.2" \
  -prompt "Write a haiku about compilers."
```

`-mode test` runs the same Podman command and prints the same `--network host` warning, but bypasses libp2p entirely. Useful for validating an agent image on a developer laptop before publishing it to the mesh.

## Capacity tuning

Workers reject incoming task streams when any of these limits trip:

| Limit | Default | What it bounds |
|---|:---:|---|
| `-maxtasks` | `1` | Concurrent task streams (semaphore) |
| `-maxcpu` | `80.0` | Aggregate CPU % across all cores |
| `-maxgpu` | `80.0` | GPU VRAM % (if a CUDA GPU is detected) |

Telemetry broadcasts the current values every 2s so the Boss radar shows real load. The matcher (in OpenAI-routed mode) prefers least-loaded peers within a tier.

## Related

- [OpenAI-Compatible API](openai.md) — how clients dispatch tasks to your worker
- [CLI Reference](cli.md) — full flag list
- [Architecture](architecture.md) — task-stream wire protocol
- [Security Model](security.md) — Podman + libp2p threat model
