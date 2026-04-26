"""Verify tasks.run sends a task_id and harvests artifacts by basename.

Regression for the CL-1 audit finding: pre-fix, parallel tasks.run() calls
raced on `latest by mtime` and clobbered each other's artifacts. The fix
mints a UUID per dispatch, sends it on the wire, and polls for the
specific `<task_id>.zip` filename.
"""

from __future__ import annotations

import json
import zipfile
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import httpx
import pytest
import respx

from agentfm import AgentFMClient

pytestmark = pytest.mark.integration


def _make_zip(path: Path, members: dict[str, bytes]) -> Path:
    with zipfile.ZipFile(path, "w") as zf:
        for name, data in members.items():
            zf.writestr(name, data)
    return path


def test_tasks_run_sends_task_id_in_payload(
    gateway_url: str, mock_gateway: respx.MockRouter, tmp_path: Path
):
    """The /api/execute body must contain a non-empty task_id."""
    captured: list[dict] = []

    def respond(request: httpx.Request) -> httpx.Response:
        captured.append(json.loads(request.content))
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    with AgentFMClient(gateway_url=gateway_url, artifacts_dir=tmp_path) as client:
        client.tasks.run(worker_id="12D3KooWAlpha", prompt="hi", artifact_timeout=0.1)

    assert len(captured) == 1
    body = captured[0]
    assert body["worker_id"] == "12D3KooWAlpha"
    assert body["prompt"] == "hi"
    assert isinstance(body.get("task_id"), str) and body["task_id"], (
        "tasks.run must send a non-empty task_id"
    )
    assert body["task_id"].startswith("task_")


def test_tasks_run_harvests_artifact_with_matching_basename(
    gateway_url: str, mock_gateway: respx.MockRouter, tmp_path: Path
):
    """When a zip arrives keyed by the dispatched task_id, run() returns it."""
    captured_task_id: list[str] = []

    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        captured_task_id.append(body["task_id"])
        # Drop the matching zip into the watch dir so collect_for_task picks it up.
        _make_zip(tmp_path / f"{body['task_id']}.zip", {"out.txt": b"hi"})
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    with AgentFMClient(gateway_url=gateway_url, artifacts_dir=tmp_path) as client:
        result = client.tasks.run(worker_id="12D3KooWAlpha", prompt="hi", artifact_timeout=5.0)

    assert {p.name for p in result.artifacts} == {"out.txt"}
    assert (tmp_path / f"{captured_task_id[0]}.zip").exists() is False, "zip should be cleaned up"


def test_concurrent_tasks_run_dont_steal_each_others_artifacts(
    gateway_url: str, mock_gateway: respx.MockRouter, tmp_path: Path
):
    """The whole point of CL-1: parallel runs must keep their artifacts apart."""

    def respond(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        # Each call produces a uniquely-keyed zip with a uniquely-named member.
        _make_zip(tmp_path / f"{body['task_id']}.zip", {f"{body['prompt']}.txt": body["prompt"].encode()})
        return httpx.Response(200, content=b"ok\n", headers={"Content-Type": "text/plain"})

    mock_gateway.post("/api/execute").mock(side_effect=respond)

    with (
        AgentFMClient(gateway_url=gateway_url, artifacts_dir=tmp_path) as client,
        ThreadPoolExecutor(max_workers=4) as ex,
    ):
        futures = [
            ex.submit(client.tasks.run, worker_id="12D3KooWX", prompt=f"prompt-{i}", artifact_timeout=5.0)
            for i in range(4)
        ]
        results = [f.result(timeout=10.0) for f in futures]

    artifact_names = sorted(p.name for r in results for p in r.artifacts)
    assert artifact_names == sorted(f"prompt-{i}.txt" for i in range(4)), (
        f"expected each task to recover its own artifact; got {artifact_names}"
    )
    # Every result has exactly one artifact, with the prompt-matching name.
    for r in results:
        assert len(r.artifacts) == 1, "no task should have stolen / lost artifacts"
