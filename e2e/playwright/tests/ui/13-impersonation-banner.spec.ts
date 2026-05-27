import { test, expect } from '@playwright/test';
import { loginViaCookie, isAuthenticated } from '../../helpers/explorer-auth';

/**
 * RD-928 "View as user" — UI smoke tests.
 *
 * These rely on the privacy-proxy mock-login flow (ALLOW_MOCK_LOGIN +
 * MOCK_SIGNATURES) being available so we can mint a real BFF session.
 * When that's not available the test is skipped — the API-level
 * impersonation-api.spec.ts covers the contract regardless of stack
 * state.
 */
test.describe('Impersonation banner (RD-928)', () => {
  test('banner does not render outside view-as mode', async ({ page, context }) => {
    const adminDID = `did:privado:e2e_admin_${Date.now()}`;
    try {
      await loginViaCookie(context, adminDID);
    } catch {
      test.skip(true, 'Mock login unavailable — cannot mint admin session');
      return;
    }
    const ok = await isAuthenticated(context);
    test.skip(!ok, 'Auth cookie did not stick — proxy mock login likely off');

    await page.goto('/');
    await expect(page.locator('[data-testid="impersonation-banner"]')).toBeHidden();
  });

  test('?as=<unknown-token> on cold load gracefully clears', async ({ page, context }) => {
    const adminDID = `did:privado:e2e_admin_${Date.now()}`;
    try {
      await loginViaCookie(context, adminDID);
    } catch {
      test.skip(true, 'Mock login unavailable — cannot mint admin session');
      return;
    }
    const ok = await isAuthenticated(context);
    test.skip(!ok, 'Auth cookie did not stick — proxy mock login likely off');

    // Land on home with an unknown view-as token. The frontend should
    // optimistically render the banner from the URL state, then probe
    // the BFF / fail and clear silently. The banner element should not
    // remain after the probe completes.
    await page.goto('/?as=clearly-not-a-real-token');

    // Allow up to 5s for the cleanup probe to complete.
    await expect(page.locator('[data-testid="impersonation-banner"]')).toBeHidden({ timeout: 5_000 });
    // URL should no longer carry ?as=
    await page.waitForFunction(() => !new URL(window.location.href).searchParams.has('as'), null, {
      timeout: 5_000,
    });
  });
});
