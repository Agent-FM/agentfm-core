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
    await page.locator('input[placeholder*="Team Mesh"]').fill('Nav Stress');
    await page.locator('button:has-text("Create project")').click();
    await wizard.waitFor({ state: 'hidden', timeout: 15000 });
  }
});

test.afterAll(async () => {
  await app?.close();
});

test('hammering Cmd+1..5 keeps <main> populated', async () => {
  const keys = ['Meta+1', 'Meta+2', 'Meta+3', 'Meta+4'];
  for (let round = 0; round < 4; round++) {
    for (const k of keys) {
      await page.keyboard.press(k);
      await page.waitForTimeout(80);
      const mainHasContent = await page.evaluate(() => {
        const m = document.querySelector('main');
        if (!m) return false;
        return (m.textContent ?? '').trim().length > 0;
      });
      expect(mainHasContent, `main went blank after ${k} round ${round}`).toBe(true);
    }
  }
});
