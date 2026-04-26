"""Zip-handling for artifacts the Go gateway drops into a watch directory.

Hardened vs the previous implementation:

* Zip-slip safe: rejects any entry whose resolved path escapes the extraction
  root (the classic CVE-2018-1002200 family).
* Polls atomically: detects a stable file size before declaring a zip
  fully written.
* All operations are pure functions / a small class with no global state.
"""

from __future__ import annotations

import logging
import time
import zipfile
from pathlib import Path

from .exceptions import ArtifactError

_log = logging.getLogger(__name__)

_DEFAULT_WATCH = "agentfm_artifacts"
_DEFAULT_EXTRACT = "agentfm_artifacts"

# Cap the total uncompressed bytes a single zip may produce. Defends against
# a malicious zip-bomb (high compression ratio, e.g. 1KB → 4GB) sent by a
# misbehaving worker. 1 GiB is generous for legitimate AgentFM artifact use
# (images, models, intermediate outputs); raise via ArtifactManager arg if
# you genuinely need bigger.
DEFAULT_MAX_EXTRACT_BYTES = 1 * 1024 * 1024 * 1024  # 1 GiB

# Per-chunk read size for the streaming copy. 64 KiB is a good balance
# between syscall overhead and RAM footprint.
_COPY_CHUNK_BYTES = 64 * 1024


class ArtifactManager:
    """Detects and extracts artifact zip files written by the Go gateway."""

    def __init__(
        self,
        watch_dir: str | Path = _DEFAULT_WATCH,
        extract_dir: str | Path = _DEFAULT_EXTRACT,
        max_extract_bytes: int = DEFAULT_MAX_EXTRACT_BYTES,
    ) -> None:
        self.watch_dir = Path(watch_dir).resolve()
        self.extract_dir = Path(extract_dir).resolve()
        self.max_extract_bytes = max_extract_bytes

    # -- discovery -----------------------------------------------------------

    def latest_zip(self) -> Path | None:
        """Return the most recently modified ``.zip`` in the watch dir, or None."""
        if not self.watch_dir.exists():
            return None
        candidates = [p for p in self.watch_dir.glob("*.zip") if p.is_file()]
        if not candidates:
            return None
        return max(candidates, key=lambda p: p.stat().st_mtime)

    def wait_for_new_zip(
        self,
        since: float,
        timeout: float = 120.0,
        stable_for: float = 1.0,
        poll_interval: float = 0.5,
    ) -> Path | None:
        """Block until a fresh zip appears and stops growing.

        ``since`` is a unix timestamp (seconds); only zips with ``mtime >
        since`` count. Returns ``None`` on timeout.
        """
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            zip_path = self.latest_zip()
            if zip_path is not None and zip_path.stat().st_mtime > since:
                size = zip_path.stat().st_size
                time.sleep(stable_for)
                if size > 0 and zip_path.stat().st_size == size:
                    return zip_path
            time.sleep(poll_interval)
        return None

    # -- extraction ----------------------------------------------------------

    def extract(self, zip_path: Path) -> list[Path]:
        """Extract a zip into ``extract_dir``.

        Defenses applied:
          * Zip-slip: any entry whose resolved path escapes ``extract_dir``
            triggers :class:`ArtifactError` and aborts extraction.
          * Zip-bomb: each entry is streamed in 64 KiB chunks rather than
            ``read()``-into-RAM. The cumulative uncompressed byte count is
            capped at ``self.max_extract_bytes``; a zip that would exceed
            the cap aborts mid-extraction.
        """
        if not zip_path.exists():
            raise ArtifactError(f"zip not found: {zip_path}")
        self.extract_dir.mkdir(parents=True, exist_ok=True)
        root = self.extract_dir.resolve()
        out: list[Path] = []
        bytes_written = 0
        try:
            with zipfile.ZipFile(zip_path, "r") as zf:
                for info in zf.infolist():
                    target = (root / info.filename).resolve()
                    if not _is_within(root, target):
                        raise ArtifactError(
                            f"refusing to extract '{info.filename}': escapes {root}"
                        )
                    if info.is_dir():
                        target.mkdir(parents=True, exist_ok=True)
                        continue
                    target.parent.mkdir(parents=True, exist_ok=True)
                    bytes_written += _stream_member(
                        zf, info, target, self.max_extract_bytes - bytes_written
                    )
                    out.append(target)
        except zipfile.BadZipFile as exc:
            raise ArtifactError(f"corrupt zip: {zip_path}") from exc
        return out

    def cleanup(self, zip_path: Path) -> None:
        """Best-effort removal of a zip after successful extraction."""
        try:
            zip_path.unlink(missing_ok=True)
        except OSError:
            _log.warning("failed to delete %s", zip_path, exc_info=True)

    def collect_since(self, since: float, timeout: float = 120.0) -> list[Path]:
        """Wait for a fresh zip after ``since``, extract it, and clean up.

        Returns the extracted file paths, or ``[]`` if no zip arrived within
        ``timeout``. Safe to call when ``watch_dir`` does not yet exist.

        UNSAFE under concurrent dispatch: two parallel ``tasks.run`` calls
        will race for "the latest zip" and clobber each other's artifacts.
        Prefer :meth:`collect_for_task` when a task ID is available.
        """
        if not self.watch_dir.exists():
            return []
        zip_path = self.wait_for_new_zip(since=since, timeout=timeout)
        if zip_path is None:
            return []
        files = self.extract(zip_path)
        self.cleanup(zip_path)
        return files

    def wait_for_task_zip(
        self,
        task_id: str,
        timeout: float = 120.0,
        stable_for: float = 1.0,
        poll_interval: float = 0.5,
    ) -> Path | None:
        """Block until ``<task_id>.zip`` appears in ``watch_dir`` and stops growing."""
        target = self.watch_dir / f"{task_id}.zip"
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if target.exists() and target.is_file():
                size = target.stat().st_size
                time.sleep(stable_for)
                if size > 0 and target.stat().st_size == size:
                    return target
            time.sleep(poll_interval)
        return None

    def collect_for_task(self, task_id: str, timeout: float = 120.0) -> list[Path]:
        """Wait for the worker's ``<task_id>.zip``, extract it, and clean up.

        Concurrency-safe: each call polls for a specific filename, so parallel
        ``tasks.run`` invocations cannot steal each other's artifacts. Returns
        ``[]`` if no matching zip arrives within ``timeout`` or if
        ``watch_dir`` does not exist.
        """
        if not self.watch_dir.exists() or not task_id:
            return []
        zip_path = self.wait_for_task_zip(task_id, timeout=timeout)
        if zip_path is None:
            return []
        files = self.extract(zip_path)
        self.cleanup(zip_path)
        return files


def _is_within(root: Path, target: Path) -> bool:
    """True iff ``target`` resolves to a descendant of ``root``."""
    try:
        target.relative_to(root)
    except ValueError:
        return False
    return True


def _stream_member(
    zf: zipfile.ZipFile,
    info: zipfile.ZipInfo,
    target: Path,
    remaining_budget: int,
) -> int:
    """Stream-copy a single zip entry to disk in 64 KiB chunks, enforcing budget.

    Returns bytes actually written. Raises :class:`ArtifactError` if the
    cumulative budget is exhausted by this entry.
    """
    if remaining_budget <= 0:
        raise ArtifactError(
            f"refusing to extract '{info.filename}': total uncompressed "
            "size exceeds max_extract_bytes"
        )
    written = 0
    with zf.open(info, "r") as src, target.open("wb") as dst:
        while True:
            chunk = src.read(_COPY_CHUNK_BYTES)
            if not chunk:
                break
            written += len(chunk)
            if written > remaining_budget:
                # Stop and clean up. The partial file remains on disk; the
                # caller's exception handler is responsible for surfacing the
                # error. We don't delete because the partial may itself be
                # useful diagnostic.
                raise ArtifactError(
                    f"refusing to extract '{info.filename}': total uncompressed "
                    f"size exceeds max_extract_bytes after {written} bytes"
                )
            dst.write(chunk)
    return written


__all__ = ["DEFAULT_MAX_EXTRACT_BYTES", "ArtifactManager"]
