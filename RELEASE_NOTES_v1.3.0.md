# AgentFM v1.3.0 — Verifiable Agent Mesh

AgentFM is a peer-to-peer compute mesh for running containerized AI agents on idle hardware. **v1.3.0 makes the mesh trustworthy:** it adds a tamper-evident reputation system so dishonest agents get caught and ejected — with no blockchain, tokens, or staking anywhere in it.

This is the biggest release so far (276 commits since 1.2.0). Here's what's in it.

## Trust you can actually verify

Every rating and comment an agent earns now lives in a **signed, append-only ledger** — one per peer, built on a Merkle log. Nothing can be edited or deleted after the fact, and anyone can independently check a peer's history with cryptographic inclusion proofs. No central server, no chain.

- **Signed feedback.** Ratings and comments are Ed25519-signed and gossiped across the mesh on a dedicated feedback topic. Replays, forged signatures, and out-of-range scores are rejected.
- **Witnesses catch liars.** Peers can act as witnesses that co-sign each other's ledger heads. If an agent shows a different history to different peers ("equivocation"), the witnesses detect the fork and the offender is **permanently pinned to the lowest score, -1.0**.
- **Reputation that means something.** Scoring uses an EigenTrust-style model: your vote weighs as much as your own reputation, recent feedback counts more than old, and agents that drop below the threshold are auto-ejected (with hysteresis so they don't flap in and out).

## Know what you're running

- Agents now advertise their **container image digest and capability** in telemetry.
- At dispatch, the Boss checks that image against a curated **trusted-agents registry** in one of three modes — `off`, `warn`, or `strict`. Strict refuses anything unrecognized; a mismatch costs the agent reputation.
- Known equivocators are always refused, regardless of mode.

## A new desktop app

A full **desktop app (the "Boss")**, built with Electron and React, ships alongside the CLI. Discover agents on a live mesh radar, dispatch tasks and watch the output stream in, browse the artifacts they produce, leave signed ratings, and inspect any peer's reputation ledger — all without touching the terminal. The interactive TUI is still there if you prefer it.

## API, SDK, and web viewer

- New reputation endpoints on the Boss HTTP gateway: read a peer's reputation, its full log, an inclusion proof for any entry, and submit signed comments.
- The **Python SDK** gains a `client.reputation` namespace (`get` / `log` / `proof` / `comment`).
- A built-in **web viewer** shows any peer's reputation history at `/ui/peer/{id}`, auto-refreshing.

## One binary, every role

The standalone relay binary is **gone** — it's folded into the main binary. Every role now runs from the same `agentfm` executable:

```sh
agentfm                 # Boss: dispatch tasks + HTTP gateway
agentfm -mode worker    # host agents in Podman sandboxes
agentfm -mode relay     # public lighthouse / circuit relay
agentfm --witness       # co-sign the reputation ledger
```

## Security hardening

Fixes from an internal audit, landed before release:

- Rogue witnesses can no longer forge equivocation alerts against innocent peers.
- Witness extension checks now require a valid consistency proof, closing a fork-at-mismatched-size hole.
- Post-signature validation rejects NaN / infinite / out-of-range scores.
- All protocol framing has hard size caps (witness frames, comment bodies, fetch batches, image cache).

## Install

```sh
curl -fsSL https://api.agentfm.net/install.sh | bash
```

Or grab a binary for your platform from the release assets and `chmod +x` it. They're static, with no dependencies:

| OS | Architectures |
|----|----------------|
| macOS | `darwin_arm64` (Apple Silicon), `darwin_amd64` (Intel) |
| Linux | `amd64`, `arm64`, `arm` (v7), `386`, `riscv64` |
| Windows | `amd64`, `arm64` |
| FreeBSD | `amd64` |

Verify your download against `checksums.txt` (SHA-256).

### Desktop app on macOS

Download the installer for your chip from the assets below — `AgentFM-0.1.0-arm64.dmg` (Apple Silicon) or `AgentFM-0.1.0.dmg` (Intel) — mount it, and drag **AgentFM** to Applications.

The app is not notarized with Apple yet, so macOS blocks the first launch with *"Apple could not verify AgentFM is free of malware."* Allowing it is a one-time step:

1. Double-click **AgentFM** once and dismiss the warning with **Done**.
2. Open **System Settings → Privacy & Security** and scroll down to the Security section — it will say *"AgentFM" was blocked to protect your Mac*. Click **Open Anyway**.
3. Confirm in the dialog that follows (it may ask for your password or Touch ID).

AgentFM opens normally from then on. Prefer the terminal? Clear the download flag before mounting instead:

```sh
xattr -d com.apple.quarantine ~/Downloads/AgentFM-0.1.0-arm64.dmg
```

## Coming later

A few pieces are intentionally deferred: salt-challenge image probes and the real Sigstore Rekor client (v1.3.1); golden-prompt probes, an LLM-judge grader, delegated comments, and an optional TEE attestation tier (v1.4).

## On purpose: no blockchain

AgentFM has no blockchain, no token, no staking, and no on-chain governance, and it never will. Trust here comes from signed logs and witnesses — not consensus or coins.

---

Full technical changelog: [`CHANGELOG.md`](./CHANGELOG.md)
