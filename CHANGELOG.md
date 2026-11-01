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
- `internal/worker`: the `podman run` argument vector is now produced by a pure `BuildRunArgs(SandboxSpec)` instead of being assembled inline in `executePodman`. The emitted argv is **identical on the nominal path**, pinned by golden tests across all four GPU × env-file combinations. The error path is strictly tightened: a malformed spec (empty image, unknown network mode) now fails *before* any process is spawned instead of letting `podman` reject it. This is the seam every subsequent sandbox-hardening item depends on — before it, no test could observe the container's isolation posture, so removing a security flag broke nothing.

**Per-task resource ceilings (roadmap R1, partial)**

- New worker flags `-task-cpus`, `-task-memory` and `-task-pids-limit` bound the running task container through its cgroup. These are distinct from `-maxcpu` / `-maxgpu` / `-maxtasks`, which are *admission* thresholds and constrain nothing once a container is running — until now nothing did, so a fork bomb or a runaway allocation in an agent consumed the worker host until the kernel picked a victim.
- `-task-memory` accepts podman-style sizes (`512m`, `2g`, `1GiB`) and sets `--memory-swap` to the same value, disabling swap: without that, a container at its ceiling thrashes the host's disk instead of being killed.
- `-task-pids-limit` defaults to `1024` — the one limit enabled by default, because it contains a fork bomb at a level no real agent approaches. CPU and memory default to unlimited so that an operator opts in after observing what their agent uses; a ceiling that is wrong for a given agent kills legitimate work.
- A worker with an unusable ceiling refuses to start instead of failing on its first task.
- Tasks killed for exceeding their memory ceiling report `agentfm_tasks_total{status="oom_killed"}` rather than `error`, so a limit set too low is distinguishable from an agent that genuinely crashes. Attribution is only claimed when a memory limit is actually configured — a `137` exit with no limit set is not blamed on one. It remains a heuristic: a task timeout is never mislabelled, but a kill by the *host's* OOM killer, or an agent exiting `137` deliberately, are both counted as `oom_killed`. Metric-only impact; see `docs/worker.md`.
- The desktop success-rate chart counts the new status. A status the worker emits but the chart does not enumerate would drop those tasks out of the denominator entirely, making a worker that kills every task display a 100% success rate.
- The worker prints its effective per-task ceilings at startup, so an operator can confirm the flags took effect rather than trusting that they did.

### Fixed

- `internal/network/constants.go` is under version control again. It had been excluded by a `.gitignore` rule since the initial commit, so `agentfm-go` could not be compiled from a clean clone — which is also why the module had no CI. The 16 affected symbols (5 stream protocol IDs, 3 GossipSub topics, 4 timeouts, `MaxArtifactBytes`, `PublicLighthouse`, `RendezvousString`, `MDNSServiceTag`) were reconstructed from the documentation, each with its source cited inline. See `docs/security/AUDIT-2026-08-10.md` §2.

### Security

- **Breaking for public-mesh workers:** task containers on the public mesh now run with `--network none` by default (roadmap S4, first half). New `-sandbox-network host|none` flag; an empty value defaults to `none` without `-swarmkey` and `host` with it, so private swarms are unchanged. Previously every task ran with `--network host`, and because a worker accepts prompts from any peer, a stranger could prompt-inject a trusted agent into calling the worker's loopback services or cloud metadata (`169.254.169.254`). Agents that call a local model over HTTP (Ollama, vLLM) must now pass `-sandbox-network host` on the public mesh; the egress allowlist that would let them reach only the model is still to come. `BuildRunArgs` gains a pinned golden case for `none`, and `limits_wiring_test.go` pins that the flag reaches the container argv.
- A worker no longer reports a task as successful when its sandbox never ran. `executePodman` now returns `(string, error)` and distinguishes `ErrSandboxNotRun` (output dir uncreatable, invalid spec, podman failed to spawn) from `ErrSandboxFailed` (container ran, exited non-zero or was killed). `handleTaskStream` records `StatusError` — or `StatusTimeout` when the task context expired — instead of unconditionally setting `StatusOK`, and skips the artifact path entirely when nothing ran. Previously any of these paths produced `[AGENTFM: NO_FILES]`, "Task complete" and a success metric, so a worker with a missing or stopped container runtime told every Boss it had done work it never performed — the "billed non-execution" failure mode, reachable by operational accident. This is also the first worker-side emitter of `agentfm_tasks_total{status="timeout"}`; the label existed but only the Boss ever set it. `agentfm -mode test` now exits non-zero on sandbox failure instead of printing a success banner.
- Worker task failures are signalled on the stream as `[AGENTFM: TASK_FAILED not_run|abnormal_exit]`, in the same family as the existing artifact markers. No protocol version bump: all three consumers filter on the `[AGENTFM:` prefix rather than an enumerated list (boss `isSentinelLine`, the Python SDK's `_classify`, the desktop `useDispatch` hook), so older Bosses and SDKs drop the line. **Scope limit — the Boss does not interpret it yet**: a Boss still records such a task as `ok` and fires a `completed` webhook. Until that lands (roadmap R2), the honest accounting exists only on the worker side.
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
