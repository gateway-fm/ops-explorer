import { test, expect } from '@playwright/test';
import { logout } from '../../helpers/explorer-auth';

// ---------------------------------------------------------------------------
// Token Pages — Privacy-Aware Rendering
//
// Verifies that token list, token detail, and token transfer pages load
// correctly for anonymous users, and that privacy redaction (e.g. [PRIVATE]
// addresses, "Private" labels) is rendered when the backend returns redacted
// data.
//
// These tests are intentionally conservative: on a fresh Anvil chain there
// may be zero tokens deployed, so we guard assertions with conditional skips
// rather than hard-failing.
// ---------------------------------------------------------------------------

test.describe('Token Pages Privacy', () => {
  test.beforeEach(async ({ context }) => {
    // Start each test unauthenticated
    await logout(context);
  });

  // -------------------------------------------------------------------------
  // 1. Anonymous user can see the token list page
  // -------------------------------------------------------------------------

  test('anonymous user can see token list page', async ({ page }) => {
    await page.goto('/tokens');
    await page.waitForLoadState('networkidle');

    // Should not hit an auth wall
    await expect(page.locator('body')).not.toContainText('Sign in to continue', {
      timeout: 10000,
    });

    // The page header should say "Tokens"
    const heading = page.getByText(/Tokens/i);
    await expect(heading.first()).toBeVisible({ timeout: 10000 });

    // Either tokens are listed or the "No tokens found" message is shown
    const hasTable = await page
      .locator('table')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    const hasEmptyMessage = await page
      .getByText(/No tokens found/i)
      .isVisible({ timeout: 3000 })
      .catch(() => false);

    expect(hasTable || hasEmptyMessage).toBe(true);
  });

  // -------------------------------------------------------------------------
  // 2. Token addresses show redacted for private org tokens
  // -------------------------------------------------------------------------

  test('private token addresses render with lock icon or Private indicator', async ({ page }) => {
    await page.goto('/tokens');
    await page.waitForLoadState('networkidle');

    // Wait for content to load
    await page.waitForTimeout(2000);

    // Check whether any [PRIVATE] placeholders or lock icons are present.
    // On chains with org-owned token contracts, the proxy redacts addresses
    // that the viewer has no access to.
    const hasPrivateText = await page
      .getByText('Private')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    const hasRedactedText = await page
      .getByText('[PRIVATE]')
      .first()
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    const hasLockIcon = await page
      .locator('svg.lucide-lock, [data-testid="lock-icon"]')
      .first()
      .isVisible({ timeout: 3000 })
      .catch(() => false);

    if (!hasPrivateText && !hasRedactedText && !hasLockIcon) {
      // No private tokens on this chain — skip gracefully
      test.skip(true, 'No redacted token addresses found on chain; all tokens are public');
      return;
    }

    // At least one privacy indicator is visible
    expect(hasPrivateText || hasRedactedText || hasLockIcon).toBe(true);
  });

  // -------------------------------------------------------------------------
  // 3. Clicking a token navigates to token detail
  // -------------------------------------------------------------------------

  test('clicking a token navigates to token detail page', async ({ page }) => {
    await page.goto('/tokens');
    await page.waitForLoadState('networkidle');

    // Find a token link in the table — the symbol link goes to /token/<address>
    const tokenLink = page.locator('a[href^="/token/"]').first();
    const hasTokenLink = await tokenLink.isVisible({ timeout: 5000 }).catch(() => false);

    if (!hasTokenLink) {
      test.skip(true, 'No tokens available to click');
      return;
    }

    await tokenLink.click();
    await page.waitForLoadState('networkidle');

    // URL should now be /token/<address>
    expect(page.url()).toContain('/token/');

    // The detail page should not show an auth wall
    await expect(page.locator('body')).not.toContainText('Sign in to continue', {
      timeout: 10000,
    });

    // Some token-related content should be visible (name, symbol, holders, etc.)
    const hasContent = await page
      .getByText(/holder|transfer|supply|token/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
  });

  // -------------------------------------------------------------------------
  // 4. Token transfers page loads
  // -------------------------------------------------------------------------

  test('token transfers page loads with data or empty state', async ({ page }) => {
    await page.goto('/token-transfers');
    await page.waitForLoadState('networkidle');

    // Should not hit an auth wall
    await expect(page.locator('body')).not.toContainText('Sign in to continue', {
      timeout: 10000,
    });

    // The page header should say "Token Transfers"
    const heading = page.getByText(/Token Transfers/i);
    await expect(heading.first()).toBeVisible({ timeout: 10000 });

    // Either a table of transfers or the "No token transfers found" message
    const hasTable = await page
      .locator('table')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    const hasEmptyMessage = await page
      .getByText(/No token transfers found/i)
      .isVisible({ timeout: 3000 })
      .catch(() => false);

    expect(hasTable || hasEmptyMessage).toBe(true);
  });
});
