import { test, expect, Page } from '@playwright/test'
import path from 'node:path'
import fs from 'node:fs'
import { launchApp, Launched } from './launch'

// Needs a live mesh with a SickLeave-pub/prv worker online, so it only runs
// when AGENTFM_E2E_LIVE_MESH=1. The rest of the suite is self-contained.
test.skip(!process.env.AGENTFM_E2E_LIVE_MESH, 'set AGENTFM_E2E_LIVE_MESH=1 with a SickLeave worker online')

let launched: Launched
let page: Page

const SHOTS = path.resolve(__dirname, '..', '..', 'test-results', 'dispatch-from-ui')
fs.mkdirSync(SHOTS, { recursive: true })

test.beforeAll(async () => {
  launched = await launchApp({ project: 'UI Dispatch Test' })
  page = launched.page
})

test.afterAll(async () => {
  await launched?.close()
})

test('dispatch a task from the Radar UI and stream output', async () => {
  await page.locator('a[href="#/radar"]').click()
  await expect(page.locator('h1:has-text("Your mesh")')).toBeVisible({ timeout: 15000 })

  await page.screenshot({ path: path.join(SHOTS, '01-radar.png'), fullPage: true })

  const card = page
    .locator('div')
    .filter({ hasText: /SickLeave-(pub|prv)/ })
    .filter({ has: page.locator('button:has-text("Dispatch")') })
    .first()
  await expect(card).toBeVisible({ timeout: 30_000 })

  await card.locator('button:has-text("Dispatch")').first().click()

  const drawer = page.locator('textarea[placeholder*="Describe what you want"]')
  await expect(drawer).toBeVisible({ timeout: 5000 })
  await drawer.fill('flu, ooo today, back monday')

  await page.screenshot({ path: path.join(SHOTS, '02-drawer-filled.png'), fullPage: true })

  await page.locator('button:has-text("Send to agent")').click()

  await expect(
    page.locator('button:has-text("Streaming…"), button:has-text("Sending…")').first(),
  ).toBeVisible({ timeout: 15_000 })

  await page.screenshot({ path: path.join(SHOTS, '03-streaming.png'), fullPage: true })

  await expect(page.locator('text=Live stream')).toBeVisible()

  await page
    .locator('text=/Subject:|out of office|OOO|FILES_INCOMING|✅/i')
    .first()
    .waitFor({ timeout: 120_000 })

  await page.waitForFunction(
    () => !document.body.innerText.includes('Streaming…') &&
          !document.body.innerText.includes('Sending…'),
    { timeout: 180_000, polling: 1000 },
  )

  await page.screenshot({ path: path.join(SHOTS, '04-complete.png'), fullPage: true })

  const live = page.locator('text=Live stream').locator('..').locator('..')
  const liveText = await live.innerText().catch(() => '')
  console.log('--- LIVE STREAM CONTENT ---')
  console.log(liveText)
  console.log('--- END ---')

  expect(liveText.length).toBeGreaterThan(50)
})
