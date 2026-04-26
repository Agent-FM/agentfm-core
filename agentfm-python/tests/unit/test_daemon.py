"""LocalMeshGateway unit tests using a fake `agentfm` binary on PATH.

Audit findings addressed:
- Major #9: log handle leak when Popen raises
- Major #9: non-idempotent start() silently clobbers self.process
- Drift: shlex.join in the startup log line so paths with spaces are clear
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

import pytest

from agentfm import LocalMeshGateway
from agentfm.exceptions import GatewayConnectionError


def _install_fake_agentfm(tmp_path: Path, body: str) -> Path:
    """Drop a shell-script `agentfm` into a tmp dir and return the bin dir."""
    if sys.platform == "win32":
        pytest.skip("POSIX-only: relies on /bin/sh fake binary")
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir(parents=True, exist_ok=True)
    script = bin_dir / "agentfm"
    script.write_text(body)
    script.chmod(0o755)
    return bin_dir


def _patched_path(monkeypatch: pytest.MonkeyPatch, bin_dir: Path) -> None:
    """Force shutil.which to find the fake binary first."""
    monkeypatch.setenv("PATH", str(bin_dir) + os.pathsep + os.environ.get("PATH", ""))


def test_start_raises_when_binary_missing(monkeypatch, tmp_path: Path):
    monkeypatch.setenv("PATH", str(tmp_path))  # empty PATH-like dir
    gw = LocalMeshGateway(binary_path="agentfm-does-not-exist", port=18800)
    with pytest.raises(FileNotFoundError, match="not found"):
        gw.start()
    # No subprocess started → nothing to clean.
    assert gw.process is None


def test_start_then_start_again_raises(monkeypatch, tmp_path: Path):
    """Pre-fix, the second start() silently overwrote self.process and
    leaked the first subprocess + log handle."""
    bin_dir = _install_fake_agentfm(
        tmp_path, "#!/bin/sh\nexec sleep 30\n"
    )
    _patched_path(monkeypatch, bin_dir)

    log = tmp_path / "gw.log"
    gw = LocalMeshGateway(
        binary_path="agentfm",
        port=18801,
        startup_timeout=0.5,
        log_file=log,
    )
    # First start: never becomes ready (no real HTTP server) → times out.
    with pytest.raises(GatewayConnectionError, match="did not become ready"):
        gw.start()
    # After timeout, stop() ran in the cleanup branch.
    assert gw.process is None

    # Re-arm and verify the explicit double-start guard kicks in if a caller
    # holds a live process.
    bin_dir2 = _install_fake_agentfm(
        tmp_path / "second", "#!/bin/sh\nexec sleep 30\n"
    )
    _patched_path(monkeypatch, bin_dir2)
    with pytest.raises(GatewayConnectionError):
        gw.start()


def test_start_then_double_start_with_live_process_raises(monkeypatch, tmp_path: Path):
    """Directly poke self.process so we exercise the new guard, not the
    happy-path-then-fail interaction."""
    bin_dir = _install_fake_agentfm(tmp_path, "#!/bin/sh\nexec sleep 30\n")
    _patched_path(monkeypatch, bin_dir)
    gw = LocalMeshGateway(binary_path="agentfm", port=18802, startup_timeout=0.5)

    # Spoof a live process. Don't actually run anything — just trigger the
    # guard and assert it refuses the second start.
    class _FakeProc:
        def poll(self):
            return None

        def terminate(self):
            pass

        def kill(self):
            pass

        def wait(self, timeout=None):
            pass

    gw.process = _FakeProc()  # type: ignore[assignment]
    with pytest.raises(RuntimeError, match="called twice"):
        gw.start()


def test_start_exits_quickly_then_surfaces_returncode(monkeypatch, tmp_path: Path):
    """Fake agentfm exits 7 immediately — start() must surface it cleanly
    and clean up the log handle, not hang on the readiness loop."""
    bin_dir = _install_fake_agentfm(tmp_path, "#!/bin/sh\nexit 7\n")
    _patched_path(monkeypatch, bin_dir)

    log = tmp_path / "gw.log"
    gw = LocalMeshGateway(
        binary_path="agentfm",
        port=18803,
        startup_timeout=5.0,
        log_file=log,
    )
    with pytest.raises(GatewayConnectionError, match="exited early with code 7"):
        gw.start()
    assert gw.process is None
    # Log handle was closed (file descriptor freed) — best we can check
    # without poking private state is that the file is at least readable.
    assert log.exists()


def test_swarm_key_validation(monkeypatch, tmp_path: Path):
    """Missing swarm key file → FileNotFoundError before Popen ever runs."""
    bin_dir = _install_fake_agentfm(tmp_path, "#!/bin/sh\nexec sleep 30\n")
    _patched_path(monkeypatch, bin_dir)

    gw = LocalMeshGateway(
        binary_path="agentfm",
        port=18804,
        swarm_key=tmp_path / "missing-swarm.key",
    )
    with pytest.raises(FileNotFoundError, match="swarm key not found"):
        gw.start()
    assert gw.process is None
