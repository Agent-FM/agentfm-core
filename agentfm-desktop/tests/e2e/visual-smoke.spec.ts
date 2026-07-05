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

test('AgentFM wordmark logo is shown in the header', async () => {
  const logo = page.locator('header img[alt="AgentFM"]')
  await expect(logo).toBeVisible()
})

test('active tab is bolded after Cmd+2 navigation', async () => {
  await page.keyboard.press('Meta+2')
  await page.waitForTimeout(220)
  const chatActive = page.locator('a:has-text("Chat")').first()
  await expect(chatActive).toHaveClass(/font-semibold/)
})

// Regression test for the "navigating to different tabs breaks the UI with
// black screen" bug. Cycles through every tab by clicking the TabStrip nav
// links (more reliable than relying on Cmd+N hotkeys which can be eaten by
// the OS), and asserts the <main> region renders something — non-empty
// content rules out the dark background showing through an unmounted Outlet.
test('cycles every tab without empty content', async () => {
  const tabs = ['Radar', 'Chat', 'Activity', 'Assets', 'Status']
  for (const label of tabs) {
    await page.locator(`a:has-text("${label}")`).first().click()
    await page.waitForTimeout(250) // let AnimatePresence settle
    const txt = await page.locator('main').innerText()
    expect(
      txt.trim().length,
      `tab "${label}" rendered empty main — black screen regression`,
    ).toBeGreaterThan(20)
  }
})

test('every tab shows the route-page mesh background', async () => {
  const tabs = ['Radar', 'Chat', 'Activity', 'Assets', 'Status', 'Settings']
  for (const label of tabs) {
    await page.locator(`a:has-text("${label}")`).first().click()
    await page.waitForTimeout(200)
    const grid = await page.locator('.route-page__grid').count()
    expect(grid, `tab "${label}" missing route-page mesh layer`).toBeGreaterThan(0)
  }
})
