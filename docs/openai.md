# OpenAI-Compatible API

The AgentFM gateway speaks OpenAI's wire format on `/v1/*`. Existing OpenAI SDKs (LangChain, LlamaIndex, LiteLLM, Continue, the raw `openai` Python or Node clients, Open WebUI) point at an AgentFM mesh by changing only `base_url` and `api_key`.

## Quickstart

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8080/v1",
    # Bearer token forwarded as Authorization header. Required when the
    # gateway has AGENTFM_API_KEYS set; ignored on a loopback solo-dev gateway.
    api_key="your-key-here",
)

resp = client.chat.completions.create(
    model="llama3.2",
    messages=[{"role": "user", "content": "Draft a 500-word leave policy."}],
)
print(resp.choices[0].message.content)
```

## Routes

| Route | Behaviour |
|---|---|
| `GET /v1/models` | One entry per peer currently visible. `id` is the libp2p peer ID; `agentfm_*` extension fields carry per-peer status, hardware, GPU, load. |
| `POST /v1/chat/completions` | Standard OpenAI chat. `stream:true` for SSE deltas terminating with `data: [DONE]`. |
| `POST /v1/completions` | Legacy text-completion endpoint. `prompt` must be a string (array form returns 400). |

## Hybrid `model` routing

The incoming `model` is matched against three identifiers in order, first hit wins:

1. **PeerID exact match** — pin a request to a specific machine: `model: "12D3KooW..."`.
2. **AgentName** (case-insensitive) — target a named agent: `model: "my-research-agent"`.
3. **Model engine** (case-insensitive) — standard OpenAI semantics: `model: "llama3.2"` routes to any worker advertising that engine.

Within a tier with multiple matches, the least-loaded worker wins. All-busy returns `503 mesh_overloaded`; no-match returns `404 model_not_found`. All errors use OpenAI's standard error envelope.

## Streaming

Set `stream: true` on `/v1/chat/completions` or `/v1/completions` for SSE deltas. The stream terminates with `data: [DONE]\n\n`. Internal frame markers (`[AGENTFM: FILES_INCOMING]`, `[AGENTFM: NO_FILES]`) are stripped before SSE delivery.

Streaming is **line-buffered**, not character-by-character. The 8 MiB max-line cap accommodates structured-output agents that emit large JSON state without newlines.

## Caveats

- **Auth.** Bearer-token validation is enforced when `AGENTFM_API_KEYS` is set on the gateway; see [Authentication](auth.md). Default `--api-bind` is loopback, so a fresh install is safe out of the box.
- **Token counts** in `usage` are returned as `0` — AgentFM does not tokenize.
- **Streaming is line-buffered**, not character-by-character.
- **Not yet implemented:** `tools` / `tool_choice`, `logprobs`, image / vision parts, `n>1`, `/v1/embeddings`, `/v1/images/generations`, `/v1/audio/*`.

## Related

- [Authentication](auth.md) — gateway bearer-token setup
- [Python SDK](../agentfm-python/README.md) — typed `client.openai.*` surface
- [Architecture](architecture.md) — wire-protocol overview
