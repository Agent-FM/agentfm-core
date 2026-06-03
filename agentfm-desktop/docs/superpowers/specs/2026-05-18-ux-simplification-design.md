# AgentFM Desktop — Phase 2: UX Simplification

**Status:** Approved 2026-05-18.
**Authors:** Saif (product), Claude (design).
**Scope:** Strip the navigation surface to project dropdown + tab strip + settings footer. Simplify project creation. Delete what's no longer used.
**Out of scope:** Phase 2.1 features (starred agents, chat reset, chat inspector, assets folder, analytics dashboards). Phase 3 visual overhaul. See *Deferred* at end.

## 1. Why

Phase 1 introduced a multi-project model but kept the original sidebar + per-project metadata (color, icon, reputation floor) that nobody asked for. The result is dense, mostly-decorative chrome that obscures the actual mental model the user has:

> "I pick a project. I work in its tabs. I flip light/dark in settings. That's it."

Phase 2 removes everything that isn't that. Side effects we want:

- The empty state stops trying to be clever. New users see one thing: *create your first project*.
- Relay becomes part of project identity, not editable state. This matches the user's "each project = one mesh" model and prevents accidental mesh changes mid-session.
- The Settings page collapses into a footer sheet with one control (theme). Everything else moves out or away.

## 2. New layout

```
┌──────────────────────────────────────────────────┐
│ AgentFM    [📁 My Project ▾]    relay: 127.0.0.1 │  TopBar
├──────────────────────────────────────────────────┤
│ Radar │ Chat │ Activity │ Status                  │  TabStrip
├──────────────────────────────────────────────────┤
│                                                  │
│             main route content                   │
│                                                  │
├──────────────────────────────────────────────────┤
│ ⚙ Settings                                       │  Footer
└──────────────────────────────────────────────────┘
```

### 2.1 TopBar
Flex row with three explicit slots, in left-to-right order:
- Left cluster (no shrink): `AgentFM` wordmark, then the **project dropdown chip** — `📁 <project name> ▾`. Clicking the chip opens the project menu.
- Spacer (`flex-1`).
- Right slot (no shrink): `relay: <truncated multiaddr>`, mono font. Clicking copies the full multiaddr; the truncation rule is "fit to ~28 chars then ellipsis".

### 2.2 TabStrip
- Horizontal strip under TopBar.
- Tabs: Radar / Chat / Activity / Status. The active tab is highlighted; clicking switches routes.
- Cmd+1..4 still work; Cmd+5 is no longer mapped to a route (Settings moves to footer).
- Hidden entirely when there is no active project (empty state).

### 2.3 Footer
- Single button: `⚙ Settings`. Click opens a slim sheet (see §5).
- Visible at all times, including the empty state. (User should be able to flip light/dark before having any projects.)

### 2.4 Empty state (no projects)
- TopBar shows only the AgentFM wordmark; the dropdown chip and relay text are hidden.
- TabStrip is hidden.
- Main area renders a centered card:
  > Welcome to AgentFM.
  > Create your first project to get started.
  >
  > [Create project]
- Footer settings button still visible.
- Clicking *Create project* opens the simplified wizard (§4).

## 3. Project dropdown

Anchored on the chip in the TopBar. Click → menu:

1. **Project list** — every project's name (active highlighted). Click switches to it, which triggers a backend restart with that project's relay and reputation floor.
2. **Divider.**
3. **+ New project** — opens the create wizard.
4. **Delete current project** — confirms with `window.confirm("Delete \"<name>\"? Its chat sessions will be removed.")`, then deletes. If deleting the active project: switch to the next remaining project, or fall to the empty state if none.

No rename. No edit. No icon. No color. The user's only mutations are switch, create, delete.

## 4. Create-project wizard (simplified)

Single screen, two fields:

1. **Name** (text input, required). Trimmed.
2. **Relay** — radio:
   - "Bundled public lighthouse" (default, sentinel `null` in storage)
   - "Custom multiaddr" → text input appears, validated against `/^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/`

Buttons: **Cancel** / **Create project**.

On create:
1. Validate name + relay format.
2. Check relay uniqueness (no two projects share a relay — `null` counts as a single slot too).
3. `addProject(...)` → restart backend with the new project's relay and reputation floor (default -0.5).
4. Switch the wizard out; the project becomes active.

Rejected creates surface inline error text on the offending field; the wizard stays open.

## 5. Settings sheet (footer)

Opened from the footer button. **Slide-in from right** (same motion language as the existing `DispatchDrawer` and `ProjectSettingsSheet` from Phase 1 — Framer Motion `x: '100%' → 0` with `spring stiffness 320 damping 30`). Backdrop dim + click-outside-to-close. Contents:

- **Theme**: segmented control with three options — Dark / Light / Auto. Persisted to electron-store key `theme`.

Nothing else. Removed from this surface: accent color, telemetry checkbox, reset button, API port input.

## 6. Relay immutability

After creation a project's `relayMultiaddr` is read-only. To switch relay, the user creates a new project. The Phase 1 `ProjectSettingsSheet` (which exposed relay/floor editing) is removed entirely.

The TopBar displays the active project's relay alongside the dropdown chip. Truncated to fit; full value is one click away via clipboard copy.

## 7. Data model changes

| Field | Phase 1 | Phase 2 |
|-------|---------|---------|
| `Project.id`     | unchanged | unchanged |
| `Project.name`   | unchanged | unchanged |
| `Project.relayMultiaddr` | mutable | **immutable after creation** |
| `Project.reputationFloor` | per-project mutable | per-project, default -0.5, no edit UI |
| `Project.icon`   | required field | **removed from type** |
| `Project.color`  | required field | **removed from type** |
| `Project.createdAt` | unchanged | unchanged |

Existing projects in electron-store may still have `icon` and `color` keys — they are forward-compatible (ignored on read). New projects don't write them. No destructive migration; the keys atrophy.

The `addProject` and `updateProject` actions on the Zustand store survive but their `ProjectInput` signature drops `icon` and `color`. `updateProject` keeps existing callers compatible (just narrower input), but realistically the only remaining caller is `deleteProject` adjacent code — verify during implementation.

## 8. Component changes

### Created
- `src/components/projects/ProjectDropdown.tsx` — replaces `ProjectPill`. Renders the chip + menu (project list / new / delete).
- `src/components/TabStrip.tsx` — horizontal route tabs under the TopBar.
- `src/components/EmptyState.tsx` — centered welcome card when there are no projects.
- `src/components/SettingsFooter.tsx` — the footer button.
- `src/components/SettingsSheet.tsx` — slim theme-only sheet, opened from the footer.

### Modified
- `src/components/TopBar.tsx` — drops three-column grid; new layout: wordmark + dropdown + relay text. Hides dropdown + relay in empty state.
- `src/components/Shell.tsx` — replaces `<Sidebar />` + `<Outlet />` layout with `<TopBar /> <TabStrip /> <main><Outlet /></main> <SettingsFooter />`.
- `src/components/projects/CreateProjectWizard.tsx` — drops icon, color, reputation floor fields. Two fields only (name + relay).
- `src/lib/store.ts` — `ProjectInput` and `Project` types narrow (no icon/color). `addProject` defaults reputationFloor to -0.5. `deleteProject` unchanged behavior. New action: `switchProject(id)` consolidating the existing inline switch logic.
- `src/lib/projectStore.ts` — `validateProjectInput` keeps relay-uniqueness; no longer references icon/color.
- `src/lib/projectMigration.ts` — strips `icon`/`color` from the default project it creates. No change to legacy migration (existing projects keep their extra fields).
- `src/main.tsx` — unchanged, but verify hydration code path with new types.
- `src/hooks/useGlobalShortcuts.ts` — drop Cmd+5 → Settings mapping. Cmd+1..4 remain Radar/Chat/Activity/Status.

### Deleted
- `src/components/Sidebar.tsx` — no longer rendered.
- `src/components/projects/ProjectList.tsx` — the dropdown supersedes it.
- `src/components/projects/ProjectPill.tsx` — replaced by `ProjectDropdown`.
- `src/components/projects/ProjectSettingsSheet.tsx` — no per-project edit surface anymore.
- `src/components/WelcomeModal.tsx` — `EmptyState` handles this directly.
- `src/routes/Settings.tsx` and the `/settings` route — settings sheet replaces it.

## 9. Migration

On boot (continuing from Phase 1's existing migration):

1. The Phase 1 migration still runs. No new schema change.
2. New code reads `Project.icon` / `Project.color` defensively — both ignored (default '📁' for any chrome that still references icon, no color use). After implementation, no live code reads these fields.
3. Renderer hydrates projects via `hydrateProjects(...)` as before; the Zustand store's `Project` type ignores the legacy fields (extra fields silently allowed via structural typing).

No user action required. Existing settings.json files are accepted as-is.

## 10. Behaviors

### Switching projects
Same as Phase 1: clicking another project in the dropdown sets `activeProjectId`, persists it, calls `window.api.backend.restart({ apiPort, reputationFloor, relayMultiaddr })`, shows `ProjectSwitchingOverlay` until restart resolves.

### Deleting the active project
Confirm → call `deleteProject(id)` (already drops the chat-sessions key for that id). If `projects.length > 0` after deletion, switch to `projects[0]` (full restart). If empty, set `activeProjectId = null`, dismiss switching overlay, render `EmptyState`.

### Theme application
Unchanged. `setTheme(t)` toggles `data-theme` on `<html>` and persists. The settings sheet writes via the same action.

### Default route on launch
- With an active project: `/radar` (existing default).
- With no projects: no routes navigated; `EmptyState` renders inside `<main>` regardless of route path.

## 11. Testing

### Unit
- `projectStore.test.ts` — update to reflect the narrower `ProjectInput` shape (no icon/color). Tests for uniqueness and name validation still apply.
- `projectMigration.test.ts` — the migrated Default project no longer asserts on `icon`/`color`.

### E2E
- `projects.spec.ts` — the wizard now has only two fields; assertions adapt. Duplicate-relay test continues to apply.
- `happy-path.spec.ts` — wizard completion flow already uses name input; relay defaults to bundled lighthouse. Settings expectations move from a `/settings` route to the footer-sheet trigger:
  - Replace `Meta+5` navigation with a click on the footer settings button.
  - Settings sheet should show "Theme" segmented control only.
- New: `tabstrip.spec.ts` — verify Cmd+1..4 navigate tabs; clicking a tab navigates.
- `nav-stress.spec.ts` — update keys to `Meta+1..Meta+4` only (Cmd+5 no longer mapped).

### Manual smoke
- Fresh install (delete settings.json) → empty state appears → create project → tabs appear → switch tabs → footer settings → toggle theme.
- Create second project (different relay) → switch via dropdown → confirm Radar shows different workers / boss_peer_id identical via curl.
- Delete active project → switch happens or empty state appears.

## 12. Risks and open questions

- **Cmd+5 muscle memory.** Anyone who learned Phase 1's Cmd+5 → Settings shortcut will be briefly confused. Accept the friction; the footer button is discoverable.
- **Dropdown reachability for keyboard users.** Phase 2 doesn't include a global "open project dropdown" shortcut. If we later want one, Cmd+P feels natural; tracked but not in scope.
- **Settings sheet on empty state.** The footer button is visible pre-project, but the sheet has no app-level controls beyond theme. Adequate but might feel empty. Acceptable.
- **Project deletion with active streaming dispatch.** If the user deletes the active project while a chat or dispatch is mid-stream, the restart kills the stream. The existing `useChat` / `useDispatch` abort handling covers this; no new code needed. Verify during manual smoke.

## 13. Deferred (Phase 2.1 / Phase 3)

- Starred agents per project · chat reset · chat agent inspector · assets folder · analytics dashboards via `/metrics` · guided onboarding tour overlay · swarm-key support for private meshes.
- Visual overhaul: colorful gradients, motion language, micro-interactions, refined typography.
- Per-project reputation-floor editing UI.
- Project rename.
- Settings sheet additions (telemetry, advanced, backend port).
