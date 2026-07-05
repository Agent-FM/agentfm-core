"""Peers namespace for the AgentFM Python SDK (v1.3.1, Phase 9).

Surface:

    client.peers.list(include_offline=False)      -> list[KnownPeer]
    client.peers.get(peer_id)                     -> PeerSummary
    client.peers.log(peer_id, limit=50, offset=0) -> list[PeerEntry]
    client.peers.comment_body(peer_id, cid)       -> str
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import List, Optional

import httpx

from ._transport import raise_for_response


@dataclass(frozen=True)
class KnownPeer:
    peer_id: str
    online: bool
    honesty_score: float = 0.0
    is_equivocator: bool = False
    last_seen: Optional[str] = None
    name: Optional[str] = None


@dataclass(frozen=True)
class RaterSummary:
    verified_raters_count: int
    unverified_raters_count: int


@dataclass(frozen=True)
class PeerSummary:
    peer_id: str
    honesty_score: float
    is_equivocator: bool
    dispatch_allowed: bool
    dispatch_refuse_reason: Optional[str] = None
    agent_name: Optional[str] = None
    online: bool = False
    last_seen: Optional[str] = None
    entries_count: int = 0
    last_entry_at: Optional[str] = None
    advertised_image_ref: Optional[str] = None
    advertised_image_digest: Optional[str] = None
    advertised_capability: Optional[str] = None
    rater_summary: Optional[RaterSummary] = None


@dataclass(frozen=True)
class PeerEntry:
    received_at: str
    kind: str
    rater_peer_id: str
    rater_status: str
    rater_honesty_score: float = 0.0
    dimension: Optional[str] = None
    score: Optional[float] = None
    context: Optional[str] = None
    language: Optional[str] = None
    text_cid: Optional[str] = None


def _from_known_peer(d: dict) -> KnownPeer:
    return KnownPeer(
        peer_id=d["peer_id"],
        online=d.get("online", True),
        honesty_score=float(d.get("honesty_score", 0.0)),
        is_equivocator=bool(d.get("is_equivocator", False)),
        last_seen=d.get("last_seen"),
        name=d.get("name"),
    )


def _from_peer_entry(d: dict) -> PeerEntry:
    return PeerEntry(
        received_at=d["received_at"],
        kind=d["kind"],
        rater_peer_id=d["rater_peer_id"],
        rater_status=d.get("rater_status", "unverified"),
        rater_honesty_score=float(d.get("rater_honesty_score", 0.0)),
        dimension=d.get("dimension"),
        score=float(d["score"]) if d.get("score") is not None else None,
        context=d.get("context"),
        language=d.get("language"),
        text_cid=d.get("text_cid"),
    )


def _from_peer_summary(d: dict) -> PeerSummary:
    rs_raw = d.get("rater_summary")
    rs = (
        RaterSummary(
            verified_raters_count=rs_raw.get("verified_raters_count", 0),
            unverified_raters_count=rs_raw.get("unverified_raters_count", 0),
        )
        if rs_raw
        else None
    )
    return PeerSummary(
        peer_id=d["peer_id"],
        honesty_score=float(d.get("honesty_score", 0.0)),
        is_equivocator=bool(d.get("is_equivocator", False)),
        dispatch_allowed=bool(d.get("dispatch_allowed", True)),
        dispatch_refuse_reason=d.get("dispatch_refuse_reason") or None,
        agent_name=d.get("agent_name"),
        online=bool(d.get("online", False)),
        last_seen=d.get("last_seen"),
        entries_count=int(d.get("entries_count", 0)),
        last_entry_at=d.get("last_entry_at"),
        advertised_image_ref=d.get("advertised_image_ref"),
        advertised_image_digest=d.get("advertised_image_digest"),
        advertised_capability=d.get("advertised_capability"),
        rater_summary=rs,
    )


class PeersNamespace:
    """``client.peers.*`` — peer discovery and trust inspection (v1.3.1)."""

    def __init__(self, http: httpx.Client) -> None:
        self._http = http

    def list(self, *, include_offline: bool = False) -> List[KnownPeer]:
        """GET /api/workers — optionally include offline peers."""
        params = {"include_offline": "true"} if include_offline else None
        r = self._http.get("/api/workers", params=params)
        raise_for_response(r)
        return [_from_known_peer(a) for a in r.json().get("agents", [])]

    def get(self, peer_id: str) -> PeerSummary:
        """GET /v1/peers/{peer_id} — single-peer summary."""
        r = self._http.get(f"/v1/peers/{peer_id}")
        raise_for_response(r)
        return _from_peer_summary(r.json())

    def log(self, peer_id: str, *, limit: int = 50, offset: int = 0) -> List[PeerEntry]:
        """GET /v1/peers/{peer_id}/log — paginated ledger entries."""
        r = self._http.get(
            f"/v1/peers/{peer_id}/log",
            params={"limit": limit, "offset": offset},
        )
        raise_for_response(r)
        return [_from_peer_entry(e) for e in r.json().get("entries", [])]

    def comment_body(self, peer_id: str, cid: str) -> str:
        """GET /v1/peers/{peer_id}/comments/{cid} — hydrate comment body."""
        r = self._http.get(f"/v1/peers/{peer_id}/comments/{cid}")
        raise_for_response(r)
        return r.text


class AsyncPeersNamespace:
    """``client.peers.*`` — async peer discovery and trust inspection (v1.3.1)."""

    def __init__(self, http: httpx.AsyncClient) -> None:
        self._http = http

    async def list(self, *, include_offline: bool = False) -> List[KnownPeer]:
        """GET /api/workers — optionally include offline peers."""
        params = {"include_offline": "true"} if include_offline else None
        r = await self._http.get("/api/workers", params=params)
        raise_for_response(r)
        return [_from_known_peer(a) for a in r.json().get("agents", [])]

    async def get(self, peer_id: str) -> PeerSummary:
        """GET /v1/peers/{peer_id} — single-peer summary."""
        r = await self._http.get(f"/v1/peers/{peer_id}")
        raise_for_response(r)
        return _from_peer_summary(r.json())

    async def log(self, peer_id: str, *, limit: int = 50, offset: int = 0) -> List[PeerEntry]:
        """GET /v1/peers/{peer_id}/log — paginated ledger entries."""
        r = await self._http.get(
            f"/v1/peers/{peer_id}/log",
            params={"limit": limit, "offset": offset},
        )
        raise_for_response(r)
        return [_from_peer_entry(e) for e in r.json().get("entries", [])]

    async def comment_body(self, peer_id: str, cid: str) -> str:
        """GET /v1/peers/{peer_id}/comments/{cid} — hydrate comment body."""
        r = await self._http.get(f"/v1/peers/{peer_id}/comments/{cid}")
        raise_for_response(r)
        return r.text


__all__ = [
    "AsyncPeersNamespace",
    "KnownPeer",
    "PeerEntry",
    "PeerSummary",
    "PeersNamespace",
    "RaterSummary",
]
