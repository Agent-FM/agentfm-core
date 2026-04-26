from __future__ import annotations

import time
import zipfile
from pathlib import Path

import pytest

from agentfm.artifacts import ArtifactManager
from agentfm.exceptions import ArtifactError


def _make_zip(path: Path, members: dict[str, bytes]) -> Path:
    with zipfile.ZipFile(path, "w") as zf:
        for name, data in members.items():
            zf.writestr(name, data)
    return path


def test_extract_round_trip(tmp_path: Path):
    src = _make_zip(tmp_path / "in.zip", {"hello.txt": b"hi", "nested/world.txt": b"wow"})
    extract_dir = tmp_path / "out"
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=extract_dir)
    extracted = mgr.extract(src)
    assert {p.name for p in extracted} == {"hello.txt", "world.txt"}
    assert (extract_dir / "hello.txt").read_bytes() == b"hi"
    assert (extract_dir / "nested" / "world.txt").read_bytes() == b"wow"


def test_extract_rejects_zip_slip(tmp_path: Path):
    bad = tmp_path / "bad.zip"
    with zipfile.ZipFile(bad, "w") as zf:
        zf.writestr("../../escape.txt", b"pwned")
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=tmp_path / "out")
    with pytest.raises(ArtifactError, match="escapes"):
        mgr.extract(bad)
    assert not (tmp_path.parent.parent / "escape.txt").exists()


def test_extract_handles_corrupt_zip(tmp_path: Path):
    bad = tmp_path / "broken.zip"
    bad.write_bytes(b"not a zip at all")
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=tmp_path / "out")
    with pytest.raises(ArtifactError, match="corrupt"):
        mgr.extract(bad)


def test_extract_missing_zip_raises(tmp_path: Path):
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=tmp_path / "out")
    with pytest.raises(ArtifactError, match="not found"):
        mgr.extract(tmp_path / "nope.zip")


def test_latest_zip_returns_most_recent(tmp_path: Path):
    _make_zip(tmp_path / "a.zip", {"x": b"x"})
    time.sleep(0.05)
    p2 = _make_zip(tmp_path / "b.zip", {"y": b"y"})
    mgr = ArtifactManager(watch_dir=tmp_path)
    assert mgr.latest_zip() == p2


def test_latest_zip_returns_none_when_empty(tmp_path: Path):
    mgr = ArtifactManager(watch_dir=tmp_path)
    assert mgr.latest_zip() is None


def test_cleanup_is_idempotent(tmp_path: Path):
    p = _make_zip(tmp_path / "x.zip", {"a": b"a"})
    mgr = ArtifactManager(watch_dir=tmp_path)
    mgr.cleanup(p)
    assert not p.exists()
    mgr.cleanup(p)  # second call: no error


def test_collect_since_returns_extracted_files(tmp_path: Path):
    extract_dir = tmp_path / "out"
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=extract_dir)
    started_at = time.time() - 10.0
    zip_path = _make_zip(tmp_path / "task.zip", {"a.txt": b"hello"})
    extracted = mgr.collect_since(since=started_at, timeout=2.0)
    assert {p.name for p in extracted} == {"a.txt"}
    assert (extract_dir / "a.txt").read_bytes() == b"hello"
    assert not zip_path.exists(), "zip should be cleaned up after extract"


def test_collect_since_returns_empty_when_watch_dir_missing(tmp_path: Path):
    mgr = ArtifactManager(watch_dir=tmp_path / "nope", extract_dir=tmp_path / "out")
    assert mgr.collect_since(since=time.time(), timeout=0.1) == []


def test_collect_since_returns_empty_on_timeout(tmp_path: Path):
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=tmp_path / "out")
    assert mgr.collect_since(since=time.time(), timeout=0.1) == []


def test_collect_since_skips_old_zips(tmp_path: Path):
    _make_zip(tmp_path / "old.zip", {"old.txt": b"stale"})
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=tmp_path / "out")
    after_old = time.time()
    assert mgr.collect_since(since=after_old, timeout=0.1) == []


def test_collect_for_task_finds_specific_basename(tmp_path: Path):
    extract_dir = tmp_path / "out"
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=extract_dir)
    _make_zip(tmp_path / "task_abc123.zip", {"hello.txt": b"hi"})
    files = mgr.collect_for_task("task_abc123", timeout=2.0)
    assert {p.name for p in files} == {"hello.txt"}
    assert not (tmp_path / "task_abc123.zip").exists(), "zip should be cleaned up"


def test_collect_for_task_ignores_other_zips(tmp_path: Path):
    """Concurrency-safety: a parallel task's zip must not be returned."""
    extract_dir = tmp_path / "out"
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=extract_dir)
    _make_zip(tmp_path / "task_other.zip", {"other.txt": b"not mine"})
    files = mgr.collect_for_task("task_mine", timeout=0.2)
    assert files == []
    # The other task's zip must remain untouched.
    assert (tmp_path / "task_other.zip").exists(), "must not steal another task's zip"


def test_collect_for_task_empty_id_returns_empty(tmp_path: Path):
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=tmp_path / "out")
    assert mgr.collect_for_task("", timeout=0.1) == []


def test_collect_for_task_returns_empty_when_watch_dir_missing(tmp_path: Path):
    mgr = ArtifactManager(watch_dir=tmp_path / "nope", extract_dir=tmp_path / "out")
    assert mgr.collect_for_task("task_x", timeout=0.1) == []


def test_collect_for_task_returns_empty_on_timeout(tmp_path: Path):
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=tmp_path / "out")
    assert mgr.collect_for_task("task_never_arrives", timeout=0.1) == []


def test_extract_streams_in_chunks_does_not_oom(tmp_path: Path):
    """Pre-fix, extract() did dst.write(src.read()) loading entire entry to RAM.
    Verify the chunked path is exercised even on tiny inputs (no behaviour
    regression).
    """
    src = _make_zip(tmp_path / "in.zip", {"big.bin": b"a" * (1024 * 1024)})  # 1 MiB
    extract_dir = tmp_path / "out"
    mgr = ArtifactManager(watch_dir=tmp_path, extract_dir=extract_dir)
    extracted = mgr.extract(src)
    assert {p.name for p in extracted} == {"big.bin"}
    assert (extract_dir / "big.bin").stat().st_size == 1024 * 1024


def test_extract_rejects_zip_bomb_above_budget(tmp_path: Path):
    """A single member larger than max_extract_bytes is rejected."""
    src = _make_zip(tmp_path / "bomb.zip", {"huge.bin": b"x" * (256 * 1024)})  # 256 KiB
    mgr = ArtifactManager(
        watch_dir=tmp_path, extract_dir=tmp_path / "out", max_extract_bytes=128 * 1024
    )
    with pytest.raises(ArtifactError, match="exceeds max_extract_bytes"):
        mgr.extract(src)


def test_extract_rejects_zip_bomb_cumulative_budget(tmp_path: Path):
    """Multiple small members that together exceed the budget are rejected."""
    src = _make_zip(
        tmp_path / "many.zip",
        {f"f{i}.bin": b"y" * (50 * 1024) for i in range(5)},  # 250 KiB total
    )
    mgr = ArtifactManager(
        watch_dir=tmp_path, extract_dir=tmp_path / "out", max_extract_bytes=128 * 1024
    )
    with pytest.raises(ArtifactError, match="exceeds max_extract_bytes"):
        mgr.extract(src)


def test_extract_zip_bomb_partial_file_is_unlinked(tmp_path: Path):
    """The partially-written file from a budget overflow must NOT remain on
    disk — otherwise a follow-on collect_for_task would surface a corrupted
    file as legitimate output."""
    src = _make_zip(
        tmp_path / "many.zip",
        {
            "small_ok.bin": b"a" * (60 * 1024),
            "huge_overflows.bin": b"b" * (200 * 1024),
        },
    )
    extract_dir = tmp_path / "out"
    mgr = ArtifactManager(
        watch_dir=tmp_path, extract_dir=extract_dir, max_extract_bytes=128 * 1024
    )
    with pytest.raises(ArtifactError, match="exceeds max_extract_bytes"):
        mgr.extract(src)
    # The first complete entry survives as diagnostic evidence.
    assert (extract_dir / "small_ok.bin").exists(), "complete previous entries should remain"
    # The partial overflow entry is gone — no corruption left behind.
    assert not (extract_dir / "huge_overflows.bin").exists(), (
        "partial entry must be unlinked after budget overflow"
    )
