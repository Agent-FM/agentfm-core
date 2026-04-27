"""Warning classes shared across the SDK.

Lives outside :mod:`agentfm.openai` so that ``import agentfm`` does not pay
the cost of importing the OpenAI namespace and its httpx-backed resources.
"""

from __future__ import annotations

# stacklevel=3 attributes routing warnings to the user's `create()` line
# rather than to internal SDK frames. Stack at warn time:
#   1) warn_if_not_peer_id (the warn() call site)
#   2) namespace.create
#   3) user's call (this is what the user sees in their IDE)
ROUTING_WARNING_STACKLEVEL = 3


class AgentFMRoutingWarning(UserWarning):
    """Emitted when the caller passes a non-peer-id ``model`` to /v1/* endpoints."""


__all__ = ["ROUTING_WARNING_STACKLEVEL", "AgentFMRoutingWarning"]
