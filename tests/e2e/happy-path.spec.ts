import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test';
import path from 'node:path';

let app: ElectronApplication;
let page: Page;

test.beforeAll(async () => {
  app = await electron.launch({
    args: ['.'],
    env: {
      ...process.env,
      AGENTFM_BIN: path.resolve(__dirname, '..', '..', '..', 'agentfm-core', 'agentfm-go', 'agentfm'),
    },
    cwd: path.resolve(__dirname, '..', '..'),
  });
  page = await app.firstWindow();
  await page.waitForLoadState('domcontentloaded');

  // Wait until the backend is healthy from the renderer's perspective.
  // The BackendManager spawns the binary in parallel with window creation,
  // so we poll window.api.backend.health() before asserting on UI text.
  await page.waitForFunction(
    async () => {
      const api = (window as unknown as { api?: { backend: { health: () => Promise<{ ok: boolean }> } } }).api;
      if (!api) return false;
      try {
        const r = await api.backend.health();
        return r.ok === true;
      } catch {
        return false;
      }
    },
    { timeout: 30000, polling: 500 },
  );

  const wizard = page.locator('h2:has-text("New project")');
  if (await wizard.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.locator('input[placeholder*="Team Mesh"]').fill('E2E Default');
    await page.locator('button:has-text("Create project")').click();
    await wizard.waitFor({ state: 'hidden', timeout: 15000 });
  }

  // The ErrorBoundary or per-route loading state may show on initial mount
  // if a fetch fired before the backend was ready. Click Retry if visible,
  // and give a couple of attempts to settle.
  for (let i = 0; i < 3; i++) {
    const retry = page.locator('button:has-text("Retry")');
    if (await retry.isVisible().catch(() => false)) {
      await retry.click();
      await page.waitForTimeout(500);
    }
    const radarHeading = page.locator('h1:has-text("Agent Radar")');
    if (await radarHeading.isVisible().catch(() => false)) break;
    await page.waitForTimeout(500);
  }
});

test.afterAll(async () => {
  await app?.close();
});

test('app boots and renders the Radar shell', async () => {
  // The radar sidebar nav must be present and route is /radar (default).
  await expect(page.locator('a[href="#/radar"]')).toBeVisible();
  // The route content is either the radar grid header or the loading state.
  // We accept either: both prove the route mounted without crashing.
  const radarHeading = page.locator('h1:has-text("Agent Radar")');
  const loading = page.locator('text=Loading agents…');
  await expect(async () => {
    const r = await radarHeading.isVisible().catch(() => false);
    const l = await loading.isVisible().catch(() => false);
    expect(r || l).toBe(true);
  }).toPass({ timeout: 15000 });
});

test('settings sheet opens from footer with theme control', async () => {
  await page.locator('footer button:has-text("Settings")').click();
  await expect(page.locator('h2:has-text("Settings")')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('text=Theme').first()).toBeVisible();
  await page.locator('button:has-text("✕")').click();
});

test('status dashboard renders friendly view', async () => {
  await page.keyboard.press('Meta+4');
  await expect(page.locator('h1:has-text("Your mesh")')).toBeVisible({ timeout: 5000 });
  // Hero banner shows either healthy or issue summary
  const hero = page.locator('text=/All systems are healthy|issue.*detected/i');
  await expect(hero).toBeVisible();
  // The three friendly tiles
  for (const tile of ['Workers', 'Relay', 'Ledger entries']) {
    await expect(page.locator(`text=${tile}`).first()).toBeVisible();
  }
  // Trust gate strip
  await expect(page.locator('text=Trust gate').first()).toBeVisible();
});

test('activity screen renders with grouped layout or empty state', async () => {
  await page.keyboard.press('Meta+3');
  await expect(page.locator('h1:has-text("My activity")')).toBeVisible({ timeout: 5000 });
  // The route should show either the empty state copy or at least one date
  // bucket heading. Either is healthy.
  const empty = page.locator('text=No outgoing entries yet.');
  const buckets = page.locator('h2:has-text("Today"), h2:has-text("Yesterday"), h2:has-text("Older")');
  await expect(async () => {
    const e = await empty.isVisible().catch(() => false);
    const b = (await buckets.count()) > 0;
    expect(e || b).toBe(true);
  }).toPass({ timeout: 5000 });
});
