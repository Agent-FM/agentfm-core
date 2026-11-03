import { _electron as electron, ElectronApplication, Page } from '@playwright/test'
import path from 'node:path'
import fs from 'node:fs'
import os from 'node:os'

// Shared launcher for the e2e specs. Each launch gets its own throwaway
// userData dir so projects/settings never leak between specs or into the
// developer's real ~/Library/Application Support/agentfm-desktop.

const DESKTOP_DIR = path.resolve(__dirname, '..', '..')

// Honor an explicit AGENTFM_BIN, else use the binary built in the monorepo:
// `go build -o agentfm ./cmd/agentfm` inside agentfm-go/.
export const AGENTFM_BIN =
  process.env.AGENTFM_BIN ?? path.resolve(DESKTOP_DIR, '..', 'agentfm-go', 'agentfm')

export interface LaunchOptions {
  /** Name for the first project. Pass null to leave the app on the empty state. */
  project?: string | null
  /** Pre-seeded electron-store settings.json contents. */
  settings?: Record<string, unknown>
}

export interface Launched {
  app: ElectronApplication
  page: Page
  userDataDir: string
  close: () => Promise<void>
}

export async function launchApp(opts: LaunchOptions = {}): Promise<Launched> {
  if (!fs.existsSync(AGENTFM_BIN)) {
    throw new Error(
      `agentfm binary not found at ${AGENTFM_BIN}. Build it with ` +
        '`go build -o agentfm ./cmd/agentfm` in agentfm-go/, or set AGENTFM_BIN.',
    )
  }

  const userDataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'agentfm-e2e-'))
  if (opts.settings) {
    fs.writeFileSync(path.join(userDataDir, 'settings.json'), JSON.stringify(opts.settings, null, 2))
  }

  const app = await electron.launch({
    args: ['.', `--user-data-dir=${userDataDir}`],
    env: { ...process.env, AGENTFM_BIN },
    cwd: DESKTOP_DIR,
  })
  const page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')
  await waitForBackend(page)

  const project = opts.project === undefined ? 'E2E' : opts.project
  if (project !== null) await ensureProject(page, project)

  return {
    app,
    page,
    userDataDir,
    close: async () => {
      await app.close().catch(() => {})
      fs.rmSync(userDataDir, { recursive: true, force: true })
    },
  }
}

// The BackendManager spawns the binary in parallel with window creation, so
// poll window.api.backend.health() before asserting on UI text.
export async function waitForBackend(page: Page, timeout = 30_000): Promise<void> {
  await page.waitForFunction(
    async () => {
      const api = (window as unknown as {
        api?: { backend: { health: () => Promise<{ ok: boolean }> } }
      }).api
      if (!api) return false
      try {
        const r = await api.backend.health()
        return r.ok === true
      } catch {
        return false
      }
    },
    undefined,
    { timeout, polling: 500 },
  )
}

// With no projects the shell renders the "Welcome to AgentFM" empty state;
// create one through its wizard so the navigator and routes mount.
export async function ensureProject(page: Page, name: string): Promise<void> {
  const nav = page.locator('nav[aria-label="Primary"]')
  const welcome = page.getByRole('heading', { name: 'Welcome to AgentFM' })
  await nav.or(welcome).first().waitFor({ timeout: 15_000 })
  if (await nav.isVisible()) return

  await page.getByRole('button', { name: 'Create project' }).click()
  const wizard = page.getByRole('heading', { name: 'New project' })
  await wizard.waitFor()
  await page.getByLabel('Project name').fill(name)
  await page.getByRole('button', { name: 'Create project' }).last().click()
  await wizard.waitFor({ state: 'hidden', timeout: 15_000 })
  await nav.waitFor()
  // Creating a project restarts the backend.
  await waitForBackend(page)
}

/** Navigator link by its visible label ("Mesh Radar", "Chat", "Artifacts", ...). */
export function navLink(page: Page, label: string) {
  return page.locator('nav[aria-label="Primary"]').getByRole('link', { name: label, exact: true })
}
