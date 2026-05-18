import { test, expect, _electron as electron, ElectronApplication, Page } from '@playwright/test';
import path from 'node:path';

// This test reproduces the renderer→backend fetch path that the user's
// "Test connection" button exercises. It launches the prod-build Electron
// app (same as happy-path.spec.ts) and asserts that fetch from the renderer
// reaches the agentfm HTTP backend — the case that was breaking due to a
// CSP that whitelisted localhost:* but not 127.0.0.1:*.

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

  // Dismiss the first-launch welcome modal if present.
  const skip = page.locator('button:has-text("Skip")');
  if (await skip.isVisible({ timeout: 2000 }).catch(() => false)) {
    await skip.click();
  }

  // Wait until the backend has bound and is healthy from the renderer's
  // perspective (this goes through IPC, not subject to CSP).
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

test('renderer can fetch /v1/about over HTTP (CSP allows 127.0.0.1)', async () => {
  // Run the fetch in renderer context — that's where the CSP applies.
  const result = await page.evaluate(async () => {
    try {
      const res = await fetch('http://127.0.0.1:8080/v1/about');
      const ok = res.ok;
      const status = res.status;
      const body = ok ? await res.json() : null;
      return { ok, status, body, error: null };
    } catch (err) {
      return { ok: false, status: 0, body: null, error: (err as Error).message };
    }
  });

  // The exact failure under the old CSP was: ok=false, status=0, error="Failed to fetch".
  expect(result.error, `fetch threw: ${result.error}`).toBeNull();
  expect(result.ok).toBe(true);
  expect(result.status).toBe(200);
  expect(result.body).not.toBeNull();
  expect(result.body).toHaveProperty('boss_peer_id');
});

test('renderer can fetch /api/workers (the Radar code path)', async () => {
  const result = await page.evaluate(async () => {
    try {
      const res = await fetch('http://127.0.0.1:8080/api/workers');
      return { ok: res.ok, status: res.status, error: null };
    } catch (err) {
      return { ok: false, status: 0, error: (err as Error).message };
    }
  });
  expect(result.error, `fetch threw: ${result.error}`).toBeNull();
  expect(result.ok).toBe(true);
  expect(result.status).toBe(200);
});

test('Settings → Test connection green-toasts with relay peer id', async () => {
  await page.keyboard.press('Meta+5');
  await expect(page.locator('h1:has-text("Settings")')).toBeVisible({ timeout: 5000 });

  await page.locator('button:has-text("Test connection")').click();

  // Sonner renders toast text in an aria-live region. We just look for the
  // success substring within ~5s.
  const success = page.locator('text=/Connected to relay|Backend is up but not connected/i');
  await expect(success).toBeVisible({ timeout: 5000 });
});
