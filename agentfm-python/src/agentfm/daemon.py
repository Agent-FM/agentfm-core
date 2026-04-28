"""Spawn an ephemeral ``agentfm -mode api`` Go daemon for the lifetime of a context.

Hardened vs the previous implementation:

* In non-debug mode, subprocess output is captured to a rotating log file
  rather than thrown away (so post-mortem diagnostics are possible).
* Startup-readiness check uses the SDK's own ``ping()`` rather than a
  hand-rolled requests loop.
* Returns a usable ``url`` attribute so callers don't have to format ports.
"""

from __future__ import annotations

import contextlib
import logging
import os
import shlex
import shutil
import subprocess
import time
from pathlib import Path
from types import TracebackType
from typing import IO

import httpx

from .exceptions import GatewayConnectionError

_log = logging.getLogger(__name__)


class LocalMeshGateway:
    """Spawn a local ``agentfm -mode api`` Go daemon for the duration of a ``with`` block.

    Example::

        with LocalMeshGateway(port=8080) as gw:
            client = AgentFMClient(gateway_url=gw.url)
            ...
    """

    def __init__(
        self,
        *,
        binary_path: str = "agentfm",
        port: int = 8080,
        swarm_key: str | Path | None = None,
        bootstrap: str | None = None,
        debug: bool = False,
        log_file: str | Path | None = None,
        startup_timeout: float = 15.0,
        api_key: str | None = None,
    ) -> None:
        self.binary_path = binary_path
        self.port = port
        self.swarm_key = Path(swarm_key) if swarm_key is not None else None
        self.bootstrap = bootstrap
        self.debug = debug
        self.startup_timeout = startup_timeout
        self.log_file = Path(log_file) if log_file is not None else None
        self.process: subprocess.Popen[bytes] | None = None
        self._log_handle: IO[bytes] | None = None
        self.url = f"http://127.0.0.1:{self.port}"
        # api_key is forwarded as Authorization: Bearer ... on the readiness
        # probe so a gateway with AGENTFM_API_KEYS set doesn't reject the
        # probe with 401. Falls back to AGENTFM_API_KEY env var if unset.
        self.api_key = api_key if api_key is not None else os.environ.get("AGENTFM_API_KEY") or None

    # -- context-manager protocol -------------------------------------------

    def __enter__(self) -> LocalMeshGateway:
        self.start()
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        self.stop()

    # -- explicit lifecycle -------------------------------------------------

    def start(self) -> None:
        # Refuse a re-entrant start: a prior call holds self.process and
        # silently overwriting it would leak a subprocess + its log handle.
        if self.process is not None:
            raise RuntimeError(
                "LocalMeshGateway.start() called twice; call stop() first"
            )

        resolved = shutil.which(self.binary_path)
        if not resolved:
            raise FileNotFoundError(
                f"agentfm binary not found: {self.binary_path!r}. "
                "Install it (see https://agentfm.net) or pass an absolute path."
            )

        cmd = [resolved, "-mode", "api", "-apiport", str(self.port)]
        if self.swarm_key is not None:
            if not self.swarm_key.exists():
                raise FileNotFoundError(f"swarm key not found: {self.swarm_key}")
            cmd.extend(["-swarmkey", str(self.swarm_key)])
        if self.bootstrap:
            cmd.extend(["-bootstrap", self.bootstrap])

        if self.debug:
            stdout: int | IO[bytes] = subprocess.STDOUT
            stderr: int | IO[bytes] | None = None
            self._log_handle = None
        else:
            log_path = self.log_file or Path(f".agentfm-gateway-{self.port}.log")
            self._log_handle = log_path.open("ab")  # closed by stop()/_cleanup_log
            stdout = self._log_handle
            stderr = self._log_handle

        _log.info("starting %s", shlex.join(cmd))
        try:
            self.process = subprocess.Popen(
                cmd,
                stdout=stdout,
                stderr=stderr,
                env=os.environ.copy(),
            )
        except OSError:
            # Popen can fail with PermissionError, OSError (ENOENT race), etc.
            # Without this except the log handle would leak until GC.
            self._cleanup_log()
            raise

        try:
            deadline = time.monotonic() + self.startup_timeout
            while time.monotonic() < deadline:
                if self._is_ready():
                    _log.info("gateway online at %s", self.url)
                    return
                if self.process.poll() is not None:
                    rc = self.process.returncode
                    self._cleanup_log()
                    self.process = None
                    raise GatewayConnectionError(
                        f"agentfm exited early with code {rc}"
                    )
                time.sleep(0.25)
            # timed out
            self.stop()
            raise GatewayConnectionError(
                f"agentfm did not become ready on {self.url} within {self.startup_timeout}s"
            )
        except BaseException:
            # Anything raised after Popen succeeded — including KeyboardInterrupt
            # from a slow startup — must not leak the subprocess.
            if self.process is not None:
                self.stop()
            raise

    def stop(self) -> None:
        if self.process is None:
            return
        proc = self.process
        try:
            proc.terminate()
            try:
                proc.wait(timeout=3.0)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=3.0)
        finally:
            self.process = None
            self._cleanup_log()

    def _is_ready(self) -> bool:
        # Prefer /health (always unauthenticated; cheaper than /api/workers
        # which does a map walk under lock). Falls back to /api/workers if
        # the gateway is too old to expose /health.
        headers = (
            {"Authorization": f"Bearer {self.api_key}"}
            if self.api_key
            else {}
        )
        for path in ("/health", "/api/workers"):
            try:
                r = httpx.get(f"{self.url}{path}", timeout=1.0, headers=headers)
            except httpx.HTTPError:
                continue
            if r.status_code == 200:
                return True
        return False

    def _cleanup_log(self) -> None:
        if self._log_handle is not None:
            with contextlib.suppress(OSError):
                self._log_handle.close()
            self._log_handle = None


__all__ = ["LocalMeshGateway"]
