import { test, expect } from '@playwright/test';
import { ProxyAdminFixture } from '../../helpers/proxy-admin';
import { loginViaCookie, logout } from '../../helpers/explorer-auth';

// ---------------------------------------------------------------------------
// Cross-Org Isolation — Security Boundary Tests
//
// The MOST IMPORTANT security tests in the suite. Verifies that Org A's
// members cannot access Org B's private contracts, and vice versa. Anonymous
// users and outsiders are also blocked.
//
// No Anvil transactions needed — uses fixture.address() for synthetic
// contract addresses registered via the admin API.
// ---------------------------------------------------------------------------

test.describe('Cross-Org Isolation @security', () => {
  let fixture: ProxyAdminFixture;

  // Org A
  let orgAAddr: string;
  let memberADid: string;

  // Org B
  let orgBAddr: string;
  let memberBDid: string;

  // Outsider (neither org)
  let outsiderDid: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // --- Org A setup ---

    const orgA = await fixture.createOrg('orgalpha', 'Organization Alpha');
    const { group: groupA } = await fixture.createGroup(
      orgA.id,
      'team_a',
      'Team Alpha',
      ['read'],
    );

    orgAAddr = fixture.address();
    await fixture.createContract(orgA.id, orgAAddr, 'Alpha Private Contract');
    await fixture.createContractGrant(orgA.id, orgAAddr, groupA.id);

    memberADid = fixture.did();
    const { user: memberAUser } = await fixture.ensureUser(memberADid);
    await fixture.addMembership(memberAUser.id, groupA.id);

    // --- Org B setup ---

    const orgB = await fixture.createOrg('orgbeta', 'Organization Beta');
    const { group: groupB } = await fixture.createGroup(
      orgB.id,
      'team_b',
      'Team Beta',
      ['read'],
    );

    orgBAddr = fixture.address();
    await fixture.createContract(orgB.id, orgBAddr, 'Beta Private Contract');
    await fixture.createContractGrant(orgB.id, orgBAddr, groupB.id);

    memberBDid = fixture.did();
    const { user: memberBUser } = await fixture.ensureUser(memberBDid);
    await fixture.addMembership(memberBUser.id, groupB.id);

    // --- Outsider (no org membership) ---

    outsiderDid = fixture.did();
    await fixture.ensureUser(outsiderDid);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  // --- Positive access: members can access their own org's contracts ---

  test('Org A member can access Org A contract', async ({ page, context }) => {
    await loginViaCookie(context, memberADid);

    await page.goto(`/address/${orgAAddr}`);
    await page.waitForLoadState('networkidle');

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

    // Page should have meaningful content
    const hasContent = await page
      .getByText(/balance|transaction|address|contract/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
  });

  test('Org B member can access Org B contract', async ({ page, context }) => {
    await loginViaCookie(context, memberBDid);

    await page.goto(`/address/${orgBAddr}`);
    await page.waitForLoadState('networkidle');

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

    const hasContent = await page
      .getByText(/balance|transaction|address|contract/i)
      .first()
      .isVisible({ timeout: 10000 })
      .catch(() => false);
    expect(hasContent).toBe(true);
  });

  // --- Cross-org denial: members CANNOT access the other org's contracts ---

  test('Org A member CANNOT access Org B contract — sees Address Restricted', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, memberADid);

    await page.goto(`/address/${orgBAddr}`);
    await page.waitForLoadState('networkidle');

    const restrictedHeading = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeading).toBeVisible({ timeout: 15000 });
  });

  test('Org B member CANNOT access Org A contract — sees Address Restricted', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, memberBDid);

    await page.goto(`/address/${orgAAddr}`);
    await page.waitForLoadState('networkidle');

    const restrictedHeading = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeading).toBeVisible({ timeout: 15000 });
  });

  // --- Anonymous denial: unauthenticated users see Authentication Required ---

  test('anonymous user sees Authentication Required for Org A contract', async ({
    page,
    context,
  }) => {
    await logout(context);

    await page.goto(`/address/${orgAAddr}`);
    await page.waitForLoadState('networkidle');

    const authRequiredHeading = page.getByRole('heading', {
      name: /Authentication Required/i,
    });
    await expect(authRequiredHeading).toBeVisible({ timeout: 15000 });
  });

  test('anonymous user sees Authentication Required for Org B contract', async ({
    page,
    context,
  }) => {
    await logout(context);

    await page.goto(`/address/${orgBAddr}`);
    await page.waitForLoadState('networkidle');

    const authRequiredHeading = page.getByRole('heading', {
      name: /Authentication Required/i,
    });
    await expect(authRequiredHeading).toBeVisible({ timeout: 15000 });
  });

  // --- Cross-org transaction list redaction ---

  test('Org A contract does not appear unredacted in transaction list for Org B member', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, memberBDid);

    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // Org A's contract address must NEVER appear as plaintext for Org B members.
    // It may appear as [PRIVATE] or not at all, but never the real value.
    const bodyText = await page.locator('body').textContent();
    const orgAFragment = orgAAddr.toLowerCase().slice(2, 10);
    const orgAExposed = bodyText?.toLowerCase().includes(orgAFragment);

    expect(orgAExposed).toBe(false);
  });

  // --- Outsider denial: authenticated but no org membership ---

  test('outsider sees Address Restricted for both orgs contracts', async ({
    page,
    context,
  }) => {
    await loginViaCookie(context, outsiderDid);

    // Check Org A contract
    await page.goto(`/address/${orgAAddr}`);
    await page.waitForLoadState('networkidle');

    const restrictedA = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedA).toBeVisible({ timeout: 15000 });

    // Check Org B contract
    await page.goto(`/address/${orgBAddr}`);
    await page.waitForLoadState('networkidle');

    const restrictedB = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedB).toBeVisible({ timeout: 15000 });
  });
});
