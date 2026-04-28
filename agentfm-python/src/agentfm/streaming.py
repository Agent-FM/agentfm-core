"""Sentinel-stripping for AgentFM stdout streams.

Mirrors the Go side's ``sentinelFilterReader``. Lines beginning with
``[AGENTFM:`` are protocol markers (artifacts arriving / not arriving) and
must be removed before the stream is shown to a user. Other content,
including human-facing error markers like ``❌ ERROR:``, passes through.

Implementation is line-buffered: chunks may split a sentinel across HTTP
reads, so we keep a carry buffer until we see a newline.
"""

from __future__ import annotations

from collections.abc import Iterable, Iterator

SENTINEL_PREFIX = "[AGENTFM:"
ARTIFACT_INCOMING = "[AGENTFM: FILES_INCOMING]"
ARTIFACT_NONE = "[AGENTFM: NO_FILES]"


class SentinelFilter:
    """Streaming text filter.

    Feed it raw chunks via :meth:`feed`; it yields cleaned text. Trailing
    output (no final newline) flushes via :meth:`finalize`.
    Side-effect flags (:attr:`artifacts_incoming`, :attr:`artifacts_complete`)
    let the caller know whether to wait for an artifact stream.
    """

    __slots__ = ("_buf", "artifacts_complete", "artifacts_incoming")

    def __init__(self) -> None:
        self._buf = ""
        self.artifacts_incoming: bool = False
        self.artifacts_complete: bool = False

    def feed(self, chunk: str) -> Iterator[str]:
        """Consume a chunk; yield cleaned line-by-line text."""
        self._buf += chunk
        while True:
            newline = self._buf.find("\n")
            if newline < 0:
                break
            line = self._buf[:newline]
            self._buf = self._buf[newline + 1 :]
            cleaned = self._classify(line)
            if cleaned is not None:
                yield cleaned + "\n"

    def finalize(self) -> str:
        """Flush any trailing partial line.

        Returns "" if the trailing buffer is itself a sentinel.
        """
        if not self._buf:
            return ""
        line = self._buf
        self._buf = ""
        cleaned = self._classify(line)
        return cleaned if cleaned is not None else ""

    def _classify(self, line: str) -> str | None:
        stripped = line.lstrip(" \t")
        if not stripped.startswith(SENTINEL_PREFIX):
            return line
        # Sentinel line: detect kind, then drop.
        if ARTIFACT_INCOMING in stripped:
            self.artifacts_incoming = True
        elif ARTIFACT_NONE in stripped:
            self.artifacts_complete = True
        return None


def filter_iter(chunks: Iterable[str]) -> Iterator[str]:
    """Convenience: filter an iterable of chunks all at once."""
    f = SentinelFilter()
    for chunk in chunks:
        yield from f.feed(chunk)
    tail = f.finalize()
    if tail:
        yield tail


# ---------------------------------------------------------------------------
# Server-Sent Events (SSE) parsing for /v1/* streaming responses
# ---------------------------------------------------------------------------

SSE_DONE = object()
"""Sentinel returned by :func:`classify_sse_line` for the ``data: [DONE]``
terminator. Callers should stop iterating when they see it."""


def classify_sse_line(raw: str) -> str | object | None:
    """Classify a single SSE line.

    Returns:
    - the payload string when the line carries ``data: <body>``
    - :data:`SSE_DONE` when the line is the terminator ``data: [DONE]``
    - ``None`` for empty lines, comments, and non-data fields

    Shared by the sync :func:`parse_sse_lines` (operating on an
    ``Iterable[str]``) and the async ``_aiter_sse`` in the OpenAI
    namespace (operating on an ``AsyncIterator[str]``). Keeping a single
    classifier means the two iterators can never drift on what counts as
    a payload vs a terminator.
    """
    line = raw.rstrip("\r\n")
    if not line or line.startswith(":") or not line.startswith("data:"):
        return None
    body = line[len("data:") :].lstrip()
    if body == "[DONE]":
        return SSE_DONE
    return body or None


def parse_sse_lines(lines: Iterable[str]) -> Iterator[str]:
    """Decode an SSE byte/line stream into raw ``data:`` payload bodies.

    Yields the JSON body of each ``data:`` event. Skips comments, empty
    lines, the ``[DONE]`` terminator, and non-data fields (we don't use
    ``event:``/``id:`` for anything).
    """
    for raw in lines:
        result = classify_sse_line(raw)
        if result is SSE_DONE:
            return
        if isinstance(result, str):
            yield result


__all__ = [
    "ARTIFACT_INCOMING",
    "ARTIFACT_NONE",
    "SENTINEL_PREFIX",
    "SSE_DONE",
    "SentinelFilter",
    "classify_sse_line",
    "filter_iter",
    "parse_sse_lines",
]
