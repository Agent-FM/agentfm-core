# Integration Tests

End-to-end scenarios that wire multiple `internal/*` packages together and
exercise the real libp2p stack. Skipped by default during unit-test runs
(`go test ./internal/...` does not traverse this directory).

## Running

```bash
go test -race ./test/integration/...
# or
make test-integration
```

## What belongs here

- Scenarios that require **real network stack** — hosts with TCP transports
  on `127.0.0.1`, real Noise handshakes, real pubsub message propagation.
- Scenarios that **span 2+ internal packages** and validate their
  interaction contract (e.g. Boss's telemetry listener consumes exactly the
  JSON Worker publishes).
- **Black-box contract tests** — tests written against exported APIs that
  demonstrate how external callers should assemble the system.

## What does NOT belong here

- Tests of a single function's edge cases → put in the unit suite
  (`internal/<pkg>/<name>_test.go`).
- Tests that require **Podman / nvidia-smi / external network** → gate
  them with a skip + environment check, or keep them in a separate
  `e2e/` directory that CI runs only in specially-configured environments.

## Conventions

- Package declaration: `package integration`.
- Every test imports from `agentfm/test/testutil` for fixtures.
- Every test bounds its context with a `context.WithTimeout` (≤ 15s).
- Every test registers cleanup via `t.Cleanup` so hosts / subscriptions /
  goroutines are torn down deterministically under `go test -race`.
