import { test, expect } from '@playwright/test';
import { ProxyAdminFixture } from '../../helpers/proxy-admin';
import { loginViaCookie, logout } from '../../helpers/explorer-auth';

// ---------------------------------------------------------------------------
// Shared Logs — now embedded as a tab inside the "Shared with Me" page at
// /privacy. The old /shared-logs route redirects there for backward compat.
//
//   1. Unauthenticated users see a sign-in prompt
//   2. Authenticated users see the Shared with Me page with Shared Logs tab
//   3. The API endpoint returns the correct structure when authenticated
//   4. The API endpoint rejects unauthenticated requests
//   5. The Shared Logs tab is clickable on the Shared with Me page
//   6. /shared-logs redirects to /privacy
//   7. Sign-in prompt disappears after login
//
// Requires both privacy-proxy and block-explorer stacks to be running.
// ---------------------------------------------------------------------------

test.describe('Shared Logs Tab', () => {
  test.describe.configure({ mode: 'serial' });

  let fixture: ProxyAdminFixture;
  let viewerDid: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Create a viewer user (the "settlement bank" who would see shared logs)
    viewerDid = fixture.did();
    await fixture.ensureUser(viewerDid);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  // -------------------------------------------------------------------------
  // 1. Unauthenticated users see sign-in prompt
  // -------------------------------------------------------------------------

  test('shared with me page requires authentication', async ({ page, context }) => {
    await logout(context);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // The page should show the authentication prompt with "Sign In" button
    const signInBtn = page.getByRole('button', { name: /Sign In/i });
    await expect(signInBtn).toBeVisible({ timeout: 10000 });

    // The prompt text should mention signing in
    const promptText = page.getByText(/Sign in to view addresses and logs shared with you/i);
    await expect(promptText).toBeVisible({ timeout: 5000 });

    // The table should not be visible (auth gate replaces page content)
    const table = page.locator('table');
    const tableVisible = await table.isVisible({ timeout: 3000 }).catch(() => false);
    expect(tableVisible).toBe(false);
  });

  // -------------------------------------------------------------------------
  // 2. Authenticated user sees the page with Shared Logs tab
  // -------------------------------------------------------------------------

  test('authenticated user sees shared with me page', async ({ page, context }) => {
    await loginViaCookie(context, viewerDid);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // The page header should show "Shared with Me"
    const heading = page.getByText('Shared with Me', { exact: false });
    await expect(heading.first()).toBeVisible({ timeout: 10000 });

    // The "Shared Logs" tab button should be visible
    const sharedLogsTab = page.getByRole('button', { name: /Shared Logs/i });
    await expect(sharedLogsTab).toBeVisible({ timeout: 5000 });
  });

  // -------------------------------------------------------------------------
  // 3. API returns correct structure for authenticated user
  // -------------------------------------------------------------------------

  test('shared logs API returns correct structure', async ({ context }) => {
    await loginViaCookie(context, viewerDid);

    const response = await context.request.get(
      '/api/privacy/shared-logs?limit=10&offset=0',
    );
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data).toHaveProperty('logs');
    expect(data).toHaveProperty('total');
    expect(Array.isArray(data.logs)).toBeTruthy();
    expect(typeof data.total).toBe('number');

    // Verify limit/offset are echoed back
    expect(data).toHaveProperty('limit');
    expect(data).toHaveProperty('offset');
    expect(data.limit).toBe(10);
    expect(data.offset).toBe(0);
  });

  // -------------------------------------------------------------------------
  // 4. API rejects unauthenticated requests
  // -------------------------------------------------------------------------

  test('shared logs API requires authentication', async ({ context }) => {
    await logout(context);

    const response = await context.request.get(
      '/api/privacy/shared-logs?limit=10&offset=0',
    );

    // Should return 401 Unauthorized
    expect(response.status()).toBe(401);
  });

  // -------------------------------------------------------------------------
  // 5. Shared Logs tab is clickable and shows content
  // -------------------------------------------------------------------------

  test('shared logs tab shows content when clicked', async ({ page, context }) => {
    await loginViaCookie(context, viewerDid);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // Click the "Shared Logs" tab
    const sharedLogsTab = page.getByRole('button', { name: /Shared Logs/i });
    await expect(sharedLogsTab).toBeVisible({ timeout: 10000 });
    await sharedLogsTab.click();

    // Either a table (if logs exist) or the empty state should be visible
    const table = page.locator('table');
    const emptyState = page.getByText(/No logs have been shared with you/i);

    const hasTable = await table.isVisible({ timeout: 5000 }).catch(() => false);
    const hasEmpty = await emptyState.isVisible({ timeout: 3000 }).catch(() => false);

    // One of these must be true — either data or empty state
    expect(hasTable || hasEmpty).toBe(true);
  });

  // -------------------------------------------------------------------------
  // 6. /shared-logs redirects to /privacy
  // -------------------------------------------------------------------------

  test('/shared-logs redirects to /privacy', async ({ page, context }) => {
    await loginViaCookie(context, viewerDid);

    await page.goto('/shared-logs');
    await page.waitForLoadState('networkidle');

    // Should have been redirected to /privacy
    expect(page.url()).toContain('/privacy');

    // The page should show "Shared with Me"
    const heading = page.getByText('Shared with Me', { exact: false });
    await expect(heading.first()).toBeVisible({ timeout: 10000 });
  });

  // -------------------------------------------------------------------------
  // 7. Sign-in prompt disappears after login
  // -------------------------------------------------------------------------

  test('sign-in prompt disappears after login', async ({ page, context }) => {
    // Start logged out
    await logout(context);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // Verify sign-in prompt is shown
    const signInBtn = page.getByRole('button', { name: /Sign In/i });
    await expect(signInBtn).toBeVisible({ timeout: 10000 });

    // Now log in via cookie
    await loginViaCookie(context, viewerDid);

    // Reload the page
    await page.reload();
    await page.waitForLoadState('networkidle');

    // Sign-in prompt should be gone
    const signInBtnAfter = await page
      .getByRole('button', { name: /Sign In/i })
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    expect(signInBtnAfter).toBe(false);

    // Should now see the page content (header + tabs)
    const heading = page.getByText('Shared with Me', { exact: false });
    await expect(heading.first()).toBeVisible({ timeout: 10000 });
  });
});
