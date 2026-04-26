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
