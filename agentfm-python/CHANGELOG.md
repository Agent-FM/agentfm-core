# Changelog

All notable changes to the `agentfm` Python SDK are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed

- v0.1 deprecated method names: `discover_workers`, `execute_task`,
  `batch_execute`, `submit_async_task`. Use the namespaced equivalents on
  `client.workers.*` and `client.tasks.*` instead. The `agentfm._compat`
  module is gone.

### Added

- Bearer-token auth for the Boss HTTP gateway (issue #13). Pass
  `api_key="..."` to `AgentFMClient` / `AsyncAgentFMClient` to send
  `Authorization: Bearer <key>` on every request. Falls back to
  `AGENTFM_API_KEY` env var when the argument is omitted; pass an explicit
  `None` to disable auth (no env fallback). New `AuthenticationError`
  raised on HTTP 401 envelopes. `with_options(api_key=...)` accepts the
  same three modes (string override, explicit `None` to clear, omit to
  inherit).
- `client.with_options(...)` and `async_client.with_options(...)` shallow-clone
  helpers for one-off requests with overridden timeout / retries / gateway URL.
- `ArtifactManager.collect_since(since, timeout)` consolidates the
  wait-for-zip → extract → cleanup loop previously inlined in `tasks.run`.
- `@overload` typing on `chat.completions.create(stream=...)` and
  `completions.create(stream=...)` so callers get the precise return type
  (`ChatCompletion` vs `Iterator[ChatCompletionChunk]`) without `assert isinstance`.
- `WebhookReceiver` now defaults `host="127.0.0.1"`. Pass `host="0.0.0.0"`
  explicitly to accept off-host callbacks.

### Changed

- `_RoutingWarner` instance state replaces module-global
  `_warned_about_routing`. Per-namespace dedup is now thread-safe.
- `retry_sync` / `retry_async` now retry on transient HTTP statuses
  (408, 429, 500, 502, 503, 504) with exponential backoff + jitter, in
  addition to connection / read errors.
- `_Unset`, `_UNSET`, and `DEFAULT_GATEWAY` lifted into `agentfm._shared`
  so `async_client` no longer back-imports from `client`.
- Lazy namespace instantiation via `@cached_property`; `import agentfm` no
  longer pulls the OpenAI sub-package.

### Fixed

- `_TasksNamespace.run` no longer stores per-call timing on instance state,
  fixing a race when the same namespace is shared across threads (e.g. by
  `tasks.scatter`).

## [0.2.0] - 2026-04-26

Complete rewrite of the SDK. The public surface keeps backwards-compatible
shims for one release; everything else is new.

### Fixed

- `WorkerProfile` now uses the correct `snake_case` JSON keys (`peer_id`, `cpu_usage_pct`)
  instead of the broken PascalCase aliases. The previous version could not parse a single
  response from the live Go gateway.
- `discover_workers` now correctly unwraps the `{"success": true, "agents": [...]}`
  envelope. Previously it tried to parse the envelope itself as a worker.
- Sentinel filtering for `[AGENTFM: FILES_INCOMING]` / `[AGENTFM: NO_FILES]` markers is
  now line-buffered with proper handling of partial sentinels split across HTTP chunks
  (replaces the fragile substring-search heuristic).
- Zip extraction is now safe against path-traversal (zip-slip) attacks.
- `SwarmKey` now validates the hex string length, writes in binary mode for cross-platform
  consistency, and sets file permissions on Windows where supported.

### Added

- `AsyncAgentFMClient` for async/await usage (mirrors the sync surface).
- First-class OpenAI-compatible namespace (`client.openai.chat.completions.create`,
  `client.openai.completions.create`, `client.openai.models.list`) with both streaming
  and non-streaming support.
- Strong typing: `PeerID = NewType("PeerID", str)` so `mypy` catches mistakes like
  passing an `agent_name` where a `peer_id` is expected.
- Generator-based streaming API: `client.tasks.stream(...)` yields `TaskChunk` objects
  for programmatic consumption (no more forced `print()`-only output).
- `WebhookReceiver` helper (stdlib only) for receiving async-task callbacks from the
  Go gateway without pulling in FastAPI.
- `agentfm-py` CLI for quick interactive testing.
- Full exception hierarchy mapped to the Go gateway's error envelope codes
  (`ModelNotFoundError`, `MeshOverloadedError`, `WorkerUnreachableError`, etc.).
- `logging.getLogger("agentfm")` integration; the SDK no longer prints to stdout
  unless explicitly told to.
- Comprehensive test suite (`pytest`, `respx`-mocked HTTP) with ~85% coverage target.
- `py.typed` marker so consumers get full type information from `pip install`.
- CI matrix on Python 3.10, 3.11, 3.12, 3.13 across Linux and macOS.

### Changed

- License is now Apache-2.0 (matches the parent project). Previous declaration of MIT
  was inconsistent and is corrected.
- Minimum Python version is now 3.10 (was 3.8, which is end-of-life).
- HTTP transport switched from `requests` to `httpx` for unified sync/async, HTTP/2,
  and proper timeout / retry control.
- Build backend switched from setuptools to `hatchling`.
- `LocalMeshGateway` now logs subprocess stdout/stderr to a rotating log file in
  non-debug mode (was silently discarded).
- Identifier hierarchy: `peer_id` is the canonical addressing primitive throughout.
  `agent_name` and `model` are convenience routing strings only, with documented
  tradeoffs.

### Deprecated

The following methods still work but emit `DeprecationWarning`. They will be
removed in v0.4.

- `client.discover_workers(...)` → use `client.workers.list(...)`.
- `client.execute_task(...)` → use `client.tasks.run(...)` or `client.tasks.stream(...)`.
- `client.batch_execute(...)` → use `client.tasks.scatter(...)`.
- `client.submit_async_task(...)` → use `client.tasks.submit_async(...)`.

### Removed

- `requirements.txt` (server-side dependencies that leaked into the SDK package).
  The server example now lives in `examples/playground_server/` with its own
  dependency file.
- Repo no longer carries `.env`, identity keys, swarm keys, build artifacts, or
  Python virtualenvs.
