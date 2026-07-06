import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 60000,
  reporter: 'list',
  use: { trace: 'on-first-retry' },
  workers: 1,
});
