# agentfm

Official Python SDK for [AgentFM](https://agentfm.net), a peer-to-peer compute mesh
for containerized AI agents.

```bash
pip install agentfm
```

## Hello World

```python
from agentfm import AgentFMClient

client = AgentFMClient(gateway_url="http://127.0.0.1:8080")

# 1. Discover: see every candidate with full metadata
for w in client.workers.list(model="llama3.2"):
    print(f"{w.peer_id[:12]}...  {w.author!r}  load={w.current_tasks}/{w.max_tasks}")

# 2. Pick: explicit choice (peer_id is the cryptographically verifiable identifier)
worker = client.workers.list(model="llama3.2")[0]

# 3. Dispatch: by peer_id, always
result = client.tasks.run(worker_id=worker.peer_id, prompt="Draft a leave policy.")
print(result.text)
print(result.artifacts)   # list[Path] of files the worker produced
```

## Streaming

```python
for chunk in client.tasks.stream(worker_id=worker.peer_id, prompt="..."):
    print(chunk.text, end="", flush=True)
```

## OpenAI-compatible

```python
# Same client; first-class /v1/* support
resp = client.openai.chat.completions.create(
    model=worker.peer_id,                    # peer_id pin-routes; any string also works
    messages=[{"role": "user", "content": "hi"}],
)
print(resp.choices[0].message.content)

# Streaming
for chunk in client.openai.chat.completions.create(
    model=worker.peer_id,
    messages=[{"role": "user", "content": "hi"}],
    stream=True,
):
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

## Authentication

```python
from agentfm import AgentFMClient, AuthenticationError

# Three modes:
client = AgentFMClient(api_key="your-key")    # explicit
client = AgentFMClient()                       # falls back to AGENTFM_API_KEY env
client = AgentFMClient(api_key=None)           # explicit no-auth (skips env)

try:
    client.workers.list()
except AuthenticationError as e:
    print(e.code, e.status)  # "invalid_api_key" 401
```

`with_options(api_key="other")` derives a client with a different key; pass
`api_key=None` to clear auth on the derived client. See the gateway-side
[Authentication](https://github.com/Agent-FM/agentfm-core#authentication)
docs for setting up `AGENTFM_API_KEYS` on the boss.

## Async

```python
import asyncio
from agentfm import AsyncAgentFMClient

async def main() -> None:
    async with AsyncAgentFMClient() as client:
        workers = await client.workers.list()
        worker = workers[0]
        async for chunk in client.tasks.stream(worker_id=worker.peer_id, prompt="hi"):
            print(chunk.text, end="", flush=True)

asyncio.run(main())
```

## Fire-and-forget with webhook

```python
from agentfm import WebhookReceiver

task_id = client.tasks.submit_async(
    worker_id=worker.peer_id,
    prompt="Long-running job",
    webhook_url="http://my-host.example.com:8000/cb",
)

# Receive the completion callback (stdlib HTTP server, no FastAPI required):
WebhookReceiver(port=8000, callback=lambda payload: print(payload)).serve_forever()
```

## Spin up an ephemeral gateway

```python
from agentfm import LocalMeshGateway, AgentFMClient

with LocalMeshGateway(port=8080) as gw:
    client = AgentFMClient(gateway_url=gw.url)
    workers = client.workers.list(model="llama3.2", wait_for_workers=1)
    print(workers)
```

## CLI

```bash
agentfm-py models                          # list peers on the mesh
agentfm-py chat --peer 12D3KooW... --prompt "hi"
```

## Why peer_id, not model name

In a federated mesh, anyone can advertise any name. `agent_name` and `model` are
user-supplied strings with no uniqueness or authenticity guarantee. `peer_id` is
the only cryptographically verifiable identifier. The SDK is built around this:
`tasks.run`, `tasks.stream`, and `tasks.submit_async` accept `peer_id` only.
The OpenAI namespace accepts any string for `model` (per OpenAI's spec), but
emits a one-time warning recommending `peer_id` for production use.

The discovery → pick → dispatch flow (above) is the canonical pattern.

## Python and license

* Requires Python 3.10+.
* Apache 2.0 (matches the parent project).
* Tested on macOS and Linux against CPython 3.10, 3.11, 3.12, 3.13.

## Documentation

* Examples: see [`examples/`](./examples/) in the repo.
* Changelog: [`CHANGELOG.md`](./CHANGELOG.md).
* Issues: [github.com/Agent-FM/agentfm-core/issues](https://github.com/Agent-FM/agentfm-core/issues).
