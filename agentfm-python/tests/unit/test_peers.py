"""Unit tests for the SDK peers namespace (v1.3.1, Phase 9)."""

from __future__ import annotations

import respx
from httpx import Response

from agentfm import AgentFMClient, AsyncAgentFMClient
from agentfm.peers import (
    KnownPeer,
    PeerEntry,
    PeerSummary,
    PeersNamespace,
    RaterSummary,
)

GATEWAY = "http://test-gateway"


# ---------------------------------------------------------------------------
# Sync: list
# ---------------------------------------------------------------------------


def test_peers_list_returns_online_and_offline():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/api/workers").mock(
            return_value=Response(
                200,
                json={
                    "agents": [
                        {"peer_id": "12D3KooWA", "online": True, "honesty_score": 0.2},
                        {"peer_id": "12D3KooWB", "online": False, "honesty_score": -0.1},
                    ]
                },
            )
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            peers = c.peers.list(include_offline=True)

    assert len(peers) == 2
    assert isinstance(peers[0], KnownPeer)
    assert peers[0].peer_id == "12D3KooWA"
    assert peers[0].online is True
    assert peers[1].peer_id == "12D3KooWB"
    assert peers[1].online is False


def test_peers_list_include_offline_sends_query_param():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        route = router.get("/api/workers").mock(
            return_value=Response(200, json={"agents": []})
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            c.peers.list(include_offline=True)

    sent_url = str(route.calls.last.request.url)
    assert "include_offline=true" in sent_url


def test_peers_list_default_no_offline_param():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        route = router.get("/api/workers").mock(
            return_value=Response(
                200,
                json={"agents": [{"peer_id": "12D3KooWA", "online": True}]},
            )
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            peers = c.peers.list()

    assert len(peers) == 1
    sent_url = str(route.calls.last.request.url)
    assert "include_offline" not in sent_url


def test_peers_list_missing_fields_use_defaults():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/api/workers").mock(
            return_value=Response(
                200,
                # minimal — no honesty_score, no is_equivocator
                json={"agents": [{"peer_id": "12D3KooWX", "online": True}]},
            )
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            peers = c.peers.list()

    assert peers[0].honesty_score == 0.0
    assert peers[0].is_equivocator is False
    assert peers[0].last_seen is None
    assert peers[0].name is None


# ---------------------------------------------------------------------------
# Sync: get
# ---------------------------------------------------------------------------


def test_peers_get_returns_summary():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWA").mock(
            return_value=Response(
                200,
                json={
                    "peer_id": "12D3KooWA",
                    "honesty_score": 0.2,
                    "is_equivocator": False,
                    "dispatch_allowed": True,
                    "entries_count": 5,
                    "rater_summary": {
                        "verified_raters_count": 2,
                        "unverified_raters_count": 1,
                    },
                },
            )
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            p = c.peers.get("12D3KooWA")

    assert isinstance(p, PeerSummary)
    assert p.peer_id == "12D3KooWA"
    assert p.honesty_score == 0.2
    assert p.dispatch_allowed is True
    assert p.entries_count == 5
    assert isinstance(p.rater_summary, RaterSummary)
    assert p.rater_summary.verified_raters_count == 2
    assert p.rater_summary.unverified_raters_count == 1


def test_peers_get_without_rater_summary():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWB").mock(
            return_value=Response(
                200,
                json={
                    "peer_id": "12D3KooWB",
                    "honesty_score": -0.5,
                    "is_equivocator": True,
                    "dispatch_allowed": False,
                    "dispatch_refuse_reason": "equivocator",
                },
            )
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            p = c.peers.get("12D3KooWB")

    assert p.is_equivocator is True
    assert p.dispatch_allowed is False
    assert p.dispatch_refuse_reason == "equivocator"
    assert p.rater_summary is None


# ---------------------------------------------------------------------------
# Sync: log
# ---------------------------------------------------------------------------


def test_peers_log_returns_entries_with_rater_status():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWA/log").mock(
            return_value=Response(
                200,
                json={
                    "subject": "12D3KooWA",
                    "entries": [
                        {
                            "received_at": "2026-05-18T00:00:00Z",
                            "kind": "Rating",
                            "rater_peer_id": "12D3KooWB",
                            "score": -0.3,
                            "context": "test",
                            "dimension": "honesty",
                            "rater_status": "verified",
                            "rater_honesty_score": 0.8,
                        }
                    ],
                },
            )
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            entries = c.peers.log("12D3KooWA")

    assert len(entries) == 1
    e = entries[0]
    assert isinstance(e, PeerEntry)
    assert e.kind == "Rating"
    assert e.rater_status == "verified"
    assert e.rater_honesty_score == 0.8
    assert e.score == -0.3
    assert e.dimension == "honesty"


def test_peers_log_default_rater_status_unverified():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWA/log").mock(
            return_value=Response(
                200,
                json={
                    "entries": [
                        {
                            "received_at": "2026-05-18T00:00:00Z",
                            "kind": "Comment",
                            "rater_peer_id": "12D3KooWC",
                            # rater_status absent → defaults to "unverified"
                        }
                    ]
                },
            )
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            entries = c.peers.log("12D3KooWA")

    assert entries[0].rater_status == "unverified"


def test_peers_log_sends_limit_and_offset():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        route = router.get("/v1/peers/12D3KooWA/log").mock(
            return_value=Response(200, json={"entries": []})
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            c.peers.log("12D3KooWA", limit=10, offset=20)

    sent_url = str(route.calls.last.request.url)
    assert "limit=10" in sent_url
    assert "offset=20" in sent_url


# ---------------------------------------------------------------------------
# Sync: comment_body
# ---------------------------------------------------------------------------


def test_peers_comment_body_returns_string():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWA/comments/abc123").mock(
            return_value=Response(200, text="Great agent")
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            body = c.peers.comment_body("12D3KooWA", "abc123")

    assert body == "Great agent"


def test_peers_comment_body_returns_unicode():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWZ/comments/cid99").mock(
            return_value=Response(200, text="Excellent — highly recommended")
        )
        with AgentFMClient(gateway_url=GATEWAY) as c:
            body = c.peers.comment_body("12D3KooWZ", "cid99")

    assert "—" in body


# ---------------------------------------------------------------------------
# Async: mirrors of sync tests
# ---------------------------------------------------------------------------


async def test_async_peers_list_returns_online_and_offline():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/api/workers").mock(
            return_value=Response(
                200,
                json={
                    "agents": [
                        {"peer_id": "12D3KooWA", "online": True, "honesty_score": 0.5},
                        {"peer_id": "12D3KooWB", "online": False, "honesty_score": 0.0},
                    ]
                },
            )
        )
        async with AsyncAgentFMClient(gateway_url=GATEWAY) as c:
            peers = await c.peers.list(include_offline=True)

    assert len(peers) == 2
    assert peers[0].online is True
    assert peers[1].online is False


async def test_async_peers_get_returns_summary():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWA").mock(
            return_value=Response(
                200,
                json={
                    "peer_id": "12D3KooWA",
                    "honesty_score": 0.7,
                    "is_equivocator": False,
                    "dispatch_allowed": True,
                    "rater_summary": {
                        "verified_raters_count": 3,
                        "unverified_raters_count": 0,
                    },
                },
            )
        )
        async with AsyncAgentFMClient(gateway_url=GATEWAY) as c:
            p = await c.peers.get("12D3KooWA")

    assert p.honesty_score == 0.7
    assert p.rater_summary is not None
    assert p.rater_summary.verified_raters_count == 3


async def test_async_peers_log_returns_entries():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWA/log").mock(
            return_value=Response(
                200,
                json={
                    "entries": [
                        {
                            "received_at": "2026-05-18T00:00:00Z",
                            "kind": "Rating",
                            "rater_peer_id": "12D3KooWB",
                            "rater_status": "verified",
                        }
                    ]
                },
            )
        )
        async with AsyncAgentFMClient(gateway_url=GATEWAY) as c:
            entries = await c.peers.log("12D3KooWA")

    assert len(entries) == 1
    assert entries[0].rater_status == "verified"


async def test_async_peers_comment_body_returns_string():
    with respx.mock(base_url=GATEWAY, assert_all_called=True) as router:
        router.get("/v1/peers/12D3KooWA/comments/cid42").mock(
            return_value=Response(200, text="Async comment body")
        )
        async with AsyncAgentFMClient(gateway_url=GATEWAY) as c:
            body = await c.peers.comment_body("12D3KooWA", "cid42")

    assert body == "Async comment body"
