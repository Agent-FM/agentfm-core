from __future__ import annotations

import pytest
import respx

from agentfm import AsyncAgentFMClient, ModelNotFoundError

pytestmark = pytest.mark.integration


async def test_async_workers_list(gateway_url: str, mock_workers: respx.MockRouter):
    async with AsyncAgentFMClient(gateway_url=gateway_url) as client:
        workers = await client.workers.list()
    assert len(workers) == 3


async def test_async_workers_filter(gateway_url: str, mock_workers: respx.MockRouter):
    async with AsyncAgentFMClient(gateway_url=gateway_url) as client:
        avail = await client.workers.list(available_only=True)
    assert len(avail) == 2


async def test_async_openai_chat_completion(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    mock_gateway.post("/v1/chat/completions").respond(
        json={
            "id": "x",
            "object": "chat.completion",
            "created": 1,
            "model": "12D3KooWAlpha",
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": "async hi"},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        },
    )
    async with AsyncAgentFMClient(gateway_url=gateway_url) as client:
        resp = await client.openai.chat.completions.create(
            model="12D3KooWAlphaPid",
            messages=[{"role": "user", "content": "hi"}],
        )
    assert resp.choices[0].message.content == "async hi"  # type: ignore[union-attr]


async def test_async_chat_streaming(gateway_url: str, mock_gateway: respx.MockRouter):
    sse = (
        b'data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m",'
        b'"choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}\n\n'
        b'data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m",'
        b'"choices":[{"index":0,"delta":{"content":"alpha"},"finish_reason":null}]}\n\n'
        b"data: [DONE]\n\n"
    )
    mock_gateway.post("/v1/chat/completions").respond(
        status_code=200, content=sse, headers={"Content-Type": "text/event-stream"}
    )
    async with AsyncAgentFMClient(gateway_url=gateway_url) as client:
        stream = await client.openai.chat.completions.create(
            model="12D3KooWAlphaPid",
            messages=[{"role": "user", "content": "hi"}],
            stream=True,
        )
        chunks = []
        async for chunk in stream:  # type: ignore[union-attr]
            chunks.append(chunk)
    assert len(chunks) >= 1
    contents = [c.choices[0].delta.content for c in chunks if c.choices]
    assert "alpha" in (contents or [""])


async def test_async_openai_chat_streaming_wraps_mid_stream_httpx_error(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Same regression as the sync version — a mid-SSE httpx.RemoteProtocolError
    must surface as WorkerStreamError, not a raw httpx exception."""
    import httpx

    from agentfm.exceptions import WorkerStreamError

    mock_gateway.post("/v1/chat/completions").mock(
        side_effect=httpx.RemoteProtocolError("connection closed mid-stream")
    )
    async with AsyncAgentFMClient(gateway_url=gateway_url, retries=0) as client:
        with pytest.raises(WorkerStreamError):
            stream = await client.openai.chat.completions.create(
                model="12D3KooWAlphaPid",
                messages=[{"role": "user", "content": "hi"}],
                stream=True,
            )
            async for _ in stream:  # type: ignore[union-attr]
                pass


async def test_async_envelope_error(gateway_url: str, mock_gateway: respx.MockRouter):
    mock_gateway.post("/v1/chat/completions").respond(
        status_code=404,
        json={
            "error": {
                "message": "nope",
                "type": "invalid_request_error",
                "code": "model_not_found",
            }
        },
    )
    async with AsyncAgentFMClient(gateway_url=gateway_url) as client:
        with pytest.raises(ModelNotFoundError):
            await client.openai.chat.completions.create(
                model="12D3KooWXxx",
                messages=[{"role": "user", "content": "hi"}],
            )
