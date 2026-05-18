"""Unit tests for the SDK reputation namespace (P4-4)."""

from __future__ import annotations

import base64
import hashlib

import httpx

from agentfm.reputation import (
    CommentReceipt,
    InclusionProof,
    LogEntry,
    LogHead,
    LogResponse,
    ReputationScore,
    _canonical_digest,
    _ReputationNamespace,
)


def _http_with_handler(handler):
    """Return an httpx.Client wired to a MockTransport calling handler."""
    transport = httpx.MockTransport(handler)
    return httpx.Client(transport=transport, base_url="http://gateway.test")


def test_get_decodes_reputation_response():
    def handler(req: httpx.Request) -> httpx.Response:
        assert req.url.path == "/v1/peers/12D3Koo/reputation"
        return httpx.Response(
            200,
            json={
                "peer_id": "12D3Koo",
                "scores": {"honesty": -1.0},
                "rating_count": 7,
                "last_updated": "2026-05-16T08:12:33Z",
                "is_equivocator": True,
                "agent_image_ref": "ghcr.io/example/x:v1",
                "agent_image_digest": "sha256:abc",
                "agent_capability": "test",
            },
        )

    rep = _ReputationNamespace(_http_with_handler(handler)).get("12D3Koo")
    assert isinstance(rep, ReputationScore)
    assert rep.peer_id == "12D3Koo"
    assert rep.scores == {"honesty": -1.0}
    assert rep.is_equivocator is True
    assert rep.agent_image_ref == "ghcr.io/example/x:v1"


def test_log_decodes_entries_and_head():
    def handler(req: httpx.Request) -> httpx.Response:
        assert req.url.path == "/v1/peers/12D3Koo/log"
        assert req.url.params["from"] == "1"
        return httpx.Response(
            200,
            json={
                "entries": [
                    {
                        "idx": 1,
                        "hash": "aa",
                        "prev_hash": "bb",
                        "kind": "rating",
                        "score": 0.5,
                        "dimension": "honesty",
                        "context": "task_42",
                        "rater": "12D3Koo",
                        "subject": "12D3Boo",
                        "received_at": "2026-05-16T08:12:33Z",
                    }
                ],
                "head": {
                    "tree_size": 1,
                    "root_hash": "cc",
                    "witness_count": 3,
                    "signed_at": "2026-05-16T08:12:34Z",
                },
            },
        )

    resp = _ReputationNamespace(_http_with_handler(handler)).log("12D3Koo")
    assert isinstance(resp, LogResponse)
    assert len(resp.entries) == 1
    assert isinstance(resp.entries[0], LogEntry)
    assert resp.entries[0].score == 0.5
    assert isinstance(resp.head, LogHead)
    assert resp.head.witness_count == 3


def test_proof_decodes_response():
    def handler(req: httpx.Request) -> httpx.Response:
        assert req.url.path == "/v1/peers/12D3Koo/proof"
        assert req.url.params["entry"] == "deadbeef"
        return httpx.Response(
            200,
            json={
                "entry_hash": "deadbeef",
                "position": 17,
                "audit_path": ["a1", "a2"],
                "head": {
                    "tree_size": 18,
                    "root_hash": "rr",
                    "witness_count": 5,
                    "signed_at": "2026-05-16T08:12:34Z",
                },
            },
        )

    proof = _ReputationNamespace(_http_with_handler(handler)).proof(
        "12D3Koo", "deadbeef"
    )
    assert isinstance(proof, InclusionProof)
    assert proof.position == 17
    assert proof.audit_path == ["a1", "a2"]


def test_comment_signs_and_posts():
    captured = {}

    def handler(req: httpx.Request) -> httpx.Response:
        captured["path"] = req.url.path
        captured["body"] = req.content
        return httpx.Response(
            201, json={"cid": "cid-123", "ledger_hash": "hash-456"}
        )

    def fake_signer(digest: bytes) -> bytes:
        assert len(digest) == 32, "signer must be called with 32-byte digest"
        return b"X" * 64

    rep = _ReputationNamespace(_http_with_handler(handler))
    receipt = rep.comment(
        subject_peer_id="12D3Subject",
        text="Worked great",
        signer=fake_signer,
        rater_peer_id="12D3Rater",
    )
    assert isinstance(receipt, CommentReceipt)
    assert receipt.cid == "cid-123"
    assert receipt.ledger_hash == "hash-456"
    assert captured["path"] == "/v1/peers/12D3Subject/comments"
    # The body should carry a base64-encoded signature.
    assert b'"signature"' in captured["body"]
    assert base64.b64encode(b"X" * 64) in captured["body"]


def test_canonical_digest_is_stable():
    a = _canonical_digest(
        rater_peer_id="A",
        subject_peer_id="B",
        text="hello",
        language="en",
        attached_rating_hash=None,
    )
    b = _canonical_digest(
        rater_peer_id="A",
        subject_peer_id="B",
        text="hello",
        language="en",
        attached_rating_hash=None,
    )
    assert a == b
    # Sanity: matches our own re-derivation.
    h = hashlib.sha256()
    h.update(b"agentfm/comment/v1\n")
    h.update(b"A\n")
    h.update(b"B\n")
    h.update(b"en\n")
    h.update(b"hello\n")
    assert h.digest() == a


def test_canonical_digest_differs_on_field_change():
    base = _canonical_digest(
        rater_peer_id="A", subject_peer_id="B", text="hello",
        language="en", attached_rating_hash=None,
    )
    # Flipping any field should produce a different digest.
    for fields in [
        {"rater_peer_id": "Z"},
        {"subject_peer_id": "Z"},
        {"text": "world"},
        {"language": "ja"},
        {"attached_rating_hash": "ff"},
    ]:
        kwargs = dict(
            rater_peer_id="A",
            subject_peer_id="B",
            text="hello",
            language="en",
            attached_rating_hash=None,
        )
        kwargs.update(fields)
        assert _canonical_digest(**kwargs) != base
