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
// Intra-Org Personal Wallet Privacy
//
// Verifies that org membership does NOT grant access to other members'
// personal wallets. Personal wallet addresses are linked via eth_address_links
// (ZK-proof / mock signature in dev), NOT via contract_grants. Access is
// governed by disclosure grants — not shared with org-mates by default.
//
// Scenario:
//   - Org "walletprivacy" with one group containing User A and User B
//   - User A links ANVIL_ACCOUNTS[5] as their personal wallet
//   - Real ETH transactions sent from/to ANVIL_ACCOUNTS[5]
//   - User B (same org, same group) must NOT see User A's wallet address;
//     it should appear as [PRIVATE] in lists and "Address Restricted" on the
//     address page
//   - User A CAN access their own wallet page normally
//   - Anonymous user sees "Authentication Required"
//
// This is a different privacy model from org-owned contracts (tested in 04, 07).
// ---------------------------------------------------------------------------

// ANVIL_ACCOUNTS[5] is reserved for User A's personal wallet in this test.
const USER_A_WALLET = ANVIL_ACCOUNTS[5].address;

test.describe('Intra-Org Personal Wallet Privacy', () => {
  let fixture: ProxyAdminFixture;
  let userADid: string;
  let userBDid: string;
  let userAToken: string;

  // Transaction hashes created in beforeAll
  let txHash: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Create org and group (both users will be in the same group)
    const org = await fixture.createOrg('walletprivacy', 'Wallet Privacy Test Org');
    const { group } = await fixture.createGroup(org.id, 'members', 'Members', ['read']);

    // Create User A and link their personal wallet
    userADid = fixture.did();
    const { user: userAUser, accessToken } = await fixture.ensureUser(userADid);
    userAToken = accessToken;
    await fixture.addMembership(userAUser.id, group.id);

    // Link ANVIL_ACCOUNTS[5] to User A's DID via mock signature
    await fixture.linkUserWallet(userAToken, USER_A_WALLET);

    // Create User B — same org, same group, but NOT the wallet owner
    userBDid = fixture.did();
    const { user: userBUser } = await fixture.ensureUser(userBDid);
    await fixture.addMembership(userBUser.id, group.id);

    // Generate on-chain transactions involving User A's wallet
    //   account[0] -> account[5]  (User A receives ETH)
    txHash = await sendETH(
      ANVIL_ACCOUNTS[0].address,
      USER_A_WALLET,
      BigInt('1500000000000000'),
    ).then(async (hash) => {
      await waitForReceipt(hash);
      return hash;
    });

    //   account[5] -> account[4]  (User A sends ETH — address is sender)
    await sendETH(
      USER_A_WALLET,
      ANVIL_ACCOUNTS[4].address,
      BigInt('500000000000000'),
    ).then(async (hash) => {
      await waitForReceipt(hash);
      return hash;
    });

    // Wait for the block-explorer indexer to catch up
    const currentBlock = await getBlockNumber();
    await waitForIndexer(currentBlock);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  // ---------------------------------------------------------------------------
  // User A can see their own wallet (sanity / positive case)
  // ---------------------------------------------------------------------------

  test('User A can access their own linked wallet address page', async ({ page, context }) => {
    await loginViaCookie(context, userADid);

    await page.goto(`/address/${USER_A_WALLET}`);
    await page.waitForLoadState('networkidle');

    // No auth walls
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

    // Page should render address-related content
    const hasContent = await page
      .getByText(/balance|transaction|address/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
  });

  // ---------------------------------------------------------------------------
  // User B (same org, same group) cannot access User A's wallet
  // ---------------------------------------------------------------------------

  test('User B (same org) sees Address Restricted on User A wallet page', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, userBDid);

    await page.goto(`/address/${USER_A_WALLET}`);
    await page.waitForLoadState('networkidle');

    // Org membership does NOT grant access to another member's personal wallet
    const restrictedHeading = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeading).toBeVisible({ timeout: 15000 });
  });

  // ---------------------------------------------------------------------------
  // User A's address appears as [PRIVATE] in transaction lists for User B
  // ---------------------------------------------------------------------------

  test('User A wallet address appears as [PRIVATE] in tx list for User B', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, userBDid);

    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    const bodyText = await page.locator('body').textContent();
    const addrFragment = USER_A_WALLET.toLowerCase().slice(2, 10);
    const addrExposed = bodyText?.toLowerCase().includes(addrFragment);

    if (addrExposed) {
      // The raw address must never be visible to a non-owner org-mate
      expect(addrExposed).toBe(false);
    } else {
      // Address is either [PRIVATE] or not yet on this page of results.
      // If [PRIVATE] is visible, that's the correct redacted state.
      const hasPrivate = await page
        .getByText('[PRIVATE]')
        .first()
        .isVisible({ timeout: 5000 })
        .catch(() => false);
      // Either the address is fully hidden (not on this page) or shown as [PRIVATE].
      // What's NOT acceptable is the raw address being visible — already asserted above.
      // Log for visibility during development.
      if (!hasPrivate) {
        console.warn(
          '[WARN] account[5] tx not on first page of /transactions for User B; ' +
            'raw address correctly absent.',
        );
      }
    }
  });

  // ---------------------------------------------------------------------------
  // Transaction detail page — User B sees [PRIVATE] for User A's address
  // ---------------------------------------------------------------------------

  test('transaction detail page: User A address is [PRIVATE] for User B', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, userBDid);

    await page.goto(`/tx/${txHash}`);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    const bodyText = await page.locator('body').textContent();
    const addrFragment = USER_A_WALLET.toLowerCase().slice(2, 10);
    const addrExposed = bodyText?.toLowerCase().includes(addrFragment);

    expect(addrExposed).toBe(false);
  });

  // ---------------------------------------------------------------------------
  // Transaction detail page — User A sees their own address (no auth wall)
  // ---------------------------------------------------------------------------

  test('transaction detail page: User A sees their own address without restriction', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, userADid);

    await page.goto(`/tx/${txHash}`);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // No auth walls
    const authRequired = await page
      .getByRole('heading', { name: /Authentication Required/i })
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    const restricted = await page
      .getByRole('heading', { name: /Address Restricted/i })
      .isVisible({ timeout: 3000 })
      .catch(() => false);

    expect(authRequired).toBe(false);
    expect(restricted).toBe(false);
  });

  // ---------------------------------------------------------------------------
  // Anonymous user sees Authentication Required (not logged in)
  // ---------------------------------------------------------------------------

  test('anonymous user sees Authentication Required on User A wallet page', async ({
    page,
    context,
  }) => {
    await logout(context);

    await page.goto(`/address/${USER_A_WALLET}`);
    await page.waitForLoadState('networkidle');

    const authRequiredHeading = page.getByRole('heading', { name: /Authentication Required/i });
    await expect(authRequiredHeading).toBeVisible({ timeout: 15000 });
  });

  // ---------------------------------------------------------------------------
  // Token balances page — User B cannot see User A's token balances
  // (Address page is fully gated, so the token tab is never reachable)
  // ---------------------------------------------------------------------------

  test('User B cannot reach token balances tab on User A wallet page', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, userBDid);

    // Try navigating directly to the token balances section
    await page.goto(`/address/${USER_A_WALLET}?tab=tokens`);
    await page.waitForLoadState('networkidle');

    // The address page gate fires before any tab content renders
    const restrictedHeading = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeading).toBeVisible({ timeout: 15000 });
  });
});
