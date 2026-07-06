"""Verify the retry policy: 5xx/429 responses are retried with backoff."""

from __future__ import annotations

import httpx
import pytest
import respx

from agentfm import AgentFMClient
from agentfm.exceptions import GatewayProtocolError, MeshOverloadedError

pytestmark = pytest.mark.integration


def test_retries_on_503_then_succeeds(gateway_url: str, mock_gateway: respx.MockRouter):
    """Gateway returns 503 once, then 200 — the SDK should succeed after retry."""
    route = mock_gateway.get("/api/workers")
    route.side_effect = [
        httpx.Response(503, json={"error": {"message": "transient", "type": "x", "code": "y"}}),
        httpx.Response(200, json={"success": True, "agents": []}),
    ]
    with AgentFMClient(gateway_url=gateway_url, retries=2) as client:
        workers = client.workers.list()
    assert workers == []
    assert route.call_count == 2


def test_retries_on_429_then_succeeds(gateway_url: str, mock_gateway: respx.MockRouter):
    route = mock_gateway.get("/api/workers")
    route.side_effect = [
        httpx.Response(429, json={"error": {"message": "rate limit", "type": "x", "code": "y"}}),
        httpx.Response(200, json={"success": True, "agents": []}),
    ]
    with AgentFMClient(gateway_url=gateway_url, retries=2) as client:
        workers = client.workers.list()
    assert workers == []
    assert route.call_count == 2


def test_does_not_retry_on_4xx_other_than_408_429(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """A 503 from a different code path: real client error like 400 should NOT retry."""
    route = mock_gateway.post("/v1/chat/completions")
    route.respond(
        status_code=400,
        json={"error": {"message": "bad", "type": "invalid_request_error", "code": "model_required"}},
    )
    with AgentFMClient(gateway_url=gateway_url, retries=3) as client:
        from agentfm.exceptions import InvalidRequestError

        with pytest.raises(InvalidRequestError):
            client.openai.chat.completions.create(
                model="12D3KooWX", messages=[{"role": "user", "content": "hi"}]
            )
    assert route.call_count == 1, "400 should not have been retried"


def test_gives_up_after_exhausting_retries_on_persistent_503(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """If 503 persists, we surface the typed error envelope."""
    mock_gateway.post("/v1/chat/completions").respond(
        status_code=503,
        json={"error": {"message": "all busy", "type": "server_error", "code": "mesh_overloaded"}},
    )
    with (
        AgentFMClient(gateway_url=gateway_url, retries=2) as client,
        pytest.raises(MeshOverloadedError),
    ):
        client.openai.chat.completions.create(
            model="12D3KooWX", messages=[{"role": "user", "content": "hi"}]
        )


def test_request_wraps_remote_protocol_error_as_connection_error(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Non-stream paths going through SyncResource._request now wrap the
    full httpx.HTTPError family (including RemoteProtocolError, WriteError,
    PoolTimeout) as GatewayConnectionError. Pre-fix only ConnectError and
    ReadError were caught."""
    from agentfm.exceptions import GatewayConnectionError

    mock_gateway.get("/api/workers").mock(
        side_effect=httpx.RemoteProtocolError("connection closed by peer")
    )
    with AgentFMClient(gateway_url=gateway_url, retries=0) as client, pytest.raises(GatewayConnectionError):
        client.workers.list()


def test_remote_protocol_error_is_retried(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Pre-fix, retry_sync's default on=(ConnectError, ReadError) didn't
    cover RemoteProtocolError, so a transient mid-stream blip surfaced as
    a single-shot failure even with retries=3. After widening the default,
    the call should be re-issued."""
    route = mock_gateway.get("/api/workers")
    route.side_effect = [
        httpx.RemoteProtocolError("connection closed by peer"),
        httpx.Response(200, json={"success": True, "agents": []}),
    ]
    with AgentFMClient(gateway_url=gateway_url, retries=3) as client:
        workers = client.workers.list()
    assert workers == []
    assert route.call_count == 2, (
        f"expected retry to fire (call_count=2); got {route.call_count}"
    )


def test_pool_timeout_is_retried(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Same regression for httpx.PoolTimeout — also part of the widened
    retryable family."""
    route = mock_gateway.get("/api/workers")
    route.side_effect = [
        httpx.PoolTimeout("connection pool exhausted"),
        httpx.Response(200, json={"success": True, "agents": []}),
    ]
    with AgentFMClient(gateway_url=gateway_url, retries=3) as client:
        workers = client.workers.list()
    assert workers == []
    assert route.call_count == 2


def test_unsupported_protocol_surfaces_as_invalid_request():
    """A misconfigured gateway URL must surface as InvalidRequestError, not
    as GatewayConnectionError — otherwise user config bugs masquerade as
    transient outages and trigger pointless retries.

    This test deliberately does NOT use respx so the real httpx transport
    is exercised — respx would intercept before httpx could raise
    UnsupportedProtocol.
    """
    from agentfm.exceptions import InvalidRequestError

    # ftp:// is unsupported by httpx — passing it as gateway_url triggers
    # UnsupportedProtocol on the first request.
    with AgentFMClient(gateway_url="ftp://nope/", retries=3) as client, pytest.raises(InvalidRequestError) as ei:
        client.workers.list()
    assert ei.value.code == "invalid_gateway_url"


def test_does_not_retry_post_on_500(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """A non-idempotent POST that returns 500 must NOT be retried — the
    gateway may have processed the request before crashing, so re-issuing it
    risks a duplicate side effect (double task submission / duplicate
    comment). See audit M11."""
    from agentfm.exceptions import AgentFMError

    route = mock_gateway.post("/v1/chat/completions")
    route.respond(
        status_code=500,
        json={"error": {"message": "boom", "type": "server_error", "code": "internal"}},
    )
    with (
        AgentFMClient(gateway_url=gateway_url, retries=3) as client,
        pytest.raises(AgentFMError),
    ):
        client.openai.chat.completions.create(
            model="12D3KooWX", messages=[{"role": "user", "content": "hi"}]
        )
    assert route.call_count == 1, "POST 500 must not be retried"


def test_retries_idempotent_get_on_500(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """An idempotent GET is still retried on 500 — safe to re-issue."""
    route = mock_gateway.get("/api/workers")
    route.side_effect = [
        httpx.Response(500, json={"error": {"message": "x", "type": "server_error", "code": "internal"}}),
        httpx.Response(200, json={"success": True, "agents": []}),
    ]
    with AgentFMClient(gateway_url=gateway_url, retries=3) as client:
        workers = client.workers.list()
    assert workers == []
    assert route.call_count == 2, "GET 500 should still be retried"


def test_post_still_retries_on_transport_error(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """A transport-level blip on a POST (request likely never reached the
    server) is still safe to retry — only 5xx *responses* are excluded."""
    route = mock_gateway.post("/v1/chat/completions")
    route.side_effect = [
        httpx.ConnectError("refused"),
        httpx.Response(
            200,
            json={
                "id": "c1",
                "object": "chat.completion",
                "created": 1,
                "model": "12D3KooWX",
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": "hi"},
                        "finish_reason": "stop",
                    }
                ],
            },
        ),
    ]
    with AgentFMClient(gateway_url=gateway_url, retries=3) as client:
        client.openai.chat.completions.create(
            model="12D3KooWX", messages=[{"role": "user", "content": "hi"}]
        )
    assert route.call_count == 2, "POST transport error should still be retried"


def test_protocol_error_for_non_envelope_5xx(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """5xx without our envelope shape -> GatewayProtocolError after retries."""
    mock_gateway.get("/api/workers").respond(status_code=500, text="upstream broke")
    with (
        AgentFMClient(gateway_url=gateway_url, retries=1) as client,
        pytest.raises(GatewayProtocolError),
    ):
        client.workers.list()
