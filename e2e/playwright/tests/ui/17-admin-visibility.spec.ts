import { test, expect } from '@playwright/test';
import { ProxyAdminFixture } from '../../helpers/proxy-admin';
import { loginViaCookie, logout } from '../../helpers/explorer-auth';
import {
  ANVIL_ACCOUNTS,
  sendETH,
  waitForReceipt,
  getBlockNumber,
  waitForIndexer,
} from '../../helpers/blockchain';

// ---------------------------------------------------------------------------
// Admin vs Regular User Visibility
//
// Admins have broader visibility into transactions and addresses than
// regular users or anonymous visitors. This test suite verifies that:
//
//   1. An admin user sees more data (transactions, addresses) than an
//      anonymous user when org-owned private addresses are involved.
//   2. A regular org member (with only 'read' claim) does not get extra
//      visibility beyond what they would normally have access to.
//
// The tests create a controlled scenario with org-owned contracts and
// private wallet links, then compare what each user type can see.
// ---------------------------------------------------------------------------

test.describe('Admin vs Regular User Visibility', () => {
  test.slow(); // beforeAll setup creates org, users, sends tx, waits for indexer
  let fixture: ProxyAdminFixture;
  let adminDid: string;
  let regularDid: string;
  let orgId: string;
  let contractAddress: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Create org with an admin group and a regular group
    const org = await fixture.createOrg('adminvis', 'Admin Visibility Test Org');
    orgId = org.id;

    const { group: adminGroup } = await fixture.createGroup(
      orgId,
      'admins',
      'Admins',
      ['admin'],
    );
    const { group: regularGroup } = await fixture.createGroup(
      orgId,
      'readers',
      'Readers',
      ['read'],
    );

    // Register a fake contract address as org-owned
    contractAddress = fixture.address();
    await fixture.createContract(orgId, contractAddress, 'Admin Vis Test Contract');
    await fixture.createContractGrant(orgId, contractAddress, adminGroup.id);
    await fixture.createContractGrant(orgId, contractAddress, regularGroup.id);

    // Create admin user
    adminDid = fixture.did();
    const { user: adminUser } = await fixture.ensureUser(adminDid);
    await fixture.addMembership(adminUser.id, adminGroup.id);

    // Create regular user (read only)
    regularDid = fixture.did();
    const { user: regularUser } = await fixture.ensureUser(regularDid);
    await fixture.addMembership(regularUser.id, regularGroup.id);

    // Generate some transactions so there is data to compare
    await sendETH(
      ANVIL_ACCOUNTS[0].address,
      ANVIL_ACCOUNTS[1].address,
      BigInt(String(1000000000000000 + Math.floor(Math.random() * 1000000))),
    ).then(async (hash) => {
      await waitForReceipt(hash);
    });

    const currentBlock = await getBlockNumber();
    await waitForIndexer(currentBlock);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  // -------------------------------------------------------------------------
  // 1. Admin sees at least as many transactions as anonymous
  // -------------------------------------------------------------------------

  test('admin sees at least as many transactions as anonymous user', async ({
    page,
    context,
  }) => {
    // First, check anonymous transaction count
    await logout(context);
    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    // Extract the total transaction count from the page header
    // The Transactions page shows "{N} transactions" in the header
    let anonBodyText = await page.locator('body').textContent() || '';
    const anonCountMatch = anonBodyText.match(/(\d[\d,]*)\s+transactions/i);
    const anonCount = anonCountMatch
      ? parseInt(anonCountMatch[1].replace(/,/g, ''), 10)
      : null;

    // Count visible table rows as a fallback
    const anonRows = await page.locator('table tbody tr').count();

    // Now log in as admin
    await loginViaCookie(context, adminDid);
    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    let adminBodyText = await page.locator('body').textContent() || '';
    const adminCountMatch = adminBodyText.match(/(\d[\d,]*)\s+transactions/i);
    const adminCount = adminCountMatch
      ? parseInt(adminCountMatch[1].replace(/,/g, ''), 10)
      : null;

    const adminRows = await page.locator('table tbody tr').count();

    // Admin should see >= anonymous count (never fewer)
    if (anonCount !== null && adminCount !== null) {
      expect(adminCount).toBeGreaterThanOrEqual(anonCount);
    } else {
      // Fall back to row count comparison
      expect(adminRows).toBeGreaterThanOrEqual(anonRows);
    }
  });

  // -------------------------------------------------------------------------
  // 2. Regular org member does not see extra transactions
  // -------------------------------------------------------------------------

  test('regular org member does not gain extra visibility over anonymous', async ({
    page,
    context,
  }) => {
    // Anonymous count
    await logout(context);
    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    let anonBodyText = await page.locator('body').textContent() || '';
    const anonCountMatch = anonBodyText.match(/(\d[\d,]*)\s+transactions/i);
    const anonCount = anonCountMatch
      ? parseInt(anonCountMatch[1].replace(/,/g, ''), 10)
      : null;
    const anonRows = await page.locator('table tbody tr').count();

    // Regular user count
    await loginViaCookie(context, regularDid);
    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    let regularBodyText = await page.locator('body').textContent() || '';
    const regularCountMatch = regularBodyText.match(/(\d[\d,]*)\s+transactions/i);
    const regularCount = regularCountMatch
      ? parseInt(regularCountMatch[1].replace(/,/g, ''), 10)
      : null;
    const regularRows = await page.locator('table tbody tr').count();

    // A regular read-only member should not see significantly more txs than
    // anonymous. Allow a small margin (3) for race conditions with new blocks.
    if (anonCount !== null && regularCount !== null) {
      expect(regularCount).toBeLessThanOrEqual(anonCount + 3);
    } else {
      expect(regularRows).toBeLessThanOrEqual(anonRows + 3);
    }
  });

  // -------------------------------------------------------------------------
  // 3. Admin can access org-owned contract address page
  // -------------------------------------------------------------------------

  test('admin can access org-owned contract page', async ({ page, context }) => {
    await loginViaCookie(context, adminDid);

    await page.goto(`/address/${contractAddress}`);
    await page.waitForLoadState('networkidle');

    // Should NOT show "Authentication Required" or "Address Restricted"
    const authRequired = await page
      .getByRole('heading', { name: /Authentication Required/i })
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    const restricted = await page
      .getByRole('heading', { name: /Address Restricted/i })
      .isVisible({ timeout: 5000 })
      .catch(() => false);

    expect(authRequired).toBe(false);
    expect(restricted).toBe(false);

    // Page should have address-related content
    const hasContent = await page
      .getByText(/balance|transaction|address|contract/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
  });

  // -------------------------------------------------------------------------
  // 4. Anonymous user cannot access org-owned contract address page
  // -------------------------------------------------------------------------

  test('anonymous user sees Authentication Required on org-owned address', async ({
    page,
    context,
  }) => {
    await logout(context);

    await page.goto(`/address/${contractAddress}`);
    await page.waitForLoadState('networkidle');

    // Should show "Authentication Required"
    const authRequiredHeading = page.getByRole('heading', {
      name: /Authentication Required/i,
    });
    await expect(authRequiredHeading).toBeVisible({ timeout: 15000 });
  });
});
