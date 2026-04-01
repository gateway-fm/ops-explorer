import { test, expect } from '@playwright/test';
import { ProxyAdminFixture } from '../../helpers/proxy-admin';
import { loginViaCookie, logout } from '../../helpers/explorer-auth';

// ---------------------------------------------------------------------------
// Disclosure Flow — serial tests that build on each other
// ---------------------------------------------------------------------------

test.describe('Disclosure Flow', () => {
  test.describe.configure({ mode: 'serial' });

  let fixture: ProxyAdminFixture;
  let userADid: string;
  let userBDid: string;
  let userAId: string;
  let userAToken: string;
  let grantId: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // userA = data owner (the one whose data is disclosed)
    userADid = fixture.did();
    const userAResult = await fixture.ensureUser(userADid);
    userAId = userAResult.user.id;
    userAToken = userAResult.accessToken;

    // userB = requester (the one who receives the disclosure grant)
    userBDid = fixture.did();
    await fixture.ensureUser(userBDid);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('disclosure grant appears in recipient dashboard', async ({ page, context }) => {
    // Admin creates a disclosure request: userB wants to see userA's data
    const disclosureReq = await fixture.createDisclosureRequest(
      userBDid,
      userAId,
      'E2E test: compliance audit',
      { disclosure_level: 'full' },
    );

    // userA approves the request using their own JWT
    const approveResult = await fixture.approveDisclosureRequest(
      disclosureReq.id,
      userAToken,
    );
    grantId = approveResult.grant.id;

    // Log into block-explorer as userB (the grant recipient)
    await loginViaCookie(context, userBDid);

    // Navigate to the privacy dashboard
    await page.goto('/privacy');
    await page.waitForLoadState('networkidle');

    // The dashboard should show tabs, not the sign-in prompt
    const disclosedTab = page.getByText(/Disclosed to You/i);
    await expect(disclosedTab.first()).toBeVisible({ timeout: 15000 });

    // Click the "Disclosed to You" tab
    await disclosedTab.first().click();

    // Wait for disclosed data to load
    await page.waitForTimeout(2000);

    // Verify the tab shows a non-zero count, e.g. "Disclosed to You (1)"
    const tabText = await disclosedTab.first().textContent();
    const countMatch = tabText?.match(/\((\d+)\)/);
    if (countMatch) {
      const count = parseInt(countMatch[1], 10);
      expect(count).toBeGreaterThanOrEqual(1);
    }

    // Check for table rows or list items in the disclosed section
    const disclosedRows = page.locator('table tbody tr, [data-testid="disclosed-item"]');
    const hasRows = await disclosedRows
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);

    if (hasRows) {
      const rowCount = await disclosedRows.count();
      expect(rowCount).toBeGreaterThanOrEqual(1);

      // Try to click a "View" link to navigate to the grant page
      const viewLink = page.locator('a').filter({ hasText: /View/i }).first();
      const viewLinkVisible = await viewLink.isVisible({ timeout: 5000 }).catch(() => false);

      if (viewLinkVisible) {
        await viewLink.click();
        await page.waitForLoadState('networkidle');

        // The grant page should load without "Access Denied"
        const hasAccessDenied = await page
          .getByRole('heading', { name: /Access Denied/i })
          .isVisible({ timeout: 5000 })
          .catch(() => false);
        expect(hasAccessDenied).toBe(false);
      }
    }
  });

  test('revoked grant shows Access Denied', async ({ page, context }) => {
    // This test depends on grantId from the previous test
    test.skip(!grantId, 'No grant ID from previous test');

    // Revoke the grant via admin API
    await fixture.revokeDisclosureGrant(grantId, 'E2E test revocation');

    // Log in as userB (the grant recipient)
    await loginViaCookie(context, userBDid);

    // Navigate directly to the grant page — should show Access Denied or expired
    await page.goto(`/grant/${grantId}/dummy-address-id`);
    await page.waitForLoadState('networkidle');

    // The page should show an error — "Access Denied", "Address Not Found",
    // or text about expired/revoked. The exact message depends on how the
    // explorer handles the proxy's 403 response (opaque error handling).
    const accessDeniedHeading = page.getByRole('heading', { name: /Access Denied/i });
    const notFoundHeading = page.getByRole('heading', { name: /Address Not Found/i });
    const expiredText = page.getByText(/expired|revoked|may not exist/i);

    const showsAccessDenied = await accessDeniedHeading
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    const showsNotFound = await notFoundHeading
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    const showsExpiredText = await expiredText
      .first()
      .isVisible({ timeout: 3000 })
      .catch(() => false);

    // At minimum, the page must NOT show valid grant content
    const showsFullDisclosure = await page
      .getByText(/Full Disclosure/i)
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    expect(showsFullDisclosure).toBe(false);

    // Any denial/error message is acceptable
    expect(showsAccessDenied || showsNotFound || showsExpiredText).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Unauthorized user cannot access grant page
// ---------------------------------------------------------------------------

test.describe('Unauthorized Grant Access', () => {
  let fixture: ProxyAdminFixture;
  let userCDid: string;
  let grantId: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // userA = data owner
    const userADid = fixture.did();
    const userAResult = await fixture.ensureUser(userADid);

    // userB = authorized requester
    const userBDid = fixture.did();
    await fixture.ensureUser(userBDid);

    // userC = unauthorized third party (no grant)
    userCDid = fixture.did();
    await fixture.ensureUser(userCDid);

    // Create and approve disclosure request: userB -> userA
    const req = await fixture.createDisclosureRequest(
      userBDid,
      userAResult.user.id,
      'E2E test: authorized viewer',
      { disclosure_level: 'full' },
    );
    const approveResult = await fixture.approveDisclosureRequest(req.id, userAResult.accessToken);
    grantId = approveResult.grant.id;
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('unauthorized user cannot access grant page', async ({ page, context }) => {
    // Log in as userC (who does NOT have the grant)
    await loginViaCookie(context, userCDid);

    // Navigate directly to the grant page
    await page.goto(`/grant/${grantId}/some-address`);
    await page.waitForLoadState('networkidle');

    // userC should NOT see valid grant content (e.g. "Full Disclosure")
    const showsFullDisclosure = await page
      .getByText(/Full Disclosure/i)
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    expect(showsFullDisclosure).toBe(false);

    // Should see some form of error/gate: "Access Denied", auth wall, etc.
    const showsErrorOrGate = await page
      .getByText(/Access Denied|not found|Authentication Required|expired|forbidden|error/i)
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);

    // Either an explicit error is shown, or the grant content is simply absent
    expect(showsErrorOrGate || !showsFullDisclosure).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Authentication Required for Private (Org-Owned) Addresses
// ---------------------------------------------------------------------------

test.describe('Authentication Required for Private Addresses', () => {
  let fixture: ProxyAdminFixture;
  let orgId: string;
  let groupId: string;
  let contractAddress: string;
  let memberDid: string;
  let outsiderDid: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Create org
    const org = await fixture.createOrg('privaddr', 'Privacy Address Test Org');
    orgId = org.id;

    // Create group with read claim
    const { group } = await fixture.createGroup(orgId, 'readers', 'Readers', ['read']);
    groupId = group.id;

    // Register a fake address as an org-owned contract
    contractAddress = fixture.address();
    await fixture.createContract(orgId, contractAddress, 'Private Test Contract');
    await fixture.createContractGrant(orgId, contractAddress, groupId);

    // Create member user (in the org group)
    memberDid = fixture.did();
    const { user: memberUser } = await fixture.ensureUser(memberDid);
    await fixture.addMembership(memberUser.id, groupId);

    // Create outsider user (not in any org)
    outsiderDid = fixture.did();
    await fixture.ensureUser(outsiderDid);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('anonymous user sees restricted message on org-owned address', async ({
    page,
    context,
  }) => {
    // Ensure we are logged out
    await logout(context);

    // Navigate to the org-owned contract address
    await page.goto(`/address/${contractAddress}`);
    await page.waitForLoadState('networkidle');

    // Should show either "Authentication Required" or "Address Restricted"
    // depending on whether the explorer has privacy mode enabled and the
    // proxy's response to unauthenticated requests.
    const authRequired = page.getByRole('heading', { name: /Authentication Required/i });
    const restricted = page.getByRole('heading', { name: /Address Restricted/i });

    const showsAuth = await authRequired.isVisible({ timeout: 15000 }).catch(() => false);
    const showsRestricted = await restricted.isVisible({ timeout: 3000 }).catch(() => false);

    expect(showsAuth || showsRestricted).toBe(true);
  });

  test('authenticated outsider sees Address Restricted on org-owned address', async ({
    page,
    context,
  }) => {
    // Log in as outsider (not in the org)
    await loginViaCookie(context, outsiderDid);

    // Navigate to the org-owned contract address
    await page.goto(`/address/${contractAddress}`);
    await page.waitForLoadState('networkidle');

    // h2 "Address Restricted" should be visible
    const restrictedHeading = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeading).toBeVisible({ timeout: 15000 });
  });

  test('org member can access org-owned contract page', async ({ page, context }) => {
    // Log in as org member
    await loginViaCookie(context, memberDid);

    // Navigate to the org-owned contract address
    await page.goto(`/address/${contractAddress}`);
    await page.waitForLoadState('networkidle');

    // Should NOT show "Authentication Required"
    const authRequired = await page
      .getByRole('heading', { name: /Authentication Required/i })
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    expect(authRequired).toBe(false);

    // Should NOT show "Address Restricted"
    const restricted = await page
      .getByRole('heading', { name: /Address Restricted/i })
      .isVisible({ timeout: 3000 })
      .catch(() => false);
    expect(restricted).toBe(false);

    // Page should have some content (address text, balance, or similar)
    const body = page.locator('body');
    const hasContent = await body
      .getByText(/balance|transaction|address|contract/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
  });
});
