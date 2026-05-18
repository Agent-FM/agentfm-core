# Changelog

## [1.3.0] — Verifiable Agent Mesh — unreleased

The v1.3 release adds a tamper-evident reputation ledger plus runtime verification so dishonest agents can be caught and ejected without a blockchain.

### Added

**Trust substrate (Phase 1 — ledger spine)**

- New `internal/ledger/` package with a per-peer signed Merkle log over SQLite (modernc.org/sqlite, pure-Go, no CGO).
- RFC 6962 Merkle tree primitive with leaf/node domain separation, inclusion proofs, and consistency proofs.
- Append-only enforcement at BOTH the Go API layer and the database trigger layer.
- Ed25519 signing over `SHA-256(canonical_bytes)` for cross-language verifier compatibility.
- Gossip publish/subscribe on `agentfm-feedback-v1` GossipSub topic.
- Inbox with chain-extension validation, orphan queuing + BFS promotion, replay dedup, signature verification, and post-sig range validation (rejects NaN/Inf/out-of-range scores).
- `agentfm reputation show <peer_id>` CLI subcommand for casual auditing.
- New CLI flags: `--witness`, `--witness-threshold`, `--witness-set`, `--capability`, `--attestation-mode`, `--reject-unknown-images`, `--trusted-agents`.

**Witnesses (Phase 2)**

- New `internal/witness/` package implementing the `/agentfm/witness/1.0.0` co-sign protocol.
- Witnesses store the last LogHead per peer and refuse to co-sign non-extending heads.
- M-of-N gather configuration in the ledger; per-witness consistency proofs prevent fork attacks at non-overlapping tree sizes.
- `EquivocationAlert` envelope gossiped on `agentfm-equivocation-v1`; offenders pinned to `-1.0` permanently after end-to-end alert validation (offender sig + conflict check).
- New `/agentfm/ledger-fetch/1.0.0` stream protocol for pulling entries from another peer's log.
- `Ledger.Prove(entryHash)` returns full RFC 6962 inclusion proofs; standalone `VerifyInclusionProof` for offline validation.

**Agent verification (Phase 3 — L1 only in v1.3)**

- Telemetry envelope extended with `agent_image_digest`, `agent_image_ref`, `agent_capability`.
- Curated `manifests/trusted-agents.json` registry of recognised agent images.
- L1 commitment check at dispatch with three modes (`off` / `warn` / `strict`); mismatch writes a `-0.5` rating to the ledger; strict mode refuses dispatch with `403 agent_attestation_failed`.
- Equivocator gate: dispatch always refuses peers marked equivocator (regardless of attestation mode).

**API + SDK + UI (Phase 4)**

- New HTTP endpoints on the Boss gateway:
  - `GET /v1/peers/{id}/reputation`
  - `GET /v1/peers/{id}/log`
  - `GET /v1/peers/{id}/proof?entry={hex}`
  - `POST /v1/peers/{id}/comments` (signed submission; v1.3 self-only)
- Content-addressed comment body storage at `~/.agentfm/comments/` with 10 KiB cap, fetchable on demand via `/agentfm/comment-fetch/1.0.0`.
- Python SDK adds `client.reputation.get/log/proof/comment` namespace.
- Web UI viewer at `/ui/peer/{peer_id}` (single static HTML page, no framework, auto-refreshes every 10s).

**Reputation engine + release polish (Phase 5)**

- EigenTrust-lite scoring in `internal/reputation/`: rater vote weight = rater's own current score, 30-day half-life age decay, configurable mixing factor, ejection at `-0.5` with `0.2` hysteresis.
- Curated `manifests/genesis-seeds.json` (bundled into the binary as a fallback).
- Optional Sigstore Rekor anchoring (`--rekor-anchor`) via the new `internal/rekor/` package — Stub anchor ships in v1.3; real Rekor v2 REST client lands in v1.3.1.
- New `docs/trust.md` (~2000 words) covering the threat model, defenses, trust assumptions, manual verification, and non-goals.

### Changed

- `internal/types/types.go::WorkerProfile` adds `IsWitness`, `AgentImageDigest`, `AgentImageRef`, `AgentCapability` (all `omitempty` for backward compatibility).
- `internal/boss.New(node)` retained as before; new `NewWithOptions(node, Options{...})` constructor for callers that want to wire the ledger, attestation registry, and comment store.
- `pb.InclusionProof.Entry` now carries the full `SignedEntry` wrapper instead of an inner-only `oneof{Rating|Comment}` — future SignedEntry-level fields survive proof round-trip without silent verification failures.

### Security

- Strict equivocation-alert validation prevents a rogue witness from forging brands against innocent peers (audit fix).
- Witness extension check requires a valid RFC 6962 consistency proof; closes the fork-at-non-overlapping-size hole (audit fix).
- Post-signature payload validation in the inbox rejects NaN/Inf scores and other out-of-range values (audit fix).
- Length-prefixed protocol framing has hard size caps (1 MiB witness frames, 10 KiB comment bodies, 1000 entry fetch batches, 20 GB image cache LRU).

### Not in this release (deferred)

- Salt-challenge over container image layers (P3-4) — deferred to v1.3.1.
- Golden-prompt probe coordinator (P3-5) — deferred to v1.4.
- Multi-strategy probe grader (P3-6, including LLM-judge / code-execute) — deferred to v1.4.
- External-submitter signed comments (P4-3 delegation) — deferred to v1.4.
- TEE attestation tier (L4) — deferred to v1.4 as an opt-in premium tier.
- Privacy / encrypted ratings — deferred to v1.4.
- Cross-mesh reputation portability — v1.5 problem.
- Real Sigstore Rekor v2 REST client (Stub anchor only in v1.3) — v1.3.1.

### Hard project constraints (intentional non-goals)

- No blockchain, tokens, staking, or on-chain governance.
- No zkML or cryptographic proof of inference correctness.
- No sybil IMMUNITY (the design only provides sybil RESISTANCE via EigenTrust + curated seeds).

---

## [1.2.0] — 2026-XX-XX

Previous releases — see git history.
