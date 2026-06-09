import { test, expect, apiMock, expectLoadedOK } from '../../fixtures/api-mock';

test('harness smoke — Home renders against the mock with no unmocked /api 404s', async ({ page }) => {
  // A *missing* mock fails loudly via the catalog's sentinel body; deliberate
  // 404s (e.g. an EOA's /contract → "not a contract") are valid mocks and must
  // not be flagged. So we only fail on the sentinel.
  const missing: string[] = [];
  page.on('response', async (res) => {
    if (res.url().includes('/api/') && res.status() === 404) {
      const body = await res.text().catch(() => '');
      if (body.includes('[apiMock] no mock')) missing.push(res.url());
    }
  });

  await apiMock(page);
  await page.goto('/');

  await expectLoadedOK(page, page.getByTestId('stat-card'));

  expect(missing, `unmocked /api routes:\n${missing.join('\n')}`).toEqual([]);
});
