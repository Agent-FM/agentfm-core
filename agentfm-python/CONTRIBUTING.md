# Contributing to the AgentFM Python SDK

Bug reports and PRs welcome. The SDK is small enough that you can read the
whole source in one sitting; please do that first.

## Local development

```bash
git clone https://github.com/Agent-FM/agentfm-core.git
cd agentfm-core/agentfm-python
python3 -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"
```

That installs the SDK in editable mode plus the dev tools (ruff, mypy,
pytest, respx).

## Quality bar before submitting a PR

```bash
ruff check src/ tests/         # lint
ruff format src/ tests/        # auto-format
mypy src/agentfm/              # strict type-check
pytest                          # unit + integration suite
pytest --cov=agentfm           # with coverage report
```

The CI matrix runs Python 3.10, 3.11, 3.12, 3.13 on Linux and macOS. All
must pass.

## Style rules

* Type hints everywhere. `mypy --strict` clean.
* Pydantic v2 idioms (`model_config = ConfigDict(...)`, no `class Config:`).
* No `print()` in library code — use `agentfm.logging.get_logger(__name__)`.
* No new runtime dependencies without discussion. The SDK has three:
  `httpx`, `pydantic`, `typing-extensions`. Anything else lives in the
  `cli` or `dev` extras.
* `peer_id` is the canonical addressing primitive. Anywhere a public API
  takes a worker reference, accept ``PeerID``, not ``str``-typed model names.
  Name-based routing is a documented convenience with a `RoutingWarning`.
* Tests should never hit a real gateway. Use the `respx` fixtures in
  `tests/conftest.py`.

## Adding a new public API

1. Update `src/agentfm/__init__.py` to re-export your new symbols.
2. Add at least one unit test exercising pure-function behavior.
3. Add an integration test against the mocked gateway if the API touches HTTP.
4. Add an entry to `CHANGELOG.md` under `[Unreleased]`.
5. Update `README.md` if the API is user-facing.

## Releasing

Releases are tag-driven. The `python-sdk-publish` workflow builds and
publishes to PyPI via Trusted Publishing on any tag matching `agentfm-py-vX.Y.Z`.

1. Bump `__version__` in `src/agentfm/_version.py`.
2. Move the `[Unreleased]` section in `CHANGELOG.md` under the new version.
3. Commit, push, and tag: `git tag agentfm-py-v0.2.1 && git push --tags`.
