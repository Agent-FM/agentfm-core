# AgentFM Desktop

A native desktop console for the AgentFM mesh, for **macOS and Windows**. It bundles a full mesh backend, so installing the app gives you a complete node — no separate binary, no terminal.

The desktop app is a **Boss**: it discovers agents, dispatches tasks, streams results, collects artifacts, and lets you rate the agents you use. It talks to the exact same mesh as the [CLI](cli.md) and [Python SDK](../agentfm-python/README.md).

## Install

Download the latest build from **[Releases](https://github.com/Agent-FM/agentfm-core/releases)**:

| Platform | File |
|---|---|
| macOS (Apple Silicon) | `AgentFM-<version>-arm64.dmg` |
| macOS (Intel) | `AgentFM-<version>-x64.dmg` |
| Windows (x64) | `AgentFM-Setup-<version>.exe` |

- **macOS:** open the `.dmg`, drag **AgentFM** to Applications. The build is currently unsigned, so on first launch right-click the app → **Open** to bypass Gatekeeper.
- **Windows:** run the installer. SmartScreen may warn on an unsigned build — choose **More info → Run anyway**.

To run an agent for the app to dispatch to, you still need [Podman](https://podman.io) and (for local LLM agents) [Ollama](https://ollama.com) — see the [worker guide](worker.md). The app orchestrates; workers do the compute.

## Feature tour

### Radar — your mesh at a glance

![Radar](../assets/screenshots/radar.png)

Every agent the app has heard about, live. Each card shows the agent's name, description, capability, hardware, real-time CPU/GPU/queue, and a reputation score rendered as stars. Online agents have a one-click **Dispatch**; offline ones you've seen before keep their name and history. Toggle **Hide offline** to focus on what's live.

### Chat — dispatch and stream

![Chat](../assets/screenshots/chat.png)

Pick an agent, type a prompt, and watch the response stream token-by-token. When the agent produces files, an **artifact** chip appears — open it in Finder/Explorer. Every response can be rated, which signs a receipt into the mesh's trust ledger.

### Agent profile — reputation & trust

![Agent history](../assets/screenshots/history.png)

Click **History** on any agent to see its full reputation view: an overall honesty score (as stars), each individual signed rating and comment, the container image digest (copyable), verified-rater counts, and a live telemetry strip (CPU/GPU/RAM/queue over the last 5 minutes). This is the tamper-evident [trust ledger](trust.md) made visible.

### Assets — everything your agents produced

![Assets](../assets/screenshots/assets.png)

A searchable table of every artifact zip produced across your projects, attributed to the agent that made it, with the prompt, task ID, size, and age. Open any of them directly.

### Projects & the API explorer

- **Projects** let you switch between the public mesh and your own **private swarms** (each project pins its own relay, swarm key, and reputation floor). Switching a project restarts the bundled backend against that project's isolated ledger.
- The **Developer** tab is a built-in API explorer: browse every endpoint, see live examples, and stream `/v1/events`.

## How it maps to the CLI / API

The desktop app is a thin, friendly client over the same HTTP gateway the CLI exposes:

| In the app | Under the hood |
|---|---|
| Radar cards | `GET /api/workers` (telemetry gossip) |
| Dispatch / Chat | `POST /api/execute` (streamed) |
| Rate a response | `POST /v1/peers/:id/comments/self` → signed `Rating` on the ledger |
| Agent profile | `GET /v1/peers/:id/reputation` · `/log` · `/proof` |
| Assets | artifact zips written to the project's workspace |
| Live updates | `GET /v1/events` (SSE) |

Anything you can do in the app you can script — see the [HTTP API reference](http-api.md).

## Building from source

```bash
cd agentfm-desktop
npm install
npm run dev        # hot-reloading dev app
npm run build      # bundle renderer + main into out/
npx electron-builder --mac dmg --arm64   # or --win nsis --x64 on Windows / CI
```

The Windows installer is produced by CI (`.github/workflows/desktop-windows-build.yml`) on a `windows-latest` runner — building an NSIS installer from macOS requires wine and is not reliable. See [development](development.md).
