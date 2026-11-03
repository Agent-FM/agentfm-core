import { test, expect, Page } from '@playwright/test'
import path from 'node:path'
import fs from 'node:fs'
import { launchApp, Launched } from './launch'

let launched: Launched
let page: Page

const SHOTS = path.resolve(__dirname, '..', '..', 'test-results', 'developer-explorer')
fs.mkdirSync(SHOTS, { recursive: true })

test.beforeAll(async () => {
  launched = await launchApp({ project: 'Developer Explorer Test' })
  page = launched.page
})

test.afterAll(async () => {
  await launched?.close()
})

test('developer tab: live read-only call + dispatch confirm gate', async () => {
  // Navigate to Developer tab (the Navigator renders its NavLinks once a project is active).
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
