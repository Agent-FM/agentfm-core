"""Shared fixtures: stubbed gateway responses for the SDK tests."""

from __future__ import annotations

from collections.abc import Generator
from typing import Any

import pytest
import respx

GATEWAY = "http://test-gateway"

SAMPLE_WORKERS_BODY: dict[str, Any] = {
    "success": True,
    "agents": [
        {
            "peer_id": "12D3KooWAlpha9XzHaaaa",
            "author": "alice",
            "name": "research-agent",
            "status": "AVAILABLE",
            "hardware": "llama3.2 (CPU: 12 Cores)",
            "description": "Multi-source research",
            "cpu_usage_pct": 14.2,
            "ram_free_gb": 12.5,
            "current_tasks": 1,
            "max_tasks": 10,
            "has_gpu": False,
            "gpu_used_gb": 0.0,
            "gpu_total_gb": 0.0,
            "gpu_usage_pct": 0.0,
        },
        {
            "peer_id": "12D3KooWBravo7Yqabbbb",
            "author": "bob",
            "name": "research-agent",
            "status": "BUSY",
            "hardware": "llama3.2 (GPU VRAM: 6.4/24.0 GB)",
            "description": "Bob's variant",
            "cpu_usage_pct": 92.0,
            "ram_free_gb": 2.1,
            "current_tasks": 8,
            "max_tasks": 10,
            "has_gpu": True,
            "gpu_used_gb": 6.4,
            "gpu_total_gb": 24.0,
            "gpu_usage_pct": 26.7,
        },
        {
            "peer_id": "12D3KooWCharlie3Mncccc",
            "author": "carol",
            "name": "image-gen",
            "status": "AVAILABLE",
            "hardware": "flux (GPU VRAM: 0.5/40.0 GB)",
            "description": "FLUX 1024x1024",
            "cpu_usage_pct": 5.0,
            "ram_free_gb": 28.0,
            "current_tasks": 0,
            "max_tasks": 4,
            "has_gpu": True,
            "gpu_used_gb": 0.5,
            "gpu_total_gb": 40.0,
            "gpu_usage_pct": 1.25,
        },
    ],
}

SAMPLE_MODELS_BODY: dict[str, Any] = {
    "object": "list",
    "data": [
        {
            "id": "12D3KooWAlpha9XzHaaaa",
            "object": "model",
            "created": 1745452800,
            "owned_by": "alice",
            "description": "research-agent · llama3.2 — Multi-source research",
            "agentfm_name": "research-agent",
            "agentfm_engine": "llama3.2",
            "agentfm_status": "AVAILABLE",
            "agentfm_hardware": "llama3.2 (CPU: 12 Cores)",
            "agentfm_current_tasks": 1,
            "agentfm_max_tasks": 10,
        },
    ],
}


@pytest.fixture
def gateway_url() -> str:
    return GATEWAY


@pytest.fixture
def mock_gateway() -> Generator[respx.MockRouter, None, None]:
    """Yields a respx router pre-loaded with the gateway base URL."""
    with respx.mock(base_url=GATEWAY, assert_all_called=False) as router:
        yield router


@pytest.fixture
def mock_workers(mock_gateway: respx.MockRouter) -> respx.MockRouter:
    mock_gateway.get("/api/workers").respond(json=SAMPLE_WORKERS_BODY)
    return mock_gateway


@pytest.fixture
def mock_models(mock_gateway: respx.MockRouter) -> respx.MockRouter:
    mock_gateway.get("/v1/models").respond(json=SAMPLE_MODELS_BODY)
    return mock_gateway
