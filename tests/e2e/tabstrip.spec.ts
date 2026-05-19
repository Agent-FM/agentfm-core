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
    await page.locator('input[placeholder*="Team Mesh"]').fill('TabStrip Test');
    await page.locator('button:has-text("Create project")').click();
    await wizard.waitFor({ state: 'hidden', timeout: 15000 });
  }
});

test.afterAll(async () => {
  await app?.close();
});

test('clicking each tab navigates and highlights', async () => {
  for (const label of ['Chat', 'Activity', 'Status', 'Radar']) {
    await page.locator(`a:has-text("${label}")`).click();
    const active = page.locator(`a:has-text("${label}")`).first();
    await expect(active).toHaveClass(/font-semibold/);
  }
});

test('Cmd+1..4 navigate through tabs', async () => {
  const cases = [
    { key: 'Meta+2', label: 'Chat' },
    { key: 'Meta+3', label: 'Activity' },
    { key: 'Meta+4', label: 'Status' },
    { key: 'Meta+1', label: 'Radar' },
  ];
  for (const c of cases) {
    await page.keyboard.press(c.key);
    await page.waitForTimeout(80);
    const active = page.locator(`a:has-text("${c.label}")`).first();
    await expect(active).toHaveClass(/font-semibold/);
  }
});
