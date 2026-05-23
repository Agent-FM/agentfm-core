import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test'
import path from 'node:path'

let app: ElectronApplication
let page: Page

test.beforeAll(async () => {
  app = await electron.launch({
    args: ['.'],
    env: {
      ...process.env,
      AGENTFM_BIN: path.resolve(
        __dirname, '..', '..', '..', 'agentfm-core', 'agentfm-go', 'agentfm',
      ),
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
    await page.locator('input[placeholder*="Team Mesh"]').fill('E2E Dashboard')
    await page.locator('button:has-text("Create project")').click()
    await wizard.waitFor({ state: 'hidden', timeout: 15_000 })
  }
})

test.afterAll(async () => {
  await app?.close()
})

test('dashboard tab is reachable and renders the TASKS section', async () => {
  await page.locator('a[href="#/dashboard"]').click()

  await expect(page.locator('text=/TASKS/i').first()).toBeVisible({ timeout: 10_000 })

  await expect(async () => {
    const heroText = await page.locator('text=/TASKS · LAST 5 MIN/i').locator('..').textContent()
    expect(heroText).toMatch(/\d+/)
  }).toPass({ timeout: 15_000 })
})
