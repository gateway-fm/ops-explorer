import { test, expect } from '@playwright/test';

test.describe('Privacy Proxy Authorization & Visibility', () => {

  test.beforeEach(async ({ page }) => {
    // Globally mock stats to ensure privacy features are enabled in the UI
    await page.route('**/api/stats', route => 
      route.fulfill({ 
        status: 200, 
        contentType: 'application/json', 
        body: JSON.stringify({ 
          totalBlocks: 1000,
          totalTransactions: 5000,
          totalAddresses: 200,
          avgBlockTime: 12,
          privacyEnabled: true 
        }) 
      })
    );

    // Default mock for auth status (unauthenticated)
    await page.route('**/api/auth/status', route => 
      route.fulfill({ 
        status: 200, 
        contentType: 'application/json', 
        body: JSON.stringify({ authenticated: false }) 
      })
    );
  });
  
  test('Authentication Gate - Privacy Dashboard shows login prompt when unauthenticated', async ({ page }) => {
    await page.goto('/privacy');
    await expect(page.getByText(/Sign in with Privado ID/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('button', { name: /Sign in with Privado/i })).toBeVisible();
  });

  test('Dashboard Data Display - Shows own and disclosed addresses when authenticated', async ({ page }) => {
    await page.route('**/api/auth/status', route => 
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ 
        authenticated: true, 
        did: 'did:privado:e2e_test_user' 
      }) })
    );

    await page.route('**/api/privacy/viewable-addresses', route => 
      route.fulfill({ 
        status: 200, 
        contentType: 'application/json', 
        body: JSON.stringify({
          viewer_wallet: '0x123',
          viewer_did: 'did:privado:e2e_test_user',
          own_addresses: [{ address: '0xOwnAddress123', ens_name: 'own.eth' }],
          disclosed_addresses: [
            { 
              address: '0xDisclosedAddress456', 
              address_id: 'addr_id_1', 
              owner_did: 'did:privado:owner', 
              disclosure_level: 'full', 
              grant_id: 'grant_1' 
            }
          ]
        }) 
      })
    );

    await page.goto('/privacy');
    await expect(page.getByText('0xOwnAddress123')).toBeVisible({ timeout: 10000 });
    
    await page.getByRole('button', { name: /Disclosed to You/i }).click();
    await expect(page.getByText('0xDisclosedAddress456')).toBeVisible({ timeout: 10000 });
  });

  test('Address Visibility - Gating and Redaction', async ({ page }) => {
    const publicAddr = '0xPublicAddr789';
    const privateAddr = '0xPrivateAddr012';

    await page.route('**/api/auth/status', route => 
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: true }) })
    );

    // Mock Address Visibility
    await page.route('**/api/privacy/check-address/**', route => {
      const url = route.request().url().toLowerCase();
      if (url.includes(publicAddr.toLowerCase())) {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
          address: publicAddr, visible: true, level: 'full', reason: 'public_address'
        })});
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
        address: privateAddr, visible: false, level: 'hidden', reason: 'no_access'
      })});
    });

    // Mock Address Stats (Dynamic)
    await page.route('**/api/addresses/*', route => {
      const url = route.request().url();
      if (url.includes('/transactions') || url.includes('/contract')) return route.continue();
      
      // Extract address from URL
      const parts = url.split('/');
      const addr = parts[parts.length - 1];

      // If it's the private address, return 403 to trigger the "Address Restricted" view
      if (addr.toLowerCase() === privateAddr.toLowerCase()) {
        return route.fulfill({ 
          status: 403, 
          contentType: 'application/json', 
          body: JSON.stringify({ error: 'forbidden: address is private' }) 
        });
      }

      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
        address: addr, balance: '1000000000000000000', txCount: 1, isContract: false
      })});
    });

    // Mock Address Transactions
    await page.route('**/api/addresses/*/transactions*', route => 
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
        data: [{
          hash: '0xTxHash1', from: publicAddr, to: '0xRecipient', value: '1000000000',
          gasUsed: 21000, gasPrice: 20000, blockNumber: 1, blockTimestamp: 1700000000, status: 1
        }], hasMore: false
      })})
    );

    // Mock Contract (404 for EOA)
    await page.route('**/api/addresses/*/contract', route => 
      route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'not a contract' }) })
    );

    await page.goto(`/address/${publicAddr}`);
    await expect(page.getByText(publicAddr, { exact: false })).toBeVisible({ timeout: 10000 });

    await page.goto(`/address/${privateAddr}`);
    await expect(page.getByText(/Address Restricted/i)).toBeVisible({ timeout: 10000 });
  });

  test('Granted Address Navigation - Pseudonymized View', async ({ page }) => {
    const grantId = 'grant_abc';
    const addressId = 'addr_xyz';
    const pseudonym = 'Address-Alpha';

    await page.route('**/api/auth/status', route => 
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: true }) })
    );

    await page.route(`**/api/privacy/grant/${grantId}/${addressId}`, route => 
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
        display_address: pseudonym, disclosure_level: 'pseudonymous', grant_id: grantId,
        balance: '1000000000', tx_count: 1, is_contract: false
      })})
    );

    await page.route(`**/api/privacy/grant/${grantId}/${addressId}/transactions*`, route => 
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
        transactions: [{
          block_number: 1, block_timestamp: 1700000000, from: pseudonym, to: 'External-BETA',
          value: '1000000000', gas_used: 21000, gas_price: 20000, status: 1, direction: 'out'
        }], disclosure_level: 'pseudonymous',
        address_labels: { [pseudonym]: 'This Address', 'External-BETA': 'External' }, has_more: false
      })})
    );

    await page.goto(`/grant/${grantId}/${addressId}`);
    await expect(page.getByRole('heading', { name: /Pseudonymous Disclosure/i })).toBeVisible({ timeout: 10000 });
    // Use a more specific locator to avoid strict mode violation (pseudonym appears twice)
    await expect(page.locator('.card').getByText(pseudonym).first()).toBeVisible();
    await expect(page.getByText('External-BETA')).toBeVisible();
  });

  test('Error Handling - Access Denied (403)', async ({ page }) => {
    const grantId = 'expired_grant';
    const addressId = 'some_addr';

    await page.route('**/api/auth/status', route => 
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: true }) })
    );

    await page.route(`**/api/privacy/grant/${grantId}/${addressId}`, route => 
      route.fulfill({ status: 403, contentType: 'text/plain', body: 'access denied: grant expired' })
    );

    await page.goto(`/grant/${grantId}/${addressId}`);
    await expect(page.getByText(/Access Denied/i)).toBeVisible({ timeout: 10000 });
  });
});
