"""Pydantic models for the AgentFM API surface.

JSON field names follow the Go gateway's snake_case wire format directly
(``peer_id``, ``cpu_usage_pct`` etc) so no aliasing is needed for ingestion.
The previous SDK used PascalCase aliases by mistake; everything would
silently fail to populate against the real gateway.
"""

from __future__ import annotations

from pathlib import Path
from typing import Literal, NewType

from pydantic import BaseModel, ConfigDict, Field

# ---------------------------------------------------------------------------
# Identifier types
# ---------------------------------------------------------------------------

PeerID = NewType("PeerID", str)
"""Cryptographically unique libp2p peer identifier (e.g. ``12D3KooW...``).

This is the only worker identifier that is verifiable. Use it for dispatch.
``agent_name`` and ``model`` are user-supplied labels with no uniqueness or
authenticity guarantee.
"""


# ---------------------------------------------------------------------------
# Worker telemetry
# ---------------------------------------------------------------------------


class WorkerProfile(BaseModel):
    """An advertised worker on the mesh, as seen by ``/api/workers``."""

    model_config = ConfigDict(populate_by_name=True, extra="ignore")

    peer_id: PeerID = Field(description="libp2p peer identifier (cryptographically unique)")
    author: str = Field(default="Anonymous", description="Operator-supplied author handle")
    name: str = Field(default="", description="Operator-supplied agent name")
    status: str = Field(default="UNKNOWN", description="AVAILABLE, BUSY, etc.")
    hardware: str = Field(default="", description="Rendered hardware string")
    description: str = Field(default="", description="Operator-supplied agent description")

    cpu_usage_pct: float = Field(default=0.0, ge=0.0)
    ram_free_gb: float = Field(default=0.0, ge=0.0)
    current_tasks: int = Field(default=0, ge=0)
    max_tasks: int = Field(default=0, ge=0)

    has_gpu: bool = Field(default=False)
    gpu_used_gb: float = Field(default=0.0, ge=0.0)
    gpu_total_gb: float = Field(default=0.0, ge=0.0)
    gpu_usage_pct: float = Field(default=0.0, ge=0.0)

    @property
    def agent_name(self) -> str:
        """Backwards-compatible alias for ``name``."""
        return self.name

    @property
    def model(self) -> str:
        """The engine string parsed out of ``hardware``.

        ``/api/workers`` doesn't expose ``model`` as a top-level field; the Go
        side renders it inline as ``"<model> (CPU: N Cores)"`` or
        ``"<model> (GPU VRAM: x/y GB)"``. We split on the first ``" ("``.
        Empty string if the hardware field is unparseable.
        """
        if not self.hardware:
            return ""
        idx = self.hardware.find(" (")
        return self.hardware[:idx] if idx > 0 else self.hardware

    @property
    def is_available(self) -> bool:
        return self.status.upper() == "AVAILABLE"

    @property
    def load_ratio(self) -> float:
        if self.max_tasks <= 0:
            return 0.0
        return self.current_tasks / self.max_tasks


class WorkersResponse(BaseModel):
    """Envelope returned by ``GET /api/workers``."""

    success: bool = True
    agents: list[WorkerProfile] = Field(default_factory=list)


# ---------------------------------------------------------------------------
# Task execution
# ---------------------------------------------------------------------------


class TaskChunk(BaseModel):
    """One unit of streamed output from a running task.

    ``kind == "text"`` is normal stdout. ``kind == "marker"`` is an internal
    AgentFM sentinel that the SDK already handled (e.g. artifact-incoming
    notification). Most callers only care about ``"text"``.
    """

    model_config = ConfigDict(frozen=True)

    text: str
    kind: Literal["text", "marker"] = "text"


class TaskResult(BaseModel):
    """The final outcome of a non-streaming task."""

    model_config = ConfigDict(arbitrary_types_allowed=True)

    worker_id: PeerID
    text: str
    artifacts: list[Path] = Field(default_factory=list)
    duration_seconds: float = 0.0


class AsyncTaskAck(BaseModel):
    """Server response to ``POST /api/execute/async``."""

    task_id: str
    status: str = "queued"
    message: str = ""


class ScatterResult(BaseModel):
    """One slot of a scatter/gather batch."""

    model_config = ConfigDict(arbitrary_types_allowed=True)

    prompt: str
    worker_id: PeerID
    status: Literal["success", "failed"]
    text: str = ""
    artifacts: list[Path] = Field(default_factory=list)
    error: str | None = None


__all__ = [
    "AsyncTaskAck",
    "PeerID",
    "ScatterResult",
    "TaskChunk",
    "TaskResult",
    "WorkerProfile",
    "WorkersResponse",
]
