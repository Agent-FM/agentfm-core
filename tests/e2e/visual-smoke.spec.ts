import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test'
import path from 'node:path'

let app: ElectronApplication
let page: Page

test.beforeAll(async () => {
  app = await electron.launch({
    args: ['.'],
    env: {
      ...process.env,
      AGENTFM_BIN: path.resolve(__dirname, '..', '..', '..', 'agentfm-core', 'agentfm-go', 'agentfm'),
    },
    cwd: path.resolve(__dirname, '..', '..'),
  })
  page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')

  const wizard = page.locator('h2:has-text("New project")')
  if (await wizard.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.locator('input[placeholder*="Team Mesh"]').fill('Visual Smoke')
    await page.locator('button:has-text("Create project")').click()
    await wizard.waitFor({ state: 'hidden', timeout: 15000 })
  }
})

test.afterAll(async () => { await app?.close() })

test('AgentFM wordmark shows with glow on FM', async () => {
  const fm = page.locator('header span.text-accent.glow-text-cyan')
  await expect(fm).toBeVisible()
  await expect(fm).toContainText('FM')
})

test('active tab is bolded after Cmd+2 navigation', async () => {
  await page.keyboard.press('Meta+2')
  await page.waitForTimeout(220)
  const chatActive = page.locator('a:has-text("Chat")').first()
  await expect(chatActive).toHaveClass(/font-semibold/)
})

test('status hero shows healthy or issues summary', async () => {
  await page.keyboard.press('Meta+4')
  await expect(page.locator('text=/All systems are healthy|issues? detected/i')).toBeVisible({ timeout: 5000 })
})
