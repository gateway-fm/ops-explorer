import { defineConfig, devices } from '@playwright/test';

const EXPLORER_URL = process.env.EXPLORER_URL || 'http://localhost:3001';
const PROXY_URL = process.env.PROXY_URL || 'http://localhost:8080';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 4 : undefined,
  reporter: [
    ['html', { outputFolder: 'playwright-report' }],
    ['list'],
  ],
  use: {
    trace: 'on-first-retry',
  },
  timeout: 120000,
  expect: {
    timeout: 5000,
  },
  projects: [
    {
      name: 'api',
      testMatch: /^(?!.*\/ui\/).*\.spec\.ts$/,
      use: {
        baseURL: EXPLORER_URL,
      },
    },
    {
      name: 'ui',
      testDir: './tests/ui',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: EXPLORER_URL,
        viewport: { width: 1280, height: 720 },
        actionTimeout: 10000,
        navigationTimeout: 15000,
        screenshot: 'only-on-failure',
        video: 'retain-on-failure',
      },
    },
  ],
});
