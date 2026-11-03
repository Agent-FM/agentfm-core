import { test, expect, Page } from '@playwright/test'
import { launchApp, navLink, Launched } from './launch'

let launched: Launched
let page: Page

test.beforeAll(async () => {
  launched = await launchApp({ project: 'E2E Dashboard' })
  page = launched.page
})

test.afterAll(async () => {
  await launched?.close()
})

test('dashboard tab is reachable and renders the task totals', async () => {
  await navLink(page, 'Dashboard').click()

  await expect(page.getByRole('heading', { name: 'Live mesh' })).toBeVisible({ timeout: 10_000 })
  await expect(page.locator('text=/\\d+ tasks since start/')).toBeVisible({ timeout: 10_000 })

  // The "Total tasks" card shows a numeric counter once metrics arrive.
  await expect(async () => {
    const cardText = await page.getByText('Total tasks', { exact: true }).locator('..').textContent()
    expect(cardText).toMatch(/\d+/)
  }).toPass({ timeout: 15_000 })
})

test('dashboard surfaces assets built tile, success rate card, and freshness header', async () => {
  await navLink(page, 'Dashboard').click()

  await expect(
    page.locator('text=Assets built').first(),
  ).toBeVisible({ timeout: 10_000 })

  await expect(
    page.locator('text=Success rate').first(),
  ).toBeVisible({ timeout: 10_000 })

  await expect(
    page
      .locator('text=/connecting…|updated [\\d.]+s ago/')
      .first(),
  ).toBeVisible({ timeout: 10_000 })
})
