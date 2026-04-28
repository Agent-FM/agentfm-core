"""Asynchronous AgentFM client.

Mirrors :class:`agentfm.AgentFMClient` with ``async`` / ``await`` semantics.
"""

from __future__ import annotations

import asyncio
import logging
import os
import time
import uuid
from collections.abc import AsyncIterator, Callable
from functools import cached_property
from pathlib import Path
from types import TracebackType
from typing import TYPE_CHECKING, Any, List  # noqa: UP035 - List used to disambiguate from def list

import httpx

from ._internal.resource import AsyncResource
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
    make_async_client,
    raise_for_response,
    raise_translated_stream_error,
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
    from .openai import AsyncOpenAINamespace

_log = logging.getLogger(__name__)


class _AsyncWorkersNamespace(AsyncResource):
    async def list(
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
        deadline = time.monotonic() + poll_timeout
        last: List[WorkerProfile] = []
        while True:
            last = filter_workers(
                await self._fetch_once(),
                model=model,
                agent_name=agent_name,
                author=author,
                available_only=available_only,
            )
            if len(last) >= wait_for_workers or time.monotonic() >= deadline:
                return last
            await asyncio.sleep(poll_interval)

    async def get(self, peer_id: PeerID | str) -> WorkerProfile:
        peer_id = str(peer_id)
        for w in await self._fetch_once():
            if w.peer_id == peer_id:
                return w
        raise WorkerNotFoundError(f"peer {peer_id!r} not in current telemetry")

    async def _fetch_once(self) -> List[WorkerProfile]:
        r = await self._request("GET", "/api/workers")
        return parse_workers_envelope(r.json())


class _AsyncTasksNamespace(AsyncResource):
    async def run(
        self,
        *,
        worker_id: PeerID | str,
        prompt: str,
        artifact_timeout: float = 120.0,
    ) -> TaskResult:
        chunks: list[str] = []
        task_id = f"task_{uuid.uuid4().hex}"
        started_mono = time.monotonic()
        async for chunk in self.stream(
            worker_id=worker_id, prompt=prompt, task_id=task_id
        ):
            if chunk.kind == "text":
                chunks.append(chunk.text)
        text = "".join(chunks)

        am = self._client.artifacts
        artifacts: list[Path] = (
            await asyncio.to_thread(am.collect_for_task, task_id, artifact_timeout)
            if am is not None
            else []
        )

        return TaskResult(
            worker_id=PeerID(str(worker_id)),
            text=text,
            artifacts=artifacts,
            duration_seconds=time.monotonic() - started_mono,
        )

    async def stream(
        self,
        *,
        worker_id: PeerID | str,
        prompt: str,
        task_id: str | None = None,
    ) -> AsyncIterator[TaskChunk]:
        """Stream worker stdout chunk-by-chunk as ``TaskChunk`` objects.

        Early termination of the consumer (``async for ... break``) requires
        explicit cleanup so the underlying httpx response is released
        promptly. Wrap the iteration with :func:`contextlib.aclosing`::

            from contextlib import aclosing
            async with aclosing(client.tasks.stream(...)) as gen:
                async for chunk in gen:
                    if want_to_stop: break

        CPython's reference-counting usually finalises the generator on
        the next GC cycle, but on PyPy / under pressure / if the generator
        is stored in a future, the response can outlive the loop.
        """
        payload = build_task_payload(str(worker_id), prompt, task_id=task_id)
        filter_ = SentinelFilter()
        try:
            async with self._client._http.stream(
                "POST", "/api/execute", json=payload, timeout=STREAMING_TIMEOUT
            ) as response:
                # Streaming responses don't auto-load the body, so an error
                # envelope on a non-2xx status would be unreachable from
                # raise_for_response without an explicit aread.
                if response.status_code >= 400:
                    await response.aread()
                raise_for_response(response, expected_text=True)
                async for raw in response.aiter_text():
                    for clean in filter_.feed(raw):
                        yield TaskChunk(text=clean)
                tail = filter_.finalize()
                if tail:
                    yield TaskChunk(text=tail)
                if filter_.artifacts_incoming:
                    yield TaskChunk(text="", kind="marker")
        except httpx.HTTPError as exc:
            raise_translated_stream_error(
                exc, base_url=self._client.gateway_url, label="worker"
            )

    async def submit_async(
        self,
        *,
        worker_id: PeerID | str,
        prompt: str,
        webhook_url: str | None = None,
    ) -> AsyncTaskAck:
        payload = build_async_task_payload(str(worker_id), prompt, webhook_url)
        r = await self._request("POST", "/api/execute/async", json=payload)
        return parse_async_task_ack(r.json())

    async def scatter(
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
        sem = asyncio.Semaphore(max_concurrency)
        peers_str = coerce_peer_ids(peer_ids)
        results_by_idx: dict[int, ScatterResult] = {}

        async def worker(idx: int, prompt: str) -> None:
            # Iterative retry: each attempt acquires the semaphore fresh, so
            # max_concurrency=1 doesn't deadlock and a failing peer is not
            # reused on retry. The cursor advances by attempt count, giving
            # automatic failover across the supplied peer pool.
            last_exc: BaseException | None = None
            last_peer = peers_str[idx % len(peers_str)]
            for attempt in range(max_retries + 1):
                peer = peers_str[(idx + attempt) % len(peers_str)]
                last_peer = peer
                async with sem:
                    try:
                        res = await self.run(worker_id=peer, prompt=prompt)
                    except Exception as exc:
                        # Wider than AgentFMError on purpose: tasks.run can
                        # raise OSError from artifact harvesting which would
                        # otherwise propagate via asyncio.gather and cancel
                        # siblings, breaking the "scatter never raises"
                        # contract. Anything unexpected is logged.
                        if not isinstance(exc, AgentFMError):
                            _log.exception(
                                "scatter prompt #%s raised non-AgentFMError; treating as failure",
                                idx,
                            )
                        last_exc = exc
                        if attempt < max_retries:
                            _log.info(
                                "retrying prompt #%s (attempt %s)", idx, attempt + 1
                            )
                        continue
                results_by_idx[idx] = ScatterResult(
                    prompt=prompt,
                    worker_id=PeerID(peer),
                    status="success",
                    text=res.text,
                    artifacts=res.artifacts,
                )
                return
            results_by_idx[idx] = ScatterResult(
                prompt=prompt,
                worker_id=PeerID(last_peer),
                status="failed",
                error=str(last_exc) if last_exc is not None else "unknown error",
            )

        await asyncio.gather(*(worker(i, p) for i, p in enumerate(prompts)))
        return [results_by_idx[i] for i in range(len(prompts))]

    async def scatter_by_model(
        self,
        prompts: list[str],
        *,
        model: str,
        max_workers: int | None = None,
        pick: Callable[[List[WorkerProfile]], List[WorkerProfile]] | None = None,
        **scatter_opts: Any,
    ) -> list[ScatterResult]:
        """Convenience: discover workers by ``model``, then ``scatter`` across them.

        Mirrors :meth:`AgentFMClient.tasks.scatter_by_model`. ``pick`` overrides
        ``max_workers`` when both are passed.
        """
        candidates = await self._client.workers.list(model=model, available_only=True)
        if pick is not None:
            candidates = pick(candidates)
        elif max_workers is not None:
            candidates = candidates[:max_workers]
        if not candidates:
            raise WorkerNotFoundError(f"no available workers advertise model={model!r}")
        return await self.scatter(
            prompts,
            peer_ids=[w.peer_id for w in candidates],
            **scatter_opts,
        )


class AsyncAgentFMClient:
    """Asynchronous AgentFM client."""

    def __init__(
        self,
        gateway_url: str = DEFAULT_GATEWAY,
        *,
        timeout: float | httpx.Timeout | None = None,
        retries: int = 2,
        artifacts_dir: str | Path | None = None,
        api_key: str | None | _Unset = _UNSET,
    ) -> None:
        self.gateway_url = gateway_url.rstrip("/")
        self.retries = retries
        if isinstance(api_key, _Unset):
            self.api_key = os.environ.get("AGENTFM_API_KEY") or None
        else:
            self.api_key = api_key or None
        self._http = make_async_client(self.gateway_url, timeout=timeout, api_key=self.api_key)
        self.artifacts: ArtifactManager | None = (
            ArtifactManager(watch_dir=artifacts_dir, extract_dir=artifacts_dir)
            if artifacts_dir is not None
            else None
        )

    @cached_property
    def workers(self) -> _AsyncWorkersNamespace:
        return _AsyncWorkersNamespace(self)

    @cached_property
    def tasks(self) -> _AsyncTasksNamespace:
        return _AsyncTasksNamespace(self)

    @cached_property
    def openai(self) -> AsyncOpenAINamespace:
        from .openai import AsyncOpenAINamespace

        return AsyncOpenAINamespace(self)

    def with_options(
        self,
        *,
        gateway_url: str | _Unset = _UNSET,
        timeout: float | httpx.Timeout | None | _Unset = _UNSET,
        retries: int | _Unset = _UNSET,
        artifacts_dir: str | Path | None | _Unset = _UNSET,
        api_key: str | None | _Unset = _UNSET,
    ) -> AsyncAgentFMClient:
        """Return a new client with the given options overridden.

        Each unspecified option is inherited from this client. The returned
        client owns a fresh ``httpx.AsyncClient`` — close it independently.

        ``api_key`` follows the same sentinel rule: pass an explicit ``None``
        to drop authentication on the derived client, omit to inherit.
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
            api_key=self.api_key if isinstance(api_key, _Unset) else api_key,
        )

    async def aclose(self) -> None:
        await self._http.aclose()

    def __del__(self) -> None:
        # Defensive close: synchronous in __del__ because there is no event
        # loop guarantee at GC time. We close the transport directly to free
        # sockets without scheduling on a running loop. Wrapped because
        # __del__ during interpreter teardown can raise on already-collected
        # attributes.
        import contextlib
        with contextlib.suppress(Exception):
            transport = getattr(self._http, "_transport", None)
            close = getattr(transport, "close", None)
            if close is not None:
                close()

    async def __aenter__(self) -> AsyncAgentFMClient:
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        await self.aclose()

    async def ping(self) -> bool:
        try:
            r = await self._http.get("/api/workers")
            return r.status_code == 200
        except (httpx.HTTPError, GatewayConnectionError):
            return False


__all__ = ["DEFAULT_GATEWAY", "AsyncAgentFMClient"]
