import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test';
import path from 'node:path';
import fs from 'node:fs';
import os from 'node:os';

let app: ElectronApplication;
let page: Page;

const SETTINGS_PATH = path.join(
  os.homedir(),
  'Library',
  'Application Support',
  'agentfm-desktop',
  'settings.json',
);

test.beforeAll(async () => {
  try {
    const dir = path.dirname(SETTINGS_PATH);
    fs.mkdirSync(dir, { recursive: true });
    const seed = {
      projects: [
        {
          id: 'prj_seed_custom',
          name: 'Seed',
          icon: '🧪',
          color: 'violet',
          relayMultiaddr: '/ip4/198.51.100.55/tcp/4001/p2p/12D3KooWSeedPlaceholder',
          reputationFloor: -0.5,
          createdAt: Date.now(),
        },
      ],
      activeProjectId: 'prj_seed_custom',
    };
    fs.writeFileSync(SETTINGS_PATH, JSON.stringify(seed, null, 2));
  } catch (e) {
    console.warn('seed settings failed', e);
  }

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
});

test.afterAll(async () => {
  await app?.close();
});

test('wizard opens on first launch and creates the default project', async () => {
  const wizard = page.locator('h2:has-text("New project")');
  if (!(await wizard.isVisible({ timeout: 3000 }).catch(() => false))) {
    await page.locator('header button:has-text("📁")').first().click();
    await page.locator('button:has-text("New project")').click();
  }
  await expect(wizard).toBeVisible({ timeout: 8000 });

  await page.locator('input[placeholder*="Team Mesh"]').fill('Smoke Project');
  await page.locator('button:has-text("Create project")').click();
  await expect(wizard).toBeHidden({ timeout: 15000 });

  await expect(page.locator('text=Smoke Project').first()).toBeVisible();
  await expect(page.locator('header button:has-text("Smoke Project")')).toBeVisible();
});

test('rejects a duplicate relay when creating a second project', async () => {
  await page.locator('header button:has-text("📁")').first().click();
  await page.locator('button:has-text("New project")').click();
  await expect(page.locator('h2:has-text("New project")')).toBeVisible();

  await page.locator('input[placeholder*="Team Mesh"]').fill('Dupe');
  await page.locator('button:has-text("Create project")').click();

  await expect(page.locator('h2:has-text("New project")')).toBeVisible();
  await expect(
    page.locator('text=/already uses the bundled|already uses /i'),
  ).toBeVisible({ timeout: 3000 });

  await page.locator('button:has-text("Cancel")').click();
});
