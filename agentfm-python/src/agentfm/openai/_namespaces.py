"""``OpenAINamespace`` and ``AsyncOpenAINamespace``.

Each resource class is a thin wrapper over a single endpoint. All HTTP plumbing
(retry, error envelope translation, response parsing) lives in
:class:`agentfm._internal.resource.SyncResource` /
:class:`AsyncResource`. The classes here only know about endpoint paths and
typed request/response shapes.

The nested attribute chain ``client.openai.chat.completions.create(...)``
matches the OpenAI Python SDK convention so users transferring code from the
``openai`` package don't have to relearn anything.
"""

from __future__ import annotations

import contextlib
import json
import threading
import warnings
from collections.abc import AsyncIterator, Iterator
from typing import TYPE_CHECKING, Any, Literal, overload

import httpx

from .._internal.resource import AsyncResource, SyncResource
from .._transport import STREAMING_TIMEOUT, raise_for_response, raise_translated_stream_error
from .._warnings import ROUTING_WARNING_STACKLEVEL, AgentFMRoutingWarning
from ..streaming import parse_sse_lines
from .models import (
    ChatCompletion,
    ChatCompletionChunk,
    ChatMessage,
    ModelsList,
    TextCompletion,
    TextCompletionChunk,
    coerce_messages,
)

if TYPE_CHECKING:
    from ..async_client import AsyncAgentFMClient
    from ..client import AgentFMClient


_PEER_ID_PREFIX = "12D3KooW"


class _RoutingWarner:
    """Per-namespace dedup of :class:`AgentFMRoutingWarning`.

    Each ``OpenAINamespace`` / ``AsyncOpenAINamespace`` owns one. Sharing a
    single instance across the chat and text-completion resources within a
    namespace gives "warn once per model per client" semantics. Thread-safe.
    """

    __slots__ = ("_lock", "_seen")

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._seen: set[str] = set()

    def warn_if_not_peer_id(self, model: str) -> None:
        if model.startswith(_PEER_ID_PREFIX):
            return
        with self._lock:
            if model in self._seen:
                return
            self._seen.add(model)
        warnings.warn(
            f"model={model!r} routes to *any* worker advertising that name on the mesh; "
            "the gateway picks for you. To pin to a specific worker (recommended), pass "
            "a peer_id from client.workers.list(). This warning fires once per model string.",
            AgentFMRoutingWarning,
            stacklevel=ROUTING_WARNING_STACKLEVEL,
        )

    def reset(self) -> None:
        """Clear the dedup state. Used by tests."""
        with self._lock:
            self._seen.clear()


def _build_chat_body(
    model: str,
    messages: list[ChatMessage] | list[dict[str, Any]],
    *,
    stream: bool,
    extra: dict[str, Any],
) -> dict[str, Any]:
    return {
        "model": model,
        "messages": coerce_messages(messages),
        "stream": bool(stream),
        **extra,
    }


def _build_completion_body(
    model: str, prompt: str, *, stream: bool, extra: dict[str, Any]
) -> dict[str, Any]:
    return {"model": model, "prompt": prompt, "stream": bool(stream), **extra}


# ---------------------------------------------------------------------------
# Sync namespace
# ---------------------------------------------------------------------------


class OpenAINamespace:
    """Sync ``/v1/*`` wrapper. Attached as ``client.openai``."""

    def __init__(self, client: AgentFMClient) -> None:
        self._client = client
        self._warner = _RoutingWarner()
        self.models = _Models(client)
        self.chat = _Chat(client, self._warner)
        self.completions = _Completions(client, self._warner)


class _Models(SyncResource):
    def list(self) -> ModelsList:
        return self._get("/v1/models", parse=ModelsList)


class _Chat:
    """Holder for ``client.openai.chat.completions`` attribute chain."""

    def __init__(self, client: AgentFMClient, warner: _RoutingWarner) -> None:
        self.completions = _ChatCompletions(client, warner)


class _ChatCompletions(SyncResource):
    def __init__(self, client: AgentFMClient, warner: _RoutingWarner) -> None:
        super().__init__(client)
        self._warner = warner

    @overload
    def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: Literal[False] = False,
        **extra: Any,
    ) -> ChatCompletion: ...
    @overload
    def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: Literal[True],
        **extra: Any,
    ) -> Iterator[ChatCompletionChunk]: ...

    def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: bool = False,
        **extra: Any,
    ) -> ChatCompletion | Iterator[ChatCompletionChunk]:
        self._warner.warn_if_not_peer_id(model)
        body = _build_chat_body(model, messages, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return self._post("/v1/chat/completions", body=body, parse=ChatCompletion)

    def _stream(self, body: dict[str, Any]) -> Iterator[ChatCompletionChunk]:
        try:
            with self._client._http.stream(
                "POST", "/v1/chat/completions", json=body, timeout=STREAMING_TIMEOUT
            ) as r:
                if r.status_code >= 400:
                    r.read()
                raise_for_response(r, expected_text=True)
                for payload in parse_sse_lines(r.iter_lines()):
                    with contextlib.suppress(ValueError):
                        yield ChatCompletionChunk.model_validate(json.loads(payload))
        except httpx.HTTPError as exc:
            raise_translated_stream_error(
                exc, base_url=self._client.gateway_url, label="openai"
            )


class _Completions(SyncResource):
    def __init__(self, client: AgentFMClient, warner: _RoutingWarner) -> None:
        super().__init__(client)
        self._warner = warner

    @overload
    def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: Literal[False] = False,
        **extra: Any,
    ) -> TextCompletion: ...
    @overload
    def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: Literal[True],
        **extra: Any,
    ) -> Iterator[TextCompletionChunk]: ...

    def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: bool = False,
        **extra: Any,
    ) -> TextCompletion | Iterator[TextCompletionChunk]:
        self._warner.warn_if_not_peer_id(model)
        body = _build_completion_body(model, prompt, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return self._post("/v1/completions", body=body, parse=TextCompletion)

    def _stream(self, body: dict[str, Any]) -> Iterator[TextCompletionChunk]:
        try:
            with self._client._http.stream(
                "POST", "/v1/completions", json=body, timeout=STREAMING_TIMEOUT
            ) as r:
                if r.status_code >= 400:
                    r.read()
                raise_for_response(r, expected_text=True)
                for payload in parse_sse_lines(r.iter_lines()):
                    with contextlib.suppress(ValueError):
                        yield TextCompletionChunk.model_validate(json.loads(payload))
        except httpx.HTTPError as exc:
            raise_translated_stream_error(
                exc, base_url=self._client.gateway_url, label="openai"
            )


# ---------------------------------------------------------------------------
# Async namespace
# ---------------------------------------------------------------------------


class AsyncOpenAINamespace:
    """Async ``/v1/*`` wrapper. Attached as ``async_client.openai``."""

    def __init__(self, client: AsyncAgentFMClient) -> None:
        self._client = client
        self._warner = _RoutingWarner()
        self.models = _AsyncModels(client)
        self.chat = _AsyncChat(client, self._warner)
        self.completions = _AsyncCompletions(client, self._warner)


class _AsyncModels(AsyncResource):
    async def list(self) -> ModelsList:
        return await self._get("/v1/models", parse=ModelsList)


class _AsyncChat:
    def __init__(self, client: AsyncAgentFMClient, warner: _RoutingWarner) -> None:
        self.completions = _AsyncChatCompletions(client, warner)


class _AsyncChatCompletions(AsyncResource):
    def __init__(self, client: AsyncAgentFMClient, warner: _RoutingWarner) -> None:
        super().__init__(client)
        self._warner = warner

    @overload
    async def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: Literal[False] = False,
        **extra: Any,
    ) -> ChatCompletion: ...
    @overload
    async def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: Literal[True],
        **extra: Any,
    ) -> AsyncIterator[ChatCompletionChunk]: ...

    async def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: bool = False,
        **extra: Any,
    ) -> ChatCompletion | AsyncIterator[ChatCompletionChunk]:
        """Create a chat completion (sync or streaming).

        Streaming usage — ``await`` first, then ``async for``::

            stream = await client.openai.chat.completions.create(
                model="...", messages=[...], stream=True,
            )
            async for chunk in stream:
                ...

        The double-step is required because ``async def create()`` returns
        a coroutine; awaiting it yields the AsyncIterator. Writing
        ``async for chunk in client.openai.chat.completions.create(...):``
        directly raises ``TypeError: 'coroutine' object is not async iterable``.
        """
        self._warner.warn_if_not_peer_id(model)
        body = _build_chat_body(model, messages, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return await self._post("/v1/chat/completions", body=body, parse=ChatCompletion)

    async def _stream(self, body: dict[str, Any]) -> AsyncIterator[ChatCompletionChunk]:
        try:
            async with self._client._http.stream(
                "POST", "/v1/chat/completions", json=body, timeout=STREAMING_TIMEOUT
            ) as r:
                if r.status_code >= 400:
                    await r.aread()
                raise_for_response(r, expected_text=True)
                async for payload in _aiter_sse(r):
                    with contextlib.suppress(ValueError):
                        yield ChatCompletionChunk.model_validate(json.loads(payload))
        except httpx.HTTPError as exc:
            raise_translated_stream_error(
                exc, base_url=self._client.gateway_url, label="openai"
            )


class _AsyncCompletions(AsyncResource):
    def __init__(self, client: AsyncAgentFMClient, warner: _RoutingWarner) -> None:
        super().__init__(client)
        self._warner = warner

    @overload
    async def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: Literal[False] = False,
        **extra: Any,
    ) -> TextCompletion: ...
    @overload
    async def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: Literal[True],
        **extra: Any,
    ) -> AsyncIterator[TextCompletionChunk]: ...

    async def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: bool = False,
        **extra: Any,
    ) -> TextCompletion | AsyncIterator[TextCompletionChunk]:
        """Create a text completion (sync or streaming).

        Streaming usage — ``await`` first, then ``async for``::

            stream = await client.openai.completions.create(
                model="...", prompt="...", stream=True,
            )
            async for chunk in stream:
                ...

        See :meth:`_AsyncChatCompletions.create` for the explanation.
        """
        self._warner.warn_if_not_peer_id(model)
        body = _build_completion_body(model, prompt, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return await self._post("/v1/completions", body=body, parse=TextCompletion)

    async def _stream(self, body: dict[str, Any]) -> AsyncIterator[TextCompletionChunk]:
        try:
            async with self._client._http.stream(
                "POST", "/v1/completions", json=body, timeout=STREAMING_TIMEOUT
            ) as r:
                if r.status_code >= 400:
                    await r.aread()
                raise_for_response(r, expected_text=True)
                async for payload in _aiter_sse(r):
                    with contextlib.suppress(ValueError):
                        yield TextCompletionChunk.model_validate(json.loads(payload))
        except httpx.HTTPError as exc:
            raise_translated_stream_error(
                exc, base_url=self._client.gateway_url, label="openai"
            )


async def _aiter_sse(r: httpx.Response) -> AsyncIterator[str]:
    """Async equivalent of :func:`agentfm.streaming.parse_sse_lines`."""
    async for raw in r.aiter_lines():
        line = raw.rstrip("\r\n")
        if not line or line.startswith(":") or not line.startswith("data:"):
            continue
        body = line[len("data:") :].lstrip()
        if body == "[DONE]":
            return
        if body:
            yield body
