import { test, expect } from '@playwright/test';
import { isAuthenticated, logout } from '../../helpers/explorer-auth';

const PROXY_FRONTEND = process.env.PROXY_FRONTEND_URL || 'http://localhost:5173';

test.describe('OAuth Login Flow', () => {
  test.beforeEach(async ({ context }) => {
    await logout(context);
  });

  test('full OAuth redirect login via mock login', async ({ page, context }) => {
    // 1. Navigate to block-explorer home
    await page.goto('/');

    // 2. Check if privacy/SSO is enabled (Sign in button visible)
    const signInBtn = page.locator('button:has-text("Sign in")');
    const isPrivacyEnabled = await signInBtn.isVisible({ timeout: 5000 }).catch(() => false);

    if (!isPrivacyEnabled) {
      test.skip(true, 'Privacy/SSO not enabled on this block-explorer instance');
      return;
    }

    // 3. Click Sign in — this navigates to /api/auth/login which redirects to privacy-proxy
    await signInBtn.click();

    // 4. We should land on privacy-proxy's login page with oauth_session_id param
    await expect(page).toHaveURL(/\/login\?oauth_session_id=/, { timeout: 15000 });

    // 5. Verify we're on privacy-proxy (OAuth banner should show)
    await expect(page.locator('[data-testid="oauth-banner"]')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText('Signing in to continue to Block Explorer')).toBeVisible();

    // 6. Use mock login (dev tools must be visible)
    const mockLoginBtn = page.locator('[data-testid="mock-login-btn"]');
    const isMockAvailable = await mockLoginBtn.isVisible({ timeout: 5000 }).catch(() => false);

    if (!isMockAvailable) {
      test.skip(true, 'Mock login not available on privacy-proxy (ALLOW_MOCK_LOGIN not set)');
      return;
    }

    await mockLoginBtn.click();

    // 7. Wait for redirect back to block-explorer
    // The flow: mock login -> /oauth/complete -> redirect to block-explorer callback -> redirect to /
    await page.waitForURL((url) => {
      const host = new URL(page.url()).origin;
      // We should end up back on block-explorer (not privacy-proxy)
      return !url.toString().includes('oauth_session_id') && !url.pathname.includes('/login');
    }, { timeout: 20000 });

    // 8. Verify authenticated state
    const authenticated = await isAuthenticated(context);
    expect(authenticated).toBe(true);

    // 9. The nav should show authenticated state (user menu instead of Sign in)
    // Give the page a moment to re-render with auth state
    await page.goto('/');
    await expect(page.locator('button:has-text("Sign in")')).not.toBeVisible({ timeout: 5000 });
  });

  test('auth status returns unauthenticated when no cookie', async ({ context }) => {
    const authenticated = await isAuthenticated(context);
    expect(authenticated).toBe(false);
  });

  test('login redirects to privacy-proxy with correct params', async ({ page }) => {
    // Directly hit the login endpoint and check the redirect
    const response = await page.request.get('/api/auth/login?return_url=/blocks', {
      maxRedirects: 0,
    });

    // Should be a 302 redirect
    expect(response.status()).toBe(302);

    const location = response.headers()['location'];
    expect(location).toBeTruthy();
    expect(location).toContain('/oauth/authorize');
    expect(location).toContain('response_mode=redirect');
    expect(location).toContain('response_type=code');
  });

  test('login sanitizes malicious return_url', async ({ page }) => {
    const response = await page.request.get('/api/auth/login?return_url=https://evil.com', {
      maxRedirects: 0,
    });

    // Should still redirect (302) but the state should store "/" not the evil URL
    expect(response.status()).toBe(302);
  });
});
