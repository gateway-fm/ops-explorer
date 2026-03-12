import { test, expect } from '@playwright/test';
import { ProxyAdminFixture } from '../../helpers/proxy-admin';
import { loginViaCookie } from '../../helpers/explorer-auth';

// ---------------------------------------------------------------------------
// Same-Org Shared Access
//
// Verifies that multiple members within the same org can all access the
// same org-owned contract, while outsiders are blocked. Also tests that
// contracts granted to a different group within the same org are NOT
// accessible to members of the first group.
//
// No Anvil transactions needed — uses fixture.address() for synthetic
// contract addresses.
// ---------------------------------------------------------------------------

test.describe('Same-Org Shared Access', () => {
  let fixture: ProxyAdminFixture;

  let memberOneDid: string;
  let memberTwoDid: string;
  let outsiderDid: string;

  let sharedContractAddr: string;
  let restrictedContractAddr: string;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Create one org
    const org = await fixture.createOrg('sharedorg', 'Shared Access Test Org');

    // Create the primary group that both members belong to
    const { group: sharedGroup } = await fixture.createGroup(
      org.id,
      'shared_team',
      'Shared Team',
      ['read'],
    );

    // Register a contract and grant it to the shared group
    sharedContractAddr = fixture.address();
    await fixture.createContract(org.id, sharedContractAddr, 'Shared Org Contract');
    await fixture.createContractGrant(org.id, sharedContractAddr, sharedGroup.id);

    // Create a SECOND group that neither member belongs to
    const { group: restrictedGroup } = await fixture.createGroup(
      org.id,
      'restricted_team',
      'Restricted Team',
      ['read'],
    );

    // Register a second contract granted ONLY to the restricted group
    restrictedContractAddr = fixture.address();
    await fixture.createContract(org.id, restrictedContractAddr, 'Restricted Org Contract');
    await fixture.createContractGrant(org.id, restrictedContractAddr, restrictedGroup.id);

    // Create member one — in the shared group
    memberOneDid = fixture.did();
    const { user: memberOneUser } = await fixture.ensureUser(memberOneDid);
    await fixture.addMembership(memberOneUser.id, sharedGroup.id);

    // Create member two — also in the shared group
    memberTwoDid = fixture.did();
    const { user: memberTwoUser } = await fixture.ensureUser(memberTwoDid);
    await fixture.addMembership(memberTwoUser.id, sharedGroup.id);

    // Create outsider — no group membership at all
    outsiderDid = fixture.did();
    await fixture.ensureUser(outsiderDid);
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('first member can access shared org contract', async ({ page, context }) => {
    await loginViaCookie(context, memberOneDid);

    await page.goto(`/address/${sharedContractAddr}`);
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

  test('second member can also access shared org contract', async ({ page, context }) => {
    await loginViaCookie(context, memberTwoDid);

    await page.goto(`/address/${sharedContractAddr}`);
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

  test('outsider cannot access shared org contract', async ({ page, context }) => {
    await loginViaCookie(context, outsiderDid);

    await page.goto(`/address/${sharedContractAddr}`);
    await page.waitForLoadState('networkidle');

    const restrictedHeading = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeading).toBeVisible({ timeout: 15000 });
  });

  test('contract not in the members group is not accessible to org members', async ({
    page,
    context,
  }) => {
    // Member one is in shared_team but NOT in restricted_team.
    // The restricted contract is granted only to restricted_team.
    await loginViaCookie(context, memberOneDid);

    await page.goto(`/address/${restrictedContractAddr}`);
    await page.waitForLoadState('networkidle');

    const restrictedHeading = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeading).toBeVisible({ timeout: 15000 });

    // Verify member two also cannot access (same group, same restriction)
    await loginViaCookie(context, memberTwoDid);

    await page.goto(`/address/${restrictedContractAddr}`);
    await page.waitForLoadState('networkidle');

    const restrictedHeadingTwo = page.getByRole('heading', { name: /Address Restricted/i });
    await expect(restrictedHeadingTwo).toBeVisible({ timeout: 15000 });
  });
});
