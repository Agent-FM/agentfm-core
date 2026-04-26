"""OpenAI-compatible namespace for the AgentFM SDK.

Mirrors the gateway's ``/v1/models``, ``/v1/chat/completions``, and
``/v1/completions`` endpoints with typed responses.
"""

from __future__ import annotations

from .._warnings import AgentFMRoutingWarning
from ._namespaces import (
    AsyncOpenAINamespace,
    OpenAINamespace,
)
from .models import (
    ChatCompletion,
    ChatCompletionChunk,
    ChatMessage,
    ModelEntry,
    ModelsList,
    TextCompletion,
    TextCompletionChunk,
    Usage,
)

__all__ = [
    "AgentFMRoutingWarning",
    "AsyncOpenAINamespace",
    "ChatCompletion",
    "ChatCompletionChunk",
    "ChatMessage",
    "ModelEntry",
    "ModelsList",
    "OpenAINamespace",
    "TextCompletion",
    "TextCompletionChunk",
    "Usage",
]
