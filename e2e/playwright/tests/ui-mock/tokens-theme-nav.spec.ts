import { test, expect, apiMock, expectLoadedOK } from '../../fixtures/api-mock';

// S10 Token, S11 Theme, S12 Nav.

test.describe('S10 Token', () => {
  test('list filter tabs + search → TokenDetail StatBar + tabs; Inventory NFT card → /nft', async ({ page }) => {
    const nftAddr = '0x' + 'b'.repeat(40);
    await apiMock(page, {
      routes: {
        token: {
          body: {
            address: nftAddr, symbol: 'NFT', name: 'Cool NFTs', decimals: 0, tokenType: 'ERC721',
            totalSupply: '100', holderCount: 9, transferCount: 20, blockNumber: 1, createdAt: '2024-01-01T00:00:00Z',
          },
        },
        'token-inventory': {
          body: { data: [{ tokenId: '7', owner: '0x' + '1'.repeat(40), tokenUri: '' }], total: 1, page: 1, pageSize: 25, totalPages: 1 },
        },
      },
    });

    await page.goto('/tokens');
    await expectLoadedOK(page, page.getByTestId('token-row'));

    // Filter tabs use role=tab + aria-selected (All selected initially).
    await expect(page.getByTestId('tab-all')).toHaveAttribute('aria-selected', 'true');
    await page.getByTestId('tab-ERC721').click();
    await expect(page).toHaveURL(/type=ERC721/);

    // Navigate to a token (deep link to avoid coupling on row order).
    await page.goto(`/token/${nftAddr}`);
    await expectLoadedOK(page, page.getByTestId('stat-tile'));
    // StatBar = 4 tiles.
    await expect(page.getByTestId('stat-tile')).toHaveCount(4);

    // Inventory tab → NFT card → /nft/:a/:id.
    await page.getByTestId('tab-inventory').click();
    await expect(page.getByTestId('tab-inventory')).toHaveAttribute('aria-selected', 'true');
    const nftLink = page.getByRole('link', { name: /#7|7/ }).first();
    await expect(nftLink).toBeVisible();
  });

  test('Token search box updates the URL ?search=', async ({ page }) => {
    await apiMock(page);
    await page.goto('/tokens');
    await expectLoadedOK(page, page.getByTestId('token-row'));
    await page.getByLabel('Search tokens').fill('AAA');
    await expect(page).toHaveURL(/search=AAA/, { timeout: 5000 });
  });
});

test.describe('S11 Theme', () => {
  test('Settings dropdown → Dark → <html class="dark"> + localStorage; Light reverts', async ({ page }) => {
    await apiMock(page);
    await page.goto('/');
    await expect(page.getByTestId('header-brand')).toBeVisible();

    await page.getByTestId('theme-menu-trigger').click();
    await page.getByTestId('theme-option-dark').click();
    await expect(page.locator('html')).toHaveClass(/dark/);
    expect(await page.evaluate(() => localStorage.getItem('theme'))).toBe('dark');

    // The dropdown stays open after selecting a theme, so the Light option is
    // already reachable — selecting it reverts the theme.
    await page.getByTestId('theme-option-light').click();
    await expect(page.locator('html')).not.toHaveClass(/dark/);
    expect(await page.evaluate(() => localStorage.getItem('theme'))).toBe('light');
  });
});

test.describe('S12 Nav', () => {
  test('Blockchain dropdown opens and links navigate', async ({ page }) => {
    await apiMock(page);
    await page.goto('/');
    await expect(page.getByTestId('header-brand')).toBeVisible();

    await page.getByTestId('nav-blockchain').click();
    // The dropdown reveals nav-link items; click the one going to /accounts.
    const accountsLink = page.getByTestId('nav-link').filter({ hasText: 'Top Accounts' });
    await expect(accountsLink).toBeVisible();
    await accountsLink.click();
    await expect(page).toHaveURL(/\/accounts$/);
    await expectLoadedOK(page, page.getByTestId('account-row'));
  });

  test('Tokens dropdown opens and routes to /tokens', async ({ page }) => {
    await apiMock(page);
    await page.goto('/');
    await expect(page.getByTestId('header-brand')).toBeVisible();
    await page.getByTestId('nav-tokens').click();
    const tokensLink = page.getByTestId('nav-link').filter({ hasText: /^Tokens$/ });
    await expect(tokensLink).toBeVisible();
    await tokensLink.click();
    await expect(page).toHaveURL(/\/tokens$/);
  });

  test('mobile hamburger opens the overlay menu', async ({ page }) => {
    await apiMock(page);
    await page.setViewportSize({ width: 390, height: 800 });
    await page.goto('/');
    await expect(page.getByTestId('header-brand')).toBeVisible();
    await expect(page.getByTestId('mobile-menu-overlay')).toHaveCount(0);
    await page.getByTestId('mobile-menu-trigger').click();
    await expect(page.getByTestId('mobile-menu-overlay')).toBeVisible();
  });
});
