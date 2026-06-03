# UX Simplification (Phase 2) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip the navigation surface to TopBar (with project dropdown + relay) + horizontal TabStrip + footer settings. Simplify project creation to name+relay. Make relay immutable. Delete the sidebar, ProjectPill, ProjectList, ProjectSettingsSheet, WelcomeModal, and the /settings route.

**Architecture:** No backend changes. Renderer-only refactor. The Phase 1 Zustand store and persistence stay, with `Project.icon` and `Project.color` dropped from the type (forward-compatible — legacy fields ignored in storage). The Phase 1 `BackendManager` and migration are unchanged.

**Tech Stack:** React 18 + TypeScript, Zustand, electron-store, TanStack Query, React Router (HashRouter), Framer Motion, Tailwind, Vitest, Playwright. No new dependencies.

**Reference spec:** `docs/superpowers/specs/2026-05-18-ux-simplification-design.md`.

---

## File map

**Create:**
- `src/components/projects/ProjectDropdown.tsx` — chip + menu (project list + new + delete)
- `src/components/TabStrip.tsx` — horizontal route tabs
- `src/components/EmptyState.tsx` — centered "Create your first project" card
- `src/components/SettingsSheet.tsx` — slide-in theme-only sheet
- `src/components/SettingsFooter.tsx` — footer button that opens the sheet
- `tests/e2e/tabstrip.spec.ts` — keyboard + click nav

**Modify:**
- `src/types/project.ts` — drop `icon`, `color`
- `src/lib/projectStore.ts` — `ProjectInput` no longer has `icon`/`color`
- `src/lib/store.ts` — `addProject` defaults; new `switchProject(id)` action
- `src/lib/projectMigration.ts` — default project without `icon`/`color`
- `src/components/projects/CreateProjectWizard.tsx` — name + relay only
- `src/components/TopBar.tsx` — wordmark + dropdown + relay; hidden parts in empty state
- `src/components/Shell.tsx` — new layout: TopBar / TabStrip / main / footer
- `src/hooks/useGlobalShortcuts.ts` — drop Cmd+5 mapping
- `src/App.tsx` — drop `/settings` route; drop mounts for `WelcomeModal` + `ProjectSettingsSheet`; mount `SettingsSheet`
- `tests/unit/projectStore.test.ts` — drop icon/color from test fixtures
- `tests/unit/projectMigration.test.ts` — drop icon/color from default-project assertion
- `tests/e2e/happy-path.spec.ts` — wizard fields; footer settings; drop Cmd+5
- `tests/e2e/projects.spec.ts` — narrower wizard
- `tests/e2e/nav-stress.spec.ts` — Cmd+1..4 only

**Delete:**
- `src/components/Sidebar.tsx`
- `src/components/projects/ProjectList.tsx`
- `src/components/projects/ProjectPill.tsx`
- `src/components/projects/ProjectSettingsSheet.tsx`
- `src/components/WelcomeModal.tsx`
- `src/routes/Settings.tsx`

---

## Task list

### Task 1: Narrow Project + ProjectInput types

**Files:**
- Modify: `src/types/project.ts`
- Modify: `src/lib/projectStore.ts`

- [ ] **Step 1: Replace `src/types/project.ts`**

```ts
export interface Project {
  id: string
  name: string
  relayMultiaddr: string | null
  reputationFloor: number
  createdAt: number
}
```

(Drops `icon`, `color`, `ProjectColor`, `PROJECT_COLORS`.)

- [ ] **Step 2: Replace `src/lib/projectStore.ts`**

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

- [ ] **Step 3: Update `tests/unit/projectStore.test.ts`**

Replace the `base()` helper at the top with:

```ts
const base = (overrides: Partial<Project> = {}): Project => ({
  id: 'prj_test',
  name: 'Default',
  relayMultiaddr: null,
  reputationFloor: -0.5,
  createdAt: 0,
  ...overrides,
})
```

(Drops `icon` and `color` from the fixture. Tests themselves are unchanged.)

- [ ] **Step 4: Run unit tests**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx vitest run tests/unit/projectStore.test.ts
```

Expected: 8 passed.

- [ ] **Step 5: Commit**

```bash
git add src/types/project.ts src/lib/projectStore.ts tests/unit/projectStore.test.ts
git commit -m "feat(projects): narrow Project type to name + relay + floor"
```

---

### Task 2: Update Zustand store

**Files:**
- Modify: `src/lib/store.ts`

- [ ] **Step 1: Patch `addProject` to drop icon/color**

Open `src/lib/store.ts`. Find the `addProject` action. Replace the body with:

```ts
    addProject: (input) => {
      const projects = get().projects
      validateProjectInput(projects, input)
      const project: Project = {
        id: newProjectId(),
        name: input.name.trim(),
        relayMultiaddr: input.relayMultiaddr,
        reputationFloor: input.reputationFloor ?? -0.5,
        createdAt: Date.now(),
      }
      const nextProjects = [...projects, project]
      set({ projects: nextProjects })
      window.api?.settings.set('projects', nextProjects).catch(() => {})
      return project
    },
```

- [ ] **Step 2: Patch `updateProject` to drop icon/color**

Find `updateProject`. Replace its body with:

```ts
    updateProject: (id, patch) => {
      const projects = get().projects
      const current = projects.find((p) => p.id === id)
      if (!current) return
      const merged: ProjectInput = {
        name: patch.name ?? current.name,
        relayMultiaddr:
          patch.relayMultiaddr === undefined ? current.relayMultiaddr : patch.relayMultiaddr,
        reputationFloor: patch.reputationFloor ?? current.reputationFloor,
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
```

- [ ] **Step 3: Add `switchProject(id)` action**

In the `UIState` interface, add the signature alongside the other project actions:

```ts
  switchProject: (id: string) => Promise<void>
```

In the store implementation, add the implementation alongside the other project actions:

```ts
    switchProject: async (id) => {
      const state = get()
      if (id === state.activeProjectId) return
      const project = state.projects.find((p) => p.id === id)
      if (!project) return
      set({ isProjectSwitching: true })
      try {
        await window.api?.settings.set('activeProjectId', id)
        set({ activeProjectId: id })
        await window.api?.backend.restart({
          apiPort: state.apiPort,
          reputationFloor: project.reputationFloor,
          relayMultiaddr: project.relayMultiaddr ?? undefined,
        })
      } catch (e) {
        console.warn('switchProject: backend restart failed', e)
      } finally {
        set({ isProjectSwitching: false })
      }
    },
```

- [ ] **Step 4: Drop the `openProjectSettings` / `closeProjectSettings` actions and the `isProjectSettingsOpen` field**

These are unused after the ProjectSettingsSheet is deleted.

Remove the field:
- In `UIState` interface, delete the line: `isProjectSettingsOpen: boolean`
- In the store's initial state, delete: `isProjectSettingsOpen: false,`
- In the action signatures, delete: `openProjectSettings: () => void` and `closeProjectSettings: () => void`
- In the action implementations, delete the two implementing lines for these.

- [ ] **Step 5: Typecheck**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx tsc -p tsconfig.web.json --noEmit 2>&1 | grep -v "ExperimentalWarning" | grep -v "^node_modules" | head -30
```

Expected: errors in the about-to-be-deleted files (`ProjectSettingsSheet.tsx`, `ProjectPill.tsx`, `ProjectList.tsx`, `Sidebar.tsx`, `WelcomeModal.tsx`, `Settings.tsx`, `CreateProjectWizard.tsx`) referencing dropped fields and the dropped store actions. Leave them alone — they get deleted or rewritten in later tasks.

- [ ] **Step 6: Run unit tests**

```bash
npx vitest run tests/unit/
```

Expected: all green (store isn't directly tested; helper tests already updated in Task 1).

- [ ] **Step 7: Commit**

```bash
git add src/lib/store.ts
git commit -m "feat(store): drop icon/color from project actions; add switchProject; remove ProjectSettings modal state"
```

---

### Task 3: Update projectMigration

**Files:**
- Modify: `src/lib/projectMigration.ts`
- Modify: `tests/unit/projectMigration.test.ts`

- [ ] **Step 1: Replace `src/lib/projectMigration.ts`**

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

- [ ] **Step 2: Update `tests/unit/projectMigration.test.ts`**

Find the test `'does nothing when projects already exist'`. Update its fixture project to drop `icon` and `color`:

```ts
  it('does nothing when projects already exist', async () => {
    const store = fakeStore({
      projects: [{ id: 'prj_keep', name: 'X', relayMultiaddr: null, reputationFloor: -0.5, createdAt: 1 }],
    })
    await migrateLegacySettings(store)
    expect(await store.get('projects')).toHaveLength(1)
  })
```

(The other three tests don't reference `icon`/`color` and don't need changes.)

- [ ] **Step 3: Run unit tests**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx vitest run tests/unit/projectMigration.test.ts
```

Expected: 4 passed.

- [ ] **Step 4: Commit**

```bash
git add src/lib/projectMigration.ts tests/unit/projectMigration.test.ts
git commit -m "feat(projects): default project no longer carries icon/color"
```

---

### Task 4: Simplify CreateProjectWizard

**Files:**
- Modify: `src/components/projects/CreateProjectWizard.tsx`

- [ ] **Step 1: Rewrite the component**

Replace the entire `src/components/projects/CreateProjectWizard.tsx` with:

```tsx
import { useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../../lib/store'
import { Button } from '../primitives/Button'
import { Input } from '../primitives/Input'
import { toast } from 'sonner'
import { DuplicateRelayError } from '../../lib/projectStore'

const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/

export function CreateProjectWizard() {
  const open = useUIStore((s) => s.isCreateWizardOpen)
  const close = useUIStore((s) => s.closeCreateWizard)
  const addProject = useUIStore((s) => s.addProject)
  const setSwitching = useUIStore((s) => s.setProjectSwitching)

  const [name, setName] = useState('')
  const [useDefault, setUseDefault] = useState(true)
  const [relay, setRelay] = useState('')
  const [saving, setSaving] = useState(false)

  function reset() {
    setName('')
    setUseDefault(true)
    setRelay('')
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
      project = addProject({ name, relayMultiaddr: relayValue })
    } catch (e) {
      const msg = e instanceof DuplicateRelayError ? e.message : (e as Error).message
      toast.error(msg)
      setSaving(false)
      return
    }
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
            className="w-[460px] bg-bg-1 border border-border-0 rounded-xl p-7 shadow-2xl"
          >
            <div className="flex justify-between items-start mb-5">
              <div>
                <h2 className="text-xl font-semibold text-text-0">New project</h2>
                <p className="text-sm text-text-2 mt-1">
                  A project pairs a name with a relay. You can't change the relay later.
                </p>
              </div>
              <button onClick={() => { close(); reset() }} className="text-text-2 hover:text-text-0 text-lg">✕</button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-1.5">Name</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Team Mesh" autoFocus />

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
                <div className="text-sm text-text-0">Custom multiaddr</div>
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

            <div className="flex justify-end gap-2 mt-7">
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

- [ ] **Step 2: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -5
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add src/components/projects/CreateProjectWizard.tsx
git commit -m "feat(projects): simplify CreateProjectWizard to name+relay only"
```

---

### Task 5: ProjectDropdown component

**Files:**
- Create: `src/components/projects/ProjectDropdown.tsx`

- [ ] **Step 1: Implement**

Create `src/components/projects/ProjectDropdown.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { toast } from 'sonner'
import { useUIStore } from '../../lib/store'

export function ProjectDropdown() {
  const projects = useUIStore((s) => s.projects)
  const activeId = useUIStore((s) => s.activeProjectId)
  const active = useUIStore((s) => s.activeProject())
  const switchProject = useUIStore((s) => s.switchProject)
  const deleteProject = useUIStore((s) => s.deleteProject)
  const openWizard = useUIStore((s) => s.openCreateWizard)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    if (open) {
      document.addEventListener('mousedown', onDown)
      document.addEventListener('keydown', onKey)
      return () => {
        document.removeEventListener('mousedown', onDown)
        document.removeEventListener('keydown', onKey)
      }
    }
  }, [open])

  if (!active) return null

  function handleDelete() {
    if (!active) return
    if (!window.confirm(`Delete "${active.name}"? Its chat sessions will be removed.`)) return
    deleteProject(active.id)
    toast.success(`Project "${active.name}" deleted`)
    setOpen(false)
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        className="inline-flex items-center gap-2 bg-bg-1 hover:bg-bg-2 border border-border-0 rounded-full px-3 py-1.5 text-xs text-text-1 transition-colors"
      >
        <span>📁</span>
        <span className="font-medium text-text-0 max-w-[200px] truncate">{active.name}</span>
        <span className="text-text-2">▾</span>
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.12 }}
            className="absolute top-full mt-1 left-0 bg-bg-1 border border-border-0 rounded-md shadow-xl w-72 max-h-80 overflow-auto z-50"
          >
            {projects.map((p) => (
              <button
                key={p.id}
                onClick={() => {
                  setOpen(false)
                  switchProject(p.id)
                }}
                className={`w-full text-left px-3 py-2 text-xs hover:bg-bg-2 ${
                  p.id === activeId ? 'text-accent' : 'text-text-1'
                }`}
              >
                <div className="flex items-center gap-2">
                  <span>📁</span>
                  <span className="font-medium truncate">{p.name}</span>
                </div>
              </button>
            ))}
            <div className="border-t border-border-0" />
            <button
              onClick={() => {
                setOpen(false)
                openWizard()
              }}
              className="w-full text-left px-3 py-2 text-xs text-text-1 hover:bg-bg-2 hover:text-accent"
            >
              + New project
            </button>
            <button
              onClick={handleDelete}
              className="w-full text-left px-3 py-2 text-xs text-rose-400 hover:bg-bg-2 hover:text-rose-300"
            >
              Delete "{active.name}"
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
git add src/components/projects/ProjectDropdown.tsx
git commit -m "feat(projects): ProjectDropdown chip with switch/new/delete"
```

---

### Task 6: TabStrip component

**Files:**
- Create: `src/components/TabStrip.tsx`

- [ ] **Step 1: Implement**

Create `src/components/TabStrip.tsx`:

```tsx
import { NavLink } from 'react-router-dom'
import { useUIStore } from '../lib/store'

const tabs = [
  { to: '/radar', label: 'Radar' },
  { to: '/chat', label: 'Chat' },
  { to: '/activity', label: 'Activity' },
  { to: '/status', label: 'Status' },
]

function tabClass({ isActive }: { isActive: boolean }) {
  return `px-4 py-2 text-sm border-b-2 transition-colors ${
    isActive
      ? 'border-accent text-text-0'
      : 'border-transparent text-text-2 hover:text-text-0'
  }`
}

export function TabStrip() {
  const active = useUIStore((s) => s.activeProject())
  if (!active) return null

  return (
    <div className="border-b border-border-0 bg-bg-0 px-3 flex gap-1">
      {tabs.map((t) => (
        <NavLink key={t.to} to={t.to} className={tabClass}>
          {t.label}
        </NavLink>
      ))}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git add src/components/TabStrip.tsx
git commit -m "feat(shell): TabStrip horizontal route tabs"
```

---

### Task 7: EmptyState component

**Files:**
- Create: `src/components/EmptyState.tsx`

- [ ] **Step 1: Implement**

Create `src/components/EmptyState.tsx`:

```tsx
import { motion } from 'framer-motion'
import { useUIStore } from '../lib/store'
import { Button } from './primitives/Button'

export function EmptyState() {
  const openWizard = useUIStore((s) => s.openCreateWizard)

  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="flex-1 flex items-center justify-center p-8"
    >
      <div className="max-w-md text-center bg-bg-1 border border-border-0 rounded-2xl p-10">
        <div className="text-5xl mb-4">📁</div>
        <h1 className="text-xl font-semibold text-text-0">Welcome to AgentFM</h1>
        <p className="text-text-2 mt-2 mb-6">
          Create your first project to get started. A project pairs a name with a relay; you can
          add more later.
        </p>
        <Button variant="primary" onClick={openWizard}>
          Create project
        </Button>
      </div>
    </motion.div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git add src/components/EmptyState.tsx
git commit -m "feat(shell): EmptyState centered welcome card"
```

---

### Task 8: SettingsSheet component

**Files:**
- Create: `src/components/SettingsSheet.tsx`

- [ ] **Step 1: Implement**

Create `src/components/SettingsSheet.tsx`:

```tsx
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../lib/store'
import { SegGroup } from './primitives/SegGroup'

export function SettingsSheet() {
  const open = useUIStore((s) => s.isSettingsSheetOpen)
  const close = useUIStore((s) => s.closeSettingsSheet)
  const theme = useUIStore((s) => s.theme)
  const setTheme = useUIStore((s) => s.setTheme)

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/65 backdrop-blur-sm z-[65] flex justify-end"
          onClick={close}
        >
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 320, damping: 30 }}
            onClick={(e) => e.stopPropagation()}
            className="w-[380px] h-full bg-bg-1 border-l border-border-0 p-6"
          >
            <div className="flex justify-between items-start mb-6">
              <h2 className="text-xl font-semibold text-text-0">Settings</h2>
              <button onClick={close} className="text-text-2 hover:text-text-0 text-lg">✕</button>
            </div>

            <label className="block text-xs uppercase tracking-wider text-text-2 mb-2">Theme</label>
            <SegGroup
              options={[
                { value: 'dark', label: 'Dark' },
                { value: 'light', label: 'Light' },
                { value: 'auto', label: 'Auto' },
              ]}
              value={theme}
              onChange={setTheme}
            />
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
```

This references `isSettingsSheetOpen` / `closeSettingsSheet` which will be added to the store in the next task.

- [ ] **Step 2: Add store state for the settings sheet**

In `src/lib/store.ts`:

In the `UIState` interface, add:
```ts
  isSettingsSheetOpen: boolean
  openSettingsSheet: () => void
  closeSettingsSheet: () => void
```

In the initial state, add:
```ts
    isSettingsSheetOpen: false,
```

In the action implementations, add:
```ts
    openSettingsSheet: () => set({ isSettingsSheetOpen: true }),
    closeSettingsSheet: () => set({ isSettingsSheetOpen: false }),
```

- [ ] **Step 3: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

- [ ] **Step 4: Commit**

```bash
git add src/components/SettingsSheet.tsx src/lib/store.ts
git commit -m "feat(settings): SettingsSheet slide-in panel; theme-only"
```

---

### Task 9: SettingsFooter component

**Files:**
- Create: `src/components/SettingsFooter.tsx`

- [ ] **Step 1: Implement**

Create `src/components/SettingsFooter.tsx`:

```tsx
import { useUIStore } from '../lib/store'

export function SettingsFooter() {
  const openSettings = useUIStore((s) => s.openSettingsSheet)
  return (
    <footer className="border-t border-border-0 bg-bg-0 px-3 py-2 flex items-center">
      <button
        onClick={openSettings}
        className="inline-flex items-center gap-2 text-xs text-text-2 hover:text-text-0 transition-colors px-2 py-1"
      >
        <span className="text-sm leading-none">⚙</span>
        <span>Settings</span>
      </button>
    </footer>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git add src/components/SettingsFooter.tsx
git commit -m "feat(settings): SettingsFooter button"
```

---

### Task 10: Rewrite TopBar

**Files:**
- Modify: `src/components/TopBar.tsx`

- [ ] **Step 1: Replace the file**

Replace `src/components/TopBar.tsx` with:

```tsx
import { toast } from 'sonner'
import { useUIStore } from '../lib/store'
import { ProjectDropdown } from './projects/ProjectDropdown'

function truncateMultiaddr(m: string): string {
  if (m.length <= 32) return m
  return m.slice(0, 14) + '…' + m.slice(-14)
}

export function TopBar() {
  const active = useUIStore((s) => s.activeProject())

  async function copyRelay() {
    if (!active) return
    const value = active.relayMultiaddr ?? '(bundled public lighthouse)'
    try {
      await navigator.clipboard.writeText(value)
      toast.success('Relay copied')
    } catch {
      toast.error('Could not copy')
    }
  }

  return (
    <header className="h-11 border-b border-border-0 bg-bg-0 flex items-center gap-4 px-3 select-none">
      <div className="text-sm font-semibold tracking-tight text-text-0">AgentFM</div>
      {active && <ProjectDropdown />}
      <div className="flex-1" />
      {active && (
        <button
          onClick={copyRelay}
          className="text-2xs text-text-2 hover:text-text-0 font-mono transition-colors"
          title={active.relayMultiaddr ?? 'bundled public lighthouse'}
        >
          relay: {active.relayMultiaddr ? truncateMultiaddr(active.relayMultiaddr) : 'bundled'}
        </button>
      )}
    </header>
  )
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

- [ ] **Step 3: Commit**

```bash
git add src/components/TopBar.tsx
git commit -m "feat(topbar): wordmark + project dropdown + relay display"
```

---

### Task 11: Rewrite Shell

**Files:**
- Modify: `src/components/Shell.tsx`

- [ ] **Step 1: Replace the file**

Replace `src/components/Shell.tsx` with:

```tsx
import { Outlet, useLocation } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import { useUIStore } from '../lib/store'
import { TopBar } from './TopBar'
import { TabStrip } from './TabStrip'
import { EmptyState } from './EmptyState'
import { SettingsFooter } from './SettingsFooter'
import { RouteErrorBoundary } from './RouteErrorBoundary'
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts'

export function Shell() {
  useGlobalShortcuts()
  const loc = useLocation()
  const active = useUIStore((s) => s.activeProject())

  return (
    <div className="h-screen flex flex-col bg-bg-0 text-text-0 font-sans">
      <TopBar />
      <TabStrip />
      <main className="flex-1 overflow-hidden flex flex-col">
        {!active ? (
          <EmptyState />
        ) : (
          <AnimatePresence mode="wait">
            <motion.div
              key={loc.pathname}
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.18 }}
              className="h-full overflow-auto"
            >
              <RouteErrorBoundary>
                <Outlet />
              </RouteErrorBoundary>
            </motion.div>
          </AnimatePresence>
        )}
      </main>
      <SettingsFooter />
    </div>
  )
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

Expected: build may fail because `App.tsx` still references `Sidebar`, `WelcomeModal`, etc. That's fine — they get cleaned up in Task 13.

- [ ] **Step 3: Commit**

```bash
git add src/components/Shell.tsx
git commit -m "feat(shell): TopBar + TabStrip + EmptyState + SettingsFooter layout"
```

---

### Task 12: Update useGlobalShortcuts (drop Cmd+5)

**Files:**
- Modify: `src/hooks/useGlobalShortcuts.ts`

- [ ] **Step 1: Read current**

```bash
cat /Users/saif/Desktop/agentfm-prod/agentfm-desktop/src/hooks/useGlobalShortcuts.ts
```

- [ ] **Step 2: Remove Cmd+5 → /settings mapping**

In the hook, find the binding for `cmd+5` / `meta+5` (it navigates to `/settings`). Delete that binding entirely. Keep `cmd+1..4` mapping to `/radar`, `/chat`, `/activity`, `/status`.

If there's a binding for `Esc` that closes overlays, keep it. Add a new binding for `cmd+,` (the macOS convention) that opens the settings sheet:

```ts
  useHotkeys('mod+,', (e) => {
    e.preventDefault()
    useUIStore.getState().openSettingsSheet()
  }, [])
```

Add the import if not present: `import { useUIStore } from '../lib/store'`.

- [ ] **Step 3: Build**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx electron-vite build 2>&1 | tail -3
```

- [ ] **Step 4: Commit**

```bash
git add src/hooks/useGlobalShortcuts.ts
git commit -m "feat(shortcuts): drop Cmd+5; add Cmd+, for settings sheet"
```

---

### Task 13: Update App.tsx + delete unused files

**Files:**
- Modify: `src/App.tsx`
- Delete: `src/components/Sidebar.tsx`
- Delete: `src/components/projects/ProjectList.tsx`
- Delete: `src/components/projects/ProjectPill.tsx`
- Delete: `src/components/projects/ProjectSettingsSheet.tsx`
- Delete: `src/components/WelcomeModal.tsx`
- Delete: `src/routes/Settings.tsx`

- [ ] **Step 1: Replace `src/App.tsx`**

```tsx
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Shell } from './components/Shell'
import { BackendDownOverlay } from './components/BackendDownOverlay'
import { ErrorBoundary } from './components/ErrorBoundary'
import { DispatchDrawer } from './components/DispatchDrawer'
import { FeedbackModal } from './components/FeedbackModal'
import { ProjectSwitchingOverlay } from './components/projects/ProjectSwitchingOverlay'
import { CreateProjectWizard } from './components/projects/CreateProjectWizard'
import { SettingsSheet } from './components/SettingsSheet'
import { useBackend } from './hooks/useBackend'
import { useEventStream } from './hooks/useEventStream'
import Radar from './routes/Radar'
import Chat from './routes/Chat'
import PeerView from './routes/PeerView'
import Activity from './routes/Activity'
import Status from './routes/Status'

export default function App() {
  const backend = useBackend()
  useEventStream()
  const showOverlay = backend.consecutiveFailures >= 3

  return (
    <>
      <HashRouter>
        <ErrorBoundary>
          <Routes>
            <Route element={<Shell />}>
              <Route index element={<Navigate to="/radar" replace />} />
              <Route path="radar" element={<Radar />} />
              <Route path="chat" element={<Chat />} />
              <Route path="chat/:sessionId" element={<Chat />} />
              <Route path="peer/:peerId" element={<PeerView />} />
              <Route path="activity" element={<Activity />} />
              <Route path="status" element={<Status />} />
            </Route>
          </Routes>
        </ErrorBoundary>
      </HashRouter>
      <BackendDownOverlay show={showOverlay} />
      <DispatchDrawer />
      <FeedbackModal />
      <ProjectSwitchingOverlay />
      <CreateProjectWizard />
      <SettingsSheet />
    </>
  )
}
```

(Drops imports of `WelcomeModal`, `ProjectSettingsSheet`, `Settings` route. Adds `SettingsSheet`.)

- [ ] **Step 2: Delete the now-unused files**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git rm src/components/Sidebar.tsx \
  src/components/projects/ProjectList.tsx \
  src/components/projects/ProjectPill.tsx \
  src/components/projects/ProjectSettingsSheet.tsx \
  src/components/WelcomeModal.tsx \
  src/routes/Settings.tsx
```

- [ ] **Step 3: Build**

```bash
npx electron-vite build 2>&1 | tail -5
```

Expected: clean build (the previously-broken type errors against `isProjectSettingsOpen` etc. are now gone since the consumers are deleted).

- [ ] **Step 4: Commit**

```bash
git add src/App.tsx
git commit -m "feat(shell): drop Sidebar, ProjectList/Pill/SettingsSheet, WelcomeModal, /settings route"
```

---

### Task 14: First end-to-end smoke

- [ ] **Step 1: Kill orphans**

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
```

- [ ] **Step 2: Reset settings to test fresh-install flow**

```bash
mv "$HOME/Library/Application Support/agentfm-desktop/settings.json" "$HOME/Library/Application Support/agentfm-desktop/settings.json.bak.phase2" 2>/dev/null || true
```

- [ ] **Step 3: Launch dev and verify**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
./scripts/dev.sh &
sleep 12
```

Verify in the open Electron window:

1. **Empty state.** With no `projects` in settings, the migration creates a default project, so you actually land on the `/radar` route immediately. (The "true empty state" only fires if migration is bypassed; the legacy migration creates a Default project on every fresh install. This is expected from Phase 1.)
2. **TopBar.** AgentFM wordmark on the left; project dropdown chip ("📁 Default ▾") next to it; relay text on the right.
3. **TabStrip.** Radar / Chat / Activity / Status as horizontal tabs under TopBar. Active tab highlighted.
4. **Footer.** `⚙ Settings` button at the bottom.
5. **Settings sheet.** Click footer → slim sheet slides in from the right with just a Theme segmented control.
6. **Project dropdown.** Click chip → menu with the Default project, divider, + New project, Delete "Default".

If anything looks off, fix inline before moving to e2e tasks. Stop the dev app:

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
```

- [ ] **Step 4: Restore the backup if you want to keep your data**

```bash
ls "$HOME/Library/Application Support/agentfm-desktop/settings.json.bak.phase2" 2>/dev/null && \
  mv "$HOME/Library/Application Support/agentfm-desktop/settings.json.bak.phase2" \
     "$HOME/Library/Application Support/agentfm-desktop/settings.json"
```

(Optional.)

- [ ] **Step 5: Commit any inline fixes (if any)**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git status
git add -A
git commit -m "fix(shell): minor inline fixes from first smoke" || echo "nothing to commit"
```

---

### Task 15: Update happy-path e2e

**Files:**
- Modify: `tests/e2e/happy-path.spec.ts`

- [ ] **Step 1: Update settings test for footer-driven flow**

Find the test:
```ts
test('settings route renders app-level sections + reset button', async () => {
  await page.keyboard.press('Meta+5');
  ...
});
```

Replace with:
```ts
test('settings sheet opens from footer with theme control', async () => {
  await page.locator('footer button:has-text("Settings")').click();
  await expect(page.locator('h2:has-text("Settings")')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('text=Theme').first()).toBeVisible();
  await page.locator('button:has-text("✕")').click();
});
```

- [ ] **Step 2: Update the settings-reset test (drop it)**

Find `test('settings reset shows confirm and cancels cleanly', ...)`. **Delete the entire test** — the reset button is gone.

- [ ] **Step 3: Update the activity tab navigation**

If the suite uses `Meta+5` anywhere, change those to `footer button` clicks or `Meta+,`.

- [ ] **Step 4: Run e2e**

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx playwright test tests/e2e/happy-path.spec.ts --timeout=120000 2>&1 | tail -15
```

Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add tests/e2e/happy-path.spec.ts
git commit -m "test(e2e): happy-path uses footer settings sheet"
```

---

### Task 16: Update projects e2e

**Files:**
- Modify: `tests/e2e/projects.spec.ts`

- [ ] **Step 1: Update wizard-fields expectations**

Read:
```bash
cat /Users/saif/Desktop/agentfm-prod/agentfm-desktop/tests/e2e/projects.spec.ts
```

The wizard now has only Name + Relay (no icon, no color picker, no reputation-floor slider). Update any assertion that expected icon/color UI — drop those assertions. Keep:
- The wizard heading "New project".
- Name input fill via `input[placeholder*="Team Mesh"]`.
- "Create project" button click.
- Duplicate-relay error path.

The `+ new project` button is now inside the **project dropdown chip**, not the sidebar. Update the test that opens the wizard for a second project:

Replace:
```ts
  await page.locator('button:has-text("+ new project")').click();
```

with:
```ts
  await page.locator('header button:has(:text("📁"))').first().click();
  await page.locator('text=+ New project').click();
```

- [ ] **Step 2: Run e2e**

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx playwright test tests/e2e/projects.spec.ts --timeout=120000 2>&1 | tail -10
```

Expected: 2 passed.

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/projects.spec.ts
git commit -m "test(e2e): projects spec uses dropdown for + new project; drop icon/color asserts"
```

---

### Task 17: Update nav-stress e2e

**Files:**
- Modify: `tests/e2e/nav-stress.spec.ts`

- [ ] **Step 1: Drop Meta+5 from the key set**

Find the line:
```ts
const keys = ['Meta+1', 'Meta+2', 'Meta+3', 'Meta+4', 'Meta+5'];
```

Replace with:
```ts
const keys = ['Meta+1', 'Meta+2', 'Meta+3', 'Meta+4'];
```

- [ ] **Step 2: Run**

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx playwright test tests/e2e/nav-stress.spec.ts --timeout=120000 2>&1 | tail -10
```

Expected: 1 passed.

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/nav-stress.spec.ts
git commit -m "test(e2e): nav-stress restricted to Meta+1..4"
```

---

### Task 18: New tabstrip e2e

**Files:**
- Create: `tests/e2e/tabstrip.spec.ts`

- [ ] **Step 1: Write the spec**

Create `tests/e2e/tabstrip.spec.ts`:

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

  await page.waitForFunction(
    async () => {
      const api = (window as unknown as { api?: { backend: { health: () => Promise<{ ok: boolean }> } } }).api;
      if (!api) return false;
      try {
        const r = await api.backend.health();
        return r.ok === true;
      } catch {
        return false;
      }
    },
    { timeout: 30000, polling: 500 },
  );

  const wizard = page.locator('h2:has-text("New project")');
  if (await wizard.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.locator('input[placeholder*="Team Mesh"]').fill('TabStrip Test');
    await page.locator('button:has-text("Create project")').click();
    await wizard.waitFor({ state: 'hidden', timeout: 15000 });
  }
});

test.afterAll(async () => {
  await app?.close();
});

test('clicking each tab navigates and highlights', async () => {
  for (const label of ['Chat', 'Activity', 'Status', 'Radar']) {
    await page.locator(`a:has-text("${label}")`).click();
    const active = page.locator(`a:has-text("${label}")`).first();
    await expect(active).toHaveClass(/border-accent/);
  }
});

test('Cmd+1..4 navigate through tabs', async () => {
  const cases = [
    { key: 'Meta+2', label: 'Chat' },
    { key: 'Meta+3', label: 'Activity' },
    { key: 'Meta+4', label: 'Status' },
    { key: 'Meta+1', label: 'Radar' },
  ];
  for (const c of cases) {
    await page.keyboard.press(c.key);
    await page.waitForTimeout(80);
    const active = page.locator(`a:has-text("${c.label}")`).first();
    await expect(active).toHaveClass(/border-accent/);
  }
});
```

- [ ] **Step 2: Run**

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx playwright test tests/e2e/tabstrip.spec.ts --timeout=120000 2>&1 | tail -10
```

Expected: 2 passed.

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/tabstrip.spec.ts
git commit -m "test(e2e): tabstrip click + Cmd+1..4 keyboard nav"
```

---

### Task 19: Final full-suite verification

- [ ] **Step 1: Run all unit tests**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
npx vitest run tests/unit/
```

Expected: green; ~50 tests across 7 files.

- [ ] **Step 2: Run all e2e tests**

```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
sleep 2
npx playwright test --timeout=120000 2>&1 | tail -20
```

Expected: green across happy-path, connect-flow, projects, nav-stress, tabstrip.

- [ ] **Step 3: Manual smoke**

```bash
./scripts/dev.sh &
sleep 12
```

Verify in the Electron window:
- Top bar shows wordmark + dropdown + relay.
- Tab strip works; active tab highlighted.
- Project dropdown lists projects + "+ New project" + "Delete".
- Footer "Settings" opens a slide-in sheet with a Theme segmented control.
- Cmd+1..4 navigate tabs; Cmd+, opens settings.

Stop the app:
```bash
pkill -9 -f "Electron.app|electron-vite|/agentfm-go/agentfm" 2>/dev/null || true
```

- [ ] **Step 4: Tag**

```bash
cd /Users/saif/Desktop/agentfm-prod/agentfm-desktop
git tag -a phase2-complete -m "Phase 2: UX simplification"
```

---

## Self-review

**Spec coverage (cross-referenced against `docs/superpowers/specs/2026-05-18-ux-simplification-design.md`):**
- §2.1 TopBar (wordmark + dropdown chip + relay) → Task 10
- §2.2 TabStrip → Task 6
- §2.3 Footer + sheet → Tasks 8, 9, 12
- §2.4 EmptyState → Task 7
- §3 Project dropdown (list + new + delete) → Task 5
- §4 Simplified wizard → Task 4
- §5 Settings sheet (theme only) → Task 8
- §6 Relay immutability → spec is read-only (no edit surface created); enforced by deleting `ProjectSettingsSheet` in Task 13.
- §7 Data model changes → Tasks 1, 2, 3
- §8 Component create/modify/delete → Tasks 5–13 cover all entries
- §9 Migration → Task 3 retains the existing migration; no new schema
- §10 Behaviors (switch, delete, theme, default route) → Tasks 2, 5, 11
- §11 Testing → Tasks 15, 16, 17, 18, 19
- §12 Risks (Cmd+5 muscle memory mitigated by Cmd+,) → Task 12
- §13 Deferred → out of scope

No gaps identified.

**Placeholder scan:** No "TBD", "TODO", or "fill in details". Every step shows complete code or exact commands.

**Type consistency:** `Project` (Task 1) is the same shape used in Tasks 2, 3, 4, 5. `ProjectInput` matches the wizard's call in Task 4 (`addProject({ name, relayMultiaddr })`). `switchProject` (Task 2) is consumed in Task 5 (`ProjectDropdown`). `isSettingsSheetOpen` / `openSettingsSheet` / `closeSettingsSheet` (Task 8) are consumed in Task 9 (`SettingsFooter`) and Task 12 (`useGlobalShortcuts` for Cmd+,).

Plan complete.
