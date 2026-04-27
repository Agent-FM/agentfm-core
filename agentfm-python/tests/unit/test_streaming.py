from __future__ import annotations

import pytest

from agentfm.streaming import (
    ARTIFACT_INCOMING,
    ARTIFACT_NONE,
    SentinelFilter,
    filter_iter,
    parse_sse_lines,
)


def _drain(filter_: SentinelFilter, chunks: list[str]) -> str:
    out: list[str] = []
    for chunk in chunks:
        out.extend(filter_.feed(chunk))
    tail = filter_.finalize()
    if tail:
        out.append(tail)
    return "".join(out)


def test_passes_clean_text_through():
    f = SentinelFilter()
    assert _drain(f, ["Hello world\n"]) == "Hello world\n"


def test_strips_files_incoming_sentinel():
    f = SentinelFilter()
    out = _drain(f, [f"alpha\n{ARTIFACT_INCOMING}\nbeta\n"])
    assert out == "alpha\nbeta\n"
    assert f.artifacts_incoming is True
    assert f.artifacts_complete is False


def test_strips_no_files_sentinel():
    f = SentinelFilter()
    out = _drain(f, [f"first\n{ARTIFACT_NONE}\nlast\n"])
    assert out == "first\nlast\n"
    assert f.artifacts_complete is True
    assert f.artifacts_incoming is False


def test_passes_through_user_facing_error_marker():
    f = SentinelFilter()
    src = "❌ ERROR: Worker is at max capacity (3/3). Try another worker.\n"
    assert _drain(f, [src]) == src


def test_handles_partial_sentinel_split_across_chunks():
    f = SentinelFilter()
    # Sentinel arrives in three pieces of an HTTP chunk
    out = _drain(f, ["alpha\n[AGENTFM:", " NO_FILES", "]\nbeta\n"])
    assert out == "alpha\nbeta\n"
    assert f.artifacts_complete is True


def test_strips_adjacent_sentinels():
    f = SentinelFilter()
    src = f"{ARTIFACT_NONE}\n{ARTIFACT_INCOMING}\nrest\n"
    assert _drain(f, [src]) == "rest\n"
    assert f.artifacts_incoming is True
    assert f.artifacts_complete is True


def test_only_sentinels_yields_empty():
    f = SentinelFilter()
    assert _drain(f, [f"{ARTIFACT_NONE}\n"]) == ""


def test_leading_whitespace_sentinel_still_recognised():
    f = SentinelFilter()
    out = _drain(f, [f"  {ARTIFACT_NONE}\nkeep\n"])
    assert out == "keep\n"


def test_trailing_partial_line_flushed_via_finalize():
    f = SentinelFilter()
    # No trailing newline
    out = _drain(f, ["partial line"])
    assert out == "partial line"


def test_filter_iter_helper_round_trips():
    out = "".join(filter_iter([f"x\n{ARTIFACT_NONE}\n", "y\n"]))
    assert out == "x\ny\n"


def test_finalize_drops_partial_sentinel_prefix():
    """A final chunk that opens a sentinel but never terminates it must
    not leak '[AGENTFM:...' into the user's stream. Worker is the only
    party that emits these markers; partial means truncated transport."""
    f = SentinelFilter()
    fed = list(f.feed("[AGENTFM: PARTIAL"))
    tail = f.finalize()
    assert fed == [], f"unexpected emission during feed: {fed}"
    assert tail == "", f"partial sentinel leaked into finalize tail: {tail!r}"
    assert f.artifacts_incoming is False
    assert f.artifacts_complete is False


@pytest.mark.parametrize(
    "lines,expected",
    [
        (["data: hello\n", "\n"], ["hello"]),
        (["data: a\n", "data: b\n"], ["a", "b"]),
        (["data: [DONE]\n"], []),
        ([": comment\n", "data: x\n"], ["x"]),
        (["event: foo\n", "data: x\n"], ["x"]),
        (["data: x\n", "data: [DONE]\n", "data: y\n"], ["x"]),
    ],
)
def test_parse_sse_lines(lines: list[str], expected: list[str]):
    assert list(parse_sse_lines(lines)) == expected
