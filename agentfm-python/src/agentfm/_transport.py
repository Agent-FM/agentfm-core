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
    InvalidRequestError,
    WorkerNotFoundError,
    WorkerStreamError,
    from_envelope,
)

_log = logging.getLogger(__name__)

# 30s connect / read / write / pool covers every non-streaming endpoint
# (workers list, OpenAI models, openai.chat.completions when stream=False).
# A wedged gateway that accepts the connection but never sends a body would
# otherwise hang the caller forever — the retry layer never fires because
# only ConnectError / ReadError trigger retry.
#
# Streaming endpoints (/api/execute, /v1/chat/completions?stream=true)
# override read=None at the call site via httpx_client.stream(...) — the
# `with httpx.Client.stream(..., timeout=...)` context manager accepts a
# per-request timeout. Pre-fix this default applied everywhere with read=None
# so even a non-stream call could hang.
DEFAULT_TIMEOUT = httpx.Timeout(30.0)

# Per-call streaming timeout used by tasks.run / tasks.stream and the OpenAI
# streaming endpoints. read=None lets a long-running task hold the response
# open for as long as the worker is producing output.
STREAMING_TIMEOUT = httpx.Timeout(30.0, read=None)

DEFAULT_LIMITS = httpx.Limits(max_connections=20, max_keepalive_connections=10)
DEFAULT_USER_AGENT = "agentfm-python"

# HTTP status codes that signal a transient gateway issue worth retrying.
# Matches the OpenAI Python SDK's retry policy.
RETRY_STATUSES: frozenset[int] = frozenset({408, 429, 500, 502, 503, 504})

# HTTP methods that are safe to re-issue after a 5xx response — the server
# may have processed the request before it errored, so only idempotent
# methods can be retried on a response status. Non-idempotent methods (POST,
# PATCH) are still retried on transport-level failures (see
# RETRYABLE_EXCEPTIONS), where the request likely never reached the server.
IDEMPOTENT_METHODS: frozenset[str] = frozenset(
    {"GET", "HEAD", "PUT", "DELETE", "OPTIONS", "TRACE"}
)

# Transient httpx exception classes worth retrying. Includes mid-stream
# disconnects (RemoteProtocolError — HAProxy idle drop, gateway restart) +
# write-side blips + pool exhaustion. _request wraps the broader
# httpx.HTTPError as GatewayConnectionError; this set decides which subset
# is retried before that wrap fires. Programmer errors like
# httpx.UnsupportedProtocol are deliberately NOT retried.
RETRYABLE_EXCEPTIONS: tuple[type[BaseException], ...] = (
    httpx.ConnectError,
    httpx.ReadError,
    httpx.WriteError,
    httpx.RemoteProtocolError,
    httpx.PoolTimeout,
)

# For NON-idempotent methods (POST), only these prove the request never
# reached the server. ReadError / RemoteProtocolError occur while reading the
# RESPONSE — the request already landed — so retrying a non-idempotent POST on
# them would double-submit (the /api/execute/async invariant commits the work
# BEFORE the 202 ack is written).
SAFE_NONIDEMPOTENT_RETRYABLE_EXCEPTIONS: tuple[type[BaseException], ...] = (
    httpx.ConnectError,
    httpx.WriteError,
    httpx.PoolTimeout,
)


def _effective_retry_exceptions(
    on: tuple[type[BaseException], ...], idempotent: bool
) -> tuple[type[BaseException], ...]:
    """Restrict the retryable exception set to request-never-sent failures for
    non-idempotent calls, so a POST is never re-issued after it reached the
    server."""
    if idempotent:
        return on
    return tuple(
        exc for exc in on if issubclass(exc, SAFE_NONIDEMPOTENT_RETRYABLE_EXCEPTIONS)
    )

R = TypeVar("R")


def _backoff_delay(attempt: int, base: float) -> float:
    """Exponential backoff with random jitter to avoid thundering herd."""
    return float(base * (2**attempt) + random.uniform(0, base * 0.5))


def _should_retry_response(response: httpx.Response | None) -> bool:
    return response is not None and response.status_code in RETRY_STATUSES


def _default_headers(user_agent: str, api_key: str | None) -> dict[str, str]:
    headers = {"User-Agent": user_agent}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    return headers


def make_client(
    base_url: str,
    *,
    timeout: float | httpx.Timeout | None = None,
    user_agent: str = DEFAULT_USER_AGENT,
    api_key: str | None = None,
) -> httpx.Client:
    return httpx.Client(
        base_url=base_url.rstrip("/"),
        timeout=_resolve_timeout(timeout),
        limits=DEFAULT_LIMITS,
        headers=_default_headers(user_agent, api_key),
        follow_redirects=False,
    )


def make_async_client(
    base_url: str,
    *,
    timeout: float | httpx.Timeout | None = None,
    user_agent: str = DEFAULT_USER_AGENT,
    api_key: str | None = None,
) -> httpx.AsyncClient:
    return httpx.AsyncClient(
        base_url=base_url.rstrip("/"),
        timeout=_resolve_timeout(timeout),
        limits=DEFAULT_LIMITS,
        headers=_default_headers(user_agent, api_key),
        follow_redirects=False,
    )


def _resolve_timeout(t: float | httpx.Timeout | None) -> httpx.Timeout:
    if t is None:
        return DEFAULT_TIMEOUT
    if isinstance(t, httpx.Timeout):
        return t
    # A bare float is the user saying "30s for everything including reads".
    # Streaming sites use STREAMING_TIMEOUT explicitly per call.
    return httpx.Timeout(t)


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


def raise_translated_stream_error(
    exc: httpx.HTTPError, *, base_url: str, label: str
) -> None:
    """Translate any mid-stream httpx failure into the right typed AgentFMError.

    Centralises what each ``_stream`` method used to do inline (`tasks.stream`,
    `chat.completions._stream`, `completions._stream`, plus their async
    equivalents). Adding a new transport-error class only needs to be wired
    here; the six call sites stay tiny.
    """
    if isinstance(exc, httpx.UnsupportedProtocol):
        raise InvalidRequestError(
            f"invalid gateway URL scheme: {exc}",
            code="invalid_gateway_url",
        ) from exc
    if isinstance(exc, httpx.ConnectError):
        raise wrap_connection_error(exc, base_url=base_url) from exc
    raise WorkerStreamError(
        f"{label} stream failed: {exc}",
        code="worker_stream_failed",
    ) from exc


# ---------------------------------------------------------------------------
# Retry helpers
# ---------------------------------------------------------------------------


def retry_sync(
    fn: Callable[..., R],
    *args: Any,
    retries: int = 3,
    backoff: float = 0.5,
    on: tuple[type[BaseException], ...] = RETRYABLE_EXCEPTIONS,
    idempotent: bool = True,
    **kwargs: Any,
) -> R:
    """Run ``fn`` with exponential backoff on transient failures.

    Retries on:
    * Listed exception types (default: connection / read errors).
    * For ``fn`` that returns ``httpx.Response``: status codes in
      :data:`RETRY_STATUSES` (408, 429, 5xx) — only when ``idempotent`` is
      ``True``. A non-idempotent call (POST) is never retried on a response
      status, since the server may have processed it before erroring; it is
      still retried on the transport-level failures in ``on``.

    Backoff is exponential with random jitter to avoid thundering-herd retries
    when a gateway recovers from overload.
    """
    last_exc: BaseException | None = None
    last_resp: R | None = None
    active_on = _effective_retry_exceptions(on, idempotent)
    for attempt in range(retries + 1):
        try:
            result = fn(*args, **kwargs)
        except active_on as exc:
            last_exc = exc
        else:
            resp = result if isinstance(result, httpx.Response) else None
            if not (idempotent and _should_retry_response(resp)):
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
    on: tuple[type[BaseException], ...] = RETRYABLE_EXCEPTIONS,
    idempotent: bool = True,
    **kwargs: Any,
) -> R:
    last_exc: BaseException | None = None
    last_resp: R | None = None
    active_on = _effective_retry_exceptions(on, idempotent)
    for attempt in range(retries + 1):
        try:
            result = await fn(*args, **kwargs)
        except active_on as exc:
            last_exc = exc
        else:
            resp = result if isinstance(result, httpx.Response) else None
            if not (idempotent and _should_retry_response(resp)):
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
    "IDEMPOTENT_METHODS",
    "STREAMING_TIMEOUT",
    "make_async_client",
    "make_client",
    "raise_for_response",
    "raise_translated_stream_error",
    "retry_async",
    "retry_sync",
    "wrap_connection_error",
]
