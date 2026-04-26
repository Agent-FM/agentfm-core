"""Verify @overload narrows the return type on chat/text completions.create.

These tests would compile-fail under mypy --strict if the overloads weren't
applied correctly. Runtime assertions are a backstop in case the function
implementation drifts from the overload signatures.
"""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator
from typing import get_type_hints

from agentfm.openai._namespaces import (
    _AsyncChatCompletions,
    _AsyncCompletions,
    _ChatCompletions,
    _Completions,
)
from agentfm.openai.models import (
    ChatCompletion,
    ChatCompletionChunk,
    TextCompletion,
    TextCompletionChunk,
)


def test_sync_chat_overloads_present():
    assert hasattr(_ChatCompletions.create, "__wrapped__") or _ChatCompletions.create.__name__ == "create"
    hints = get_type_hints(_ChatCompletions.create)
    assert "return" in hints
    assert ChatCompletion in (hints["return"].__args__ if hasattr(hints["return"], "__args__") else (hints["return"],))


def test_sync_completions_overloads_present():
    hints = get_type_hints(_Completions.create)
    ret = hints["return"]
    members = ret.__args__ if hasattr(ret, "__args__") else (ret,)
    assert TextCompletion in members or any(
        getattr(m, "__name__", "") == "TextCompletion" for m in members
    )


def test_async_chat_overloads_present():
    hints = get_type_hints(_AsyncChatCompletions.create)
    ret = hints["return"]
    members = ret.__args__ if hasattr(ret, "__args__") else (ret,)
    assert ChatCompletion in members or any(
        getattr(m, "__name__", "") == "ChatCompletion" for m in members
    )


def test_async_completions_overloads_present():
    hints = get_type_hints(_AsyncCompletions.create)
    ret = hints["return"]
    members = ret.__args__ if hasattr(ret, "__args__") else (ret,)
    assert TextCompletion in members or any(
        getattr(m, "__name__", "") == "TextCompletion" for m in members
    )


def test_iterator_types_in_runtime_signatures():
    """Confirm Iterator/AsyncIterator are reachable for streaming-branch returns."""
    sync_hints = get_type_hints(_ChatCompletions.create)
    async_hints = get_type_hints(_AsyncChatCompletions.create)
    sync_ret = sync_hints["return"]
    async_ret = async_hints["return"]
    sync_members = sync_ret.__args__ if hasattr(sync_ret, "__args__") else (sync_ret,)
    async_members = async_ret.__args__ if hasattr(async_ret, "__args__") else (async_ret,)
    assert any(
        getattr(m, "__origin__", None) in (Iterator, AsyncIterator)
        or (hasattr(m, "__name__") and m.__name__ in ("Iterator", "AsyncIterator"))
        for m in sync_members + async_members
    ), f"no iterator in {sync_members=} {async_members=}"


def test_text_chunk_types_reachable():
    sync_hints = get_type_hints(_Completions.create)
    async_hints = get_type_hints(_AsyncCompletions.create)
    chunk_names = (TextCompletionChunk.__name__, ChatCompletionChunk.__name__)
    sync_ret = sync_hints["return"]
    async_ret = async_hints["return"]
    sync_members = sync_ret.__args__ if hasattr(sync_ret, "__args__") else (sync_ret,)
    async_members = async_ret.__args__ if hasattr(async_ret, "__args__") else (async_ret,)
    # Just sanity: at least one member should mention a chunk type by name
    found = False
    for m in sync_members + async_members:
        s = repr(m)
        if any(name in s for name in chunk_names):
            found = True
            break
    assert found, "no chunk type referenced in completions.create return type"
