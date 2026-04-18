# AgentFM Test Layout

This directory centralises everything *around* testing — shared fixtures and
cross-package integration tests. The test suites themselves follow Go
convention and live **next to** the code they exercise (`internal/*/*_test.go`)
so they can reach unexported symbols via white-box testing.

## Directory layout

```
test/
├── testutil/        Shared fixtures importable from any _test.go file.
│                    Thin wrappers around libp2p hosts, zip builders,
│                    fake sub-processes, and sync helpers. NOT imported by
│                    production code.
│
└── integration/     Cross-package, end-to-end scenarios. Use only the
                     exported API of internal packages. Run against real
                     libp2p TCP hosts on 127.0.0.1, no external services.
```

## Test taxonomy

| Layer | Location | Purpose | Runtime |
|---|---|---|---|
| **Unit** | `internal/<pkg>/*_test.go` | White-box tests of a single package's logic, edge cases, and error paths. | Milliseconds per test. |
| **Integration** | `test/integration/*_test.go` | Cross-package flows that stitch real libp2p + pubsub together. | Seconds per test. |
| **Fixtures** | `test/testutil/*.go` | Reusable helpers. Not tests themselves; both unit and integration tests import them. | N/A |

## Running

```bash
# Just unit tests (the fast default used in pre-commit / PR checks).
make test-unit

# Integration tests only.
make test-integration

# Everything under the race detector.
make test-race

# Coverage report (unit tests only; integration coverage is not interesting).
make test-coverage
```

Or raw `go test`:

```bash
go test -race ./internal/...            # unit suite
go test -race ./test/integration/...    # integration suite
go test -race ./...                     # everything
```

## Why not move everything into `test/`?

Go's stdlib and the broader ecosystem place `_test.go` files next to the code
they test. Two reasons we honour that convention:

1. **White-box access.** Tests in the same package can exercise unexported
   functions like `handleTaskStream`, `loadOrGenerateIdentity`,
   `parseRelayInfo`, `timeoutReader`, `isDirEmpty`, `truncateWords`,
   `getGPUStats`, `progressWriter`, `corsMiddleware`. Moving them out would
   either force us to export these (API pollution) or lose coverage on the
   most bug-prone internals.
2. **Ergonomics.** `go test ./...` discovers tests by walking packages.
   Keeping `_test.go` files beside their subjects makes coverage reports,
   IDE jump-to-test, and `go test -run` all behave as developers expect.

What we DO centralise here is the **infrastructure around testing** —
fixtures and integration scenarios — which is genuinely reusable across
packages.

## Writing new tests

| If you're writing… | Put it… | Package declaration |
|---|---|---|
| A unit test for `internal/foo.Bar` | `internal/foo/bar_test.go` | `package foo` |
| A black-box test for `internal/foo`'s public API | `internal/foo/bar_api_test.go` | `package foo_test` |
| A scenario that spans `internal/boss` + `internal/network` | `test/integration/<scenario>_test.go` | `package integration` |
| A helper used in 3+ places | `test/testutil/<topic>.go` | `package testutil` |

## Guarantees

Every test in this repo must:

- Pass under `go test -race` (no goroutine leaks, no data races).
- Clean up via `t.Cleanup` — no lingering file handles, goroutines, or
  libp2p hosts after the test returns.
- Use `t.TempDir`, `t.Chdir`, `t.Setenv` instead of raw `os.*` so parallel
  runs remain isolated.
- Bound every network / sub-process call with `context.WithTimeout`.
