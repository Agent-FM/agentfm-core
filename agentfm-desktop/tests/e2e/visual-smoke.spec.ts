import { test, expect, Page } from '@playwright/test'
import { launchApp, navLink, Launched } from './launch'

let launched: Launched
let page: Page

test.beforeAll(async () => {
  launched = await launchApp({ project: 'Visual Smoke' })
  page = launched.page
})

test.afterAll(async () => { await launched?.close() })

test('AgentFM mark and wordmark are shown in the navigator header', async () => {
  const navigator = page.locator('aside[aria-label="Navigator"]')
  await expect(navigator.locator('img[src*="logo-mark"]')).toBeVisible()
  await expect(navigator.getByText('AgentFM', { exact: true })).toBeVisible()
})

test('active tab is highlighted after Cmd+2 navigation', async () => {
  await page.keyboard.press('Meta+2')
  await expect(navLink(page, 'Chat')).toHaveAttribute('aria-current', 'page')
})

test('cycles every tab without empty content', async () => {
  const tabs = ['Mesh Radar', 'Chat', 'Activity', 'Artifacts', 'Status']
  for (const label of tabs) {
    await navLink(page, label).click()
    await page.waitForTimeout(250) // let the route fade-in settle
    const txt = await page.locator('main').innerText()
    expect(
      txt.trim().length,
      `tab "${label}" rendered empty main — black screen regression`,
    ).toBeGreaterThan(20)
  }
})

test('every tab updates the editor tab title and jump bar', async () => {
  const tabs = ['Mesh Radar', 'Chat', 'Activity', 'Artifacts', 'Status', 'Settings', 'Developer']
  for (const label of tabs) {
    await navLink(page, label).click()
    // EditorChrome's jump bar ends in a breadcrumb button for the section.
    await expect(page.getByRole('button', { name: label, exact: true })).toBeVisible()
  }
})
