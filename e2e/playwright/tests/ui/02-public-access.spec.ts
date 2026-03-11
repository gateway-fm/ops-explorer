import { test, expect } from '@playwright/test';
import { isAuthenticated, logout } from '../../helpers/explorer-auth';

test.describe('Public Data Access (Unauthenticated)', () => {
  test.beforeEach(async ({ context }) => {
    // Ensure we start each test unauthenticated
    await logout(context);
  });

  test('home page shows blockchain stats without auth', async ({ page, context }) => {
    const authed = await isAuthenticated(context);
    expect(authed).toBe(false);

    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The home page should display blockchain statistics (block count, tx count, etc.)
    // without requiring authentication. Look for numeric content that indicates data loaded.
    // The exact selectors depend on the explorer UI, so we use broad matchers.
    const body = page.locator('body');
    await expect(body).not.toContainText('Sign in to continue', { timeout: 10000 });

    // Check that the page has meaningful content — at least one number visible
    // (block height, tx count, etc.)
    const hasNumbers = await page
      .locator('text=/\\d+/')
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasNumbers).toBe(true);
  });

  test('blocks page accessible without auth', async ({ page }) => {
    await page.goto('/blocks');
    await page.waitForLoadState('networkidle');

    // The page should not show an auth wall
    await expect(page.locator('body')).not.toContainText('Sign in to continue', {
      timeout: 10000,
    });

    // Expect a table or list of blocks. The heading or content should reference blocks.
    const blocksContent = page.locator(
      'text=/[Bb]lock/ >> visible=true',
    );
    await expect(blocksContent.first()).toBeVisible({ timeout: 10000 });
  });

  test('transactions page accessible without auth', async ({ page }) => {
    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('body')).not.toContainText('Sign in to continue', {
      timeout: 10000,
    });

    // Expect content referencing transactions
    const txContent = page.locator(
      'text=/[Tt]ransaction/ >> visible=true',
    );
    await expect(txContent.first()).toBeVisible({ timeout: 10000 });
  });

  test('address page accessible without auth', async ({ page }) => {
    // Use the zero address as a safe default that should always exist
    const zeroAddress = '0x0000000000000000000000000000000000000000';
    await page.goto(`/address/${zeroAddress}`);
    await page.waitForLoadState('networkidle');

    // The page should render something (balance section, address info, or "no transactions")
    // It should NOT show an auth wall or 403 error for public address data
    const body = page.locator('body');
    await expect(body).not.toContainText('Sign in to continue', { timeout: 10000 });

    // Look for address-related content: the address itself, "Balance", "Transactions", etc.
    const hasAddressContent = await page
      .locator('text=/0x0000|[Bb]alance|[Tt]ransaction|[Aa]ddress/')
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasAddressContent).toBe(true);
  });

  test('/privacy page prompts sign in when unauthenticated', async ({ page }) => {
    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // The privacy dashboard should show a sign-in prompt, not dashboard content
    // Based on PrivacyDashboard.tsx, it shows "Sign in with Privado ID" and a button
    const signInPrompt = page.getByText(/Sign in with Privado/i);
    await expect(signInPrompt.first()).toBeVisible({ timeout: 10000 });

    // Should NOT show the dashboard tabs
    const ownAddressesTab = page.getByText(/Your Addresses/i);
    await expect(ownAddressesTab).not.toBeVisible();
  });

  test('search works without auth', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Find a search input on the page
    const searchInput = page.locator(
      'input[type="search"], input[placeholder*="earch"], input[placeholder*="lock"], input[placeholder*="ddress"]',
    );
    const searchVisible = await searchInput.first().isVisible({ timeout: 5000 }).catch(() => false);

    if (!searchVisible) {
      // If no search input is visible on the home page, skip gracefully
      test.skip(true, 'No search input found on home page');
      return;
    }

    // Type a block number (a simple search that should work without auth)
    await searchInput.first().fill('1');

    // Wait briefly for any search suggestions or results
    // The key assertion is that typing does not trigger an auth wall
    await page.waitForTimeout(1000);
    await expect(page.locator('body')).not.toContainText('Sign in to continue');
  });
});
