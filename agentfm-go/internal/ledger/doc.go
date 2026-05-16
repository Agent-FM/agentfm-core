// Package ledger implements AgentFM's per-peer signed append-only Merkle log.
//
// Each peer maintains its own ledger of Rating and Comment entries it has
// issued about other peers. Entries are signed with the peer's libp2p
// Ed25519 key, hash-chained so silent edits are detectable, persisted to
// SQLite, and gossiped to the mesh over libp2p so other peers can
// replicate and verify them.
//
// This package is the v1.3 "verifiable agent mesh" trust substrate. See:
//
//   - docs/superpowers/specs/2026-05-02-verifiable-mesh-design.md
//   - docs/superpowers/plans/2026-05-16-verifiable-agent-mesh-implementation.md
//
// The Go API exposed here is the contract every other AgentFM package
// reads from when it wants to record or consult reputation signals
// (boss/execute.go for dispatch decisions; worker/* for attestation
// ratings; boss/api_handlers.go for the HTTP surface).
//
// This file (P0-3) only stubs the API. Real implementations land in:
//
//   - P1-1: merkle/   — RFC 6962 Merkle tree primitive
//   - P1-2: store/    — SQLite-backed append-only entry store
//   - P1-3..5:        — signing, append+gossip, subscribe+verify
//   - P2-*:           — witness role and equivocation detection
//   - P3-7:           — reputation aggregation + matcher hook
//
// Until P1-1 lands, every public method on Ledger returns
// ErrNotImplemented so downstream callers can compile against the
// finished interface today without waiting on the implementation.
//
// Canonical wire format: pb.SignedEntry. See internal/ledger/pb.
package ledger
