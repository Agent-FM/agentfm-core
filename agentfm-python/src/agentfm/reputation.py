"""Reputation namespace for the AgentFM Python SDK (v1.3, P4-4).

Surface:

    client.reputation.get(peer_id)          -> ReputationScore
    client.reputation.log(peer_id, ...)     -> list[LogEntry]
    client.reputation.proof(peer_id, entry) -> InclusionProof
    client.reputation.comment(
        subject_peer_id,
        text,
        language="en",
        signer=callable,
        rater_peer_id=...,
    ) -> CommentReceipt

The ``signer`` argument is a ``Callable[[bytes], bytes]`` that returns
an Ed25519 signature over a 32-byte SHA-256 digest. Keeping signing
behind a callable means the SDK doesn't impose any specific crypto
library on callers — provide a libp2p key, a hardware key, whatever.
"""

from __future__ import annotations

import base64
import hashlib
from dataclasses import dataclass, field
from typing import Any, Callable, List, Optional

import httpx

# Signer takes the 32-byte SHA-256 digest of the canonical Comment
# bytes and returns the Ed25519 signature. The host caller is
# responsible for matching the libp2p key whose PeerID is the
# `rater_peer_id` in the request body — otherwise the boss returns
# 401 bad_signature.
Signer = Callable[[bytes], bytes]


@dataclass(frozen=True)
class ReputationScore:
    """One peer's reputation snapshot as exposed by the HTTP API."""

    peer_id: str
    scores: dict[str, float]
    rating_count: int
    last_updated: Optional[str]
    is_equivocator: bool
    agent_image_ref: Optional[str] = None
    agent_image_digest: Optional[str] = None
    agent_capability: Optional[str] = None


@dataclass(frozen=True)
class LogEntry:
    """One entry from a peer's log as returned by /v1/peers/{id}/log."""

    idx: int
    hash: str
    prev_hash: str
    kind: str
    score: float = 0.0
    dimension: str = ""
    context: str = ""
    rater: str = ""
    subject: str = ""
    received_at: str = ""


@dataclass(frozen=True)
class LogHead:
    """The signed head returned alongside /v1/peers/{id}/log."""

    tree_size: int
    root_hash: str
    witness_count: int
    signed_at: str


@dataclass(frozen=True)
class LogResponse:
    """Full envelope returned by client.reputation.log."""

    entries: List[LogEntry] = field(default_factory=list)
    head: Optional[LogHead] = None


@dataclass(frozen=True)
class InclusionProof:
    """A proof that an entry sits in a peer's signed log."""

    entry_hash: str
    position: int
    audit_path: List[str]
    head: LogHead


@dataclass(frozen=True)
class CommentReceipt:
    """201 Created response from POST /v1/peers/{id}/comments."""

    cid: str
    ledger_hash: str


class _ReputationNamespace:
    """Bound namespace returned by ``client.reputation``.

    Constructed by the parent client with an ``httpx.Client`` already
    configured for the gateway URL + bearer auth. This class is
    *only* responsible for translating Python args into HTTP calls
    and shaping the responses; no business logic lives here.
    """

    def __init__(self, transport: httpx.Client):
        self._http = transport

    # ---------- reads --------------------------------------------------

    def get(self, peer_id: str) -> ReputationScore:
        """GET /v1/peers/{peer_id}/reputation."""
        resp = self._http.get(f"/v1/peers/{peer_id}/reputation")
        resp.raise_for_status()
        body = resp.json()
        return ReputationScore(
            peer_id=body.get("peer_id", peer_id),
            scores=body.get("scores") or {},
            rating_count=int(body.get("rating_count", 0)),
            last_updated=body.get("last_updated"),
            is_equivocator=bool(body.get("is_equivocator", False)),
            agent_image_ref=body.get("agent_image_ref"),
            agent_image_digest=body.get("agent_image_digest"),
            agent_capability=body.get("agent_capability"),
        )

    def log(
        self,
        peer_id: str,
        *,
        from_idx: int = 1,
        limit: int = 100,
        kind: Optional[str] = None,
    ) -> LogResponse:
        """GET /v1/peers/{peer_id}/log."""
        params: dict[str, Any] = {"from": from_idx, "limit": limit}
        if kind:
            params["kind"] = kind
        resp = self._http.get(f"/v1/peers/{peer_id}/log", params=params)
        resp.raise_for_status()
        body = resp.json()
        entries = [LogEntry(**e) for e in body.get("entries") or []]
        head_dict = body.get("head")
        head = LogHead(**head_dict) if head_dict else None
        return LogResponse(entries=entries, head=head)

    def proof(self, peer_id: str, entry_hash: str) -> InclusionProof:
        """GET /v1/peers/{peer_id}/proof?entry={hex}."""
        resp = self._http.get(
            f"/v1/peers/{peer_id}/proof", params={"entry": entry_hash}
        )
        resp.raise_for_status()
        body = resp.json()
        return InclusionProof(
            entry_hash=body["entry_hash"],
            position=int(body["position"]),
            audit_path=list(body.get("audit_path") or []),
            head=LogHead(**body["head"]),
        )

    # ---------- writes -------------------------------------------------

    def comment(
        self,
        subject_peer_id: str,
        text: str,
        *,
        language: str = "en",
        signer: Signer,
        rater_peer_id: str,
        attached_rating_hash: Optional[str] = None,
    ) -> CommentReceipt:
        """POST /v1/peers/{subject_peer_id}/comments.

        Signs the canonical Comment bytes via ``signer`` (a callable
        producing an Ed25519 signature over a SHA-256 digest), then
        ships the signed envelope to the gateway. v1.3 requires
        ``rater_peer_id`` to match the gateway's own libp2p identity
        — non-self submissions get 403.
        """
        digest = _canonical_digest(
            rater_peer_id=rater_peer_id,
            subject_peer_id=subject_peer_id,
            text=text,
            language=language,
            attached_rating_hash=attached_rating_hash,
        )
        signature = signer(digest)
        body: dict[str, Any] = {
            "rater_peer_id": rater_peer_id,
            "text": text,
            "language": language,
            "signature": base64.b64encode(signature).decode("ascii"),
        }
        if attached_rating_hash:
            body["attached_rating_hash"] = attached_rating_hash
        resp = self._http.post(
            f"/v1/peers/{subject_peer_id}/comments", json=body
        )
        resp.raise_for_status()
        out = resp.json()
        return CommentReceipt(cid=out["cid"], ledger_hash=out["ledger_hash"])


def _canonical_digest(
    *,
    rater_peer_id: str,
    subject_peer_id: str,
    text: str,
    language: str,
    attached_rating_hash: Optional[str],
) -> bytes:
    """Return SHA-256 of the canonical comment bytes the boss expects.

    The canonical form is the protobuf encoding of pb.Comment with
    Signature stripped. Python can't easily reproduce protobuf's
    deterministic marshalling without compiling the .proto, so v1.3
    uses a STABLE concatenation defined by the SDK itself. The boss
    accepts whichever bytes the SDK signs IF the signature verifies
    against the rater's libp2p key — but the canonical bytes MUST
    match the server's expectation, OR the server-side
    CanonicalComment must accept this format.

    For v1.3 self-submission, the boss re-signs the entry with the
    server's own libp2p key BEFORE persisting (the client signature
    is a sanity check that the caller controls the rater key). The
    digest below is therefore intentionally simple — its job is to
    bind the rater + subject + body so a stolen request can't be
    replayed against a different subject.
    """
    h = hashlib.sha256()
    h.update(b"agentfm/comment/v1\n")
    h.update(rater_peer_id.encode("utf-8") + b"\n")
    h.update(subject_peer_id.encode("utf-8") + b"\n")
    h.update(language.encode("utf-8") + b"\n")
    h.update(text.encode("utf-8") + b"\n")
    if attached_rating_hash:
        h.update(attached_rating_hash.encode("utf-8"))
    return h.digest()


__all__ = [
    "CommentReceipt",
    "InclusionProof",
    "LogEntry",
    "LogHead",
    "LogResponse",
    "ReputationScore",
    "Signer",
    "_ReputationNamespace",
]
