import { test, expect, Page } from '@playwright/test';
import { launchApp, Launched } from './launch';

let launched: Launched;
let page: Page;

// Seed a project that uses a custom relay so the bundled public lighthouse is
// still free for the wizard's first create.
const SEED_SETTINGS = {
  projects: [
    {
      id: 'prj_seed_custom',
      name: 'Seed',
      relayMultiaddr: '/ip4/198.51.100.55/tcp/4001/p2p/12D3KooWSeedPlaceholder',
      reputationFloor: -0.5,
      connectionMode: 'public',
      swarmKey: null,
      createdAt: Date.now(),
    },
  ],
  activeProjectId: 'prj_seed_custom',
};

test.beforeAll(async () => {
  launched = await launchApp({ project: null, settings: SEED_SETTINGS });
  page = launched.page;
});

test.afterAll(async () => {
  await launched?.close();
});

const wizard = () => page.getByRole('heading', { name: 'New project' });
const toast = (re: RegExp) => page.locator('[data-sonner-toast]').filter({ hasText: re });

async function openWizardFromProjectMenu() {
  await page.locator('header').getByRole('button').first().click();
  await page.getByRole('button', { name: 'New project' }).click();
  await expect(wizard()).toBeVisible({ timeout: 8000 });
}

test('seeded project is active and the project menu creates a new one', async () => {
  await expect(page.locator('header button:has-text("Seed")')).toBeVisible({ timeout: 15000 });

  await openWizardFromProjectMenu();
  await page.getByLabel('Project name').fill('Smoke Project');
  await page.getByRole('button', { name: 'Create project' }).click();
  await expect(wizard()).toBeHidden({ timeout: 15000 });

  await expect(page.locator('header button:has-text("Smoke Project")')).toBeVisible();
});

test('rejects a duplicate relay when creating a second project', async () => {
  await openWizardFromProjectMenu();

  await page.getByLabel('Project name').fill('Dupe');
  await page.getByRole('button', { name: 'Create project' }).click();

  await expect(wizard()).toBeVisible();
  await expect(toast(/already uses the bundled public lighthouse/)).toBeVisible({ timeout: 3000 });

  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(wizard()).toBeHidden();
});

test('private mode wizard demands a relay + 64-hex swarm key', async () => {
  await openWizardFromProjectMenu();

  await page.getByLabel('Project name').fill('Private smoke');
  await page.locator('button:has-text("Private")').click();

  await page.getByRole('button', { name: 'Create project' }).click();
  await expect(wizard()).toBeVisible();
  await expect(toast(/Private projects need a private relay multiaddr/)).toBeVisible({ timeout: 3000 });

  await page
    .getByLabel('Private relay multiaddress')
    .fill('/ip4/10.0.0.42/tcp/4001/p2p/12D3KooWPrivatePlaceholderXYZ');
  await page.locator('input[placeholder*="64 hex"]').fill('not-hex');
  await page.getByRole('button', { name: 'Create project' }).click();
  await expect(wizard()).toBeVisible();
  await expect(toast(/Swarm key must be 64 hex characters/)).toBeVisible({ timeout: 3000 });

  await page.getByRole('button', { name: 'Generate' }).click();
  const keyValue = await page.locator('input[placeholder*="64 hex"]').inputValue();
  expect(keyValue).toMatch(/^[0-9a-f]{64}$/);

  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(wizard()).toBeHidden();
});
