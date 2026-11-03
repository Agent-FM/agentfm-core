import { test, expect, Page } from '@playwright/test';
import { launchApp, Launched } from './launch';

// This test reproduces the renderer→backend fetch path that the user's
// "Test connection" button exercises. It launches the prod-build Electron
// app (same as happy-path.spec.ts) and asserts that fetch from the renderer
// reaches the agentfm HTTP backend — the case that was breaking due to a
// CSP that whitelisted localhost:* but not 127.0.0.1:*.

let launched: Launched;
let page: Page;

test.beforeAll(async () => {
  launched = await launchApp({ project: 'E2E Connect' });
  page = launched.page;
});

test.afterAll(async () => {
  await launched?.close();
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

