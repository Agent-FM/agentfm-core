from __future__ import annotations

import pytest
from pydantic import ValidationError

from agentfm.models import WorkerProfile, WorkersResponse


def test_worker_profile_parses_real_gateway_shape():
    payload = {
        "peer_id": "12D3KooWX",
        "author": "alice",
        "name": "research-agent",
        "status": "AVAILABLE",
        "hardware": "llama3.2 (CPU: 12 Cores)",
        "description": "research bot",
        "cpu_usage_pct": 14.2,
        "ram_free_gb": 12.5,
        "current_tasks": 1,
        "max_tasks": 10,
        "has_gpu": False,
    }
    w = WorkerProfile.model_validate(payload)
    assert w.peer_id == "12D3KooWX"
    assert w.name == "research-agent"
    assert w.author == "alice"
    assert w.cpu_usage_pct == pytest.approx(14.2)
    assert w.is_available is True
    assert w.load_ratio == pytest.approx(0.1)
    # Backwards-compat alias
    assert w.agent_name == "research-agent"


def test_worker_profile_model_property_parses_hardware_string():
    w = WorkerProfile(peer_id="x", hardware="llama3.2 (CPU: 12 Cores)")
    assert w.model == "llama3.2"
    w2 = WorkerProfile(peer_id="y", hardware="flux (GPU VRAM: 6.4/24.0 GB)")
    assert w2.model == "flux"
    w3 = WorkerProfile(peer_id="z", hardware="")
    assert w3.model == ""


def test_worker_profile_rejects_negative_percentages():
    with pytest.raises(ValidationError):
        WorkerProfile(peer_id="x", cpu_usage_pct=-1.0)
    with pytest.raises(ValidationError):
        WorkerProfile(peer_id="x", current_tasks=-5)


def test_worker_profile_load_ratio_handles_zero_max_tasks():
    w = WorkerProfile(peer_id="x", current_tasks=3, max_tasks=0)
    assert w.load_ratio == 0.0


def test_workers_response_unwraps_envelope():
    resp = WorkersResponse.model_validate(
        {"success": True, "agents": [{"peer_id": "x", "max_tasks": 5}]}
    )
    assert resp.success is True
    assert len(resp.agents) == 1
    assert resp.agents[0].peer_id == "x"


def test_workers_response_handles_empty_envelope():
    resp = WorkersResponse.model_validate({"success": True, "agents": []})
    assert resp.agents == []
