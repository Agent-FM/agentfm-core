"""Pydantic models mirroring the gateway's ``/v1/*`` OpenAI-compatible endpoints.

These intentionally accept extra fields (``extra="allow"``) so when the Go side
adds new ``agentfm_*`` extension fields or OpenAI adds new spec fields, decoding
keeps working without an SDK release.
"""

from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field

# ---------------------------------------------------------------------------
# Requests
# ---------------------------------------------------------------------------


class ChatMessage(BaseModel):
    """One message in a chat completion request."""

    model_config = ConfigDict(extra="allow")

    role: str
    content: str


# ---------------------------------------------------------------------------
# Non-streaming responses
# ---------------------------------------------------------------------------


class Usage(BaseModel):
    model_config = ConfigDict(extra="allow")

    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0


class ChatChoice(BaseModel):
    model_config = ConfigDict(extra="allow")

    index: int = 0
    message: ChatMessage
    finish_reason: str | None = None


class ChatCompletion(BaseModel):
    model_config = ConfigDict(extra="allow")

    id: str
    object: Literal["chat.completion"] = "chat.completion"
    created: int
    model: str
    choices: list[ChatChoice] = Field(default_factory=list)
    usage: Usage = Field(default_factory=Usage)


class CompletionChoice(BaseModel):
    model_config = ConfigDict(extra="allow")

    index: int = 0
    text: str
    finish_reason: str | None = None


class TextCompletion(BaseModel):
    model_config = ConfigDict(extra="allow")

    id: str
    object: Literal["text_completion"] = "text_completion"
    created: int
    model: str
    choices: list[CompletionChoice] = Field(default_factory=list)
    usage: Usage = Field(default_factory=Usage)


# ---------------------------------------------------------------------------
# Streaming responses (SSE chunks)
# ---------------------------------------------------------------------------


class ChatChoiceDelta(BaseModel):
    model_config = ConfigDict(extra="allow")

    role: str | None = None
    content: str | None = None


class ChatChunkChoice(BaseModel):
    model_config = ConfigDict(extra="allow")

    index: int = 0
    delta: ChatChoiceDelta = Field(default_factory=ChatChoiceDelta)
    finish_reason: str | None = None


class ChatCompletionChunk(BaseModel):
    model_config = ConfigDict(extra="allow")

    id: str
    object: Literal["chat.completion.chunk"] = "chat.completion.chunk"
    created: int
    model: str
    choices: list[ChatChunkChoice] = Field(default_factory=list)


class TextCompletionChunkChoice(BaseModel):
    model_config = ConfigDict(extra="allow")

    index: int = 0
    text: str = ""
    finish_reason: str | None = None


class TextCompletionChunk(BaseModel):
    model_config = ConfigDict(extra="allow")

    id: str
    object: Literal["text_completion"] = "text_completion"
    created: int
    model: str
    choices: list[TextCompletionChunkChoice] = Field(default_factory=list)


# ---------------------------------------------------------------------------
# /v1/models
# ---------------------------------------------------------------------------


class ModelEntry(BaseModel):
    """One entry in ``/v1/models``.

    The Go gateway publishes one entry per peer (the ``id`` is the peer ID).
    The ``agentfm_*`` extension fields carry per-peer state (status, hardware,
    GPU, load) so a client can disambiguate without a separate ``/api/workers``
    call.
    """

    model_config = ConfigDict(extra="allow")

    id: str
    object: Literal["model"] = "model"
    created: int
    owned_by: str = "agentfm"
    description: str | None = None

    # AgentFM extension fields (gateway-emitted)
    agentfm_name: str | None = None
    agentfm_engine: str | None = None
    agentfm_status: str | None = None
    agentfm_hardware: str | None = None
    agentfm_current_tasks: int | None = None
    agentfm_max_tasks: int | None = None
    agentfm_cpu_usage_pct: float | None = None
    agentfm_ram_free_gb: float | None = None
    agentfm_has_gpu: bool | None = None
    agentfm_gpu_used_gb: float | None = None
    agentfm_gpu_total_gb: float | None = None
    agentfm_gpu_usage_pct: float | None = None


class ModelsList(BaseModel):
    object: Literal["list"] = "list"
    data: list[ModelEntry] = Field(default_factory=list)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def coerce_messages(messages: list[ChatMessage] | list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Accept either ``ChatMessage`` instances or plain dicts; emit dicts."""
    out: list[dict[str, Any]] = []
    for m in messages:
        if isinstance(m, ChatMessage):
            out.append(m.model_dump(exclude_none=True))
        elif isinstance(m, dict):
            out.append(m)
        else:
            raise TypeError(f"messages[*] must be ChatMessage or dict, got {type(m).__name__}")
    return out


__all__ = [
    "ChatChoice",
    "ChatChoiceDelta",
    "ChatChunkChoice",
    "ChatCompletion",
    "ChatCompletionChunk",
    "ChatMessage",
    "CompletionChoice",
    "ModelEntry",
    "ModelsList",
    "TextCompletion",
    "TextCompletionChunk",
    "TextCompletionChunkChoice",
    "Usage",
    "coerce_messages",
]
