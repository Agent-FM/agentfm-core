"""agentfm — Official Python SDK for the AgentFM peer-to-peer compute mesh."""

from __future__ import annotations

from ._version import __version__
from ._warnings import AgentFMRoutingWarning
from .artifacts import ArtifactManager
from .async_client import AsyncAgentFMClient
from .client import AgentFMClient
from .crypto import SwarmKey
from .daemon import LocalMeshGateway
from .exceptions import (
    AgentFMError,
    GatewayConnectionError,
    GatewayInternalError,
    InvalidRequestError,
    MeshOverloadedError,
    ModelNotFoundError,
    WorkerNotFoundError,
    WorkerUnreachableError,
)
from .models import (
    PeerID,
    ScatterResult,
    TaskChunk,
    TaskResult,
    WorkerProfile,
)
from .webhook import WebhookPayload, WebhookReceiver

__all__ = [
    "AgentFMClient",
    "AgentFMError",
    "AgentFMRoutingWarning",
    "ArtifactManager",
    "AsyncAgentFMClient",
    "GatewayConnectionError",
    "GatewayInternalError",
    "InvalidRequestError",
    "LocalMeshGateway",
    "MeshOverloadedError",
    "ModelNotFoundError",
    "PeerID",
    "ScatterResult",
    "SwarmKey",
    "TaskChunk",
    "TaskResult",
    "WebhookPayload",
    "WebhookReceiver",
    "WorkerNotFoundError",
    "WorkerProfile",
    "WorkerUnreachableError",
    "__version__",
]
