from __future__ import annotations

import pytest

from agentfm.openai.models import (
    ChatCompletion,
    ChatCompletionChunk,
    ChatMessage,
    ModelEntry,
    ModelsList,
    coerce_messages,
)


def test_coerce_messages_accepts_dicts_and_models():
    out = coerce_messages(
        [
            {"role": "user", "content": "hi"},
            ChatMessage(role="assistant", content="yo"),
        ]
    )
    assert out == [
        {"role": "user", "content": "hi"},
        {"role": "assistant", "content": "yo"},
    ]


def test_coerce_messages_rejects_invalid_type():
    with pytest.raises(TypeError):
        coerce_messages(["just a string"])  # type: ignore[list-item]


def test_chat_completion_round_trip():
    raw = {
        "id": "chatcmpl-x",
        "object": "chat.completion",
        "created": 1745452800,
        "model": "12D3KooW...",
        "choices": [
            {"index": 0, "message": {"role": "assistant", "content": "hi"}, "finish_reason": "stop"}
        ],
        "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
    }
    cc = ChatCompletion.model_validate(raw)
    assert cc.id == "chatcmpl-x"
    assert cc.choices[0].message.content == "hi"
    assert cc.choices[0].finish_reason == "stop"


def test_chat_completion_chunk_accepts_unknown_fields():
    raw = {
        "id": "chatcmpl-x",
        "object": "chat.completion.chunk",
        "created": 1745452800,
        "model": "x",
        "choices": [{"index": 0, "delta": {"content": "h"}, "finish_reason": None}],
        "agentfm_extra_future_field": True,
    }
    chunk = ChatCompletionChunk.model_validate(raw)
    assert chunk.choices[0].delta.content == "h"


def test_models_list_with_agentfm_extension_fields():
    raw = {
        "object": "list",
        "data": [
            {
                "id": "12D3KooWX",
                "object": "model",
                "created": 1,
                "owned_by": "alice",
                "agentfm_name": "research-agent",
                "agentfm_engine": "llama3.2",
                "agentfm_status": "AVAILABLE",
                "agentfm_current_tasks": 1,
                "agentfm_max_tasks": 10,
            }
        ],
    }
    listing = ModelsList.model_validate(raw)
    assert listing.data[0].agentfm_engine == "llama3.2"
    assert listing.data[0].agentfm_status == "AVAILABLE"


def test_model_entry_minimal():
    e = ModelEntry(id="x", created=0)
    assert e.owned_by == "agentfm"
    assert e.agentfm_name is None
