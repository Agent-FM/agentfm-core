# AgentFM Desktop — Phase 1: Project Model + Critical Bug Fixes

**Status:** Approved 2026-05-18.
**Authors:** Saif (product), Claude (design).
**Scope:** Multi-project architecture + folded-in bug fixes that depend on (or are unblocked by) the project model.
**Out of scope:** Phase 2 features (starred agents, chat reset, chat inspector, assets folder, analytics dashboards, guided tour) and Phase 3 visual overhaul. See *Deferred work* at end.

## 1. Why

Today the desktop app assumes a single global mesh: one relay multiaddr, one reputation floor, one chat-session list, one set of workers. Real usage doesn't look like that — operators run separate meshes (public lighthouse + private team relay + per-customer darknets), and want each to feel isolated. The same operator should keep one boss identity across all of them so their reputation history follows them.

Several of the active bug reports — black-screen navigation, "unknown" agent labels, comments not appearing, my-activity showing raw peer IDs — are either downstream of unscoped global state, or are tangled with how data is loaded. We fold those into Phase 1 so we don't ship the refactor with regressions in plain sight.

## 2. User-facing changes

### 2.1 Top-level UI shape

- **Sidebar** grows a *Projects* section at the top (collapsible). It lists every project with its emoji icon and color chip; active project is highlighted. A `+ new project` button sits at the bottom of that section. Below the projects list, the existing tab icons (Radar / Chat / Activity / Status / Settings) remain — these are now scoped to the *active* project.
- **Top bar** restructures into three columns: `[left: project pill] [center: AgentFM wordmark] [right: online-counts link]`. The project pill shows the active project's icon + name and opens a project-settings sheet on click. The wordmark is the centered visual anchor that was missing.
- **No active project** (fresh install or all projects deleted): the main pane shows a full-screen *Create your first project* wizard. This replaces the previous standalone welcome modal — the welcome's "configure relay" step now sits inside the project-create form.

### 2.2 Project switching

- Clicking another project in the sidebar dispatches a `switchProject(id)` action.
- A full-pane overlay appears: *Switching to {name}…* with a subtle progress shimmer.
- Under the hood: query cache is cleared, electron-store's `activeProjectId` is updated, the backend is restarted via `window.api.backend.restart({ apiPort, relayMultiaddr, reputationFloor })` with the new project's values. The overlay dismisses on `backend:health` resolving `ok`.
- If restart fails (relay unreachable, port held), the overlay shows the error with *Try again* / *Edit project* buttons.

### 2.3 Per-project settings vs. app settings

- `Settings` page keeps only **app-level** controls: theme, accent, API port, telemetry, backend version display, *Reset all to defaults*.
- **Project-level** controls live on a dedicated *Project settings* sheet, opened by clicking the project pill in the top bar. It has: name, icon, color, relay multiaddr (with "test connection"), reputation floor, danger zone (delete project — disabled if it's the only project).

## 3. Data model

### 3.1 Project type

```ts
export interface Project {
  id: string                                   // 'prj_' + 8 random base32 chars
  name: string
  icon: string                                 // single emoji
  color: 'emerald' | 'violet' | 'rose' | 'cyan' | 'amber'
  relayMultiaddr: string | null                // null = bundled public lighthouse
  reputationFloor: number                      // [-1, 0]
  createdAt: number                            // epoch ms
}
```

`swarmKeyPath` and `tags` are reserved for Phase 2 (private mesh + organization) and **not** added in this phase to keep the migration narrow.

### 3.2 Zustand store additions

```ts
interface UIState {
  // ... existing fields except relayMultiaddr / reputationFloor (now project-owned) ...
  projects: Project[]
  activeProjectId: string | null
  isProjectSwitching: boolean

  addProject(p: Omit<Project, 'id' | 'createdAt'>): Project
  updateProject(id: string, patch: Partial<Project>): void
  deleteProject(id: string): void
  switchProject(id: string): Promise<void>     // triggers backend restart
  activeProject(): Project | undefined         // derived selector
}
```

`relayMultiaddr` and `reputationFloor` are **removed** as standalone fields on `UIState`. Any code that reads them today must read from `activeProject()` instead. This is enforced by the migration and a typecheck pass.

### 3.3 Persistence (electron-store keys)

| Key                                 | Type            | Notes                                                      |
| ----------------------------------- | --------------- | ---------------------------------------------------------- |
| `projects`                          | `Project[]`     | Source of truth                                            |
| `activeProjectId`                   | `string \| null` | Active project on next launch                              |
| `chat:sessions:{projectId}`         | `ChatSession[]` | Replaces the old global `chat:sessions`                    |
| `theme`, `accent`, `apiPort`, `telemetry` | unchanged       | App-level                                                  |
| `relayMultiaddr`, `reputationFloor` | **removed after migration** | If present on load, migrated into a "Default" project, then deleted |

### 3.4 Uniqueness invariant

A non-null `relayMultiaddr` may appear at most once across the `projects` array. `addProject` and `updateProject` validate and throw `DuplicateRelayError` on violation. The wizard UI surfaces this as inline error text on the relay field. `null` is treated as a sentinel ("bundled lighthouse") and is allowed in at most one project — second time, the user must pick a custom multiaddr. This enforces the user's rule: *no two projects share a relay*.

## 4. Boss identity (one per user, persistent)

The Go binary must always start with the same libp2p Ed25519 keypair, regardless of which project is active.

### 4.1 Current state — to verify

The boss runs in `-mode api` and writes ledger state to `~/.agentfm/api_ledger.db`. The libp2p identity is currently generated by `network.Setup` (see `agentfm-core/agentfm-go/internal/network/`). Whether it's persisted to disk between runs is the unknown.

### 4.2 Plan

- **Verify** by inspecting `network.Setup` and `createHost` in `agentfm-core/agentfm-go/internal/network/`.
- If the key is persisted to a stable path (e.g. `~/.agentfm/api_identity.key` or similar): no change needed; document the path.
- If the key is regenerated per run: add identity-key persistence. Path: `${app.getPath('userData')}/identity.key` written by the *desktop app* on first run, then passed to the Go binary via `-identity` flag (or env var if no such flag exists; we may need to add one to the Go binary).
- Either way, after Phase 1 the `boss_peer_id` returned by `/v1/about` must be stable across:
  - Project switches.
  - App restarts.
  - Backend crashes / auto-restarts.

### 4.3 Cross-project implication

The same boss_peer_id appears on every mesh the operator joins. From any single relay's vantage, this is just a libp2p peer that connects, dials workers, and signs comments. There's no leakage of activity between meshes — each mesh sees only its own traffic. The user's signed ratings/comments accumulate against their boss identity within each project's ledger view.

## 5. UI architecture

### 5.1 Component tree (delta only)

```
Shell
├── TopBar
│   ├── ProjectPill           // NEW — opens ProjectSettingsSheet
│   ├── AppLogo               // centered
│   └── OnlineCountsLink      // existing, right-justified
├── Sidebar
│   ├── ProjectList           // NEW — list of projects + active highlight
│   ├── NewProjectButton      // NEW
│   ├── nav icons (existing)
├── ProjectSwitchingOverlay   // NEW — full-pane during restart
├── ProjectSettingsSheet      // NEW — opened by ProjectPill
├── CreateProjectWizard       // NEW — used by NewProjectButton + empty state
└── Routes (existing)
```

### 5.2 Route-level error boundaries

Each route (`Radar`, `Chat`, `PeerView`, `Activity`, `Status`, `Settings`) is wrapped in its own `RouteErrorBoundary`. The boundary renders a small inline error card with *Retry* and *Go to Radar* buttons, not a full-pane crash. This is what fixes the *black screen on nav* — today a thrown error inside `useQuery` selectors during a state transition can leave the app pane blank.

### 5.3 Display-name helper

```ts
// src/lib/displayName.ts
export function displayName(w: Pick<WorkerProfile, 'name' | 'agent_capability' | 'agent_image_ref' | 'peer_id'>): string {
  if (w.name && w.name.trim()) return w.name.trim()
  if (w.agent_capability) return w.agent_capability
  if (w.agent_image_ref) {
    const tail = w.agent_image_ref.split('/').pop() ?? w.agent_image_ref
    return tail.split(':')[0]                 // strip tag
  }
  return shortenPeerID(w.peer_id, 6, 5)
}
```

Replaces ad-hoc `worker.name || '(unknown)'` and `worker.name || '(unknown agent)'` strings in: `AgentCard`, `AgentPicker`, `MessageBubble` (when rater is resolvable), `DispatchDrawer`, `PeerView` heading.

### 5.4 Resolve peer → name for the My Activity screen

```ts
// src/hooks/usePeerName.ts
export function usePeerName(peerId: string): string {
  const { data } = useWorkers(true)
  const w = data?.agents.find((a) => a.peer_id === peerId)
  return w ? displayName(w) : shortenPeerID(peerId, 8, 6)
}
```

`Activity.tsx` row header changes from `about peer 12D3Koo…` to `about HR Specialist` (with the peer ID in a `title` tooltip and in the EntryRow below).

## 6. Bug fix details

### 6.1 Black-screen on navigation

**Suspected cause** — when navigating away from a route mid-stream (e.g., chat) or mid-fetch (radar), the in-flight promise resolves into a now-unmounted query observer. Combined with the top-level `ErrorBoundary` catching the resulting thrown value, the whole `<Routes>` subtree unmounts.

**Fix** — `RouteErrorBoundary` per route limits blast radius. Plus: in `useDispatch`/`useChat`, every `setState`-after-fetch path is gated on `mountedRef.current` (we already abort the fetch, but the rAF commit callback could fire after unmount). Add `mountedRef` and bail.

**Verification** — manual repro: open dev, fire chat, immediately press Cmd+1 mid-stream. Then assert via Playwright (new e2e: `tests/e2e/nav-stress.spec.ts`) that hammering nav keys after a dispatch never produces an empty `<main>`.

### 6.2 Chat horizontal scroll, no exit

**Cause** — worker stdout includes ANSI escape sequences and Unicode box-drawing characters that form very long unbroken runs. `whitespace-pre-wrap` preserves the formatting but `break-words` won't split inside a non-breaking glyph sequence. The bubble overflows its 75% width, pushes the column, and the chat container ends up scrollable horizontally. The Composer also goes off-screen if the container scrolls, hence "no way to exit".

**Fix**
1. `MessageBubble` body: `overflow-wrap: anywhere` and `min-w-0`.
2. Chat scroll container: `overflow-x: hidden` (was implicit, make it explicit).
3. Strip raw ANSI escape codes (`\x1b\[[0-9;]*[a-zA-Z]`) from streamed content before storing. Box-drawing chars are kept (they're meaningful), but they wrap.
4. Chat header gains a persistent *Back to Radar* link so the user is never stuck.

### 6.3 "Unknown" agent labels

Covered by §5.3 `displayName` helper. Verify by adding a unit test fixture for each fallback rung.

### 6.4 Comments don't show after submission

Re-audit the invalidation in `FeedbackModal.submit`:
- Cache key today: `['peer-log', peerId, opts.limit ?? 50, opts.offset ?? 0]`.
- Invalidation called: `qc.invalidateQueries({ queryKey: qk.peerLog(ctx.peerId) })` — but `qk.peerLog` requires opts; called with no opts it defaults to limit 50/offset 0, missing the Activity screen's `limit: 200` call.
- **Fix**: invalidate by prefix — `qc.invalidateQueries({ queryKey: ['peer-log', ctx.peerId] })`. React Query matches all subkeys starting with this prefix.

### 6.5 My Activity peer ID → agent name

Covered by §5.4 `usePeerName` hook. The row header becomes: `about <strong>{displayName}</strong> <span class="font-mono text-text-2">({shortPeerId})</span>`. Clicking still navigates to `/peer/{id}`.

### 6.6 AgentFM title centering

`TopBar` is restructured: `flex justify-between items-center` → three explicit columns each `flex-1`, with the wordmark centered. The online-counts link drops to the right column.

## 7. Migration

On app launch (in `main.tsx` before first render):

1. Read `projects`. If non-empty, done.
2. Otherwise (legacy install): read legacy `relayMultiaddr` and `reputationFloor` (if present). Construct a Default project:
   ```ts
   { id: 'prj_default', name: 'Default', icon: '🌐', color: 'emerald',
     relayMultiaddr: legacyRelay, reputationFloor: legacyFloor ?? -0.5,
     createdAt: Date.now() }
   ```
   Persist as the only project. Set `activeProjectId = 'prj_default'`.
3. Migrate `chat:sessions` → `chat:sessions:prj_default`. Delete the legacy `chat:sessions` key.
4. Delete the legacy `relayMultiaddr` and `reputationFloor` keys.

Idempotent: re-running migration does nothing once `projects` is non-empty.

## 8. Testing strategy

### 8.1 Unit (vitest)
- `displayName.test.ts`: name → capability → image_ref tail → peer_id fallback ladder.
- `project-store.test.ts`: add / update / delete / switch / uniqueness invariant.
- `migration.test.ts`: legacy keys → default project, idempotency.
- `chat-stream.test.ts`: ANSI strip leaves printable content intact; wrapping works on long unbroken glyphs.

### 8.2 E2E (playwright)
- `projects.spec.ts`: fresh launch shows wizard; create project; switch projects; relay-uniqueness validation.
- `nav-stress.spec.ts`: hammer Cmd+1..5 during a dispatch; assert no blank `<main>`.
- Extend existing `happy-path.spec.ts` to assert Activity row shows an agent display name (not a bare peer_id).

### 8.3 Manual smoke
- Submit a comment via FeedbackModal, confirm it shows in Activity *and* on PeerView within ≤2s.
- Create two projects with different relays, switch between them, confirm Radar shows the correct workers and the boss_peer_id is identical on both.
- Send a very long ANSI-laden chat response; confirm no horizontal scroll, Composer remains visible, *Back to Radar* link works.

## 9. Risks & open questions

- **Boss identity persistence on Go side** — depends on what `network.Setup` does today. If it generates fresh, we need to add a `-identity` flag and may need to ship a Go-side change in lockstep. Phase 1 cannot complete until this is verified and (if needed) added.
- **Chat session migration ergonomics** — when a user deletes a project, do we delete its chat sessions too? Decision: yes, with confirmation. Documented in the danger zone.
- **Project deletion when active** — switch to the next project in the list; if none, show the empty-state wizard.
- **Identity collision across machines** — two installs of the desktop app on different machines generate different keys. Same user, different boss_peer_ids. That's fine for now; multi-device identity sync is out of scope.

## 10. Deferred work (Phase 2/3)

- Phase 2 features: starred agents per project · chat reset · chat agent inspector (capabilities, honesty, hardware) · assets folder · analytics dashboards via `/metrics` · guided onboarding tour overlay · swarm-key support for private meshes.
- Phase 3: visual overhaul — colorful gradient theme, richer Framer Motion language, micro-interactions, custom skill-based components.
