"""Shared HTTP plumbing.

* Builds httpx ``Client`` / ``AsyncClient`` instances with sane defaults.
* Centralises error-envelope translation (Go ``{"error": ...}`` -> typed exception).
* Provides a tiny retry-with-backoff helper used by both sync and async clients.
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
import random
import time
from collections.abc import Awaitable, Callable
from typing import Any, TypeVar

import httpx

from .exceptions import (
    GatewayConnectionError,
    GatewayProtocolError,
    WorkerNotFoundError,
    from_envelope,
)

_log = logging.getLogger(__name__)

DEFAULT_TIMEOUT = httpx.Timeout(30.0, read=None)  # No read-timeout for streaming
DEFAULT_LIMITS = httpx.Limits(max_connections=20, max_keepalive_connections=10)
DEFAULT_USER_AGENT = "agentfm-python"

# HTTP status codes that signal a transient gateway issue worth retrying.
# Matches the OpenAI Python SDK's retry policy.
RETRY_STATUSES: frozenset[int] = frozenset({408, 429, 500, 502, 503, 504})

R = TypeVar("R")


def _backoff_delay(attempt: int, base: float) -> float:
    """Exponential backoff with random jitter to avoid thundering herd."""
    return float(base * (2**attempt) + random.uniform(0, base * 0.5))


def _should_retry_response(response: httpx.Response | None) -> bool:
    return response is not None and response.status_code in RETRY_STATUSES


def make_client(
    base_url: str,
    *,
    timeout: float | httpx.Timeout | None = None,
    user_agent: str = DEFAULT_USER_AGENT,
) -> httpx.Client:
    return httpx.Client(
        base_url=base_url.rstrip("/"),
        timeout=_resolve_timeout(timeout),
        limits=DEFAULT_LIMITS,
        headers={"User-Agent": user_agent},
        follow_redirects=False,
    )


def make_async_client(
    base_url: str,
    *,
    timeout: float | httpx.Timeout | None = None,
    user_agent: str = DEFAULT_USER_AGENT,
) -> httpx.AsyncClient:
    return httpx.AsyncClient(
        base_url=base_url.rstrip("/"),
        timeout=_resolve_timeout(timeout),
        limits=DEFAULT_LIMITS,
        headers={"User-Agent": user_agent},
        follow_redirects=False,
    )


def _resolve_timeout(t: float | httpx.Timeout | None) -> httpx.Timeout:
    if t is None:
        return DEFAULT_TIMEOUT
    if isinstance(t, httpx.Timeout):
        return t
    return httpx.Timeout(t, read=None)


# ---------------------------------------------------------------------------
# Error mapping
# ---------------------------------------------------------------------------


def raise_for_response(response: httpx.Response, *, expected_text: bool = False) -> None:
    """Translate non-2xx responses into the appropriate AgentFMError subclass.

    Returns silently for 2xx. ``expected_text`` indicates the endpoint normally
    returns plain text (the legacy ``/api/execute`` stream); error envelopes
    still arrive as JSON, so we only treat 2xx text as success.
    """
    if 200 <= response.status_code < 300:
        return
    body: dict[str, Any] | None = None
    content_type = response.headers.get("content-type", "")
    if "json" in content_type or response.text.lstrip().startswith("{"):
        with contextlib.suppress(ValueError):
            body = response.json()

    if response.status_code == 404 and not body:
        raise WorkerNotFoundError(
            response.text or "worker not found",
            status=404,
        )
    if body is not None:
        raise from_envelope(body, response.status_code)
    if expected_text:
        raise GatewayProtocolError(
            f"unexpected status {response.status_code}: {response.text[:200]}",
            status=response.status_code,
        )
    raise GatewayProtocolError(
        f"unexpected status {response.status_code} (no error envelope)",
        status=response.status_code,
    )


def wrap_connection_error(exc: Exception, *, base_url: str) -> GatewayConnectionError:
    """Wrap an httpx connection failure into our typed exception."""
    return GatewayConnectionError(
        f"could not reach gateway at {base_url}: {exc}",
    )


# ---------------------------------------------------------------------------
# Retry helpers
# ---------------------------------------------------------------------------


def retry_sync(
    fn: Callable[..., R],
    *args: Any,
    retries: int = 3,
    backoff: float = 0.5,
    on: tuple[type[BaseException], ...] = (httpx.ConnectError, httpx.ReadError),
    **kwargs: Any,
) -> R:
    """Run ``fn`` with exponential backoff on transient failures.

    Retries on:
    * Listed exception types (default: connection / read errors).
    * For ``fn`` that returns ``httpx.Response``: status codes in
      :data:`RETRY_STATUSES` (408, 429, 5xx).

    Backoff is exponential with random jitter to avoid thundering-herd retries
    when a gateway recovers from overload.
    """
    last_exc: BaseException | None = None
    last_resp: R | None = None
    for attempt in range(retries + 1):
        try:
            result = fn(*args, **kwargs)
        except on as exc:
            last_exc = exc
        else:
            if not _should_retry_response(result if isinstance(result, httpx.Response) else None):
                return result
            last_resp = result
            last_exc = None
        if attempt == retries:
            break
        sleep = _backoff_delay(attempt, backoff)
        _log.debug(
            "retry %s/%s after %.2fs (exc=%s, status=%s)",
            attempt + 1, retries, sleep, last_exc,
            last_resp.status_code if isinstance(last_resp, httpx.Response) else None,
        )
        time.sleep(sleep)
    if last_exc is not None:
        raise last_exc
    assert last_resp is not None
    return last_resp


async def retry_async(
    fn: Callable[..., Awaitable[R]],
    *args: Any,
    retries: int = 3,
    backoff: float = 0.5,
    on: tuple[type[BaseException], ...] = (httpx.ConnectError, httpx.ReadError),
    **kwargs: Any,
) -> R:
    last_exc: BaseException | None = None
    last_resp: R | None = None
    for attempt in range(retries + 1):
        try:
            result = await fn(*args, **kwargs)
        except on as exc:
            last_exc = exc
        else:
            if not _should_retry_response(result if isinstance(result, httpx.Response) else None):
                return result
            last_resp = result
            last_exc = None
        if attempt == retries:
            break
        sleep = _backoff_delay(attempt, backoff)
        _log.debug(
            "retry %s/%s after %.2fs (exc=%s, status=%s)",
            attempt + 1, retries, sleep, last_exc,
            last_resp.status_code if isinstance(last_resp, httpx.Response) else None,
        )
        await asyncio.sleep(sleep)
    if last_exc is not None:
        raise last_exc
    assert last_resp is not None
    return last_resp


__all__ = [
    "DEFAULT_LIMITS",
    "DEFAULT_TIMEOUT",
    "DEFAULT_USER_AGENT",
    "make_async_client",
    "make_client",
    "raise_for_response",
    "retry_async",
    "retry_sync",
    "wrap_connection_error",
]
