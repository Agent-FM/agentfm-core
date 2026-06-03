# Project Model + Critical Bug Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat single-mesh model with a multi-project architecture where each project owns its relay + reputation floor + chat sessions; persistent boss identity is shared across projects; fix the navigation, chat-overflow, agent-naming, comment-visibility, and topbar-centering bugs that block daily use.

**Architecture:** Frontend introduces a `Project` Zustand entity persisted in electron-store. Switching a project triggers a backend restart with the project's relay/threshold. Existing routes are scoped to the active project. Boss libp2p identity is already persistent on the Go side (verified — `network.LoadOrGenerateIdentity` writes `.agentfm_api_identity.key` to the BackendManager's cwd `{userData}/workspace/`), so no Go changes are needed.

**Tech Stack:** React 18 + TypeScript, Zustand (UI state), electron-store (persistence), TanStack Query (server cache), React Router (HashRouter), Framer Motion, Tailwind, Vitest, Playwright. Go (read-only — verified).

**Reference spec:** `docs/superpowers/specs/2026-05-18-project-model-design.md`.

---

## File map

**Create:**
- `src/types/project.ts`
- `src/lib/displayName.ts`
- `src/lib/projectStore.ts` — pure project-state operations, exported for the Zustand integration in `lib/store.ts`
- `src/lib/projectMigration.ts`
- `src/lib/ansi.ts`
- `src/hooks/usePeerName.ts`
- `src/components/RouteErrorBoundary.tsx`
- `src/components/projects/ProjectList.tsx`
- `src/components/projects/ProjectPill.tsx`
- `src/components/projects/CreateProjectWizard.tsx`
- `src/components/projects/ProjectSettingsSheet.tsx`
- `src/components/projects/ProjectSwitchingOverlay.tsx`
- `tests/unit/displayName.test.ts`
- `tests/unit/projectStore.test.ts`
- `tests/unit/projectMigration.test.ts`
- `tests/unit/ansi.test.ts`
- `tests/e2e/projects.spec.ts`
- `tests/e2e/nav-stress.spec.ts`

**Modify:**
- `src/lib/store.ts` — add `projects`, `activeProjectId`, actions; remove `relayMultiaddr` & `reputationFloor` as standalone fields
- `src/lib/sessions.ts` — key sessions by project ID
- `src/main.tsx` — run migration before render
- `src/hooks/useChat.ts` — ANSI strip, per-project session key, mountedRef guard
- `src/components/Shell.tsx` — wrap `<Outlet />` with `RouteErrorBoundary`
- `src/components/TopBar.tsx` — three-column layout, `<ProjectPill />`
- `src/components/Sidebar.tsx` — mount `<ProjectList />` + new-project button
- `src/components/AgentCard.tsx` — `displayName(worker)`
- `src/components/chat/AgentPicker.tsx` — `displayName`
- `src/components/chat/MessageBubble.tsx` — wrap, `displayName`, `overflow-wrap: anywhere`
- `src/components/DispatchDrawer.tsx` — `displayName`
- `src/components/FeedbackModal.tsx` — invalidation by prefix
- `src/components/WelcomeModal.tsx` — open `<CreateProjectWizard />` if no projects, else legacy welcome
- `src/routes/Radar.tsx` — read `activeProject().reputationFloor` for filter copy
- `src/routes/Chat.tsx` — back link, `overflow-x-hidden`
- `src/routes/Activity.tsx` — `usePeerName`
- `src/routes/PeerView.tsx` — `displayName`
- `src/routes/Settings.tsx` — drop project-level fields (moved to sheet)
- `src/routes/Status.tsx` — read floor from active project
- `src/App.tsx` — mount `<ProjectSwitchingOverlay />` + `<ProjectSettingsSheet />`
- `tests/e2e/happy-path.spec.ts` — dismiss wizard instead of welcome on first launch

---

## Task list

### Task 1: Verify and document boss identity persistence

**Files:**
- Modify: `docs/superpowers/specs/2026-05-18-project-model-design.md` (note resolution)

No code changes — verification only.

- [ ] **Step 1: Verify identity file path matches expectations**

Run from the project root:

```bash
ls -la "$HOME/Library/Application Support/agentfm-desktop/workspace/.agentfm_api_identity.key" 2>&1 | head -3
```

Expected: file exists, owner saif, mode 0600 (or similar restrictive perms).

- [ ] **Step 2: Cross-check the path is set by BackendManager**

Read `electron/backend-manager.ts:39-58` and confirm `this.artifactsDir = join(app.getPath('userData'), 'workspace')` then `spawn(bin, args, { ..., cwd: this.artifactsDir })`. Confirm there is no code path that changes cwd later in `start()`.

- [ ] **Step 3: Confirm identity reuse across two consecutive launches**

```bash
curl -s http://127.0.0.1:8080/v1/about | python3 -c "import sys,json;print(json.load(sys.stdin)['boss_peer_id'])"
```

Note the printed peer ID. Restart the backend via DevTools (`window.api.backend.restart()`), wait 3 s, run the curl again. The two peer IDs must be identical.

- [ ] **Step 4: Update spec §4 with resolution**

Edit `docs/superpowers/specs/2026-05-18-project-model-design.md`. Replace §4.2 "Plan" with:

```markdown
### 4.2 Resolution (2026-05-18)

Verified: `network.LoadOrGenerateIdentity` (in `agentfm-core/agentfm-go/internal/network/host.go:20`) writes `.agentfm_<mode>_identity.key` to the working directory. `BackendManager.start()` sets `cwd: {userData}/workspace`, so the desktop app's boss identity persists at:

`{userData}/workspace/.agentfm_api_identity.key`

No Go-side changes are needed. The path is stable across backend restarts, crash recoveries, and project switches because cwd never changes.
```

- [ ] **Step 5: Commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git add docs/superpowers/specs/2026-05-18-project-model-design.md
git commit -m "docs(spec): resolve identity-persistence risk (already persisted)"
```

---

### Task 2: Project type

**Files:**
- Create: `src/types/project.ts`

- [ ] **Step 1: Write the type module**

`src/types/project.ts`:

```ts
export type ProjectColor = 'emerald' | 'violet' | 'rose' | 'cyan' | 'amber'

export interface Project {
  id: string
  name: string
  icon: string                          // single emoji
  color: ProjectColor
  relayMultiaddr: string | null         // null = bundled public lighthouse
  reputationFloor: number               // [-1, 0]
  createdAt: number                     // epoch ms
}

export const PROJECT_COLORS: ProjectColor[] = [
  'emerald',
  'violet',
  'rose',
  'cyan',
  'amber',
]
```

- [ ] **Step 2: Commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git add src/types/project.ts
git commit -m "feat(types): add Project type"
```

---

### Task 3: Project store helpers (pure functions)

These are framework-free helpers exported for `lib/store.ts` to compose into Zustand actions. Keeping them pure makes them trivially unit-testable.

**Files:**
- Create: `src/lib/projectStore.ts`
- Create: `tests/unit/projectStore.test.ts`

- [ ] **Step 1: Write the failing test**

`tests/unit/projectStore.test.ts`:

```ts
import { describe, it, expect } from 'vitest'
import {
  newProjectId,
  validateProjectInput,
  DuplicateRelayError,
} from '../../src/lib/projectStore'
import type { Project } from '../../src/types/project'

const base = (overrides: Partial<Project> = {}): Project => ({
  id: 'prj_test',
  name: 'Default',
  icon: '🌐',
  color: 'emerald',
  relayMultiaddr: null,
  reputationFloor: -0.5,
  createdAt: 0,
  ...overrides,
})

describe('newProjectId', () => {
  it('returns prj_-prefixed ids', () => {
    const id = newProjectId()
    expect(id).toMatch(/^prj_[a-z0-9]{8}$/)
  })
  it('returns distinct ids on repeated calls', () => {
    expect(newProjectId()).not.toBe(newProjectId())
  })
})

describe('validateProjectInput', () => {
  it('passes when name is set and relay is unique', () => {
    expect(() =>
      validateProjectInput([base({ relayMultiaddr: '/ip4/1.2.3.4/tcp/4001/p2p/12D3Test' })],
        { name: 'Two', relayMultiaddr: '/ip4/5.6.7.8/tcp/4001/p2p/12D3Other' }),
    ).not.toThrow()
  })

  it('rejects empty name', () => {
    expect(() => validateProjectInput([], { name: '   ', relayMultiaddr: null })).toThrow(
      /name is required/i,
    )
  })

  it('throws DuplicateRelayError when relay is already used', () => {
    expect(() =>
      validateProjectInput(
        [base({ relayMultiaddr: '/ip4/1.2.3.4/tcp/4001/p2p/12D3Same' })],
        { name: 'New', relayMultiaddr: '/ip4/1.2.3.4/tcp/4001/p2p/12D3Same' },
      ),
    ).toThrow(DuplicateRelayError)
  })

  it('treats null relays as one slot only', () => {
    expect(() =>
      validateProjectInput([base({ relayMultiaddr: null })], {
        name: 'Second default',
        relayMultiaddr: null,
      }),
    ).toThrow(DuplicateRelayError)
  })

  it('allows null when editing the same project that already owns null', () => {
    expect(() =>
      validateProjectInput(
        [base({ id: 'prj_keep', relayMultiaddr: null })],
        { name: 'Default renamed', relayMultiaddr: null },
        'prj_keep',
      ),
    ).not.toThrow()
  })

  it('rejects out-of-range reputationFloor', () => {
    expect(() =>
      validateProjectInput([], {
        name: 'x',
        relayMultiaddr: null,
        reputationFloor: 0.5,
      }),
    ).toThrow(/reputation floor/i)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx vitest run tests/unit/projectStore.test.ts
```

Expected: FAIL with "Cannot find module".

- [ ] **Step 3: Write the implementation**

`src/lib/projectStore.ts`:

```ts
import type { Project } from '../types/project'

export class DuplicateRelayError extends Error {
  constructor(public relay: string | null) {
    super(
      relay === null
        ? 'another project already uses the bundled public lighthouse'
        : `another project already uses ${relay}`,
    )
    this.name = 'DuplicateRelayError'
  }
}

export function newProjectId(): string {
  const tail = Math.random().toString(36).slice(2, 10).padEnd(8, '0')
  return 'prj_' + tail
}

export interface ProjectInput {
  name: string
  relayMultiaddr: string | null
  reputationFloor?: number
  icon?: string
  color?: Project['color']
}

export function validateProjectInput(
  existing: Project[],
  input: ProjectInput,
  editingId?: string,
): void {
  if (!input.name || !input.name.trim()) {
    throw new Error('Project name is required')
  }
  const floor = input.reputationFloor ?? -0.5
  if (floor < -1 || floor > 0) {
    throw new Error('Reputation floor must be between -1.0 and 0.0')
  }
  const conflict = existing.find(
    (p) => p.relayMultiaddr === input.relayMultiaddr && p.id !== editingId,
  )
  if (conflict) throw new DuplicateRelayError(input.relayMultiaddr)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
npx vitest run tests/unit/projectStore.test.ts
```

Expected: PASS, 6 tests.

- [ ] **Step 5: Commit**

```bash
git add src/lib/projectStore.ts tests/unit/projectStore.test.ts
git commit -m "feat(projects): pure store helpers with uniqueness validation"
```

---

### Task 4: displayName helper

**Files:**
- Create: `src/lib/displayName.ts`
- Create: `tests/unit/displayName.test.ts`

- [ ] **Step 1: Write the failing test**

`tests/unit/displayName.test.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { displayName } from '../../src/lib/displayName'

const peer = '12D3KooWMP2KCh1qKk6PPw8oH6GXUwRYjMYEbGYdzMZ5fygdt4Es'

describe('displayName', () => {
  it('prefers explicit name', () => {
    expect(
      displayName({
        peer_id: peer,
        name: 'HR Specialist',
        agent_capability: 'hr',
        agent_image_ref: 'ghcr.io/yourorg/hr-agent:v1',
      }),
    ).toBe('HR Specialist')
  })

  it('trims whitespace-only names and falls back to capability', () => {
    expect(
      displayName({
        peer_id: peer,
        name: '   ',
        agent_capability: 'hr-specialist',
      }),
    ).toBe('hr-specialist')
  })

  it('uses image basename (strip tag) when no name or capability', () => {
    expect(
      displayName({
        peer_id: peer,
        agent_image_ref: 'ghcr.io/yourorg/hr-agent:v1',
      }),
    ).toBe('hr-agent')
  })

  it('handles an image_ref without a tag', () => {
    expect(
      displayName({ peer_id: peer, agent_image_ref: 'local/notebook' }),
    ).toBe('notebook')
  })

  it('falls back to short peer id when nothing else is set', () => {
    expect(displayName({ peer_id: peer })).toMatch(/^12D3Ko…/)
  })

  it('handles totally empty input gracefully', () => {
    expect(displayName({ peer_id: '' })).toBe('(unknown agent)')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
npx vitest run tests/unit/displayName.test.ts
```

Expected: FAIL with "Cannot find module".

- [ ] **Step 3: Implement the helper**

`src/lib/displayName.ts`:

```ts
import { shortenPeerID } from './peer'

interface NameSource {
  name?: string
  agent_capability?: string
  agent_image_ref?: string
  peer_id: string
}

export function displayName(w: NameSource): string {
  if (w.name && w.name.trim()) return w.name.trim()
  if (w.agent_capability && w.agent_capability.trim()) return w.agent_capability.trim()
  if (w.agent_image_ref && w.agent_image_ref.trim()) {
    const tail = w.agent_image_ref.split('/').pop() ?? w.agent_image_ref
    const noTag = tail.split(':')[0]
    if (noTag) return noTag
  }
  if (w.peer_id) return shortenPeerID(w.peer_id, 6, 5)
  return '(unknown agent)'
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
npx vitest run tests/unit/displayName.test.ts
```

Expected: PASS, 6 tests.

- [ ] **Step 5: Commit**

```bash
git add src/lib/displayName.ts tests/unit/displayName.test.ts
git commit -m "feat(lib): displayName fallback ladder for worker names"
```

---

### Task 5: ANSI strip helper

Worker stdout often contains ANSI escape codes from rich-text loggers. They render as garbage in the chat bubble and break line wrapping.

**Files:**
- Create: `src/lib/ansi.ts`
- Create: `tests/unit/ansi.test.ts`

- [ ] **Step 1: Write the failing test**

`tests/unit/ansi.test.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { stripAnsi } from '../../src/lib/ansi'

describe('stripAnsi', () => {
  it('removes color escape codes', () => {
    expect(stripAnsi('\x1b[92mhello\x1b[0m world')).toBe('hello world')
  })

  it('removes 256-color codes', () => {
    expect(stripAnsi('\x1b[38;5;202mhi\x1b[0m')).toBe('hi')
  })

  it('removes cursor-motion codes', () => {
    expect(stripAnsi('a\x1b[2Kclear\x1b[1Aup')).toBe('aclearup')
  })

  it('leaves regular text untouched', () => {
    expect(stripAnsi('plain text')).toBe('plain text')
  })

  it('preserves box-drawing characters', () => {
    const box = '╭──────╮\n│ hi   │\n╰──────╯'
    expect(stripAnsi(box)).toBe(box)
  })

  it('handles empty input', () => {
    expect(stripAnsi('')).toBe('')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
npx vitest run tests/unit/ansi.test.ts
```

Expected: FAIL with "Cannot find module".

- [ ] **Step 3: Implement**

`src/lib/ansi.ts`:

```ts
// Matches CSI sequences: ESC [ <params> <final byte>
// and the rarer OSC sequences terminated by BEL or ST.
const CSI = /\x1b\[[0-?]*[ -/]*[@-~]/g
const OSC = /\x1b\][^\x07\x1b]*(\x07|\x1b\\)/g

export function stripAnsi(s: string): string {
  if (!s) return s
  return s.replace(CSI, '').replace(OSC, '')
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
npx vitest run tests/unit/ansi.test.ts
```

Expected: PASS, 6 tests.

- [ ] **Step 5: Commit**

```bash
git add src/lib/ansi.ts tests/unit/ansi.test.ts
git commit -m "feat(lib): stripAnsi helper for worker stdout sanitisation"
```

---

### Task 6: Project migration

**Files:**
- Create: `src/lib/projectMigration.ts`
- Create: `tests/unit/projectMigration.test.ts`

- [ ] **Step 1: Write the failing test**

`tests/unit/projectMigration.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { migrateLegacySettings, DEFAULT_PROJECT_ID } from '../../src/lib/projectMigration'

interface Store {
  get<T>(key: string): Promise<T | undefined>
  set(key: string, value: unknown): Promise<void>
  delete(key: string): Promise<void>
}

function fakeStore(initial: Record<string, unknown>): Store {
  const data: Record<string, unknown> = { ...initial }
  return {
    get: async <T,>(k: string) => data[k] as T | undefined,
    set: async (k: string, v: unknown) => {
      data[k] = v
    },
    delete: async (k: string) => {
      delete data[k]
    },
  }
}

describe('migrateLegacySettings', () => {
  it('does nothing when projects already exist', async () => {
    const store = fakeStore({
      projects: [{ id: 'prj_keep', name: 'X', icon: '🌐', color: 'emerald', relayMultiaddr: null, reputationFloor: -0.5, createdAt: 1 }],
    })
    await migrateLegacySettings(store)
    expect(await store.get('projects')).toHaveLength(1)
  })

  it('creates a Default project from legacy relay + floor', async () => {
    const store = fakeStore({
      relayMultiaddr: '/ip4/127.0.0.1/tcp/4001/p2p/12D3LegacyTest',
      reputationFloor: -0.3,
      'chat:sessions': [{ id: 'chat_x', title: 'legacy', pinnedPeerId: null, preferredModel: 'auto', messages: [], createdAt: 1, updatedAt: 1 }],
    })
    await migrateLegacySettings(store)
    const projects = (await store.get('projects')) as unknown[]
    expect(projects).toHaveLength(1)
    expect((projects[0] as { id: string }).id).toBe(DEFAULT_PROJECT_ID)
    expect((projects[0] as { relayMultiaddr: string }).relayMultiaddr).toBe(
      '/ip4/127.0.0.1/tcp/4001/p2p/12D3LegacyTest',
    )
    expect((projects[0] as { reputationFloor: number }).reputationFloor).toBe(-0.3)
    expect(await store.get('activeProjectId')).toBe(DEFAULT_PROJECT_ID)
    expect(await store.get(`chat:sessions:${DEFAULT_PROJECT_ID}`)).toHaveLength(1)
    expect(await store.get('chat:sessions')).toBeUndefined()
    expect(await store.get('relayMultiaddr')).toBeUndefined()
    expect(await store.get('reputationFloor')).toBeUndefined()
  })

  it('uses defaults when legacy fields are missing', async () => {
    const store = fakeStore({})
    await migrateLegacySettings(store)
    const projects = (await store.get('projects')) as unknown[]
    expect(projects).toHaveLength(1)
    expect((projects[0] as { relayMultiaddr: string | null }).relayMultiaddr).toBeNull()
    expect((projects[0] as { reputationFloor: number }).reputationFloor).toBe(-0.5)
  })

  it('is idempotent', async () => {
    const store = fakeStore({})
    await migrateLegacySettings(store)
    const after1 = await store.get('projects')
    await migrateLegacySettings(store)
    const after2 = await store.get('projects')
    expect(after2).toEqual(after1)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
npx vitest run tests/unit/projectMigration.test.ts
```

Expected: FAIL with "Cannot find module".

- [ ] **Step 3: Implement**

`src/lib/projectMigration.ts`:

```ts
import type { Project } from '../types/project'
import type { ChatSession } from '../types/chat'

export const DEFAULT_PROJECT_ID = 'prj_default'

interface MigrationStore {
  get<T>(key: string): Promise<T | undefined>
  set(key: string, value: unknown): Promise<void>
  delete(key: string): Promise<void>
}

export async function migrateLegacySettings(store: MigrationStore): Promise<void> {
  const existing = await store.get<Project[]>('projects')
  if (Array.isArray(existing) && existing.length > 0) return

  const legacyRelay = (await store.get<string | null>('relayMultiaddr')) ?? null
  const legacyFloor = (await store.get<number>('reputationFloor')) ?? -0.5
  const legacySessions = (await store.get<ChatSession[]>('chat:sessions')) ?? []

  const defaultProject: Project = {
    id: DEFAULT_PROJECT_ID,
    name: 'Default',
    icon: '🌐',
    color: 'emerald',
    relayMultiaddr: legacyRelay,
    reputationFloor: legacyFloor,
    createdAt: Date.now(),
  }

  await store.set('projects', [defaultProject])
  await store.set('activeProjectId', DEFAULT_PROJECT_ID)
  if (legacySessions.length > 0) {
    await store.set(`chat:sessions:${DEFAULT_PROJECT_ID}`, legacySessions)
  }

  await store.delete('chat:sessions').catch(() => {})
  await store.delete('relayMultiaddr').catch(() => {})
  await store.delete('reputationFloor').catch(() => {})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
npx vitest run tests/unit/projectMigration.test.ts
```

Expected: PASS, 4 tests.

- [ ] **Step 5: Commit**

```bash
git add src/lib/projectMigration.ts tests/unit/projectMigration.test.ts
git commit -m "feat(projects): legacy → Default project migration"
```

---

### Task 7: Add electron-store `delete` to the IPC bridge

The migration uses `store.delete(key)`. The current preload only exposes `get`/`set`. We need to add `delete`.

**Files:**
- Modify: `electron/preload.ts`
- Modify: `electron/ipc.ts`

- [ ] **Step 1: Add IPC handler**

In `electron/ipc.ts`, after the `settings:set` handler, add:

```ts
  ipcMain.handle('settings:delete', (_, key: string) => {
    settingsStore.delete(key as never)
  })
```

- [ ] **Step 2: Expose on preload**

In `electron/preload.ts`, inside the `settings` object, add a `delete` method:

```ts
  settings: {
    get: <T = unknown>(key: string) =>
      ipcRenderer.invoke('settings:get', key) as Promise<T | undefined>,
    set: (key: string, value: unknown) => ipcRenderer.invoke('settings:set', key, value),
    delete: (key: string) => ipcRenderer.invoke('settings:delete', key) as Promise<void>,
  },
```

- [ ] **Step 3: Build + smoke test**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build
```

Expected: `built in ~1s`, no errors.

- [ ] **Step 4: Commit**

```bash
git add electron/preload.ts electron/ipc.ts
git commit -m "feat(ipc): expose settings.delete for migration"
```

---

### Task 8: Zustand store — projects state

Update the UI store to own `projects`, `activeProjectId`, `isProjectSwitching`, and the CRUD/switch actions. Remove `relayMultiaddr` and `reputationFloor` as standalone fields.

**Files:**
- Modify: `src/lib/store.ts`

- [ ] **Step 1: Read current store**

```bash
cat src/lib/store.ts
```

Note the existing shape — you'll keep `theme`, `accent`, `apiPort`, `searchTerm`, `filterTrustedOnly`, `dispatchTarget`, `isDispatchOpen`, `isFeedbackOpen`, `feedbackContext`, and their setters.

- [ ] **Step 2: Rewrite the store**

`src/lib/store.ts`:

```ts
import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'
import type { Project } from '../types/project'
import {
  newProjectId,
  validateProjectInput,
  ProjectInput,
} from './projectStore'

export interface UIState {
  theme: 'dark' | 'light' | 'auto'
  accent: 'emerald' | 'violet' | 'rose'
  apiPort: number
  searchTerm: string
  filterTrustedOnly: boolean
  dispatchTarget: string | null
  isDispatchOpen: boolean
  isFeedbackOpen: boolean
  feedbackContext: { peerId: string; taskId: string } | null

  // Projects
  projects: Project[]
  activeProjectId: string | null
  isProjectSwitching: boolean
  isProjectSettingsOpen: boolean
  isCreateWizardOpen: boolean

  setTheme: (t: UIState['theme']) => void
  setAccent: (a: UIState['accent']) => void
  setApiPort: (p: number) => void
  setSearchTerm: (s: string) => void
  setFilterTrustedOnly: (v: boolean) => void
  openDispatch: (peerId: string) => void
  closeDispatch: () => void
  openFeedback: (peerId: string, taskId: string) => void
  closeFeedback: () => void

  hydrateProjects: (
    projects: Project[],
    activeId: string | null,
  ) => void
  addProject: (input: ProjectInput) => Project
  updateProject: (id: string, patch: Partial<ProjectInput>) => void
  deleteProject: (id: string) => void
  setProjectSwitching: (v: boolean) => void
  openProjectSettings: () => void
  closeProjectSettings: () => void
  openCreateWizard: () => void
  closeCreateWizard: () => void

  activeProject: () => Project | undefined
}

export const useUIStore = create<UIState>()(
  subscribeWithSelector((set, get) => ({
    theme: 'dark',
    accent: 'emerald',
    apiPort: 8080,
    searchTerm: '',
    filterTrustedOnly: false,
    dispatchTarget: null,
    isDispatchOpen: false,
    isFeedbackOpen: false,
    feedbackContext: null,

    projects: [],
    activeProjectId: null,
    isProjectSwitching: false,
    isProjectSettingsOpen: false,
    isCreateWizardOpen: false,

    setTheme: (theme) => {
      set({ theme })
      document.documentElement.setAttribute('data-theme', theme)
      window.api?.settings.set('theme', theme).catch(() => {})
    },
    setAccent: (accent) => {
      set({ accent })
      document.documentElement.setAttribute('data-accent', accent)
      window.api?.settings.set('accent', accent).catch(() => {})
    },
    setApiPort: (apiPort) => set({ apiPort }),
    setSearchTerm: (searchTerm) => set({ searchTerm }),
    setFilterTrustedOnly: (filterTrustedOnly) => set({ filterTrustedOnly }),
    openDispatch: (dispatchTarget) => set({ isDispatchOpen: true, dispatchTarget }),
    closeDispatch: () => set({ isDispatchOpen: false, dispatchTarget: null }),
    openFeedback: (peerId, taskId) =>
      set({ isFeedbackOpen: true, feedbackContext: { peerId, taskId } }),
    closeFeedback: () => set({ isFeedbackOpen: false, feedbackContext: null }),

    hydrateProjects: (projects, activeId) => set({ projects, activeProjectId: activeId }),

    addProject: (input) => {
      const projects = get().projects
      validateProjectInput(projects, input)
      const project: Project = {
        id: newProjectId(),
        name: input.name.trim(),
        icon: input.icon ?? '🌐',
        color: input.color ?? 'emerald',
        relayMultiaddr: input.relayMultiaddr,
        reputationFloor: input.reputationFloor ?? -0.5,
        createdAt: Date.now(),
      }
      const nextProjects = [...projects, project]
      set({ projects: nextProjects })
      window.api?.settings.set('projects', nextProjects).catch(() => {})
      return project
    },

    updateProject: (id, patch) => {
      const projects = get().projects
      const current = projects.find((p) => p.id === id)
      if (!current) return
      const merged: ProjectInput = {
        name: patch.name ?? current.name,
        relayMultiaddr:
          patch.relayMultiaddr === undefined ? current.relayMultiaddr : patch.relayMultiaddr,
        reputationFloor: patch.reputationFloor ?? current.reputationFloor,
        icon: patch.icon ?? current.icon,
        color: patch.color ?? current.color,
      }
      validateProjectInput(projects, merged, id)
      const next = projects.map((p) =>
        p.id === id
          ? {
              ...p,
              ...merged,
              name: merged.name.trim(),
            }
          : p,
      )
      set({ projects: next })
      window.api?.settings.set('projects', next).catch(() => {})
    },

    deleteProject: (id) => {
      const projects = get().projects.filter((p) => p.id !== id)
      const activeId =
        get().activeProjectId === id ? (projects[0]?.id ?? null) : get().activeProjectId
      set({ projects, activeProjectId: activeId })
      window.api?.settings.set('projects', projects).catch(() => {})
      window.api?.settings.set('activeProjectId', activeId).catch(() => {})
      window.api?.settings.delete(`chat:sessions:${id}`).catch(() => {})
    },

    setProjectSwitching: (v) => set({ isProjectSwitching: v }),
    openProjectSettings: () => set({ isProjectSettingsOpen: true }),
    closeProjectSettings: () => set({ isProjectSettingsOpen: false }),
    openCreateWizard: () => set({ isCreateWizardOpen: true }),
    closeCreateWizard: () => set({ isCreateWizardOpen: false }),

    activeProject: () => {
      const { projects, activeProjectId } = get()
      return projects.find((p) => p.id === activeProjectId)
    },
  })),
)
```

- [ ] **Step 3: Typecheck**

```bash
npx tsc -p tsconfig.web.json --noEmit 2>&1 | grep -v "^node_modules" | grep -v "ExperimentalWarning" | head -40
```

Expect: pre-existing errors only (Button.tsx motion type, ipc.ts include path, etc.). No new errors referencing `store.ts`.

- [ ] **Step 4: Run unit tests**

```bash
npx vitest run tests/unit/
```

Expected: all existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add src/lib/store.ts
git commit -m "feat(store): projects state + CRUD actions; drop standalone relay/floor"
```

---

### Task 9: Per-project chat session storage

**Files:**
- Modify: `src/lib/sessions.ts`
- Modify: `src/hooks/useChat.ts`

- [ ] **Step 1: Update sessions.ts to accept a project ID**

`src/lib/sessions.ts`:

```ts
import type { ChatSession } from '../types/chat';

const LIMIT = 50;

function keyFor(projectId: string): string {
  return `chat:sessions:${projectId}`;
}

export async function loadSessions(projectId: string): Promise<ChatSession[]> {
  try {
    const stored = await window.api.settings.get<ChatSession[]>(keyFor(projectId));
    return Array.isArray(stored) ? stored : [];
  } catch {
    return [];
  }
}

export async function saveSessions(
  projectId: string,
  sessions: ChatSession[],
): Promise<void> {
  const trimmed = sessions.slice(-LIMIT);
  await window.api.settings.set(keyFor(projectId), trimmed);
}

export function newSession(): ChatSession {
  return {
    id: `chat_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    title: 'New chat',
    pinnedPeerId: null,
    preferredModel: 'auto',
    messages: [],
    createdAt: Date.now(),
    updatedAt: Date.now(),
  };
}
```

- [ ] **Step 2: Thread the active project ID through useChat**

In `src/hooks/useChat.ts`:

Replace:
```ts
import { loadSessions, saveSessions, newSession } from '../lib/sessions';
```
with:
```ts
import { loadSessions, saveSessions, newSession } from '../lib/sessions';
import { useUIStore } from '../lib/store';
```

Inside `useChat()`, just after `const [sessions, setSessions] = useState<ChatSession[]>([])`:

```ts
  const activeProjectId = useUIStore((s) => s.activeProjectId);
```

Replace the first `useEffect` (`// Load sessions once`) with:

```ts
  useEffect(() => {
    if (!activeProjectId) return;
    setSessions([]);
    setActiveId(null);
    loadSessions(activeProjectId).then((s) => {
      setSessions(s);
      if (s.length > 0) setActiveId(s[0].id);
    });
  }, [activeProjectId]);
```

Replace the second `useEffect` (`// Persist whenever sessions change`) with:

```ts
  useEffect(() => {
    if (!activeProjectId) return;
    if (sessions.length === 0) return;
    saveSessions(activeProjectId, sessions);
  }, [sessions, activeProjectId]);
```

- [ ] **Step 3: Add mounted-guard ref (for §6.1 of spec)**

Just after the `abortRef` declaration at the top of `useChat()`:

```ts
  const mountedRef = useRef(true);
  useEffect(() => () => { mountedRef.current = false; }, []);
```

Then in the streaming `commit` function (added in the previous chat fix), wrap the `setSessions` call:

```ts
        const commit = () => {
          rafScheduled = false;
          if (!mountedRef.current) return;
          setSessions((prev) =>
            ...
          );
        };
```

- [ ] **Step 4: Typecheck**

```bash
npx tsc -p tsconfig.web.json --noEmit 2>&1 | grep -v "^node_modules" | head -10
```

Expect: no new errors in sessions.ts or useChat.ts.

- [ ] **Step 5: Commit**

```bash
git add src/lib/sessions.ts src/hooks/useChat.ts
git commit -m "feat(chat): per-project session storage + mounted-guard against late commits"
```

---

### Task 10: Run migration in main.tsx

**Files:**
- Modify: `src/main.tsx`

- [ ] **Step 1: Read current bootstrap**

```bash
cat src/main.tsx
```

- [ ] **Step 2: Patch bootstrap to migrate then hydrate**

Add imports at top:

```ts
import { migrateLegacySettings } from './lib/projectMigration'
import type { Project } from './types/project'
```

Inside the async bootstrap (between `loadApiPortFromSettings()` and the React render), insert:

```ts
  // Migrate legacy single-mesh settings into the projects model
  await migrateLegacySettings({
    get: (k) => window.api.settings.get(k),
    set: (k, v) => window.api.settings.set(k, v),
    delete: (k) => window.api.settings.delete(k),
  } as never).catch((e) => console.warn('migration failed', e))

  const projects = (await window.api.settings.get<Project[]>('projects')) ?? []
  const activeId = (await window.api.settings.get<string | null>('activeProjectId')) ?? null
  useUIStore.getState().hydrateProjects(projects, activeId)
```

(If `useUIStore` is not already imported, add `import { useUIStore } from './lib/store'`.)

- [ ] **Step 3: Build**

```bash
npx electron-vite build 2>&1 | tail -8
```

Expected: `built in ~1s`, no errors.

- [ ] **Step 4: Manual smoke**

Stop any running backend, then:
```bash
./scripts/dev.sh &
sleep 12
cat "$HOME/Library/Application Support/agentfm-desktop/settings.json" | python3 -m json.tool | head -25
```

Confirm a `projects` array exists with one entry (the migrated Default) and `chat:sessions` is gone.

Kill the running app before the next task:
```bash
pkill -9 -f "electron-vite|Electron.app|/agentfm-go/agentfm" 2>/dev/null || true
```

- [ ] **Step 5: Commit**

```bash
git add src/main.tsx
git commit -m "feat(boot): run project migration + hydrate Zustand on app start"
```

---

### Task 11: RouteErrorBoundary

**Files:**
- Create: `src/components/RouteErrorBoundary.tsx`
- Modify: `src/components/Shell.tsx`

- [ ] **Step 1: Implement the boundary**

`src/components/RouteErrorBoundary.tsx`:

```tsx
import { Component, ReactNode } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'

interface State {
  error: Error | null
}

interface Props {
  children: ReactNode
}

class Boundary extends Component<Props & { onReset: () => void; onHome: () => void; pathname: string }, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidUpdate(prevProps: Props & { pathname: string }) {
    if (prevProps.pathname !== this.props.pathname && this.state.error) {
      this.setState({ error: null })
    }
  }

  render() {
    if (this.state.error) {
      return (
        <div className="p-8 max-w-xl">
          <div className="bg-rose-950/40 border border-rose-900/60 rounded-xl p-5">
            <div className="text-rose-300 font-semibold text-sm mb-1">This view hit a snag</div>
            <div className="text-text-1 text-sm mb-3">
              {this.state.error.message || 'Unknown error'}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => {
                  this.setState({ error: null })
                  this.props.onReset()
                }}
                className="bg-bg-2 border border-border-0 rounded-md px-3 py-1.5 text-xs text-text-1 hover:text-text-0"
              >
                Retry
              </button>
              <button
                onClick={this.props.onHome}
                className="bg-accent text-accent-fg rounded-md px-3 py-1.5 text-xs font-medium"
              >
                Go to Radar
              </button>
            </div>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

export function RouteErrorBoundary({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const location = useLocation()
  return (
    <Boundary
      onReset={() => navigate(location.pathname, { replace: true })}
      onHome={() => navigate('/radar')}
      pathname={location.pathname}
    >
      {children}
    </Boundary>
  )
}
```

- [ ] **Step 2: Wrap the route outlet in Shell**

In `src/components/Shell.tsx`, add the import:

```tsx
import { RouteErrorBoundary } from './RouteErrorBoundary'
```

Wrap the `<Outlet />`:

```tsx
              <RouteErrorBoundary>
                <Outlet />
              </RouteErrorBoundary>
```

(That replaces the bare `<Outlet />` inside the `motion.div`.)

- [ ] **Step 3: Build**

```bash
npx electron-vite build 2>&1 | tail -5
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add src/components/RouteErrorBoundary.tsx src/components/Shell.tsx
git commit -m "feat(shell): per-route error boundary stops black-screen on nav errors"
```

---

### Task 12: ProjectList (sidebar) + ProjectPill (top bar)

**Files:**
- Create: `src/components/projects/ProjectList.tsx`
- Create: `src/components/projects/ProjectPill.tsx`

- [ ] **Step 1: Write ProjectList**

`src/components/projects/ProjectList.tsx`:

```tsx
import { useUIStore } from '../../lib/store'
import { motion } from 'framer-motion'

const COLOR_HEX: Record<string, string> = {
  emerald: '#10b981',
  violet: '#8b5cf6',
  rose: '#f43f5e',
  cyan: '#22d3ee',
  amber: '#f59e0b',
}

export function ProjectList() {
  const projects = useUIStore((s) => s.projects)
  const activeId = useUIStore((s) => s.activeProjectId)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)
  const openWizard = useUIStore((s) => s.openCreateWizard)

  async function switchTo(id: string) {
    if (id === activeId) return
    setSwitching(true)
    const project = projects.find((p) => p.id === id)
    if (!project) {
      setSwitching(false)
      return
    }
    await window.api.settings.set('activeProjectId', id)
    useUIStore.setState({ activeProjectId: id })
    try {
      await window.api.backend.restart({
        apiPort: useUIStore.getState().apiPort,
        reputationFloor: project.reputationFloor,
        relayMultiaddr: project.relayMultiaddr ?? undefined,
      })
    } catch (e) {
      console.warn('project switch: backend restart failed', e)
    } finally {
      setSwitching(false)
    }
  }

  return (
    <div className="px-2 py-3 border-b border-border-0">
      <div className="text-2xs uppercase tracking-wider text-text-2 px-2 mb-2">Projects</div>
      <div className="space-y-1">
        {projects.map((p) => {
          const active = p.id === activeId
          return (
            <motion.button
              key={p.id}
              whileTap={{ scale: 0.97 }}
              onClick={() => switchTo(p.id)}
              className={`w-full text-left flex items-center gap-2 px-2 py-1.5 rounded-md text-sm transition-colors ${
                active ? 'bg-bg-2 text-text-0' : 'text-text-1 hover:bg-bg-2/60'
              }`}
            >
              <span className="text-base leading-none">{p.icon}</span>
              <span className="flex-1 truncate">{p.name}</span>
              <span
                className="w-1.5 h-1.5 rounded-full"
                style={{ background: COLOR_HEX[p.color] ?? '#10b981' }}
              />
            </motion.button>
          )
        })}
      </div>
      <button
        onClick={openWizard}
        className="w-full text-left mt-2 px-2 py-1.5 rounded-md text-xs text-text-2 hover:text-accent hover:bg-bg-2/60"
      >
        + new project
      </button>
    </div>
  )
}
```

- [ ] **Step 2: Write ProjectPill**

`src/components/projects/ProjectPill.tsx`:

```tsx
import { useUIStore } from '../../lib/store'

const COLOR_HEX: Record<string, string> = {
  emerald: '#10b981',
  violet: '#8b5cf6',
  rose: '#f43f5e',
  cyan: '#22d3ee',
  amber: '#f59e0b',
}

export function ProjectPill() {
  const active = useUIStore((s) => s.activeProject())
  const open = useUIStore((s) => s.openProjectSettings)
  if (!active) return <div className="w-32" />

  return (
    <button
      onClick={open}
      className="inline-flex items-center gap-2 bg-bg-1 hover:bg-bg-2 border border-border-0 rounded-full px-3 py-1.5 text-xs text-text-1 transition-colors"
      title="Project settings"
    >
      <span>{active.icon}</span>
      <span className="font-medium text-text-0 max-w-[180px] truncate">{active.name}</span>
      <span
        className="w-1.5 h-1.5 rounded-full"
        style={{ background: COLOR_HEX[active.color] ?? '#10b981' }}
      />
    </button>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add src/components/projects/ProjectList.tsx src/components/projects/ProjectPill.tsx
git commit -m "feat(projects): ProjectList sidebar + ProjectPill topbar chip"
```

---

### Task 13: CreateProjectWizard

**Files:**
- Create: `src/components/projects/CreateProjectWizard.tsx`

- [ ] **Step 1: Implement the wizard**

`src/components/projects/CreateProjectWizard.tsx`:

```tsx
import { useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'
import { Button } from '../primitives/Button'
import { Input } from '../primitives/Input'
import { toast } from 'sonner'
import { DuplicateRelayError } from '../../lib/projectStore'
import type { ProjectColor } from '../../types/project'

const COLORS: { key: ProjectColor; hex: string }[] = [
  { key: 'emerald', hex: '#10b981' },
  { key: 'violet', hex: '#8b5cf6' },
  { key: 'rose', hex: '#f43f5e' },
  { key: 'cyan', hex: '#22d3ee' },
  { key: 'amber', hex: '#f59e0b' },
]

const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/

export function CreateProjectWizard() {
  const open = useUIStore((s) => s.isCreateWizardOpen)
  const close = useUIStore((s) => s.closeCreateWizard)
  const addProject = useUIStore((s) => s.addProject)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)

  const [name, setName] = useState('')
  const [icon, setIcon] = useState('🌐')
  const [color, setColor] = useState<ProjectColor>('emerald')
  const [useDefault, setUseDefault] = useState(true)
  const [relay, setRelay] = useState('')
  const [floor, setFloor] = useState(-0.5)
  const [saving, setSaving] = useState(false)

  function reset() {
    setName('')
    setIcon('🌐')
    setColor('emerald')
    setUseDefault(true)
    setRelay('')
    setFloor(-0.5)
    setSaving(false)
  }

  async function create() {
    if (saving) return
    if (!name.trim()) {
      toast.error('Give the project a name')
      return
    }
    const relayValue = useDefault ? null : relay.trim()
    if (relayValue && !MULTIADDR_RE.test(relayValue)) {
      toast.error('That doesn’t look like a multiaddr')
      return
    }
    setSaving(true)
    let project
    try {
      project = addProject({
        name,
        icon,
        color,
        relayMultiaddr: relayValue,
        reputationFloor: floor,
      })
    } catch (e) {
      const msg = e instanceof DuplicateRelayError ? e.message : (e as Error).message
      toast.error(msg)
      setSaving(false)
      return
    }
    // Switch to the new project
    await window.api.settings.set('activeProjectId', project.id)
    useUIStore.setState({ activeProjectId: project.id })
    setSwitching(true)
    try {
      await window.api.backend.restart({
        apiPort: useUIStore.getState().apiPort,
        reputationFloor: project.reputationFloor,
        relayMultiaddr: project.relayMultiaddr ?? undefined,
      })
      toast.success(`Project "${project.name}" created`)
    } catch (e) {
      toast.error('Backend restart failed: ' + (e as Error).message)
    } finally {
      setSwitching(false)
      close()
      reset()
    }
  }

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[70] flex items-center justify-center"
        >
          <motion.div
            initial={{ scale: 0.96, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.96, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 380, damping: 28 }}
            className="w-[520px] bg-bg-1 border border-border-0 rounded-xl p-7 shadow-2xl"
          >
            <div className="flex justify-between items-start mb-4">
              <div>
                <h2 className="text-xl font-semibold text-text-0">New project</h2>
                <p className="text-sm text-text-2 mt-1">
                  A project bundles a relay, a reputation threshold, and its own chats and starred agents.
                </p>
              </div>
              <button onClick={() => { close(); reset() }} className="text-text-2 hover:text-text-0 text-lg">✕</button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-1.5">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Team Mesh" autoFocus />

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-4 mb-1.5">Icon &amp; color</label>
            <div className="flex items-center gap-3">
              <Input
                value={icon}
                onChange={(e) => setIcon(e.target.value.slice(0, 4))}
                className="w-16 text-center text-base"
              />
              <div className="flex gap-2">
                {COLORS.map((c) => (
                  <button
                    key={c.key}
                    onClick={() => setColor(c.key)}
                    className={`w-6 h-6 rounded-full transition-all ${
                      color === c.key ? 'ring-2 ring-white' : 'opacity-70 hover:opacity-100'
                    }`}
                    style={{ background: c.hex }}
                  />
                ))}
              </div>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">Relay</label>
            <label className="flex items-start gap-2 mb-2 cursor-pointer">
              <input type="radio" checked={useDefault} onChange={() => setUseDefault(true)} className="mt-1 accent-accent" />
              <div>
                <div className="text-sm text-text-0">Bundled public lighthouse</div>
                <div className="text-2xs text-text-2 mt-0.5">Recommended for first projects.</div>
              </div>
            </label>
            <label className="flex items-start gap-2 cursor-pointer">
              <input type="radio" checked={!useDefault} onChange={() => setUseDefault(false)} className="mt-1 accent-accent" />
              <div className="flex-1">
                <div className="text-sm text-text-0">Custom relay multiaddr</div>
                {!useDefault && (
                  <Input
                    className="mt-2 font-mono text-2xs"
                    placeholder="/ip4/198.51.100.55/tcp/4001/p2p/12D3KooW…"
                    value={relay}
                    onChange={(e) => setRelay(e.target.value)}
                  />
                )}
              </div>
            </label>

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">
              Reputation floor <span className="text-text-3 normal-case">({floor.toFixed(2)})</span>
            </label>
            <input
              type="range"
              min={-1}
              max={0}
              step={0.05}
              value={floor}
              onChange={(e) => setFloor(Number(e.target.value))}
              className="w-full accent-accent"
            />
            <div className="flex justify-between text-2xs text-text-2 font-mono">
              <span>-1.0 (allow all)</span>
              <span>0.0 (strict)</span>
            </div>

            <div className="flex justify-end gap-2 mt-6">
              <Button onClick={() => { close(); reset() }} disabled={saving}>Cancel</Button>
              <Button variant="primary" onClick={create} disabled={saving || !name.trim()}>
                {saving ? 'Creating…' : 'Create project'}
              </Button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add src/components/projects/CreateProjectWizard.tsx
git commit -m "feat(projects): CreateProjectWizard with relay/floor inputs"
```

---

### Task 14: ProjectSettingsSheet

**Files:**
- Create: `src/components/projects/ProjectSettingsSheet.tsx`

- [ ] **Step 1: Implement**

`src/components/projects/ProjectSettingsSheet.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'
import { Button } from '../primitives/Button'
import { Input } from '../primitives/Input'
import { toast } from 'sonner'
import { DuplicateRelayError } from '../../lib/projectStore'
import type { ProjectColor } from '../../types/project'

const COLORS: { key: ProjectColor; hex: string }[] = [
  { key: 'emerald', hex: '#10b981' },
  { key: 'violet', hex: '#8b5cf6' },
  { key: 'rose', hex: '#f43f5e' },
  { key: 'cyan', hex: '#22d3ee' },
  { key: 'amber', hex: '#f59e0b' },
]

const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/

export function ProjectSettingsSheet() {
  const open = useUIStore((s) => s.isProjectSettingsOpen)
  const close = useUIStore((s) => s.closeProjectSettings)
  const active = useUIStore((s) => s.activeProject())
  const projects = useUIStore((s) => s.projects)
  const updateProject = useUIStore((s) => s.updateProject)
  const deleteProject = useUIStore((s) => s.deleteProject)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)

  const [name, setName] = useState('')
  const [icon, setIcon] = useState('🌐')
  const [color, setColor] = useState<ProjectColor>('emerald')
  const [relay, setRelay] = useState('')
  const [floor, setFloor] = useState(-0.5)

  useEffect(() => {
    if (!active) return
    setName(active.name)
    setIcon(active.icon)
    setColor(active.color)
    setRelay(active.relayMultiaddr ?? '')
    setFloor(active.reputationFloor)
  }, [active?.id, open])

  if (!active) return null

  async function save() {
    const relayValue = relay.trim() || null
    if (relayValue && !MULTIADDR_RE.test(relayValue)) {
      toast.error('That doesn’t look like a multiaddr')
      return
    }
    try {
      updateProject(active.id, {
        name,
        icon,
        color,
        relayMultiaddr: relayValue,
        reputationFloor: floor,
      })
    } catch (e) {
      const msg = e instanceof DuplicateRelayError ? e.message : (e as Error).message
      toast.error(msg)
      return
    }
    const meshChanged = relayValue !== active.relayMultiaddr || floor !== active.reputationFloor
    if (meshChanged) {
      setSwitching(true)
      try {
        await window.api.backend.restart({
          apiPort: useUIStore.getState().apiPort,
          reputationFloor: floor,
          relayMultiaddr: relayValue ?? undefined,
        })
      } catch (e) {
        toast.error('Restart failed: ' + (e as Error).message)
      } finally {
        setSwitching(false)
      }
    }
    toast.success('Project updated')
    close()
  }

  async function remove() {
    if (projects.length <= 1) {
      toast.error('You need at least one project')
      return
    }
    if (!window.confirm(`Delete "${active.name}"? Its chat sessions will be removed.`)) return
    deleteProject(active.id)
    close()
    toast.success('Project deleted')
  }

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[65] flex justify-end"
          onClick={close}
        >
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 320, damping: 30 }}
            onClick={(e) => e.stopPropagation()}
            className="w-[480px] h-full bg-bg-1 border-l border-border-0 overflow-auto p-6"
          >
            <div className="flex justify-between items-start mb-5">
              <h2 className="text-xl font-semibold text-text-0">Project settings</h2>
              <button onClick={close} className="text-text-2 hover:text-text-0 text-lg">✕</button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-1.5">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-4 mb-1.5">Icon &amp; color</label>
            <div className="flex items-center gap-3">
              <Input value={icon} onChange={(e) => setIcon(e.target.value.slice(0, 4))} className="w-16 text-center text-base" />
              <div className="flex gap-2">
                {COLORS.map((c) => (
                  <button
                    key={c.key}
                    onClick={() => setColor(c.key)}
                    className={`w-6 h-6 rounded-full transition-all ${
                      color === c.key ? 'ring-2 ring-white' : 'opacity-70 hover:opacity-100'
                    }`}
                    style={{ background: c.hex }}
                  />
                ))}
              </div>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">Relay multiaddr</label>
            <Input value={relay} onChange={(e) => setRelay(e.target.value)} placeholder="(blank = bundled public lighthouse)" className="font-mono text-2xs" />

            <label className="block text-xs uppercase tracking-wider text-text-2 mt-5 mb-1.5">
              Reputation floor <span className="text-text-3 normal-case">({floor.toFixed(2)})</span>
            </label>
            <input type="range" min={-1} max={0} step={0.05} value={floor} onChange={(e) => setFloor(Number(e.target.value))} className="w-full accent-accent" />

            <div className="flex justify-end gap-2 mt-7">
              <Button onClick={close}>Cancel</Button>
              <Button variant="primary" onClick={save}>Save</Button>
            </div>

            <div className="mt-9 border-t border-border-0 pt-5">
              <div className="text-xs uppercase tracking-wider text-rose-400 mb-2">Danger zone</div>
              <Button onClick={remove}>Delete this project</Button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add src/components/projects/ProjectSettingsSheet.tsx
git commit -m "feat(projects): project settings sheet (edit + delete)"
```

---

### Task 15: ProjectSwitchingOverlay

**Files:**
- Create: `src/components/projects/ProjectSwitchingOverlay.tsx`

- [ ] **Step 1: Implement**

`src/components/projects/ProjectSwitchingOverlay.tsx`:

```tsx
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'

export function ProjectSwitchingOverlay() {
  const show = useUIStore((s) => s.isProjectSwitching)
  const active = useUIStore((s) => s.activeProject())

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.18 }}
          className="fixed inset-0 z-[80] flex items-center justify-center bg-bg-0/85 backdrop-blur"
        >
          <div className="text-center">
            <motion.div
              animate={{ rotate: 360 }}
              transition={{ duration: 1.4, repeat: Infinity, ease: 'linear' }}
              className="w-10 h-10 mx-auto rounded-full border-2 border-accent/30 border-t-accent"
            />
            <div className="mt-4 text-text-1 text-sm">
              Switching to <span className="text-text-0 font-medium">{active?.name ?? '…'}</span>…
            </div>
            <div className="mt-1 text-2xs text-text-2">restarting backend with the new relay</div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add src/components/projects/ProjectSwitchingOverlay.tsx
git commit -m "feat(projects): full-pane overlay during project switch"
```

---

### Task 16: TopBar restructure (centered logo + ProjectPill)

**Files:**
- Modify: `src/components/TopBar.tsx`

- [ ] **Step 1: Read current**

```bash
cat src/components/TopBar.tsx
```

- [ ] **Step 2: Rewrite into three columns**

`src/components/TopBar.tsx`:

```tsx
import { useNavigate } from 'react-router-dom'
import { useWorkers } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { ProjectPill } from './projects/ProjectPill'

export function TopBar() {
  const navigate = useNavigate()
  const { data } = useWorkers(false)
  const backend = useBackend()
  const online = data?.online_count ?? 0
  const offline = data?.offline_count ?? 0
  const relayOk = backend.ok

  return (
    <header className="h-10 border-b border-border-0 bg-bg-0 grid grid-cols-3 items-center px-3 select-none">
      <div className="justify-self-start">
        <ProjectPill />
      </div>
      <div className="justify-self-center text-sm font-semibold tracking-tight text-text-0">
        AgentFM
      </div>
      <div className="justify-self-end">
        <button
          onClick={() => navigate('/status')}
          className="text-2xs text-text-2 hover:text-text-0 transition-colors"
        >
          {online} online · {offline} offline · relay {relayOk ? '✓ stable' : '⚠ down'}
        </button>
      </div>
    </header>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add src/components/TopBar.tsx
git commit -m "feat(topbar): centered AgentFM wordmark + ProjectPill column"
```

---

### Task 17: Mount ProjectList in Sidebar; mount overlay/sheet/wizard in App

**Files:**
- Modify: `src/components/Sidebar.tsx`
- Modify: `src/App.tsx`

- [ ] **Step 1: Mount ProjectList**

In `src/components/Sidebar.tsx`, add the import:

```tsx
import { ProjectList } from './projects/ProjectList'
```

Place `<ProjectList />` at the very top of the sidebar's rendered output (above the existing nav icons). Leave the existing nav icons in place — they stay.

- [ ] **Step 2: Mount the global overlays in App**

In `src/App.tsx`, add imports:

```tsx
import { ProjectSwitchingOverlay } from './components/projects/ProjectSwitchingOverlay'
import { ProjectSettingsSheet } from './components/projects/ProjectSettingsSheet'
import { CreateProjectWizard } from './components/projects/CreateProjectWizard'
```

Inside the returned fragment, alongside the existing `<DispatchDrawer />`, `<FeedbackModal />`, `<WelcomeModal />`, add:

```tsx
      <ProjectSwitchingOverlay />
      <ProjectSettingsSheet />
      <CreateProjectWizard />
```

- [ ] **Step 3: Build**

```bash
npx electron-vite build 2>&1 | tail -5
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add src/components/Sidebar.tsx src/App.tsx
git commit -m "feat(shell): wire ProjectList + overlay/sheet/wizard into Shell"
```

---

### Task 18: First-launch path — open the wizard when there are no projects

**Files:**
- Modify: `src/components/WelcomeModal.tsx`

The existing WelcomeModal is the legacy welcome → relay-config flow. We replace its trigger logic with "if no projects, open the CreateProjectWizard".

- [ ] **Step 1: Rewrite WelcomeModal trigger**

`src/components/WelcomeModal.tsx`:

```tsx
import { useEffect } from 'react'
import { useUIStore } from '../lib/store'

// First-launch logic: when there are no projects yet, open the
// CreateProjectWizard exactly once. The wizard itself replaces the
// previous welcome content.
export function WelcomeModal() {
  const projects = useUIStore((s) => s.projects)
  const openWizard = useUIStore((s) => s.openCreateWizard)
  const isWizardOpen = useUIStore((s) => s.isCreateWizardOpen)

  useEffect(() => {
    if (projects.length === 0 && !isWizardOpen) {
      openWizard()
    }
  }, [projects.length, isWizardOpen, openWizard])

  return null
}
```

- [ ] **Step 2: Commit**

```bash
git add src/components/WelcomeModal.tsx
git commit -m "feat(welcome): first launch opens CreateProjectWizard when projects empty"
```

---

### Task 19: Apply displayName everywhere

**Files:**
- Modify: `src/components/AgentCard.tsx`
- Modify: `src/components/chat/AgentPicker.tsx`
- Modify: `src/components/DispatchDrawer.tsx`
- Modify: `src/routes/PeerView.tsx`

- [ ] **Step 1: AgentCard**

In `src/components/AgentCard.tsx`, add at top:

```tsx
import { displayName } from '../lib/displayName'
```

Replace `{worker.name || '(unknown)'}` with `{displayName(worker)}`. Remove the local `extractModel` helper and its call site — the displayName already handles image-ref fallbacks for the title; the model is shown separately via `worker.model`.

- [ ] **Step 2: AgentPicker**

In `src/components/chat/AgentPicker.tsx`, add:

```tsx
import { displayName } from '../../lib/displayName'
```

Replace `pinnedWorker.name` (in the pinned-label expression) with `displayName(pinnedWorker)`, and replace `{w.name}` (in the dropdown rows) with `{displayName(w)}`.

- [ ] **Step 3: DispatchDrawer**

In `src/components/DispatchDrawer.tsx`, add:

```tsx
import { displayName } from '../lib/displayName'
```

Replace the heading line:

```tsx
                <h2 className="text-base font-semibold text-text-0">
                  {worker.name}
                  {worker.model ? ` · ${worker.model}` : ''}
                </h2>
```

with:

```tsx
                <h2 className="text-base font-semibold text-text-0">
                  {displayName(worker)}
                  {worker.model ? ` · ${worker.model}` : ''}
                </h2>
```

- [ ] **Step 4: PeerView**

In `src/routes/PeerView.tsx`, add:

```tsx
import { displayName } from '../lib/displayName';
```

Replace:

```tsx
          <h1 className="text-2xl font-semibold text-text-0">
            {summary.agent_name || '(unknown agent)'}
          </h1>
```

with:

```tsx
          <h1 className="text-2xl font-semibold text-text-0">
            {displayName({ ...summary, name: summary.agent_name, peer_id: summary.peer_id })}
          </h1>
```

- [ ] **Step 5: Build + commit**

```bash
npx electron-vite build 2>&1 | tail -5
git add src/components/AgentCard.tsx src/components/chat/AgentPicker.tsx src/components/DispatchDrawer.tsx src/routes/PeerView.tsx
git commit -m "feat(ui): use displayName helper across worker labels"
```

---

### Task 20: usePeerName + Activity refactor

**Files:**
- Create: `src/hooks/usePeerName.ts`
- Modify: `src/routes/Activity.tsx`

- [ ] **Step 1: Implement the hook**

`src/hooks/usePeerName.ts`:

```ts
import { useWorkers } from '../lib/query'
import { displayName } from '../lib/displayName'
import { shortenPeerID } from '../lib/peer'

export function usePeerName(peerId: string): string {
  const { data } = useWorkers(true)
  const w = data?.agents.find((a) => a.peer_id === peerId)
  if (w) return displayName(w)
  return shortenPeerID(peerId, 8, 6)
}
```

- [ ] **Step 2: Use it in Activity**

In `src/routes/Activity.tsx`, replace the row header expression so it shows the resolved name + small peer ID:

```tsx
                    <button
                      onClick={() => navigate(`/peer/${subject}`)}
                      className="block w-full text-left text-2xs text-text-2 px-1 pt-2.5 hover:text-accent"
                    >
                      about <span className="text-text-0 font-medium"><PeerName peerId={subject} /></span>
                      <span className="ml-2 font-mono text-text-3">{subject.slice(0, 12)}…</span>
                    </button>
```

At the bottom of the same file, add a tiny inline helper:

```tsx
import { usePeerName } from '../hooks/usePeerName';

function PeerName({ peerId }: { peerId: string }) {
  return <>{usePeerName(peerId)}</>;
}
```

(The component wrapper avoids calling a hook from inside a `.map` directly.)

- [ ] **Step 3: Build + commit**

```bash
npx electron-vite build 2>&1 | tail -5
git add src/hooks/usePeerName.ts src/routes/Activity.tsx
git commit -m "feat(activity): show agent names instead of bare peer ids"
```

---

### Task 21: FeedbackModal — invalidate by prefix

**Files:**
- Modify: `src/components/FeedbackModal.tsx`

The existing invalidation uses `qk.peerLog(ctx.peerId)` which fills `limit=50, offset=0` and doesn't match the Activity screen's `limit=200` cache entry.

- [ ] **Step 1: Patch the invalidation block**

Replace:

```tsx
      await Promise.all([
        qc.invalidateQueries({ queryKey: qk.peer(ctx.peerId) }),
        qc.invalidateQueries({ queryKey: ['peer-log', ctx.peerId] }),
        qc.invalidateQueries({ queryKey: ['peer-log'] }),
      ]);
```

with:

```tsx
      await Promise.all([
        qc.invalidateQueries({ queryKey: qk.peer(ctx.peerId) }),
        qc.invalidateQueries({
          predicate: (q) =>
            Array.isArray(q.queryKey) &&
            q.queryKey[0] === 'peer-log' &&
            q.queryKey[1] === ctx.peerId,
        }),
      ]);
```

This prefix-match invalidates every cached peer-log entry for that peer regardless of pagination opts.

- [ ] **Step 2: Build + commit**

```bash
npx electron-vite build 2>&1 | tail -5
git add src/components/FeedbackModal.tsx
git commit -m "fix(feedback): invalidate peer-log by prefix so all paginations refresh"
```

---

### Task 22: Chat overflow fix + ANSI strip + back link

**Files:**
- Modify: `src/components/chat/MessageBubble.tsx`
- Modify: `src/hooks/useChat.ts`
- Modify: `src/routes/Chat.tsx`

- [ ] **Step 1: Make bubbles wrap any character**

In `src/components/chat/MessageBubble.tsx`, change the bubble container className to add `min-w-0 overflow-hidden`, and change the content div to use `overflow-wrap: anywhere`:

```tsx
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      className={`max-w-[75%] min-w-0 overflow-hidden px-3.5 py-3 rounded-xl text-sm leading-relaxed ${
        isUser
          ? 'self-end bg-accent-bg border border-accent/30 text-text-0'
          : 'self-start bg-bg-1 border border-border-0 text-text-0'
      }`}
    >
```

and:

```tsx
      <div className="whitespace-pre-wrap" style={{ overflowWrap: 'anywhere' }}>
        {msg.content}
```

(remove the existing `break-words` class — `overflowWrap: anywhere` supersedes it.)

- [ ] **Step 2: Strip ANSI in the streaming loop**

In `src/hooks/useChat.ts`, add the import:

```ts
import { stripAnsi } from '../lib/ansi';
```

Inside the for-line loop, where `assistantContent += delta` happens, wrap the delta:

```ts
              if (delta) {
                assistantContent += stripAnsi(delta);
                chunkChanged = true;
              }
```

- [ ] **Step 3: Lock chat container against horizontal scroll + add Back link**

In `src/routes/Chat.tsx`:

Wrap the right-side column with `min-w-0 overflow-hidden` so flexbox stops growing it:

```tsx
      <div className="flex flex-col flex-1 min-w-0 overflow-hidden bg-bg-0">
```

Add a Back link to the header, just before `<AgentPicker />`:

```tsx
        <header className="border-b border-border-0 px-5 py-3 flex items-center gap-3">
          <button
            onClick={() => navigate('/radar')}
            className="text-2xs text-text-2 hover:text-text-0"
          >
            ← Radar
          </button>
          <AgentPicker
```

Add the import at the top:

```tsx
import { useNavigate } from 'react-router-dom';
```

and inside the component:

```tsx
  const navigate = useNavigate();
```

Also tighten the scroll container:

```tsx
        <div
          ref={scrollRef}
          className="flex-1 overflow-y-auto overflow-x-hidden px-5 py-5 flex flex-col gap-4 min-w-0"
        >
```

- [ ] **Step 4: Build + commit**

```bash
npx electron-vite build 2>&1 | tail -5
git add src/components/chat/MessageBubble.tsx src/hooks/useChat.ts src/routes/Chat.tsx
git commit -m "fix(chat): kill horizontal scroll, strip ANSI, always-visible back link"
```

---

### Task 23: Status + Settings — read floor from active project

**Files:**
- Modify: `src/routes/Settings.tsx`
- Modify: `src/routes/Status.tsx`

`Settings.tsx` still references `ui.relayMultiaddr` and `ui.reputationFloor`. Those are gone from the store. Move project-level fields out and keep only app-level controls (theme, accent, port, telemetry, reset).

- [ ] **Step 1: Settings — drop project-level UI**

Open `src/routes/Settings.tsx` and remove:
- The `Card` titled "Mesh" (relay multiaddr + Test connection + backend port).
- The `Card` titled "Trust" (reputation floor slider).
- The `draftRelay`, `draftPort`, `draftFloor`, `MULTIADDR_RE`, `testConnection`, `saveAndRestart` items related to these fields. (Keep `setApiPort` — the port is still app-level if you want it editable; otherwise drop it too and treat 8080 as fixed for Phase 1.)
- The `Save & restart backend` button.

Replace the page body with this concise version (preserve theme/accent/telemetry/reset):

```tsx
import { useEffect, useState } from 'react'
import { useUIStore } from '../lib/store'
import { useAbout } from '../lib/query'
import { Button } from '../components/primitives/Button'
import { SegGroup } from '../components/primitives/SegGroup'
import { Card } from '../components/primitives/Card'
import { toast } from 'sonner'

const DEFAULTS = {
  theme: 'dark' as const,
  accent: 'emerald' as const,
  apiPort: 8080,
  telemetry: false,
}

export default function Settings() {
  const ui = useUIStore()
  const { data: about } = useAbout()
  const [telemetry, setTelemetry] = useState(false)

  useEffect(() => {
    window.api?.settings.get<boolean>('telemetry').then((v) => setTelemetry(!!v))
  }, [])

  async function resetToDefaults() {
    if (!window.confirm('Reset appearance + telemetry to defaults?')) return
    ui.setTheme(DEFAULTS.theme)
    ui.setAccent(DEFAULTS.accent)
    ui.setApiPort(DEFAULTS.apiPort)
    setTelemetry(DEFAULTS.telemetry)
    await Promise.all([
      window.api.settings.set('theme', DEFAULTS.theme),
      window.api.settings.set('accent', DEFAULTS.accent),
      window.api.settings.set('apiPort', DEFAULTS.apiPort),
      window.api.settings.set('telemetry', DEFAULTS.telemetry),
    ])
    toast.success('App settings reset')
  }

  return (
    <div className="p-7 max-w-3xl">
      <h1 className="text-2xl font-semibold text-text-0">App settings</h1>
      <p className="text-text-2 mt-1 mb-6">
        Per-project mesh, relay, and trust threshold live on the project pill in the top bar.
      </p>

      <Card className="p-5 mb-5">
        <div className="text-xs uppercase tracking-wider text-text-0 font-semibold mb-1">Appearance</div>
        <div className="text-xs text-text-2 mb-4">Applies instantly — no restart.</div>
        <label className="block text-xs text-text-1 mb-1.5">Theme</label>
        <SegGroup
          options={[
            { value: 'dark', label: 'Dark' },
            { value: 'light', label: 'Light' },
            { value: 'auto', label: 'Auto' },
          ]}
          value={ui.theme}
          onChange={ui.setTheme}
        />
        <label className="block text-xs text-text-1 mt-4 mb-1.5">Accent</label>
        <div className="flex gap-3 items-center">
          {(['emerald', 'violet', 'rose'] as const).map((c) => (
            <button
              key={c}
              onClick={() => ui.setAccent(c)}
              className={`w-6 h-6 rounded-full transition-all ${
                ui.accent === c ? 'ring-2 ring-white' : 'opacity-70 hover:opacity-100'
              }`}
              style={{ background: c === 'emerald' ? '#10b981' : c === 'violet' ? '#8b5cf6' : '#f43f5e' }}
            />
          ))}
        </div>
      </Card>

      <Card className="p-5 mb-5">
        <div className="text-xs uppercase tracking-wider text-text-0 font-semibold mb-1">Advanced</div>
        <div className="text-xs text-text-2 mb-3">Backend: <span className="font-mono text-text-0">agentfm {about?.version ?? '…'}</span></div>
        <label className="flex items-center gap-3 text-sm text-text-1 cursor-pointer">
          <input
            type="checkbox"
            checked={telemetry}
            onChange={async (e) => {
              setTelemetry(e.target.checked)
              await window.api.settings.set('telemetry', e.target.checked)
            }}
          />
          <div>
            Send anonymized usage telemetry
            <div className="text-2xs text-text-2">Crash counts and screen views only — never task contents.</div>
          </div>
        </label>
      </Card>

      <div className="flex justify-between items-center pt-4 border-t border-border-0">
        <Button variant="ghost" onClick={resetToDefaults}>Reset to defaults</Button>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Status — read floor from active project**

In `src/routes/Status.tsx`, replace:

```tsx
const reputationFloor = useUIStore((s) => s.reputationFloor);
```

with:

```tsx
const reputationFloor = useUIStore((s) => s.activeProject()?.reputationFloor ?? -0.5);
```

(That's the only reference.)

- [ ] **Step 3: Build + commit**

```bash
npx electron-vite build 2>&1 | tail -5
git add src/routes/Settings.tsx src/routes/Status.tsx
git commit -m "feat(settings): drop project-level fields; status reads floor from active project"
```

---

### Task 24: Update happy-path e2e for new shell

**Files:**
- Modify: `tests/e2e/happy-path.spec.ts`

The first-launch path now opens the CreateProjectWizard instead of the WelcomeModal. The wizard's primary action is "Create project". After the e2e test creates a default project, the existing assertions should continue to work.

- [ ] **Step 1: Replace the welcome-skip with wizard-complete**

In the `beforeAll` of `tests/e2e/happy-path.spec.ts`, replace the existing welcome-skip block:

```ts
  // Dismiss the first-launch welcome modal if present.
  const skip = page.locator('button:has-text("Skip")');
  if (await skip.isVisible({ timeout: 2000 }).catch(() => false)) {
    await skip.click();
  }
```

with:

```ts
  // First launch may show CreateProjectWizard. If so, fill defaults
  // and create — this puts the app into a steady "has a project" state.
  const wizard = page.locator('h2:has-text("New project")');
  if (await wizard.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.locator('input[placeholder*="Team Mesh"]').fill('E2E Default');
    await page.locator('button:has-text("Create project")').click();
    await wizard.waitFor({ state: 'hidden', timeout: 15000 });
  }
```

- [ ] **Step 2: Update settings + status expectations**

Settings page no longer has Mesh/Trust sections (those moved to the project sheet). Find this assertion block:

```ts
test('settings route renders all sections + reset button', async () => {
  await page.keyboard.press('Meta+5');
  await expect(page.locator('h1:has-text("Settings")')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('text=Mesh').first()).toBeVisible();
  await expect(page.locator('text=Trust').first()).toBeVisible();
  await expect(page.locator('text=Appearance').first()).toBeVisible();
  await expect(page.locator('text=Advanced').first()).toBeVisible();
  // Reset button must be wired (no longer shows the legacy toast)
  await expect(page.locator('button:has-text("Reset all to defaults")')).toBeVisible();
});
```

Replace with:

```ts
test('settings route renders app-level sections + reset button', async () => {
  await page.keyboard.press('Meta+5');
  await expect(page.locator('h1:has-text("App settings")')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('text=Appearance').first()).toBeVisible();
  await expect(page.locator('text=Advanced').first()).toBeVisible();
  await expect(page.locator('button:has-text("Reset to defaults")')).toBeVisible();
});
```

And replace the settings-reset test's confirm-text expectation to match the new copy:

```ts
    expect(d.message()).toMatch(/Reset appearance/i);
```

- [ ] **Step 3: Run e2e**

```bash
npx playwright test --timeout=120000 2>&1 | tail -15
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add tests/e2e/happy-path.spec.ts
git commit -m "test(e2e): update happy path for wizard + slimmed Settings"
```

---

### Task 25: New e2e — projects.spec.ts

**Files:**
- Create: `tests/e2e/projects.spec.ts`

- [ ] **Step 1: Write the spec**

`tests/e2e/projects.spec.ts`:

```ts
import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test';
import path from 'node:path';

let app: ElectronApplication;
let page: Page;

test.beforeAll(async () => {
  app = await electron.launch({
    args: ['.'],
    env: {
      ...process.env,
      AGENTFM_BIN: path.resolve(__dirname, '..', '..', '..', 'agentfm-core', 'agentfm-go', 'agentfm'),
    },
    cwd: path.resolve(__dirname, '..', '..'),
  });
  page = await app.firstWindow();
  await page.waitForLoadState('domcontentloaded');
});

test.afterAll(async () => {
  await app?.close();
});

test('wizard opens on first launch and creates the default project', async () => {
  const wizard = page.locator('h2:has-text("New project")');
  await expect(wizard).toBeVisible({ timeout: 8000 });

  await page.locator('input[placeholder*="Team Mesh"]').fill('Smoke Project');
  await page.locator('button:has-text("Create project")').click();
  await expect(wizard).toBeHidden({ timeout: 15000 });

  // Sidebar must show the project
  await expect(page.locator('text=Smoke Project').first()).toBeVisible();
  // ProjectPill in the top bar must show the project name too
  await expect(page.locator('header button:has-text("Smoke Project")')).toBeVisible();
});

test('rejects a duplicate relay when creating a second project', async () => {
  // Open the wizard from the sidebar
  await page.locator('button:has-text("+ new project")').click();
  await expect(page.locator('h2:has-text("New project")')).toBeVisible();

  await page.locator('input[placeholder*="Team Mesh"]').fill('Dupe');
  // Keep "Bundled public lighthouse" selected — same as the first project.
  await page.locator('button:has-text("Create project")').click();

  // A toast or inline error should appear; the wizard must NOT close.
  await expect(page.locator('h2:has-text("New project")')).toBeVisible();
  await expect(
    page.locator('text=/already uses the bundled|already uses /i'),
  ).toBeVisible({ timeout: 3000 });

  // Cancel out
  await page.locator('button:has-text("Cancel")').click();
});
```

- [ ] **Step 2: Run**

```bash
npx playwright test tests/e2e/projects.spec.ts --timeout=120000 2>&1 | tail -10
```

Expected: 2 passed.

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/projects.spec.ts
git commit -m "test(e2e): wizard creates project; duplicate-relay rejected"
```

---

### Task 26: New e2e — nav-stress.spec.ts

**Files:**
- Create: `tests/e2e/nav-stress.spec.ts`

- [ ] **Step 1: Write the spec**

`tests/e2e/nav-stress.spec.ts`:

```ts
import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test';
import path from 'node:path';

let app: ElectronApplication;
let page: Page;

test.beforeAll(async () => {
  app = await electron.launch({
    args: ['.'],
    env: {
      ...process.env,
      AGENTFM_BIN: path.resolve(__dirname, '..', '..', '..', 'agentfm-core', 'agentfm-go', 'agentfm'),
    },
    cwd: path.resolve(__dirname, '..', '..'),
  });
  page = await app.firstWindow();
  await page.waitForLoadState('domcontentloaded');
  const wizard = page.locator('h2:has-text("New project")');
  if (await wizard.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.locator('input[placeholder*="Team Mesh"]').fill('Nav Stress');
    await page.locator('button:has-text("Create project")').click();
    await wizard.waitFor({ state: 'hidden', timeout: 15000 });
  }
});

test.afterAll(async () => {
  await app?.close();
});

test('hammering Cmd+1..5 keeps <main> populated', async () => {
  // Cycle through tabs rapidly. After each, <main> must contain visible content
  // (not be empty / hidden / black).
  const keys = ['Meta+1', 'Meta+2', 'Meta+3', 'Meta+4', 'Meta+5'];
  for (let round = 0; round < 4; round++) {
    for (const k of keys) {
      await page.keyboard.press(k);
      await page.waitForTimeout(80);
      const mainHasContent = await page.evaluate(() => {
        const m = document.querySelector('main');
        if (!m) return false;
        return (m.textContent ?? '').trim().length > 0;
      });
      expect(mainHasContent, `main went blank after ${k} round ${round}`).toBe(true);
    }
  }
});
```

- [ ] **Step 2: Run**

```bash
npx playwright test tests/e2e/nav-stress.spec.ts --timeout=120000 2>&1 | tail -10
```

Expected: 1 passed.

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/nav-stress.spec.ts
git commit -m "test(e2e): nav-stress — main pane never blanks across tab switches"
```

---

### Task 27: Final full-suite verification

- [ ] **Step 1: Run all unit tests**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx vitest run tests/unit/
```

Expected: all green. Should be ≥30 tests now (existing 26 + the new displayName, projectStore, projectMigration, ansi suites).

- [ ] **Step 2: Run all e2e tests**

Kill any orphans first:

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
```

Then:

```bash
npx playwright test --timeout=120000 2>&1 | tail -20
```

Expected: all suites green (happy-path, connect-flow, projects, nav-stress).

- [ ] **Step 3: Manual smoke (Spec §8.3)**

Start the dev app:

```bash
./scripts/dev.sh &
sleep 12
```

Perform manually:
- Submit a comment via FeedbackModal on a peer. Confirm it appears in Activity within ≤2 s (no SSE round-trip wait).
- Create a second project with a different relay. Switch between projects. Confirm Radar shows a different worker set; confirm `boss_peer_id` from /v1/about is identical on both.
- Send a chat message; force a long ANSI-laden reply (the bundled HR agent does this). Confirm: no horizontal scroll, Composer stays visible, "← Radar" link is clickable.

Stop the app:

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
```

- [ ] **Step 4: Commit a release-readiness note**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git tag -a phase1-complete -m "Phase 1: project model + critical bug fixes"
```

---

## Self-review

**Spec coverage (cross-checked against the spec):**
- §2.1 sidebar Projects section + new project button → Task 12 + 17
- §2.1 top bar three columns + AgentFM centered + ProjectPill → Task 16
- §2.1 empty-state first-launch wizard → Task 18
- §2.2 switching project / overlay / restart → Tasks 12 + 15 + 17
- §2.3 app-level vs project-level settings split → Tasks 14 + 23
- §3.1 Project type → Task 2
- §3.2 store additions → Task 8
- §3.3 persistence keys → Tasks 6 + 8
- §3.4 relay uniqueness → Tasks 3 + 8 + 25
- §4 boss identity persistence (verify + doc) → Task 1
- §5.1 component tree → Tasks 11–17
- §5.2 route error boundaries → Task 11
- §5.3 displayName → Tasks 4 + 19
- §5.4 usePeerName → Task 20
- §6.1 black-screen → Tasks 9 (mountedRef) + 11 (boundary) + 26 (test)
- §6.2 chat overflow → Tasks 5 + 22
- §6.3 unknown labels → Tasks 4 + 19
- §6.4 comments invalidation → Task 21
- §6.5 my-activity peer→name → Task 20
- §6.6 topbar centering → Task 16
- §7 migration → Tasks 6 + 7 + 10
- §8 testing → Tasks 24 + 25 + 26 + 27

No gaps identified.

**Placeholder scan:** All steps include exact code, exact commands, and expected outputs. No TBDs.

**Type consistency:** `Project`, `ProjectInput`, `ProjectColor`, `DuplicateRelayError`, `migrateLegacySettings`, `displayName`, `stripAnsi`, `RouteErrorBoundary`, `ProjectList`, `ProjectPill`, `CreateProjectWizard`, `ProjectSettingsSheet`, `ProjectSwitchingOverlay`, `usePeerName` are each defined once and referenced consistently across later tasks.

Plan complete.
