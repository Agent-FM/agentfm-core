"""Warning classes shared across the SDK.

Lives outside :mod:`agentfm.openai` so that ``import agentfm`` does not pay
the cost of importing the OpenAI namespace and its httpx-backed resources.
"""

from __future__ import annotations


class AgentFMRoutingWarning(UserWarning):
    """Emitted when the caller passes a non-peer-id ``model`` to /v1/* endpoints."""


__all__ = ["AgentFMRoutingWarning"]
