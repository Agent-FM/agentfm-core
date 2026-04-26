"""Pure transport-agnostic helpers used by both sync and async clients.

Anything in here MUST NOT do I/O. Pulling these out keeps the two clients
honest: the only thing that should differ between them is whether the HTTP
call uses ``await``.
"""

from __future__ import annotations

from typing import Any

from .models import (
    AsyncTaskAck,
    PeerID,
    WorkerProfile,
    WorkersResponse,
)

DEFAULT_GATEWAY = "http://127.0.0.1:8080"


class _Unset:
    """Singleton sentinel for ``with_options`` — distinguishes "not provided" from ``None``."""

    _instance: _Unset | None = None

    def __new__(cls) -> _Unset:
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance


_UNSET = _Unset()

# ---------------------------------------------------------------------------
# Payload construction
# ---------------------------------------------------------------------------


def build_task_payload(
    worker_id: str, prompt: str, task_id: str | None = None
) -> dict[str, Any]:
    payload: dict[str, Any] = {"worker_id": worker_id, "prompt": prompt}
    if task_id:
        payload["task_id"] = task_id
    return payload


def build_async_task_payload(
    worker_id: str, prompt: str, webhook_url: str | None
) -> dict[str, Any]:
    payload: dict[str, Any] = {"worker_id": worker_id, "prompt": prompt}
    if webhook_url:
        payload["webhook_url"] = webhook_url
    return payload


# ---------------------------------------------------------------------------
# Response parsing
# ---------------------------------------------------------------------------


def parse_workers_envelope(body: Any) -> list[WorkerProfile]:
    """Validate the ``GET /api/workers`` envelope and return the agents list."""
    return WorkersResponse.model_validate(body).agents


def parse_async_task_ack(body: Any) -> AsyncTaskAck:
    return AsyncTaskAck.model_validate(body)


# ---------------------------------------------------------------------------
# Worker filtering (pure, used by both clients' workers.list())
# ---------------------------------------------------------------------------


def filter_workers(
    workers: list[WorkerProfile],
    *,
    model: str | None,
    agent_name: str | None,
    author: str | None,
    available_only: bool,
) -> list[WorkerProfile]:
    out = workers
    if model is not None:
        m = model.casefold()
        out = [w for w in out if w.model.casefold() == m]
    if agent_name is not None:
        n = agent_name.casefold()
        out = [w for w in out if w.name.casefold() == n]
    if author is not None:
        a = author.casefold()
        out = [w for w in out if w.author.casefold() == a]
    if available_only:
        out = [w for w in out if w.is_available]
    return out


def coerce_peer_ids(peer_ids: list[PeerID | str]) -> list[str]:
    return [str(p) for p in peer_ids]


__all__ = [
    "DEFAULT_GATEWAY",
    "build_async_task_payload",
    "build_task_payload",
    "coerce_peer_ids",
    "filter_workers",
    "parse_async_task_ack",
    "parse_workers_envelope",
]
