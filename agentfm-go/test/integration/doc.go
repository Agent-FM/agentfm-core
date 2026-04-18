// Package integration holds end-to-end tests that span multiple AgentFM
// packages and exercise the real libp2p stack over 127.0.0.1.
//
// These tests are slower than unit tests (seconds, not milliseconds) because
// they spin up real TCP-backed hosts, pubsub topics, and handler goroutines.
// They use only the exported APIs of `internal/*` packages so they can be
// run without white-box hooks.
//
// Integration tests are NOT run by `go test ./internal/...`. Run them
// explicitly with `go test ./test/integration/...` or `make test-integration`.
package integration
