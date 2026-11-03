import { test, expect, Page } from '@playwright/test';
import { launchApp, navLink, Launched } from './launch';

let launched: Launched;
let page: Page;

test.beforeAll(async () => {
  launched = await launchApp({ project: 'TabStrip Test' });
  page = launched.page;
});

test.afterAll(async () => {
  await launched?.close();
});

test('clicking each navigator row navigates and highlights', async () => {
  for (const label of ['Chat', 'Activity', 'Status', 'Mesh Radar']) {
    await navLink(page, label).click();
    await expect(navLink(page, label)).toHaveAttribute('aria-current', 'page');
  }
});

test('Cmd+1..4 navigate through tabs', async () => {
  const cases = [
    { key: 'Meta+2', label: 'Chat' },
    { key: 'Meta+3', label: 'Activity' },
    { key: 'Meta+4', label: 'Status' },
    { key: 'Meta+1', label: 'Mesh Radar' },
  ];
  for (const c of cases) {
    await page.keyboard.press(c.key);
    await expect(navLink(page, c.label)).toHaveAttribute('aria-current', 'page');
  }
});
