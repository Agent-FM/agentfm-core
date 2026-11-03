import { test, expect, Page } from '@playwright/test';
import { launchApp, Launched } from './launch';

let launched: Launched;
let page: Page;

test.beforeAll(async () => {
  launched = await launchApp({ project: 'Nav Stress' });
  page = launched.page;
});

test.afterAll(async () => {
  await launched?.close();
});

test('hammering Cmd+1..4 keeps <main> populated', async () => {
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
