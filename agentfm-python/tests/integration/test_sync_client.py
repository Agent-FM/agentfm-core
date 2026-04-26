from __future__ import annotations

import warnings

import httpx
import pytest
import respx

from agentfm import (
    AgentFMClient,
    AgentFMRoutingWarning,
    InvalidRequestError,
    MeshOverloadedError,
    ModelNotFoundError,
    WorkerNotFoundError,
)
from agentfm.openai.models import ChatCompletion

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# workers
# ---------------------------------------------------------------------------


def test_workers_list_unwraps_envelope_correctly(
    gateway_url: str, mock_workers: respx.MockRouter
):
    with AgentFMClient(gateway_url=gateway_url) as client:
        workers = client.workers.list()
    assert len(workers) == 3
    peers = {w.peer_id for w in workers}
    assert "12D3KooWAlpha9XzHaaaa" in peers
    assert "12D3KooWBravo7Yqabbbb" in peers


def test_workers_list_filters_by_model(gateway_url: str, mock_workers: respx.MockRouter):
    with AgentFMClient(gateway_url=gateway_url) as client:
        only_flux = client.workers.list(model="flux")
    assert len(only_flux) == 1
    assert only_flux[0].name == "image-gen"


def test_workers_list_filter_is_case_insensitive(
    gateway_url: str, mock_workers: respx.MockRouter
):
    with AgentFMClient(gateway_url=gateway_url) as client:
        upper = client.workers.list(model="LLAMA3.2")
    assert {w.name for w in upper} == {"research-agent"}


def test_workers_list_available_only(gateway_url: str, mock_workers: respx.MockRouter):
    with AgentFMClient(gateway_url=gateway_url) as client:
        avail = client.workers.list(available_only=True)
    assert all(w.is_available for w in avail)
    assert len(avail) == 2  # alice + carol; bob is BUSY


def test_workers_get_returns_specific_peer(
    gateway_url: str, mock_workers: respx.MockRouter
):
    with AgentFMClient(gateway_url=gateway_url) as client:
        w = client.workers.get("12D3KooWBravo7Yqabbbb")
    assert w.author == "bob"


def test_workers_get_raises_on_unknown_peer(
    gateway_url: str, mock_workers: respx.MockRouter
):
    with AgentFMClient(gateway_url=gateway_url) as client, pytest.raises(WorkerNotFoundError):
        client.workers.get("nope")


# ---------------------------------------------------------------------------
# error envelope mapping
# ---------------------------------------------------------------------------


def test_chat_model_not_found_maps_to_typed_exception(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    mock_gateway.post("/v1/chat/completions").respond(
        status_code=404,
        json={
            "error": {
                "message": "model 'nope' not available on this mesh",
                "type": "invalid_request_error",
                "code": "model_not_found",
            }
        },
    )
    with AgentFMClient(gateway_url=gateway_url) as client, pytest.raises(ModelNotFoundError) as ei:
        client.openai.chat.completions.create(
            model="12D3KooWxxxxxxxx",
            messages=[{"role": "user", "content": "hi"}],
        )
    assert ei.value.code == "model_not_found"
    assert ei.value.status == 404


def test_mesh_overloaded_maps_to_typed_exception(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    mock_gateway.post("/v1/chat/completions").respond(
        status_code=503,
        json={
            "error": {
                "message": "all matching workers are at capacity",
                "type": "server_error",
                "code": "mesh_overloaded",
            }
        },
    )
    with AgentFMClient(gateway_url=gateway_url) as client, pytest.raises(MeshOverloadedError):
        client.openai.chat.completions.create(
            model="12D3KooWxxxxxxxx",
            messages=[{"role": "user", "content": "hi"}],
        )


def test_invalid_request_maps_to_typed_exception(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    mock_gateway.post("/v1/chat/completions").respond(
        status_code=400,
        json={
            "error": {
                "message": "field 'model' is required",
                "type": "invalid_request_error",
                "code": "model_required",
            }
        },
    )
    with AgentFMClient(gateway_url=gateway_url) as client, pytest.raises(InvalidRequestError):
        client.openai.chat.completions.create(
            model="12D3KooWxxxxxxxx",
            messages=[{"role": "user", "content": "hi"}],
        )


# ---------------------------------------------------------------------------
# OpenAI namespace
# ---------------------------------------------------------------------------


def test_openai_models_list(gateway_url: str, mock_models: respx.MockRouter):
    with AgentFMClient(gateway_url=gateway_url) as client:
        listing = client.openai.models.list()
    assert listing.object == "list"
    assert len(listing.data) == 1
    assert listing.data[0].agentfm_engine == "llama3.2"


def test_openai_chat_completion_non_streaming(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    mock_gateway.post("/v1/chat/completions").respond(
        json={
            "id": "chatcmpl-x",
            "object": "chat.completion",
            "created": 1,
            "model": "12D3KooWAlpha9XzHaaaa",
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": "hi back"},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        },
    )
    with AgentFMClient(gateway_url=gateway_url) as client:
        resp = client.openai.chat.completions.create(
            model="12D3KooWAlpha9XzHaaaa",
            messages=[{"role": "user", "content": "hi"}],
        )
    assert isinstance(resp, ChatCompletion)
    assert resp.choices[0].message.content == "hi back"


def test_openai_chat_completion_streaming(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    sse = (
        b'data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m",'
        b'"choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}\n\n'
        b'data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m",'
        b'"choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}\n\n'
        b'data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m",'
        b'"choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}\n\n'
        b"data: [DONE]\n\n"
    )
    mock_gateway.post("/v1/chat/completions").respond(
        status_code=200,
        content=sse,
        headers={"Content-Type": "text/event-stream"},
    )
    with AgentFMClient(gateway_url=gateway_url) as client:
        chunks = list(
            client.openai.chat.completions.create(
                model="12D3KooWAlpha9XzHaaaa",
                messages=[{"role": "user", "content": "hi"}],
                stream=True,
            )
        )
    assert len(chunks) >= 2
    accumulated = "".join(
        c.choices[0].delta.content or "" for c in chunks if c.choices  # type: ignore[union-attr]
    )
    assert accumulated == "hello world"


def test_openai_chat_streaming_wraps_mid_stream_httpx_error(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Pre-fix, a mid-SSE httpx.RemoteProtocolError would propagate raw out of
    chat.completions.create(stream=True). Fix wraps every httpx.HTTPError in
    WorkerStreamError so user code can catch only AgentFMError subclasses.
    """
    from agentfm.exceptions import WorkerStreamError

    mock_gateway.post("/v1/chat/completions").mock(
        side_effect=httpx.RemoteProtocolError("connection closed mid-stream")
    )
    with AgentFMClient(gateway_url=gateway_url, retries=0) as client, pytest.raises(WorkerStreamError):
        for _ in client.openai.chat.completions.create(
            model="12D3KooWAlpha9XzHaaaa",
            messages=[{"role": "user", "content": "hi"}],
            stream=True,
        ):
            pass


def test_openai_text_streaming_wraps_mid_stream_httpx_error(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Same regression for /v1/completions stream=True."""
    from agentfm.exceptions import WorkerStreamError

    mock_gateway.post("/v1/completions").mock(
        side_effect=httpx.RemoteProtocolError("connection closed mid-stream")
    )
    with AgentFMClient(gateway_url=gateway_url, retries=0) as client, pytest.raises(WorkerStreamError):
        for _ in client.openai.completions.create(
            model="12D3KooWAlpha9XzHaaaa",
            prompt="hi",
            stream=True,
        ):
            pass


def test_openai_routing_warning_fires_for_non_peer_id(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    mock_gateway.post("/v1/chat/completions").respond(
        json={
            "id": "x",
            "object": "chat.completion",
            "created": 0,
            "model": "llama3.2",
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": "ok"},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        },
    )
    with AgentFMClient(gateway_url=gateway_url) as client, warnings.catch_warnings(record=True) as w:
        # Per-namespace dedup state — fresh client guarantees a fresh warner.
        client.openai._warner.reset()
        warnings.simplefilter("always")
        client.openai.chat.completions.create(
            model="test-llama3.2",
            messages=[{"role": "user", "content": "hi"}],
        )
    assert any(issubclass(item.category, AgentFMRoutingWarning) for item in w)


def test_openai_routing_warning_silent_for_peer_id(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    mock_gateway.post("/v1/chat/completions").respond(
        json={
            "id": "x",
            "object": "chat.completion",
            "created": 0,
            "model": "12D3KooWPid",
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": "ok"},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        },
    )
    with AgentFMClient(gateway_url=gateway_url) as client, warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        client.openai.chat.completions.create(
            model="12D3KooWPidExample",
            messages=[{"role": "user", "content": "hi"}],
        )
    assert not any(issubclass(item.category, AgentFMRoutingWarning) for item in w)


# ---------------------------------------------------------------------------
# /api/execute streaming with sentinel filtering
# ---------------------------------------------------------------------------


def test_tasks_stream_strips_sentinels(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    body = b"hello\n[AGENTFM: NO_FILES]\nworld\n"
    mock_gateway.post("/api/execute").respond(
        status_code=200, content=body, headers={"Content-Type": "text/plain"}
    )
    with AgentFMClient(gateway_url=gateway_url) as client:
        chunks = list(
            client.tasks.stream(worker_id="12D3KooWAlpha9XzHaaaa", prompt="hi")
        )
    text = "".join(c.text for c in chunks if c.kind == "text")
    assert text == "hello\nworld\n"


# ---------------------------------------------------------------------------
# Connection failure mapping
# ---------------------------------------------------------------------------


def test_workers_list_wraps_connection_error(gateway_url: str, mock_gateway: respx.MockRouter):
    from agentfm.exceptions import GatewayConnectionError

    mock_gateway.get("/api/workers").mock(side_effect=httpx.ConnectError("nope"))
    with AgentFMClient(gateway_url=gateway_url, retries=0) as client, pytest.raises(
        GatewayConnectionError
    ):
        client.workers.list()
