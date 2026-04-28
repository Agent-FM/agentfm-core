# agentfm-sdk

Official Python SDK for [AgentFM](https://agentfm.net), a peer-to-peer compute mesh
for containerized AI agents.

```bash
pip install agentfm-sdk
```

> **Distribution name vs import name.** The package is published as `agentfm-sdk` on PyPI (the bare name `agentfm` is taken by an unrelated project), but you import it as `agentfm` in code. So `pip install agentfm-sdk` then `from agentfm import AgentFMClient`.

Typed sync and async clients, full OpenAI-compatible namespace, scatter-gather
batch dispatch, signed webhook callbacks, and strict mypy compliance.

## Hello World

```python
from agentfm import AgentFMClient

with AgentFMClient(gateway_url="http://127.0.0.1:8080") as client:
    # 1. Discover: see every candidate with full metadata
    for w in client.workers.list(model="llama3.2"):
        print(f"{w.peer_id[:12]}...  {w.author!r}  load={w.current_tasks}/{w.max_tasks}")

    # 2. Pick: explicit choice (peer_id is the cryptographically verifiable identifier)
    worker = client.workers.list(model="llama3.2")[0]

    # 3. Dispatch: by peer_id, always
    result = client.tasks.run(
        worker_id=worker.peer_id,
        prompt="Draft a 200-word leave policy. Save it as policy.md to /tmp/output.",
    )

    print(result.text)
    print(f"\nDuration: {result.duration_seconds:.1f}s")

    # Anything the agent wrote to /tmp/output is auto-zipped, transferred,
    # and extracted client-side. result.artifacts is a list of pathlib.Path.
    for path in result.artifacts:
        print(f"  artifact: {path} ({path.stat().st_size} bytes)")
```

`tasks.run` returns a `TaskResult` with `text`, `artifacts`, `worker_id`, and `duration_seconds`. Anything else streams via `tasks.stream` or `submit_async`.

## Streaming

```python
for chunk in client.tasks.stream(worker_id=worker.peer_id, prompt="..."):
    if chunk.kind == "text":
        print(chunk.text, end="", flush=True)
```

`chunk.kind` is `"text"` for normal stdout or `"marker"` for internal sentinels (artifact-incoming notifications). Most callers only care about `"text"`.

## Batch dispatch (scatter-gather)

For batch workloads, `tasks.scatter` runs many prompts across many peers concurrently, with automatic failover and retry. Results come back in submission order; failures surface as `ScatterResult(status="failed")` rather than raised exceptions, so a single bad prompt never breaks the batch.

```python
prompts = [f"Summarise document #{i}" for i in range(50)]

# Spread across an explicit peer list
results = client.tasks.scatter(
    prompts,
    peer_ids=[w.peer_id for w in client.workers.list(available_only=True)],
    max_concurrency=8,
    max_retries=2,
)

for i, r in enumerate(results):
    if r.status == "success":
        print(f"[{i}] ok  ({len(r.text)} chars)")
    else:
        print(f"[{i}] failed: {r.error}")
```

If you want the SDK to discover the peers for you, use `scatter_by_model`:

```python
results = client.tasks.scatter_by_model(
    prompts,
    model="llama3.2",          # filters workers by their advertised model
    max_workers=4,             # use at most 4 of the matching workers
    max_concurrency=8,         # at most 8 concurrent in-flight tasks
    max_retries=2,
)
```

`pick=` accepts a custom callable for fancier worker selection (e.g. lowest GPU usage):

```python
results = client.tasks.scatter_by_model(
    prompts,
    model="flux.2",
    pick=lambda ws: sorted(ws, key=lambda w: w.gpu_usage_pct)[:3],
)
```

The `worker_id` field on each `ScatterResult` records which peer ultimately served (or failed) that prompt — useful for debugging hot spots.

## OpenAI-compatible

Drop-in replacement for the OpenAI API on `/v1/chat/completions`, `/v1/completions`, and `/v1/models`. Point any existing OpenAI SDK at AgentFM by changing only the base URL and key.

```python
# AgentFM's typed namespace (recommended for new code)
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
    if chunk.choices and chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

If you prefer the official `openai` package, it works unchanged:

```python
from openai import OpenAI

client = OpenAI(base_url="http://127.0.0.1:8080/v1", api_key="your-key")
resp = client.chat.completions.create(
    model="llama3.2",
    messages=[{"role": "user", "content": "hi"}],
)
```

The hybrid `model` matcher tries three identifiers in order, first hit wins: PeerID exact match, AgentName (case-insensitive), Model engine (case-insensitive). Within a tier, least-loaded wins.

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
[Authentication](https://github.com/Agent-FM/agentfm-core/blob/main/docs/auth.md)
docs for setting up `AGENTFM_API_KEYS` on the boss.

> **`client.api_key` is read-after-construction.** The token is baked into
> the underlying `httpx.Client.headers` once at `__init__`. Mutating
> `client.api_key = "new-key"` afterwards does NOT update the request
> header. Use `client.with_options(api_key="new-key")` to derive a client
> with a new key. Matches the OpenAI Python SDK's behaviour.

## Error handling

Every error is a typed subclass of `AgentFMError`. No raw `httpx` exceptions ever surface to user code, so a single `except AgentFMError` clause is sufficient if you don't need to discriminate:

```python
from agentfm import (
    AgentFMError, AuthenticationError, GatewayConnectionError,
    ModelNotFoundError, MeshOverloadedError, WorkerNotFoundError,
    WorkerStreamError, WorkerUnreachableError, InvalidRequestError,
)

try:
    result = client.tasks.run(worker_id=peer_id, prompt="...")
except AuthenticationError:
    print("API key rejected; check AGENTFM_API_KEY")
except WorkerNotFoundError:
    print("That peer isn't in current telemetry")
except WorkerUnreachableError:
    print("Peer is online but the gateway can't dial it")
except MeshOverloadedError:
    print("All matching workers are at capacity; retry later")
except WorkerStreamError as e:
    print(f"Stream failed mid-task: {e.message}")
except AgentFMError as e:
    print(f"Other gateway error: code={e.code} status={e.status}")
```

| Exception | Raised when |
|---|---|
| `AuthenticationError` | 401 from the gateway (missing/wrong bearer) |
| `GatewayConnectionError` | Local gateway unreachable; mid-stream transport failure |
| `GatewayInternalError` | Gateway returned an `internal_error` envelope |
| `GatewayProtocolError` | Gateway response can't be decoded |
| `WorkerNotFoundError` | Peer ID not in current telemetry |
| `WorkerStreamError` | libp2p stream failed mid-task |
| `WorkerUnreachableError` | Boss couldn't dial the worker |
| `ModelNotFoundError` | No worker advertises the requested model |
| `MeshOverloadedError` | All matching workers at capacity |
| `InvalidRequestError` | 4xx from gateway (bad model field, malformed URL, etc.) |
| `ArtifactError` | Zip extraction failed (corrupt / zip-slip / size cap) |

## Async

Mirror surface of the sync client. Every method exists with the same kwargs; just `async`/`await` instead of blocking.

```python
import asyncio
from agentfm import AsyncAgentFMClient

async def main() -> None:
    async with AsyncAgentFMClient() as client:
        workers = await client.workers.list()
        worker = workers[0]
        async for chunk in client.tasks.stream(worker_id=worker.peer_id, prompt="hi"):
            if chunk.kind == "text":
                print(chunk.text, end="", flush=True)

asyncio.run(main())
```

### Async streaming with the OpenAI namespace

The async OpenAI streaming pattern has one gotcha: `create(stream=True)` returns a coroutine you must `await`, then iterate with `async for`. Two steps, not one:

```python
async with AsyncAgentFMClient() as client:
    stream = await client.openai.chat.completions.create(
        model=worker.peer_id,
        messages=[{"role": "user", "content": "hi"}],
        stream=True,
    )
    async for chunk in stream:
        if chunk.choices and chunk.choices[0].delta.content:
            print(chunk.choices[0].delta.content, end="", flush=True)
```

For early-exit cleanup (`break` out of the loop before EOF), wrap the stream in `contextlib.aclosing` so the underlying httpx response is released promptly:

```python
from contextlib import aclosing

async with aclosing(client.tasks.stream(worker_id=peer_id, prompt="...")) as stream:
    async for chunk in stream:
        if want_to_stop:
            break  # response cleaned up via aclosing
```

## Fire-and-forget with webhook

Submit long-running jobs and receive HMAC-signed callbacks when they complete:

```python
from agentfm import WebhookReceiver, WebhookPayload

def on_done(payload: WebhookPayload) -> None:
    print(f"task {payload.task_id} -> {payload.status}")

# Verify HMAC-SHA256 signatures by setting AGENTFM_WEBHOOK_SECRET on the gateway
# and passing the same secret here. Constant-time comparison.
with WebhookReceiver(port=8000, callback=on_done, secret="shared-secret") as rx:
    ack = client.tasks.submit_async(
        worker_id=worker.peer_id,
        prompt="Long-running batch job",
        webhook_url="http://my-host.example.com:8000/cb",
    )
    print(f"queued task {ack.task_id}")
    input("Press Enter to stop receiver...\n")
```

The receiver enforces a 64 KiB body cap, validates Content-Type, and verifies HMAC in constant time. The gateway-side validator refuses webhook URLs pointing at loopback / link-local / RFC1918 addresses (set `AGENTFM_WEBHOOK_ALLOW_PRIVATE=1` to opt back in for trusted private deploys).

## Spin up an ephemeral gateway

For tests and notebooks, `LocalMeshGateway` boots a real `agentfm -mode api` subprocess, waits for it to become ready, and tears it down on context exit:

```python
from agentfm import LocalMeshGateway, AgentFMClient

with LocalMeshGateway(port=8080) as gw:
    client = AgentFMClient(gateway_url=gw.url)
    workers = client.workers.list(model="llama3.2", wait_for_workers=1)
    print(workers)
```

If your gateway requires auth, pass `api_key=` to the gateway constructor; the readiness probe will use it.

## Environment variables

| Variable | Used by | Purpose |
|---|---|---|
| `AGENTFM_API_KEY` | SDK clients | Bearer token for outbound requests when `api_key=` is omitted |
| `AGENTFM_API_KEYS` | Gateway (server-side) | Comma-separated list of accepted bearer tokens |
| `AGENTFM_ALLOW_UNAUTH_PUBLIC` | Gateway (server-side) | `1` allows non-loopback `--api-bind` without `AGENTFM_API_KEYS` |
| `AGENTFM_WEBHOOK_SECRET` | Gateway + `WebhookReceiver` | HMAC-SHA256 signing secret for async webhook callbacks |
| `AGENTFM_WEBHOOK_ALLOW_PRIVATE` | Gateway (server-side) | `1` allows webhook URLs pointing at private/loopback addresses |

## CLI

```bash
agentfm-py models                          # list peers on the mesh
agentfm-py chat --peer 12D3KooW... --prompt "hi"
```

## Why peer_id, not model name

In a federated mesh, anyone can advertise any name. `agent_name` and `model` are user-supplied strings with no uniqueness or authenticity guarantee. `peer_id` is the only cryptographically verifiable identifier. The SDK is built around this: `tasks.run`, `tasks.stream`, and `tasks.submit_async` accept `peer_id` only. The OpenAI namespace accepts any string for `model` (per OpenAI's spec), but emits a one-time warning recommending `peer_id` for production use.

The discovery → pick → dispatch flow (above) is the canonical pattern. See [`examples/06_list_and_pick_by_peer_id.py`](./examples/06_list_and_pick_by_peer_id.py) for three concrete patterns: browse-and-pick, fetch-by-ID, and pin-without-list-call.

## Python and license

* Requires Python 3.10+.
* Apache 2.0 (matches the parent project).
* Tested on macOS and Linux against CPython 3.10, 3.11, 3.12, 3.13.

## Documentation

* Examples: see [`examples/`](./examples/) in the repo. Six worked examples covering quickstart, batch scatter-gather, async webhooks, OpenAI drop-in, ephemeral gateways, and peer_id dispatch patterns.
* Gateway-side docs: [installation](https://github.com/Agent-FM/agentfm-core/blob/main/docs/install.md), [authentication](https://github.com/Agent-FM/agentfm-core/blob/main/docs/auth.md), [observability](https://github.com/Agent-FM/agentfm-core/blob/main/docs/observability.md), [private swarms](https://github.com/Agent-FM/agentfm-core/blob/main/docs/private-swarms.md), [security model](https://github.com/Agent-FM/agentfm-core/blob/main/docs/security.md).
* Changelog: [`CHANGELOG.md`](./CHANGELOG.md).
* Issues: [github.com/Agent-FM/agentfm-core/issues](https://github.com/Agent-FM/agentfm-core/issues).
