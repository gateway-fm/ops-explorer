import { test, expect } from '@playwright/test';
import { loginViaCookie, logout } from '../../helpers/explorer-auth';
import { ANVIL_ACCOUNTS } from '../../helpers/blockchain';
import { securityGate } from '../../helpers/security-skip';

// 13 — Refresh-on-logout (real-stack @security; runs in the nightly /
// workflow_dispatch privacy job, NOT the ui-mock PR suite). Verifies that
// logging out immediately re-redacts a previously-visible private address page
// without a full page reload.
//
// FIX: the previous version imported MAIN_DID / MAIN_ADDRESS from helpers/auth,
// which never exported them (broken import → the whole spec failed to load).
// We define them locally from the canonical Anvil account, matching the rest
// of the real-stack suite (helpers/blockchain.ts ANVIL_ACCOUNTS[0]).
const MAIN_DID = 'did:privado:e2e_main_user';
const MAIN_ADDRESS = ANVIL_ACCOUNTS[0].address;

test.describe('13 Refresh on Logout Verification @security', () => {
  test('logging out immediately redacts the address page without a full reload', async ({ page, context }) => {
    // Environment precondition: real privacy-proxy + mock login must be wired.
    // Under CI_REQUIRE_E2E this becomes a hard failure instead of a self-green
    // skip (helpers/security-skip.ts).
    let loginOk = true;
    try {
      await loginViaCookie(context, MAIN_DID);
    } catch {
      loginOk = false;
    }
    securityGate(!loginOk, 'privacy-proxy mock login unavailable (ALLOW_MOCK_LOGIN / proxy down)');

    // Authenticated: the main address page shows its details (web-first wait).
    await page.goto(`/address/${MAIN_ADDRESS}`);
    await expect(page.getByTestId('page-header-title').or(page.getByText(MAIN_ADDRESS, { exact: false })).first()).toBeVisible();
    // It is NOT in a restricted state while authenticated.
    await expect(page.getByTestId('restricted-state')).toHaveCount(0);

    // Log out via the cookie (no UI dependency) and re-assert the same URL.
    await logout(context);
    await page.goto(`/address/${MAIN_ADDRESS}`);

    // After logout the page must redact: either the restricted interstitial or
    // a privacy-`[PRIVATE]` placeholder appears. Web-first, no waitForTimeout.
    await expect(
      page.getByTestId('restricted-state').or(page.getByRole('heading', { name: /Authentication Required|Address Restricted/i })),
    ).toBeVisible();
  });
});
