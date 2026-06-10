import { test, expect, apiMock, expectLoadedOK } from '../../fixtures/api-mock';

// S1 — Home loads; S2–S5 — Search classification + suggestions.

test.describe('S1 Home loads', () => {
  test('4 StatCards resolve, Latest Blocks/Txs each >=1 row, chart not stuck loading', async ({ page }) => {
    await apiMock(page);
    await page.goto('/');

    await expectLoadedOK(page, page.getByTestId('stat-card'));

    // 4 StatCards.
    await expect(page.getByTestId('stat-card')).toHaveCount(4);
    // Latest Blocks + Latest Transactions each rendered rows.
    await expect(page.getByTestId('block-row').first()).toBeVisible();
    await expect(page.getByTestId('tx-row').first()).toBeVisible();
    expect(await page.getByTestId('block-row').count()).toBeGreaterThan(0);
    expect(await page.getByTestId('tx-row').count()).toBeGreaterThan(0);
    // Chart is not stuck on a loading state (no app-loading skeleton anywhere).
    await expect(page.getByTestId('app-loading')).toHaveCount(0);
  });

  test('Home shows empty state when stats/lists are empty (no rows, no crash)', async ({ page }) => {
    await apiMock(page, {
      routes: {
        blocks: { body: { data: [], hasMore: false } },
        transactions: { body: { data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 } },
      },
    });
    await page.goto('/');

    // StatCards still render (stats mock unchanged) and the tree didn't crash.
    await expect(page.getByTestId('stat-card')).toHaveCount(4);
    await expect(page.getByTestId('header-brand')).toBeVisible();
    await expect(page.getByText('No blocks yet')).toBeVisible();
    await expect(page.getByText('No transactions yet')).toBeVisible();
    await expect(page.getByTestId('block-row')).toHaveCount(0);
  });
});

test.describe('S2-S5 Search', () => {
  test('S2 block number → /block/:n (client-side, mock-free nav)', async ({ page }) => {
    await apiMock(page);
    await page.goto('/');
    const input = page.getByTestId('search-input').first();
    await input.fill('12345');
    await input.press('Enter');
    await expect(page).toHaveURL(/\/block\/12345$/);
  });

  test('S3 66-hex → /tx/:hash', async ({ page }) => {
    await apiMock(page);
    await page.goto('/');
    const hash = '0x' + 'a'.repeat(64);
    const input = page.getByTestId('search-input').first();
    await input.fill(hash);
    await input.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/tx/${hash}$`));
  });

  test('S4 42-hex → /address/:addr', async ({ page }) => {
    await apiMock(page);
    await page.goto('/');
    const addr = '0x' + 'b'.repeat(40);
    const input = page.getByTestId('search-input').first();
    await input.fill(addr);
    await input.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/address/${addr}$`));
  });

  test('S5 suggestions dropdown renders from /search/suggestions and navigates on click', async ({ page }) => {
    const block = '777';
    await apiMock(page, {
      routes: {
        'search-suggestions': {
          body: {
            query: 'foo',
            suggestions: [
              { type: 'block', value: block, label: `Block #${block}` },
              { type: 'address', value: '0x' + 'c'.repeat(40), label: 'addr' },
            ],
          },
        },
      },
    });
    await page.goto('/');
    const input = page.getByTestId('search-input').first();
    await input.fill('foo');

    // Suggestions appear (debounced ~150ms; web-first auto-wait handles it).
    await expect(page.getByTestId('search-suggestion').first()).toBeVisible();
    await expect(page.getByTestId('search-suggestion')).toHaveCount(2);

    await page.getByTestId('search-suggestion').first().click();
    await expect(page).toHaveURL(new RegExp(`/block/${block}$`));
  });

  test('S5 keyboard nav highlights a suggestion then Enter navigates to it', async ({ page }) => {
    await apiMock(page, {
      routes: {
        'search-suggestions': {
          body: {
            query: 'foo',
            suggestions: [
              { type: 'block', value: '111', label: 'Block #111' },
              { type: 'block', value: '222', label: 'Block #222' },
            ],
          },
        },
      },
    });
    await page.goto('/');
    const input = page.getByTestId('search-input').first();
    await input.fill('foo');
    await expect(page.getByTestId('search-suggestion').first()).toBeVisible();

    // ArrowDown twice selects the second suggestion (index 1), Enter navigates.
    await input.press('ArrowDown');
    await input.press('ArrowDown');
    await expect(page.getByTestId('search-suggestion').nth(1)).toHaveAttribute('aria-selected', 'true');
    await input.press('Enter');
    await expect(page).toHaveURL(/\/block\/222$/);
  });
});
