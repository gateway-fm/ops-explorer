import { defineConfig, devices } from '@playwright/test';

const EXPLORER_URL = process.env.EXPLORER_URL || 'http://localhost:3001';

// The mock UI suite is fully self-contained: it serves the built frontend via
// `vite preview` and intercepts every /api call in-browser (see
// fixtures/api-mock.ts). No backend / privacy-proxy required, so it runs in
// PR CI. Port 4173 is vite preview's default.
const UI_MOCK_PORT = Number(process.env.UI_MOCK_PORT || 4173);
const UI_MOCK_URL = `http://localhost:${UI_MOCK_PORT}`;

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
      // Mock-based basic-UI suite (PR CI). Self-contained via webServer below.
      name: 'ui-mock',
      testDir: './tests/ui-mock',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: UI_MOCK_URL,
        viewport: { width: 1280, height: 720 },
        actionTimeout: 10000,
        navigationTimeout: 15000,
        screenshot: 'only-on-failure',
        video: 'retain-on-failure',
      },
    },
    {
      // Real-stack API specs (nightly / workflow_dispatch — needs privacy-proxy).
      name: 'api',
      testMatch: /^(?!.*\/ui(-mock)?\/).*\.spec\.ts$/,
      use: {
        baseURL: EXPLORER_URL,
      },
    },
    {
      // Real-stack UI privacy/redaction specs (nightly — needs privacy-proxy).
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
  // Only the ui-mock project boots a server. The real-stack projects expect a
  // manually-running stack (EXPLORER_URL), so reuse an existing server and
  // never let Playwright tear it down for them.
  webServer: {
    command: 'npm run preview -- --host 127.0.0.1 --port ' + UI_MOCK_PORT,
    cwd: '../../frontend',
    url: UI_MOCK_URL,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    stdout: 'ignore',
    stderr: 'pipe',
  },
});
