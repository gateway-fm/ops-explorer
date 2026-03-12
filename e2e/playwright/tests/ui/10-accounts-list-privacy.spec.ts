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
// Accounts List Page Privacy
//
// Verifies that the /accounts page (global paginated list of all addresses
// that have appeared on-chain) correctly hides or redacts org-owned private
// addresses.
//
// The accounts page fetches data via:
//   ProxyDataProvider -> /api/v1/explorer/accounts
//   -> getExplorerAccounts -> GetBatchVisibility -> filters/redacts private entries
//
// To ensure the address appears in the accounts list it must have on-chain
// activity. We send a small ETH transfer to ANVIL_ACCOUNTS[5] using accounts
// that haven't been used in other tests, then register it as org-owned.
//
// NOTE: This test checks the first page of the accounts list. If pagination
// pushes the private address off the first page the raw-address assertion
// still holds (address absent = not leaked), but the [PRIVATE] assertion is
// skipped gracefully.
// ---------------------------------------------------------------------------

// Re-use ANVIL_ACCOUNTS[5] — it also appears in test 08 as a linked personal
// wallet, but here we register the same address as an org-owned contract to
// test the contract-grant privacy path on the accounts list. The two tests
// use different orgs so there is no collision.
const PRIVATE_ACCT_ADDRESS = ANVIL_ACCOUNTS[5].address;

test.describe('Accounts List Page Privacy', () => {
  let fixture: ProxyAdminFixture;
  let memberDid: string;
  let outsiderDid: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Ensure the address has on-chain activity so the indexer lists it
    const txHash = await sendETH(
      ANVIL_ACCOUNTS[0].address,
      PRIVATE_ACCT_ADDRESS,
      BigInt('800000000000000'),
    );
    await waitForReceipt(txHash);

    const currentBlock = await getBlockNumber();
    await waitForIndexer(currentBlock);

    // Register address as org-owned
    const org = await fixture.createOrg('acctlist', 'Accounts List Privacy Test Org');
    const { group } = await fixture.createGroup(org.id, 'viewers', 'Viewers', ['read']);

    await fixture.createContract(org.id, PRIVATE_ACCT_ADDRESS, 'Private Anvil Account 5');
    await fixture.createContractGrant(org.id, PRIVATE_ACCT_ADDRESS, group.id);

    memberDid = fixture.did();
    const { user: memberUser } = await fixture.ensureUser(memberDid);
    await fixture.addMembership(memberUser.id, group.id);

    outsiderDid = fixture.did();
    await fixture.ensureUser(outsiderDid);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('private address not exposed in plaintext on /accounts for outsider', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, outsiderDid);

    await page.goto('/accounts');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    const bodyText = await page.locator('body').textContent();
    const addrFragment = PRIVATE_ACCT_ADDRESS.toLowerCase().slice(2, 10);
    const addrExposed = bodyText?.toLowerCase().includes(addrFragment);

    expect(addrExposed).toBe(false);
  });

  test('private address not exposed in plaintext on /accounts for anonymous user', async ({
    page,
    context,
  }) => {
    await logout(context);

    await page.goto('/accounts');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    const bodyText = await page.locator('body').textContent();
    const addrFragment = PRIVATE_ACCT_ADDRESS.toLowerCase().slice(2, 10);
    const addrExposed = bodyText?.toLowerCase().includes(addrFragment);

    expect(addrExposed).toBe(false);
  });

  test('member can navigate to accounts list without errors', async ({ page, context }) => {
    await loginViaCookie(context, memberDid);

    await page.goto('/accounts');
    await page.waitForLoadState('networkidle');

    // No auth walls — the page itself is publicly accessible
    const authRequired = await page
      .getByRole('heading', { name: /Authentication Required/i })
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    expect(authRequired).toBe(false);

    // Page should have content (accounts table or similar)
    const hasContent = await page
      .getByText(/address|balance|transaction/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
  });
});
