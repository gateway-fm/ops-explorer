import { test, expect } from '@playwright/test';
import { loginViaCookie } from '../../helpers/explorer-auth';
import { MAIN_DID, MAIN_ADDRESS } from '../../helpers/auth';

test.describe('13 Refresh on Logout Verification', () => {
  test('user logs out and restricted txs disappear immediately without full page reload', async ({ page, context }) => {
    // 1. Log in
    await loginViaCookie(context, MAIN_DID);

    // 2. Go to main address
    await page.goto(`/address/${MAIN_ADDRESS}`);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // 3. Verify transactions are visible
    const hasContent = await page
      .getByText(/balance|transaction|address|contract/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
    
    const authRequiredBefore = await page
      .getByRole('heading', { name: /Authentication Required|Address Restricted/i })
      .isVisible({ timeout: 2000 })
      .catch(() => false);
    expect(authRequiredBefore).toBe(false);

    // 4. Click the profile dropdown and Sign Out
    const didTextBtn = page.getByText('did:privado:').first();
    await didTextBtn.click();
    
    // We expect the menu to open, then we click Sign out
    const signOutBtn = page.getByText('Sign out', { exact: true });
    await signOutBtn.click();

    // 5. Wait to see if "Authentication Required" or "Address Restricted" is shown
    const authRequiredAfter = await page
      .getByRole('heading', { name: /Authentication Required|Address Restricted/i })
      .isVisible({ timeout: 10000 });
    expect(authRequiredAfter).toBe(true);

    // The transactions SHOULD NOT be visible anymore
    const transactionsVisible = await page
      .getByText(/Transactions|Internal Transactions/i)
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    
    // Wait for the UI content to be fully redacted
    // Since page might take a moment to refresh or re-render, we assert 
    // that the transactions list specifically goes away.
    expect(transactionsVisible).toBe(false);
  });
});
