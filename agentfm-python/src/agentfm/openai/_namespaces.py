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
import warnings
from collections.abc import AsyncIterator, Iterator
from typing import TYPE_CHECKING, Any

import httpx

from .._internal.resource import AsyncResource, SyncResource
from .._transport import raise_for_response, wrap_connection_error
from .._warnings import AgentFMRoutingWarning
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
_warned_about_routing: set[str] = set()


def _warn_if_not_peer_id(model: str) -> None:
    if model.startswith(_PEER_ID_PREFIX) or model in _warned_about_routing:
        return
    _warned_about_routing.add(model)
    warnings.warn(
        f"model={model!r} routes to *any* worker advertising that name on the mesh; "
        "the gateway picks for you. To pin to a specific worker (recommended), pass "
        "a peer_id from client.workers.list(). This warning fires once per model string.",
        AgentFMRoutingWarning,
        stacklevel=4,
    )


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
        self.models = _Models(client)
        self.chat = _Chat(client)
        self.completions = _Completions(client)


class _Models(SyncResource):
    def list(self) -> ModelsList:
        return self._get("/v1/models", parse=ModelsList)


class _Chat:
    """Holder for ``client.openai.chat.completions`` attribute chain."""

    def __init__(self, client: AgentFMClient) -> None:
        self.completions = _ChatCompletions(client)


class _ChatCompletions(SyncResource):
    def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: bool = False,
        **extra: Any,
    ) -> ChatCompletion | Iterator[ChatCompletionChunk]:
        _warn_if_not_peer_id(model)
        body = _build_chat_body(model, messages, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return self._post("/v1/chat/completions", body=body, parse=ChatCompletion)

    def _stream(self, body: dict[str, Any]) -> Iterator[ChatCompletionChunk]:
        try:
            with self._client._http.stream(
                "POST", "/v1/chat/completions", json=body
            ) as r:
                raise_for_response(r, expected_text=True)
                for payload in parse_sse_lines(r.iter_lines()):
                    with contextlib.suppress(ValueError):
                        yield ChatCompletionChunk.model_validate(json.loads(payload))
        except httpx.ConnectError as exc:
            raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc


class _Completions(SyncResource):
    def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: bool = False,
        **extra: Any,
    ) -> TextCompletion | Iterator[TextCompletionChunk]:
        _warn_if_not_peer_id(model)
        body = _build_completion_body(model, prompt, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return self._post("/v1/completions", body=body, parse=TextCompletion)

    def _stream(self, body: dict[str, Any]) -> Iterator[TextCompletionChunk]:
        try:
            with self._client._http.stream("POST", "/v1/completions", json=body) as r:
                raise_for_response(r, expected_text=True)
                for payload in parse_sse_lines(r.iter_lines()):
                    with contextlib.suppress(ValueError):
                        yield TextCompletionChunk.model_validate(json.loads(payload))
        except httpx.ConnectError as exc:
            raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc


# ---------------------------------------------------------------------------
# Async namespace
# ---------------------------------------------------------------------------


class AsyncOpenAINamespace:
    """Async ``/v1/*`` wrapper. Attached as ``async_client.openai``."""

    def __init__(self, client: AsyncAgentFMClient) -> None:
        self._client = client
        self.models = _AsyncModels(client)
        self.chat = _AsyncChat(client)
        self.completions = _AsyncCompletions(client)


class _AsyncModels(AsyncResource):
    async def list(self) -> ModelsList:
        return await self._get("/v1/models", parse=ModelsList)


class _AsyncChat:
    def __init__(self, client: AsyncAgentFMClient) -> None:
        self.completions = _AsyncChatCompletions(client)


class _AsyncChatCompletions(AsyncResource):
    async def create(
        self,
        *,
        model: str,
        messages: list[ChatMessage] | list[dict[str, Any]],
        stream: bool = False,
        **extra: Any,
    ) -> ChatCompletion | AsyncIterator[ChatCompletionChunk]:
        _warn_if_not_peer_id(model)
        body = _build_chat_body(model, messages, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return await self._post("/v1/chat/completions", body=body, parse=ChatCompletion)

    async def _stream(self, body: dict[str, Any]) -> AsyncIterator[ChatCompletionChunk]:
        try:
            async with self._client._http.stream(
                "POST", "/v1/chat/completions", json=body
            ) as r:
                raise_for_response(r, expected_text=True)
                async for payload in _aiter_sse(r):
                    with contextlib.suppress(ValueError):
                        yield ChatCompletionChunk.model_validate(json.loads(payload))
        except httpx.ConnectError as exc:
            raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc


class _AsyncCompletions(AsyncResource):
    async def create(
        self,
        *,
        model: str,
        prompt: str,
        stream: bool = False,
        **extra: Any,
    ) -> TextCompletion | AsyncIterator[TextCompletionChunk]:
        _warn_if_not_peer_id(model)
        body = _build_completion_body(model, prompt, stream=stream, extra=extra)
        if stream:
            return self._stream(body)
        return await self._post("/v1/completions", body=body, parse=TextCompletion)

    async def _stream(self, body: dict[str, Any]) -> AsyncIterator[TextCompletionChunk]:
        try:
            async with self._client._http.stream(
                "POST", "/v1/completions", json=body
            ) as r:
                raise_for_response(r, expected_text=True)
                async for payload in _aiter_sse(r):
                    with contextlib.suppress(ValueError):
                        yield TextCompletionChunk.model_validate(json.loads(payload))
        except httpx.ConnectError as exc:
            raise wrap_connection_error(exc, base_url=self._client.gateway_url) from exc


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
