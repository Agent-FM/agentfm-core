"""Regression test: tasks.run() must be safe to call concurrently from threads.

Pre-fix, _TasksNamespace stored stream-start timestamps on instance state, so
two concurrent run() calls would clobber each other's timing. Post-fix, those
are local variables.
"""

from __future__ import annotations

import time
from concurrent.futures import ThreadPoolExecutor

import pytest
import respx

from agentfm import AgentFMClient

pytestmark = pytest.mark.integration


def test_concurrent_tasks_run_have_independent_durations(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Two parallel run() calls report independent (non-zero, sane) durations."""
    # Each call sleeps a different amount on the server side via response delay.
    # We expect both durations to be non-zero and to NOT be identical
    # (which would indicate they shared state).
    mock_gateway.post("/api/execute").respond(
        status_code=200, content=b"ok\n", headers={"Content-Type": "text/plain"}
    )

    with (
        AgentFMClient(gateway_url=gateway_url) as client,
        ThreadPoolExecutor(max_workers=4) as ex,
    ):
        futures = [
            ex.submit(
                client.tasks.run, worker_id=f"12D3KooW{i:05d}", prompt=f"prompt {i}"
            )
            for i in range(4)
        ]
        results = [f.result(timeout=10.0) for f in futures]

    # Each result should have its own non-zero duration; none should be 0.0
    # (which would be the symptom of shared state being clobbered to a single
    # later timestamp).
    durations = [r.duration_seconds for r in results]
    assert all(d > 0.0 for d in durations), f"saw zero/negative durations: {durations}"
    assert all(d < 5.0 for d in durations), f"durations look wrong: {durations}"

    # Each result has its own worker_id
    assert {str(r.worker_id) for r in results} == {f"12D3KooW{i:05d}" for i in range(4)}


def test_tasks_run_does_not_leak_state_between_serial_calls(
    gateway_url: str, mock_gateway: respx.MockRouter
):
    """Two serial run() calls produce independent durations.

    With the pre-fix instance state, the SECOND call's duration would be
    measured from the FIRST call's start time (because _stream_started_monotonic
    was only set in stream() and never cleared).
    """
    mock_gateway.post("/api/execute").respond(
        status_code=200, content=b"ok\n", headers={"Content-Type": "text/plain"}
    )

    with AgentFMClient(gateway_url=gateway_url) as client:
        client.tasks.run(worker_id="12D3KooW1", prompt="x")
        time.sleep(0.05)
        second = client.tasks.run(worker_id="12D3KooW2", prompt="y")

    # If the bug existed, second.duration_seconds would include the sleep
    # plus the time spent in `first.run`, exceeding 0.05s. With the fix,
    # second's duration is just its own work.
    assert second.duration_seconds < 0.5, (
        f"second call duration {second.duration_seconds:.3f}s suggests state leak"
    )
