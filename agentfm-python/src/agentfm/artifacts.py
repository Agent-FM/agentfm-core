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


class ArtifactManager:
    """Detects and extracts artifact zip files written by the Go gateway."""

    def __init__(
        self,
        watch_dir: str | Path = _DEFAULT_WATCH,
        extract_dir: str | Path = _DEFAULT_EXTRACT,
    ) -> None:
        self.watch_dir = Path(watch_dir).resolve()
        self.extract_dir = Path(extract_dir).resolve()

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
        """Extract a zip into ``extract_dir``, rejecting path traversal."""
        if not zip_path.exists():
            raise ArtifactError(f"zip not found: {zip_path}")
        self.extract_dir.mkdir(parents=True, exist_ok=True)
        root = self.extract_dir.resolve()
        out: list[Path] = []
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
                    with zf.open(info, "r") as src, target.open("wb") as dst:
                        dst.write(src.read())
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
        """
        if not self.watch_dir.exists():
            return []
        zip_path = self.wait_for_new_zip(since=since, timeout=timeout)
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


__all__ = ["ArtifactManager"]
