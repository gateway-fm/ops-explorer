import { test, expect, apiMock, expectLoadedOK, makeTx } from '../../fixtures/api-mock';

// S16 Visibility labels, S17 View-as.

test.describe('S16 Visibility labels', () => {
  test('addressMetadata reasons render AddressLabel badges (Mine / My Org / Disclosed / Counterparty)', async ({ page }) => {
    const from = '0x' + '1'.repeat(40);
    const to = '0x' + '2'.repeat(40);
    await apiMock(page, {
      authenticated: true,
      routes: {
        transactions: {
          body: {
            data: [
              makeTx(1, { from, to, addressMetadata: { [from]: 'own_address', [to]: 'rbac_group_member' } }),
              makeTx(2, { from, to, addressMetadata: { [from]: 'disclosure_grant', [to]: 'participant_override' } }),
            ],
            total: 2, page: 1, pageSize: 25, totalPages: 1,
          },
        },
      },
    });
    await page.goto('/transactions');
    await expectLoadedOK(page, page.getByTestId('tx-row'));

    // Badges keyed by data-reason.
    await expect(page.locator('[data-testid="address-label"][data-reason="own_address"]').first()).toBeVisible();
    await expect(page.locator('[data-testid="address-label"][data-reason="rbac_group_member"]').first()).toBeVisible();
    await expect(page.locator('[data-testid="address-label"][data-reason="disclosure_grant"]').first()).toBeVisible();
    await expect(page.getByText('Mine').first()).toBeVisible();
    await expect(page.getByText('My Org').first()).toBeVisible();
  });

  test('[PRIVATE] placeholder renders a lock-icon "Private" private-address', async ({ page }) => {
    await apiMock(page, {
      routes: {
        transactions: {
          body: {
            data: [makeTx(1, { from: '0x' + '1'.repeat(40), to: '[PRIVATE]' })],
            total: 1, page: 1, pageSize: 25, totalPages: 1,
          },
        },
      },
    });
    await page.goto('/transactions');
    await expectLoadedOK(page, page.getByTestId('tx-row'));
    const priv = page.locator('[data-testid="private-address"][data-redaction="private"]').first();
    await expect(priv).toBeVisible();
    await expect(priv).toContainText('Private');
  });

  test('PrivacyDashboard (authed) shows "Disclosed Addresses (N)" with N == disclosed rows', async ({ page }) => {
    await apiMock(page, {
      authenticated: true,
      routes: {
        'viewable-addresses': {
          body: {
            viewer_wallet: '0x0', viewer_did: 'did:privado:e2e_user', own_addresses: [],
            disclosed_addresses: [
              { address: '0x' + 'a'.repeat(40), address_id: 'a1', owner_did: 'did:x', disclosure_level: 'full', grant_id: 'g1' },
              { address: '0x' + 'b'.repeat(40), address_id: 'a2', owner_did: 'did:y', disclosure_level: 'pseudonymous', grant_id: 'g2' },
            ],
          },
        },
      },
    });
    await page.goto('/privacy');
    await expect(page.getByTestId('disclosure-banner')).toHaveText('Disclosed Addresses (2)');
    await expect(page.getByTestId('disclosed-row')).toHaveCount(2);
    await expect(page.getByTestId('disclosure-level-badge').first()).toBeVisible();
  });
});

test.describe('S17 View-as', () => {
  test('/view-as?did&org → POST /impersonation/start → redirect home + banner', async ({ page }) => {
    await apiMock(page, {
      authenticated: true,
      routes: {
        'impersonation-start': {
          body: { token: 'imp-tok-1', expires_at: new Date(Date.now() + 3600_000).toISOString(), target_did: 'did:privado:target', org_id: 'org1' },
        },
      },
    });
    await page.goto('/view-as?did=did:privado:target&org=org1');

    // Lands home with the banner active.
    await expect(page).toHaveURL(/\/(\?as=imp-tok-1)?$/);
    await expect(page.getByTestId('impersonation-banner')).toBeVisible();
    await expect(page.getByTestId('impersonation-banner')).toContainText('View-as mode');
  });

  test('Stop viewing-as → DELETE → banner disappears', async ({ page }) => {
    await apiMock(page, {
      authenticated: true,
      routes: {
        'impersonation-start': {
          body: { token: 'imp-tok-2', expires_at: new Date(Date.now() + 3600_000).toISOString(), target_did: 'did:privado:target', org_id: 'org1' },
        },
      },
    });
    await page.goto('/view-as?did=did:privado:target&org=org1');
    await expect(page.getByTestId('impersonation-banner')).toBeVisible();

    await page.getByRole('button', { name: /Stop viewing as/i }).click();
    await expect(page.getByTestId('impersonation-banner')).toHaveCount(0);
  });

  test('missing did → instructional error (no impersonation start)', async ({ page }) => {
    await apiMock(page, { authenticated: true });
    await page.goto('/view-as?org=org1');
    await expect(page.getByTestId('app-error')).toBeVisible();
    await expect(page.getByText(/Missing target DID/i)).toBeVisible();
  });

  test('missing org → instructional error', async ({ page }) => {
    await apiMock(page, { authenticated: true });
    await page.goto('/view-as?did=did:privado:target');
    await expect(page.getByTestId('app-error')).toBeVisible();
    await expect(page.getByText(/Missing organization/i)).toBeVisible();
  });

  test('start 404 (target not in org) → error surfaced', async ({ page }) => {
    await apiMock(page, {
      authenticated: true,
      routes: { 'impersonation-start': { status: 404, body: { error: 'not found' } } },
    });
    await page.goto('/view-as?did=did:privado:ghost&org=org1');
    await expect(page.getByTestId('app-error')).toBeVisible();
    await expect(page.getByText(/not found or not a member/i)).toBeVisible();
  });

  test('start 403 (org not administered) → error surfaced', async ({ page }) => {
    await apiMock(page, {
      authenticated: true,
      routes: { 'impersonation-start': { status: 403, body: { error: 'forbidden' } } },
    });
    await page.goto('/view-as?did=did:privado:target&org=orgX');
    await expect(page.getByTestId('app-error')).toBeVisible();
    await expect(page.getByText(/not authorised/i)).toBeVisible();
  });
});
