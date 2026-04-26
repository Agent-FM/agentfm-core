"""Synchronous AgentFM client.

Surface (canonical):

    client.workers.list(...)            # discover
    client.workers.get(peer_id)         # fetch one
    client.tasks.run(worker_id=peer_id, prompt=...)            # blocking
    client.tasks.stream(worker_id=peer_id, prompt=...)         # generator
    client.tasks.submit_async(worker_id=peer_id, ...)          # 202 + webhook
    client.tasks.scatter(prompts, peer_ids=[...])              # batch
    client.tasks.scatter_by_model(prompts, model=...)          # batch by name
    client.openai.models.list()
    client.openai.chat.completions.create(model=..., messages=[...])
    client.openai.completions.create(model=..., prompt=...)
"""

from __future__ import annotations

import logging
import time
import uuid
from collections.abc import Callable, Iterator
from concurrent.futures import FIRST_COMPLETED, Future, ThreadPoolExecutor, wait
from functools import cached_property
from pathlib import Path
from types import TracebackType
from typing import TYPE_CHECKING, Any, List  # noqa: UP035 - List used to disambiguate from def list

import httpx

from ._internal.resource import SyncResource
from ._shared import (
    _UNSET,
    DEFAULT_GATEWAY,
    _Unset,
    build_async_task_payload,
    build_task_payload,
    coerce_peer_ids,
    filter_workers,
    parse_async_task_ack,
    parse_workers_envelope,
)
from ._transport import (
    STREAMING_TIMEOUT,
    make_client,
    raise_for_response,
    wrap_connection_error,
)
from .artifacts import ArtifactManager
from .exceptions import (
    AgentFMError,
    GatewayConnectionError,
    WorkerNotFoundError,
)
from .models import (
    AsyncTaskAck,
    PeerID,
    ScatterResult,
    TaskChunk,
    TaskResult,
    WorkerProfile,
)
from .streaming import SentinelFilter

if TYPE_CHECKING:
    from .openai import OpenAINamespace

_log = logging.getLogger(__name__)


class _WorkersNamespace(SyncResource):
    """``client.workers.*`` — discovery operations."""

    def list(
        self,
        *,
        model: str | None = None,
        agent_name: str | None = None,
        author: str | None = None,
        available_only: bool = False,
        wait_for_workers: int = 0,
        poll_timeout: float = 15.0,
        poll_interval: float = 1.0,
    ) -> List[WorkerProfile]:
        """List workers visible on the mesh.

        ``model``, ``agent_name``, and ``author`` are post-fetch filters
        applied locally (string equality, case-insensitive). ``available_only``
        drops ``BUSY`` workers. If ``wait_for_workers > 0`` polls until at
        least that many matching workers appear or ``poll_timeout`` elapses.
        """
        deadline = time.monotonic() + poll_timeout
        last: List[WorkerProfile] = []
        while True:
            last = filter_workers(
                self._fetch_once(),
                model=model,
                agent_name=agent_name,
                author=author,
                available_only=available_only,
            )
            if len(last) >= wait_for_workers or time.monotonic() >= deadline:
                return last
            time.sleep(poll_interval)

    def get(self, peer_id: PeerID | str) -> WorkerProfile:
        """Return the profile for a specific peer, raising if not present."""
        peer_id = str(peer_id)
        for w in self._fetch_once():
            if w.peer_id == peer_id:
                return w
        raise WorkerNotFoundError(f"peer {peer_id!r} not in current telemetry")

    def _fetch_once(self) -> List[WorkerProfile]:
        return parse_workers_envelope(self._request("GET", "/api/workers").json())


class _TasksNamespace(SyncResource):
    """``client.tasks.*`` — task dispatch."""

    def run(
        self,
        *,
        worker_id: PeerID | str,
        prompt: str,
        artifact_timeout: float = 120.0,
    ) -> TaskResult:
        """Dispatch a task and block until it completes."""
        # Timing is recorded locally; never on instance state — multiple
        # concurrent run() calls on the same namespace would race otherwise.
        # We mint the task_id here so the gateway and the artifact harvester
        # agree on the zip basename — concurrent run() calls would otherwise
        # race for "the latest zip by mtime".
        task_id = f"task_{uuid.uuid4().hex}"
        started_mono = time.monotonic()
        chunks = [
            chunk.text
            for chunk in self.stream(worker_id=worker_id, prompt=prompt, task_id=task_id)
            if chunk.kind == "text"
        ]
        text = "".join(chunks)

        am = self._client.artifacts
        artifacts: list[Path] = (
            am.collect_for_task(task_id, timeout=artifact_timeout) if am is not None else []
        )

        return TaskResult(
            worker_id=PeerID(str(worker_id)),
            text=text,
            artifacts=artifacts,
            duration_seconds=time.monotonic() - started_mono,
        )

    def stream(
        self, *, worker_id: PeerID | str, prompt: str, task_id: str | None = None
    ) -> Iterator[TaskChunk]:
        """Stream worker stdout chunk-by-chunk as ``TaskChunk`` objects.

        ``task_id`` is optional. When provided, the gateway uses it as the
        artifact zip basename so the caller can attribute artifacts to a
        specific dispatch (see :meth:`AgentFMClient.tasks.run`).
        """
        payload = build_task_payload(str(worker_id), prompt, task_id=task_id)
        filt = SentinelFilter()
        try:
            with self._client._http.stream(
                "POST", "/api/execute", json=payload, timeout=STREAMING_TIMEOUT
            ) as response:
                # Streaming responses don't auto-load the body, so an error
                # envelope on a non-2xx status would be unreachable from
                # raise_for_response without an explicit read.
                if response.status_code >= 400:
                    response.read()
                raise_for_response(response, expected_text=True)
                for raw in response.iter_text():
                    for clean in filt.feed(raw):
                        yield TaskChunk(text=clean)
                tail = filt.finalize()
                if tail:
                    yield TaskChunk(text=tail)
                if filt.artifacts_incoming:
                    yield TaskChunk(text="", kind="marker")
        except httpx.ConnectError as exc:
            raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc

    def submit_async(
        self,
        *,
        worker_id: PeerID | str,
        prompt: str,
        webhook_url: str | None = None,
    ) -> AsyncTaskAck:
        """Submit a task asynchronously; gateway 202s with a task id."""
        payload = build_async_task_payload(str(worker_id), prompt, webhook_url)
        return parse_async_task_ack(
            self._request("POST", "/api/execute/async", json=payload).json()
        )

    def scatter(
        self,
        prompts: list[str],
        *,
        peer_ids: list[PeerID | str],
        max_concurrency: int = 8,
        max_retries: int = 2,
    ) -> list[ScatterResult]:
        """Run ``len(prompts)`` tasks across the given peers (round-robin).

        Results are returned in **submission order**: ``return[i]`` corresponds
        to ``prompts[i]``. Failed prompts (after ``max_retries`` exhaustion)
        appear as :class:`ScatterResult` with ``status="failed"`` and a populated
        ``error`` field, never as exceptions.
        """
        if not prompts:
            return []
        if not peer_ids:
            raise ValueError("scatter requires at least one peer id")

        peers = coerce_peer_ids(peer_ids)
        # Accumulate into a dict keyed by prompt index. The main thread is
        # the sole writer (workers return results via fut.result(), which the
        # main thread reads), so this is race-free even on a no-GIL CPython.
        # Returning in submission order (ScatterResult[i] corresponds to
        # prompts[i]) is also strictly more useful than completion order.
        results_by_idx: dict[int, ScatterResult] = {}
        attempts: dict[int, int] = dict.fromkeys(range(len(prompts)), 0)
        queue: list[tuple[int, str]] = list(enumerate(prompts))
        cursor = 0

        with ThreadPoolExecutor(max_workers=max_concurrency) as ex:
            in_flight: dict[Future[TaskResult], tuple[int, str, str]] = {}
            while queue or in_flight:
                while queue and len(in_flight) < max_concurrency:
                    idx, prompt = queue.pop(0)
                    peer = peers[cursor % len(peers)]
                    cursor += 1
                    fut = ex.submit(self.run, worker_id=peer, prompt=prompt)
                    in_flight[fut] = (idx, prompt, peer)
                done, _ = wait(in_flight.keys(), return_when=FIRST_COMPLETED)
                for fut in done:
                    idx, prompt, peer = in_flight.pop(fut)
                    try:
                        res = fut.result()
                        results_by_idx[idx] = ScatterResult(
                            prompt=prompt,
                            worker_id=PeerID(peer),
                            status="success",
                            text=res.text,
                            artifacts=res.artifacts,
                        )
                    except AgentFMError as exc:
                        attempts[idx] += 1
                        if attempts[idx] <= max_retries:
                            _log.info("retrying prompt #%s (attempt %s)", idx, attempts[idx])
                            queue.append((idx, prompt))
                        else:
                            results_by_idx[idx] = ScatterResult(
                                prompt=prompt,
                                worker_id=PeerID(peer),
                                status="failed",
                                error=str(exc),
                            )
        return [results_by_idx[i] for i in range(len(prompts))]

    def scatter_by_model(
        self,
        prompts: list[str],
        *,
        model: str,
        max_workers: int | None = None,
        pick: Callable[[List[WorkerProfile]], List[WorkerProfile]] | None = None,
        **scatter_opts: Any,
    ) -> list[ScatterResult]:
        """Convenience: discover workers by ``model``, then ``scatter`` across them."""
        candidates = self._client.workers.list(model=model, available_only=True)
        if pick is not None:
            candidates = pick(candidates)
        elif max_workers is not None:
            candidates = candidates[:max_workers]
        if not candidates:
            raise WorkerNotFoundError(f"no available workers advertise model={model!r}")
        return self.scatter(
            prompts,
            peer_ids=[w.peer_id for w in candidates],
            **scatter_opts,
        )


class AgentFMClient:
    """Synchronous AgentFM client."""

    def __init__(
        self,
        gateway_url: str = DEFAULT_GATEWAY,
        *,
        timeout: float | httpx.Timeout | None = None,
        retries: int = 2,
        artifacts_dir: str | Path | None = None,
    ) -> None:
        """Build a client.

        ``artifacts_dir`` should point at the directory the **boss** writes
        artifacts into (typically ``<boss_cwd>/agentfm_artifacts``). When
        unset, ``tasks.run`` does NOT attempt to harvest artifacts and
        returns ``artifacts=[]``. Set this only when the SDK process and
        the boss process share a filesystem.
        """
        self.gateway_url = gateway_url.rstrip("/")
        self.retries = retries
        self._http = make_client(self.gateway_url, timeout=timeout)
        self.artifacts: ArtifactManager | None = (
            ArtifactManager(watch_dir=artifacts_dir, extract_dir=artifacts_dir)
            if artifacts_dir is not None
            else None
        )

    @cached_property
    def workers(self) -> _WorkersNamespace:
        return _WorkersNamespace(self)

    @cached_property
    def tasks(self) -> _TasksNamespace:
        return _TasksNamespace(self)

    @cached_property
    def openai(self) -> OpenAINamespace:
        from .openai import OpenAINamespace

        return OpenAINamespace(self)

    def with_options(
        self,
        *,
        gateway_url: str | _Unset = _UNSET,
        timeout: float | httpx.Timeout | None | _Unset = _UNSET,
        retries: int | _Unset = _UNSET,
        artifacts_dir: str | Path | None | _Unset = _UNSET,
    ) -> AgentFMClient:
        """Return a new client with the given options overridden.

        Each unspecified option is inherited from this client. The returned
        client owns a fresh ``httpx.Client`` — close it independently.
        """
        return type(self)(
            gateway_url=self.gateway_url if isinstance(gateway_url, _Unset) else gateway_url,
            timeout=self._http.timeout if isinstance(timeout, _Unset) else timeout,
            retries=self.retries if isinstance(retries, _Unset) else retries,
            artifacts_dir=(
                (self.artifacts.watch_dir if self.artifacts else None)
                if isinstance(artifacts_dir, _Unset)
                else artifacts_dir
            ),
        )

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> AgentFMClient:
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        self.close()

    def ping(self) -> bool:
        """Quick liveness check on the gateway."""
        try:
            r = self._http.get("/api/workers")
        except (httpx.HTTPError, GatewayConnectionError):
            return False
        return r.status_code == 200


__all__ = ["DEFAULT_GATEWAY", "AgentFMClient"]
