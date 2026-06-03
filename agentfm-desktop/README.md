# AgentFM Desktop

Cross-platform Electron app for the [AgentFM](https://agentfm.net) P2P agent mesh.
Browse agents, dispatch tasks, leave signed feedback, manage your reputation gate
— all in a sleek dark dev-tool aesthetic UI. Bundled backend; no terminal needed.

## Screens

8 routes covering the entire v1.3.1 trust layer:

- **Radar** — online + offline agents with honesty, image digest, dispatch state
- **Dispatch (drawer)** — prompt → streaming output → artifact download
- **Chat** — multi-turn OpenAI-compatible conversation with auto-routing or pinned peer
- **Peer view** — full ledger history (ratings + comments) per peer
- **Activity** — every entry you've signed and broadcast
- **Status** — six dashboard cards covering backend / identity / ledger / relay / workers / trust gate
- **Settings** — relay multiaddr, reputation floor, theme, accent color, telemetry opt-in
- **Feedback modal** — signed comment + rating after a task

## Install

### From source (development)

```bash
git clone https://github.com/Agent-FM/agentfm-desktop.git
cd agentfm-desktop
npm install
./scripts/dev.sh
```

This auto-builds the local `agentfm` Go binary (if missing) and launches Electron + Vite.

### Cross-platform binaries

```bash
./scripts/build-binaries.sh    # cross-compiles darwin/arm64, darwin/amd64, linux/amd64, windows/amd64
npm run package                # produces dist/AgentFM-*.dmg / .AppImage / .exe
```

## First run

The app spawns a bundled `agentfm -mode api` on launch. On first run the backend
generates a fresh libp2p identity and tries to connect to the public lighthouse.
If you want a private mesh, override the relay in **Settings → Mesh**.

If macOS Gatekeeper blocks the unsigned app, **right-click → Open** instead of
double-clicking (the v1 release is unsigned; notarization lands in v1.1).

## Architecture

- **Main process** (Node.js): spawns and supervises the bundled `agentfm` Go binary
  on `127.0.0.1:8080`. Auto-restart up to 3× in 60s before showing a backend-down overlay.
- **Renderer** (React + TanStack Query): pure HTTP/SSE client to the backend.
- **State**: TanStack Query for server state, Zustand for UI state, electron-store for
  persistence (theme, accent, relay multiaddr, reputation floor, chat sessions).
- **Streaming**: SSE for `/v1/chat/completions` and `/v1/events`; fetch + ReadableStream
  for `/api/execute` raw text streams.

## Backend additions (v1.3.1 → v1.3.2-rc)

Three new HTTP endpoints land in `agentfm-core` to support this UI:

- `GET /v1/about` — backend identity + uptime + ledger size
- `GET /v1/events` — SSE event stream (worker online/offline, entry appended)
- `GET /v1/peers/{id}/comments/{cid}.json` — JSON-wrapped comment body

## Keyboard shortcuts

| Key | Action |
|---|---|
| `⌘1` | Jump to Radar |
| `⌘2` | Jump to Chat |
| `⌘3` | Jump to Activity |
| `⌘4` | Jump to Status |
| `⌘5` | Jump to Settings |
| `⌘K` | Focus Radar search |
| `⌘↵` | Submit (dispatch / feedback) |
| `Esc` | Close drawers/modals |

## Known limitations (v1)

- macOS builds are unsigned and not notarized → "right-click → Open" workaround.
- Activity screen uses N+1 fan-out (one log query per known peer); fine up to ~30 peers.
- Standalone signed comments (`POST /v1/peers/{id}/comments` with client-side Ed25519) are
  scaffolded but not wired — the Feedback modal currently posts via `/api/execute` only.

## Tech stack

Electron 30 · React 18 · TypeScript 5 · Vite 5 · TailwindCSS 3 · Framer Motion 11 ·
TanStack Query 5 · Zustand 4 · react-hook-form + zod · sonner · vitest · Playwright.

## License

MIT
