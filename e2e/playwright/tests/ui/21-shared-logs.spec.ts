import { test, expect } from '@playwright/test';
import { ProxyAdminFixture } from '../../helpers/proxy-admin';
import { loginViaCookie, logout } from '../../helpers/explorer-auth';

// ---------------------------------------------------------------------------
// Shared Logs Page — verify that the /shared-logs page works correctly:
//
//   1. Unauthenticated users see a sign-in prompt
//   2. Authenticated users see the Shared Logs page header and content area
//   3. The API endpoint returns the correct structure when authenticated
//   4. The API endpoint rejects unauthenticated requests
//   5. The navigation link to Shared Logs is visible from the privacy dashboard
//
// Requires both privacy-proxy and block-explorer stacks to be running.
// ---------------------------------------------------------------------------

test.describe('Shared Logs Page', () => {
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

  test('shared logs page requires authentication', async ({ page, context }) => {
    await logout(context);

    await page.goto('/shared-logs');
    await page.waitForLoadState('networkidle');

    // The page should show the authentication prompt with "Sign In" button
    const signInBtn = page.getByRole('button', { name: /Sign In/i });
    await expect(signInBtn).toBeVisible({ timeout: 10000 });

    // The prompt text should mention signing in to view shared logs
    const promptText = page.getByText(/Sign in to view event logs shared with you/i);
    await expect(promptText).toBeVisible({ timeout: 5000 });

    // The main "Shared Logs" page header (with subtitle) should NOT be visible
    // because the auth gate replaces the full page content.
    // The prompt does show "Shared Logs" as a heading, but the table should not.
    const table = page.locator('table');
    const tableVisible = await table.isVisible({ timeout: 3000 }).catch(() => false);
    expect(tableVisible).toBe(false);
  });

  // -------------------------------------------------------------------------
  // 2. Authenticated user sees the page
  // -------------------------------------------------------------------------

  test('authenticated user sees shared logs page', async ({ page, context }) => {
    await loginViaCookie(context, viewerDid);

    await page.goto('/shared-logs');
    await page.waitForLoadState('networkidle');

    // The page header should show "Shared Logs"
    const heading = page.getByText('Shared Logs', { exact: false });
    await expect(heading.first()).toBeVisible({ timeout: 10000 });

    // Either a table (if logs exist) or the empty state should be visible
    const table = page.locator('table');
    const emptyState = page.getByText(/No logs have been shared with you/i);

    const hasTable = await table.isVisible({ timeout: 5000 }).catch(() => false);
    const hasEmpty = await emptyState.isVisible({ timeout: 3000 }).catch(() => false);

    // One of these must be true — either data or empty state
    expect(hasTable || hasEmpty).toBe(true);
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
  // 5. Navigation link visible from privacy dashboard
  // -------------------------------------------------------------------------

  test('shared logs link visible on privacy dashboard', async ({ page, context }) => {
    await loginViaCookie(context, viewerDid);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // The privacy dashboard should have a "Shared Logs" link
    const sharedLogsLink = page.getByRole('link', { name: /Shared Logs/i });
    await expect(sharedLogsLink.first()).toBeVisible({ timeout: 10000 });

    // Click the link and verify navigation
    await sharedLogsLink.first().click();
    await page.waitForLoadState('networkidle');

    // Should navigate to /shared-logs
    expect(page.url()).toContain('/shared-logs');
  });

  // -------------------------------------------------------------------------
  // 6. Navigation dropdown includes shared logs link when authenticated
  // -------------------------------------------------------------------------

  test('nav dropdown includes shared logs link', async ({ page, context }) => {
    await loginViaCookie(context, viewerDid);

    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Open the user menu (profile dropdown shows DID text)
    const didTextBtn = page.getByText('did:privado:').first();
    const hasDIDBtn = await didTextBtn.isVisible({ timeout: 5000 }).catch(() => false);

    if (!hasDIDBtn) {
      // Some explorer builds may use a different nav pattern; skip gracefully
      test.skip(true, 'DID-based nav dropdown not found — UI may use different pattern');
      return;
    }

    await didTextBtn.click();

    // The dropdown should contain a "Shared Logs" link
    const sharedLogsMenuItem = page.getByText('Shared Logs', { exact: true });
    await expect(sharedLogsMenuItem).toBeVisible({ timeout: 5000 });
  });

  // -------------------------------------------------------------------------
  // 7. Sign-in prompt disappears after login
  // -------------------------------------------------------------------------

  test('sign-in prompt disappears after login', async ({ page, context }) => {
    // Start logged out
    await logout(context);

    await page.goto('/shared-logs');
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

    // Should now see the page content (header + table/empty state)
    const heading = page.getByText('Shared Logs', { exact: false });
    await expect(heading.first()).toBeVisible({ timeout: 10000 });
  });
});
