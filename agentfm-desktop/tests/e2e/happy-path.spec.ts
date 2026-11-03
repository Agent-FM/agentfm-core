import { test, expect, Page } from '@playwright/test';
import { launchApp, navLink, Launched } from './launch';

let launched: Launched;
let page: Page;

test.beforeAll(async () => {
  launched = await launchApp({ project: 'E2E Default' });
  page = launched.page;
});

test.afterAll(async () => {
  await launched?.close();
});

test('app boots and renders the Radar shell', async () => {
  // The navigator must be present and the default route is /radar.
  await expect(navLink(page, 'Mesh Radar')).toBeVisible();
  // The route content is either the radar header or its loading state.
  // Either proves the route mounted without crashing.
  const radarHeading = page.locator('h1:has-text("Your mesh")');
  const loading = page.locator('text=Scanning the mesh…');
  await expect(radarHeading.or(loading).first()).toBeVisible({ timeout: 15000 });
});

test('settings sheet opens from Cmd+, with theme control', async () => {
  await page.keyboard.press('Meta+,');
  await expect(page.locator('h2:has-text("Settings")')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('text=Theme').first()).toBeVisible();
  await page.locator('button[aria-label="close"]').click();
  await expect(page.locator('h2:has-text("Settings")')).toBeHidden();
});

test('status dashboard renders friendly view', async () => {
  await navLink(page, 'Status').click();
  await expect(page.getByRole('heading', { name: 'System status' })).toBeVisible({ timeout: 5000 });
  const hero = page.locator('text=/All systems healthy|Backend up, relay not connected|Backend is down/');
  await expect(hero).toBeVisible();
  for (const tile of ['Boss', 'Agents', 'Relay', 'Ledger']) {
    await expect(page.locator('main').getByText(tile, { exact: true }).first()).toBeVisible();
  }
});

test('activity screen renders with grouped layout or empty state', async () => {
  await page.keyboard.press('Meta+3');
  await expect(page.locator('h1:has-text("My activity")')).toBeVisible({ timeout: 5000 });
  // Either the empty state copy or at least one date bucket heading is healthy.
  const empty = page.getByText('No outgoing entries yet', { exact: true });
  const buckets = page.locator('main').getByText(/^(Today|Yesterday|Older)$/);
  await expect(empty.or(buckets).first()).toBeVisible({ timeout: 5000 });
});
