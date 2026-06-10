import { test, expect, apiMock, expectLoadedOK, makeBlock, makeTx } from '../../fixtures/api-mock';

// S13 Pagination, S14 Restricted, S15 Login entry.

test.describe('S13 Pagination', () => {
  test('offset pager (Transactions): Page X of Y, Prev disabled at bound, Next advances', async ({ page }) => {
    // total=120, pageSize=25 → totalPages=5.
    await apiMock(page, {
      routes: {
        transactions: (_route, url) => {
          const pageNum = parseInt(url.searchParams.get('page') || '1', 10);
          const pageSize = 25;
          const total = 120;
          const data = Array.from({ length: pageSize }, (_, i) => makeTx(i + 1 + (pageNum - 1) * pageSize));
          return { data, total, page: pageNum, pageSize, totalPages: Math.ceil(total / pageSize) };
        },
      },
    });
    await page.goto('/transactions');
    await expectLoadedOK(page, page.getByTestId('tx-row'));

    await expect(page.getByTestId('pagination-status')).toHaveText('Page 1 of 5');
    // Prev disabled on page 1.
    await expect(page.getByTestId('pagination-prev')).toBeDisabled();

    await page.getByTestId('pagination-next').click();
    await expect(page).toHaveURL(/page=2/);
    await expect(page.getByTestId('pagination-status')).toHaveText('Page 2 of 5');
    await expect(page.getByTestId('pagination-prev')).toBeEnabled();
  });

  test('cursor pager (Blocks): Load more advances via ?before=', async ({ page }) => {
    await apiMock(page, {
      routes: {
        blocks: (_route, url) => {
          const before = url.searchParams.get('before');
          const top = before ? parseInt(before, 10) - 1 : 1000;
          const data = Array.from({ length: 10 }, (_, i) => makeBlock(top - i));
          return { data, hasMore: true };
        },
      },
    });
    await page.goto('/blocks');
    await expectLoadedOK(page, page.getByTestId('block-row'));
    await expect(page.getByTestId('block-row')).toHaveCount(10);

    await page.getByTestId('load-more').click();
    await expect(page).toHaveURL(/before=/);
    await expect(page.getByTestId('block-row').first()).toBeVisible();
  });

  test('"N new items" banner appears when the live poll sees newer txns', async ({ page }) => {
    // First page request returns a static top hash; the live poll (page 1) is
    // mocked to return 2 newer entries above the snapshot, surfacing the notice.
    let call = 0;
    await apiMock(page, {
      routes: {
        transactions: (_route, url) => {
          call += 1;
          const pageSize = 25;
          // The page-1 live poll shares the endpoint; after the first main load,
          // prepend 2 newer txns so the live window finds 2 above the snapshot.
          const offset = call <= 1 ? 0 : 2;
          const data = Array.from({ length: pageSize }, (_, i) => makeTx(i + 1 + offset));
          return { data, total: 100, page: parseInt(url.searchParams.get('page') || '1', 10), pageSize, totalPages: 4 };
        },
      },
    });
    await page.goto('/transactions');
    await expectLoadedOK(page, page.getByTestId('tx-row'));
    // The live poll (3s interval) re-hits the endpoint; the notice surfaces.
    await expect(page.getByTestId('new-items-notice')).toBeVisible({ timeout: 8000 });
  });
});

test.describe('S14 Restricted (logged-out private resource)', () => {
  test('transaction 403 → "Transaction Restricted"', async ({ page }) => {
    await apiMock(page, { routes: { transaction: { status: 403, body: { error: 'forbidden' } } } });
    await page.goto('/tx/0x' + 'a'.repeat(64));
    await expect(page.getByTestId('restricted-state')).toBeVisible();
    await expect(page.getByText('Transaction Restricted')).toBeVisible();
  });

  test('address 403 → "Address restricted"', async ({ page }) => {
    await apiMock(page, { routes: { address: { status: 403, body: { error: 'forbidden' } } } });
    await page.goto('/address/0x' + 'b'.repeat(40));
    await expect(page.getByTestId('restricted-state')).toBeVisible();
    await expect(page.getByText('Address restricted')).toBeVisible();
  });

  test('token 403 → "Token Restricted"', async ({ page }) => {
    await apiMock(page, { routes: { token: { status: 403, body: { error: 'forbidden' } } } });
    await page.goto('/token/0x' + 'c'.repeat(40));
    await expect(page.getByTestId('restricted-state')).toBeVisible();
    await expect(page.getByText('Token Restricted')).toBeVisible();
  });

  test('address 500 also yields the restricted interstitial (not a generic error)', async ({ page }) => {
    await apiMock(page, { routes: { address: { status: 500, body: { error: 'boom' } } } });
    await page.goto('/address/0x' + 'd'.repeat(40));
    await expect(page.getByTestId('restricted-state')).toBeVisible();
  });
});

test.describe('S15 Login entry', () => {
  test('privacy on + unauthenticated → /privacy "Sign In" points at auth/login?return_url', async ({ page }) => {
    await apiMock(page, { authenticated: false, privacyEnabled: true });
    await page.goto('/privacy');

    // Two sign-in-buttons exist (nav AuthButton + the dashboard prompt); use
    // the dashboard one in <main>.
    const signIn = page.getByRole('main').getByTestId('sign-in-button');
    await expect(signIn).toBeVisible();

    // Intercept the top-level navigation that redirectToLogin triggers; assert
    // the target without actually following it off-app.
    await page.route('**/api/auth/login**', (route) =>
      route.fulfill({ status: 200, contentType: 'text/html', body: 'login' }),
    );
    const [request] = await Promise.all([
      page.waitForRequest('**/api/auth/login**'),
      signIn.click(),
    ]);
    expect(request.url()).toContain('/api/auth/login?return_url=');
    expect(decodeURIComponent(request.url())).toContain('/privacy');
  });
});
