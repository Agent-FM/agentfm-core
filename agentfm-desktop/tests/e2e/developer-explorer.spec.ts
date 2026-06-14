import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test'
import path from 'node:path'
import fs from 'node:fs'

let app: ElectronApplication
let page: Page

const SHOTS = path.resolve(__dirname, '..', '..', 'test-results', 'developer-explorer')
fs.mkdirSync(SHOTS, { recursive: true })

test.beforeAll(async () => {
  app = await electron.launch({
    args: ['.'],
    env: {
      ...process.env,
      AGENTFM_BIN: process.env.AGENTFM_BIN ?? '/Users/saif/Desktop/agentfm-prod/agentfm-core/agentfm-go/agentfm',
    },
    cwd: path.resolve(__dirname, '..', '..'),
  })
  page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')

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
    { timeout: 30_000, polling: 500 },
  )

  const wizard = page.locator('h2:has-text("New project")')
  if (await wizard.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.locator('input[placeholder*="Team Mesh"]').fill('Developer Explorer Test')
    await page.locator('button:has-text("Create project")').click()
    await wizard.waitFor({ state: 'hidden', timeout: 15000 })
  }
})

test.afterAll(async () => {
  await app?.close()
})

test('developer tab: live read-only call + dispatch confirm gate', async () => {
  // Navigate to Developer tab (TabStrip renders an <a> NavLink once a project is active).
  await expect(page.getByRole('link', { name: 'Developer' })).toBeVisible({ timeout: 15000 })
  await page.getByRole('link', { name: 'Developer' }).click()
  await expect(page.getByRole('heading', { name: 'Developer API' })).toBeVisible()

  await page.screenshot({ path: path.join(SHOTS, '01-developer-tab.png'), fullPage: true })

  // Select GET /api/workers (read-only) and send.
  await page.getByRole('button', { name: '/api/workers' }).first().click()
  await page.getByRole('button', { name: 'Send', exact: true }).click()

  // A status-code badge appears in ResponsePanel (200 when the backend is up).
  await expect(page.getByText(/^\d{3}$/)).toBeVisible({ timeout: 15000 })

  await page.screenshot({ path: path.join(SHOTS, '02-workers-response.png'), fullPage: true })

  // Select POST /api/execute (dispatch). Anchor the name so it does not also
  // match "POST /api/execute/async".
  await page.getByRole('button', { name: /\/api\/execute$/ }).first().click()
  await page.getByRole('button', { name: 'Send', exact: true }).click()

  // The confirm gate appears instead of dispatching.
  await expect(page.getByText(/run a real task/i)).toBeVisible()

  await page.screenshot({ path: path.join(SHOTS, '03-confirm-gate.png'), fullPage: true })
})
