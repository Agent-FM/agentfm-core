"""tasks.scatter regression coverage — sync + async, happy path + retry path.

Audit findings addressed:
- Sync scatter results race / non-deterministic order (Major #4)
- Async scatter retry deadlock at max_concurrency=1 (Blocker #1)
- Async scatter never fails over to a different peer on retry (Blocker #1)
"""

from __future__ import annotations

import asyncio
import json
from collections import Counter

import httpx
import pytest
import respx

from agentfm import AgentFMClient, AsyncAgentFMClient

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# sync
# ---------------------------------------------------------------------------


def test_sync_scatter_returns_results_in_submission_order(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        # Echo the prompt back so we can verify ordering by content.
        return httpx.Response(
            200, content=f"echo:{body['prompt']}\n".encode(), headers={"Content-Type": "text/plain"}
        )

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    prompts = [f"p{i}" for i in range(8)]
    with AgentFMClient(gateway_url=gateway_url) as client:
        results = client.tasks.scatter(prompts, peer_ids=["12D3KooWA", "12D3KooWB"], max_concurrency=4)

    assert len(results) == 8
    for i, r in enumerate(results):
        assert r.prompt == prompts[i], f"results[{i}].prompt = {r.prompt!r}, want {prompts[i]!r}"
        assert r.status == "success"
        assert r.text == f"echo:{prompts[i]}\n"


def test_sync_scatter_failed_prompt_appears_at_correct_index(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """A persistent 5xx → status=failed at the right position."""

    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        if body["prompt"] == "doomed":
            return httpx.Response(
                503,
                json={"error": {"message": "all busy", "type": "server_error", "code": "mesh_overloaded"}},
            )
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    prompts = ["a", "doomed", "c"]
    with AgentFMClient(gateway_url=gateway_url, retries=0) as client:
        results = client.tasks.scatter(prompts, peer_ids=["12D3KooWX"], max_retries=0, max_concurrency=2)

    assert [r.prompt for r in results] == prompts
    assert results[0].status == "success"
    assert results[1].status == "failed"
    assert results[2].status == "success"


# ---------------------------------------------------------------------------
# async
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_async_scatter_returns_results_in_submission_order(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        return httpx.Response(
            200, content=f"echo:{body['prompt']}\n".encode(), headers={"Content-Type": "text/plain"}
        )

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    prompts = [f"p{i}" for i in range(6)]
    async with AsyncAgentFMClient(gateway_url=gateway_url) as client:
        results = await client.tasks.scatter(
            prompts, peer_ids=["12D3KooWA", "12D3KooWB"], max_concurrency=3
        )

    assert len(results) == 6
    for i, r in enumerate(results):
        assert r.prompt == prompts[i]
        assert r.status == "success"
        assert r.text == f"echo:{prompts[i]}\n"


@pytest.mark.asyncio
async def test_async_scatter_does_not_deadlock_at_concurrency_one(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Pre-fix, retry-with-recursion held the semaphore across the recursive
    call, deadlocking at max_concurrency=1. With the iterative fix it just
    completes.
    """
    attempts: dict[str, int] = Counter()

    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        attempts[body["prompt"]] += 1
        # Fail the first time we see a prompt; succeed on retry.
        if attempts[body["prompt"]] == 1:
            return httpx.Response(
                503,
                json={"error": {"message": "busy", "type": "server_error", "code": "mesh_overloaded"}},
            )
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    async with AsyncAgentFMClient(gateway_url=gateway_url, retries=0) as client:
        # Bound by an outer timeout so a regression locks up CI in seconds, not minutes.
        results = await asyncio.wait_for(
            client.tasks.scatter(["x", "y"], peer_ids=["12D3KooWA"], max_concurrency=1, max_retries=2),
            timeout=10.0,
        )

    assert [r.status for r in results] == ["success", "success"]
    assert attempts["x"] >= 2 and attempts["y"] >= 2, "each prompt must have been retried"


@pytest.mark.asyncio
async def test_async_scatter_retry_fails_over_to_different_peer(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Pre-fix, retries reused the same peer (peers[idx % len(peers)]). The
    fix advances by attempt count so the second attempt goes to a different
    peer when one is available.
    """
    seen_peers: list[str] = []

    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        seen_peers.append(body["worker_id"])
        # Always fail so we exhaust retries and observe every peer that was tried.
        return httpx.Response(
            503,
            json={"error": {"message": "busy", "type": "server_error", "code": "mesh_overloaded"}},
        )

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    async with AsyncAgentFMClient(gateway_url=gateway_url, retries=0) as client:
        await client.tasks.scatter(
            ["only-prompt"], peer_ids=["12D3KooWA", "12D3KooWB"], max_concurrency=1, max_retries=1
        )

    assert seen_peers == ["12D3KooWA", "12D3KooWB"], (
        f"second attempt must go to a different peer; saw {seen_peers}"
    )


def test_sync_scatter_survives_mid_stream_disconnect(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Pre-fix, an httpx.ReadError mid-stream would propagate raw out of
    tasks.stream(), the scatter's `except AgentFMError` would miss it, and
    the entire scatter would raise with results half-populated. Contract
    says ScatterResult-with-status="failed", never an exception.
    """

    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        if body["prompt"] == "doomed":
            # Simulate mid-stream connection drop. respx surfaces this as
            # httpx.RemoteProtocolError to the SDK, which is httpx.HTTPError
            # but not httpx.ConnectError.
            raise httpx.RemoteProtocolError("connection closed mid-stream")
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    prompts = ["a", "doomed", "c"]
    with AgentFMClient(gateway_url=gateway_url, retries=0) as client:
        results = client.tasks.scatter(prompts, peer_ids=["12D3KooWX"], max_retries=0, max_concurrency=2)

    assert [r.prompt for r in results] == prompts, "must return all 3 results, not raise"
    assert [r.status for r in results] == ["success", "failed", "success"]
    assert "worker stream failed" in (results[1].error or "")


@pytest.mark.asyncio
async def test_async_scatter_survives_mid_stream_disconnect(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Same regression as the sync test, but exercises asyncio.gather. Pre-fix,
    one ReadError would cancel every sibling coroutine and re-raise to the caller.
    """

    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        if body["prompt"] == "doomed":
            raise httpx.RemoteProtocolError("connection closed mid-stream")
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    prompts = ["a", "doomed", "c"]
    async with AsyncAgentFMClient(gateway_url=gateway_url, retries=0) as client:
        results = await client.tasks.scatter(
            prompts, peer_ids=["12D3KooWX"], max_concurrency=3, max_retries=0
        )

    assert [r.prompt for r in results] == prompts
    assert [r.status for r in results] == ["success", "failed", "success"]


@pytest.mark.asyncio
async def test_async_scatter_failed_prompt_at_correct_index(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        if body["prompt"] == "doomed":
            return httpx.Response(
                503,
                json={"error": {"message": "busy", "type": "server_error", "code": "mesh_overloaded"}},
            )
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    prompts = ["alpha", "doomed", "gamma"]
    async with AsyncAgentFMClient(gateway_url=gateway_url, retries=0) as client:
        results = await client.tasks.scatter(
            prompts, peer_ids=["12D3KooWX"], max_concurrency=3, max_retries=0
        )

    assert [r.prompt for r in results] == prompts
    assert [r.status for r in results] == ["success", "failed", "success"]
