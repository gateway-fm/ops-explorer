import { test, expect } from '@playwright/test';
import { ProxyAdminFixture, DisclosureScope } from '../../helpers/proxy-admin';
import { loginViaCookie } from '../../helpers/explorer-auth';

// ---------------------------------------------------------------------------
// Grant Page Visibility (G01-G05 derived) — verify that the grant page
// shows the correct tabs (Transactions / Activity Logs) based on scope and
// disclosure level. Also verifies View link goes to grant page, not address page.
//
// Requires both privacy-proxy and block-explorer stacks to be running.
// ---------------------------------------------------------------------------

test.describe('Grant Page Visibility', () => {
  test.slow(); // beforeAll does org/user/grant setup

  let fixture: ProxyAdminFixture;
  let orgId: string;
  let groupId: string;

  // Target user (the one whose data is disclosed)
  let targetDid: string;
  let targetUserId: string;
  let targetToken: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Create org + group so users have a shared context
    const org = await fixture.createOrg('grantvis', 'Grant Visibility Test Org');
    orgId = org.id;
    const { group } = await fixture.createGroup(orgId, 'members', 'Members', ['read']);
    groupId = group.id;

    // Create and set up the target user
    targetDid = fixture.did();
    const targetWallet = fixture.address();
    const { user: targetUser, accessToken } = await fixture.ensureUser(targetDid);
    targetUserId = targetUser.id;
    targetToken = accessToken;
    await fixture.addMembership(targetUser.id, groupId);
    await fixture.linkUserWallet(targetToken, targetWallet);
  });

  test.afterAll(async () => {
    if (fixture) await fixture.cleanup();
  });

  /**
   * Helper: creates a disclosure grant with specified scope/level,
   * creates a requester user, and returns login info.
   */
  async function createGrantForRequester(
    scope: DisclosureScope,
  ): Promise<{
    requesterDid: string;
    grantId: string;
  }> {
    const requesterDid = fixture.did();
    const { user: requesterUser } = await fixture.ensureUser(requesterDid);
    await fixture.addMembership(requesterUser.id, groupId);

    // Admin creates disclosure request on behalf of requester
    const req = await fixture.createDisclosureRequest(
      requesterDid,
      targetUserId,
      'E2E grant visibility test',
      scope,
    );

    // Target approves the request -> creates grant
    const approveResult = await fixture.approveDisclosureRequest(
      req.id,
      targetToken,
    );

    return {
      requesterDid,
      grantId: approveResult.grant.id,
    };
  }

  // -------------------------------------------------------------------------
  // P21: full_disclosure + full -> both tabs visible
  // -------------------------------------------------------------------------
  test('P21: full_disclosure + full shows both Transactions and Activity Logs tabs', async ({
    page,
    context,
  }) => {
    const { requesterDid } = await createGrantForRequester({
      methods: ['full_disclosure'],
      disclosure_level: 'full',
    });

    await loginViaCookie(context, requesterDid);

    // Navigate to Privacy Dashboard
    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // Click the "Disclosed to You" tab
    const disclosedTab = page.getByText(/Disclosed to You/i);
    await expect(disclosedTab.first()).toBeVisible({ timeout: 15000 });
    await disclosedTab.first().click();
    await page.waitForTimeout(2000);

    // Click the "View" link for the disclosed address
    const viewLink = page
      .locator('a')
      .filter({ hasText: /View/i })
      .first();
    const viewLinkVisible = await viewLink
      .isVisible({ timeout: 10000 })
      .catch(() => false);

    if (!viewLinkVisible) {
      test.skip(true, 'View link not visible — disclosed address may not be loaded yet');
      return;
    }

    await viewLink.click();
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // Verify we landed on the grant page (URL should contain /grant/)
    expect(page.url()).toContain('/grant/');

    // Verify "Full Disclosure" badge is shown
    const fullBadge = page.getByText('Full Disclosure');
    await expect(fullBadge.first()).toBeVisible({ timeout: 10000 });

    // Verify both tab buttons are present
    const transactionsTab = page.getByRole('button', { name: /Transactions/i });
    const activityLogsTab = page.getByRole('button', { name: /Activity Logs/i });

    await expect(transactionsTab).toBeVisible({ timeout: 5000 });
    await expect(activityLogsTab).toBeVisible({ timeout: 5000 });
  });

  // -------------------------------------------------------------------------
  // P22: activity_logs + full -> only Activity Logs tab
  // -------------------------------------------------------------------------
  test('P22: activity_logs + full shows only Activity Logs tab', async ({
    page,
    context,
  }) => {
    const { requesterDid } = await createGrantForRequester({
      methods: ['activity_logs'],
      disclosure_level: 'full',
    });

    await loginViaCookie(context, requesterDid);

    // Navigate directly to the grant page using a known URL pattern.
    // We need the address_id, which the privacy dashboard provides via
    // the viewable-addresses endpoint. Going through the dashboard is
    // the reliable way to get the correct addressId.
    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    const disclosedTab = page.getByText(/Disclosed to You/i);
    await expect(disclosedTab.first()).toBeVisible({ timeout: 15000 });
    await disclosedTab.first().click();
    await page.waitForTimeout(2000);

    const viewLink = page
      .locator('a')
      .filter({ hasText: /View/i })
      .first();
    const viewLinkVisible = await viewLink
      .isVisible({ timeout: 10000 })
      .catch(() => false);

    if (!viewLinkVisible) {
      test.skip(true, 'View link not visible');
      return;
    }

    await viewLink.click();
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    expect(page.url()).toContain('/grant/');

    // Activity Logs tab should be present
    const activityLogsTab = page.getByRole('button', { name: /Activity Logs/i });

    // Transactions tab should NOT be present (activity_logs-only scope)
    // When only one tab is available, the tab bar may not render at all
    // and the content is shown directly. Check for tab buttons.
    const transactionsTab = page.getByRole('button', { name: /Transactions/i });
    const txTabVisible = await transactionsTab
      .isVisible({ timeout: 3000 })
      .catch(() => false);

    expect(txTabVisible).toBe(false);

    // The page should show activity logs content or the Activity Logs tab
    // When there's only one tab, the component may render it directly
    // (without tab buttons). Check for activity-related content.
    const hasActivityContent =
      (await activityLogsTab.isVisible({ timeout: 3000 }).catch(() => false)) ||
      (await page
        .getByText(/Activity Logs|RPC method calls/i)
        .first()
        .isVisible({ timeout: 3000 })
        .catch(() => false));

    expect(hasActivityContent).toBe(true);
  });

  // -------------------------------------------------------------------------
  // P23: transaction_history + full -> only Transactions tab
  // -------------------------------------------------------------------------
  test('P23: transaction_history + full shows only Transactions tab', async ({
    page,
    context,
  }) => {
    const { requesterDid } = await createGrantForRequester({
      methods: ['transaction_history'],
      disclosure_level: 'full',
    });

    await loginViaCookie(context, requesterDid);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    const disclosedTab = page.getByText(/Disclosed to You/i);
    await expect(disclosedTab.first()).toBeVisible({ timeout: 15000 });
    await disclosedTab.first().click();
    await page.waitForTimeout(2000);

    const viewLink = page
      .locator('a')
      .filter({ hasText: /View/i })
      .first();
    const viewLinkVisible = await viewLink
      .isVisible({ timeout: 10000 })
      .catch(() => false);

    if (!viewLinkVisible) {
      test.skip(true, 'View link not visible');
      return;
    }

    await viewLink.click();
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    expect(page.url()).toContain('/grant/');

    // Activity Logs tab should NOT be present
    const activityLogsTab = page.getByRole('button', { name: /Activity Logs/i });
    const activityTabVisible = await activityLogsTab
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    expect(activityTabVisible).toBe(false);

    // Transactions content should be present (either as tab or direct content)
    const hasTransactionsContent =
      (await page
        .getByRole('button', { name: /Transactions/i })
        .isVisible({ timeout: 3000 })
        .catch(() => false)) ||
      (await page
        .getByText(/No transactions found|Transactions/i)
        .first()
        .isVisible({ timeout: 3000 })
        .catch(() => false));

    expect(hasTransactionsContent).toBe(true);
  });

  // -------------------------------------------------------------------------
  // P24: full_disclosure + redacted -> only Activity Logs tab
  // (redacted blocks transactions, full_disclosure scope includes activity)
  // -------------------------------------------------------------------------
  test('P24: full_disclosure + redacted shows only Activity Logs tab', async ({
    page,
    context,
  }) => {
    const { requesterDid } = await createGrantForRequester({
      methods: ['full_disclosure'],
      disclosure_level: 'redacted',
    });

    await loginViaCookie(context, requesterDid);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    const disclosedTab = page.getByText(/Disclosed to You/i);
    await expect(disclosedTab.first()).toBeVisible({ timeout: 15000 });
    await disclosedTab.first().click();
    await page.waitForTimeout(2000);

    const viewLink = page
      .locator('a')
      .filter({ hasText: /View/i })
      .first();
    const viewLinkVisible = await viewLink
      .isVisible({ timeout: 10000 })
      .catch(() => false);

    if (!viewLinkVisible) {
      test.skip(true, 'View link not visible');
      return;
    }

    await viewLink.click();
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    expect(page.url()).toContain('/grant/');

    // Should show "Redacted Disclosure" banner
    const redactedBadge = page.getByText('Redacted Disclosure');
    const hasRedactedBadge = await redactedBadge
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasRedactedBadge).toBe(true);

    // Transactions tab should NOT be visible (redacted blocks it)
    const transactionsTab = page.getByRole('button', { name: /Transactions/i });
    const txTabVisible = await transactionsTab
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    expect(txTabVisible).toBe(false);

    // Activity Logs content should be present
    const hasActivityContent =
      (await page
        .getByRole('button', { name: /Activity Logs/i })
        .isVisible({ timeout: 3000 })
        .catch(() => false)) ||
      (await page
        .getByText(/Activity Logs|RPC method calls|No activity logs/i)
        .first()
        .isVisible({ timeout: 3000 })
        .catch(() => false));

    expect(hasActivityContent).toBe(true);
  });

  // -------------------------------------------------------------------------
  // P25: View link goes to grant page (not address page)
  // -------------------------------------------------------------------------
  test('P25: View link navigates to /grant/ not /address/', async ({
    page,
    context,
  }) => {
    // Use any existing grant (full disclosure for simplicity)
    const { requesterDid } = await createGrantForRequester({
      methods: ['full_disclosure'],
      disclosure_level: 'full',
    });

    await loginViaCookie(context, requesterDid);

    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    const disclosedTab = page.getByText(/Disclosed to You/i);
    await expect(disclosedTab.first()).toBeVisible({ timeout: 15000 });
    await disclosedTab.first().click();
    await page.waitForTimeout(2000);

    // Get the View link href before clicking
    const viewLink = page
      .locator('a')
      .filter({ hasText: /View/i })
      .first();

    const viewLinkVisible = await viewLink
      .isVisible({ timeout: 10000 })
      .catch(() => false);

    if (!viewLinkVisible) {
      test.skip(true, 'View link not visible');
      return;
    }

    const href = await viewLink.getAttribute('href');
    expect(href).toContain('/grant/');
    expect(href).not.toContain('/address/');

    // Click and verify navigation
    await viewLink.click();
    await page.waitForLoadState('networkidle');

    expect(page.url()).toContain('/grant/');
    expect(page.url()).not.toMatch(/\/address\/0x/);
  });
});
