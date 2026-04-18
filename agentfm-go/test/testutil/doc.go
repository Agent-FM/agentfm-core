// Package testutil provides reusable fixtures for AgentFM's test suites.
//
// Helpers fall into four groups:
//
//   - libp2p.go  — real TCP-localhost hosts and connected meshes
//   - zip.go     — deterministic zip archive builders
//   - sync.go    — context / polling helpers that keep tests race-safe
//   - nvidia.go  — fake `nvidia-smi` binary installer for telemetry tests
//
// All helpers take a testing.TB so they work from both tests and benchmarks,
// call t.Helper() for clean failure traces, and register t.Cleanup to avoid
// resource leaks under `go test -race`.
//
// This package is imported only by _test.go files; it never ships in any
// production binary.
package testutil
