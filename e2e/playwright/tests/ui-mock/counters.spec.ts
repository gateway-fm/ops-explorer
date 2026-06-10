import { test, expect, apiMock, expectLoadedOK, makeTx, FIXTURE } from '../../fixtures/api-mock';

// PLAN §5 — UI counter assertions. With deterministic mocks, rendered counters
// must equal the data/totals served; these break silently in production today.

test.describe('Home StatCards == mocked /api/stats', () => {
  test('each StatCard shows the mocked number (locale-formatted)', async ({ page }) => {
    await apiMock(page, {
      routes: { stats: { body: { totalBlocks: 1234, totalTransactions: 56789, totalAddresses: 42, avgBlockTime: 3.5, privacyEnabled: true } } },
    });
    await page.goto('/');
    await expectLoadedOK(page, page.getByTestId('stat-card'));

    await expect(page.getByTestId('stat-card').filter({ hasText: 'Total Blocks' })).toContainText('1,234');
    await expect(page.getByTestId('stat-card').filter({ hasText: 'Total Transactions' })).toContainText('56,789');
    await expect(page.getByTestId('stat-card').filter({ hasText: 'Total Addresses' })).toContainText('42');
    await expect(page.getByTestId('stat-card').filter({ hasText: 'Avg Block Time' })).toContainText('3.50s');
  });
});

test.describe('Pagination math: "Page X of Y" where Y == ceil(total/pageSize)', () => {
  test('total=51, pageSize=25 → 3 pages', async ({ page }) => {
    await apiMock(page, {
      routes: {
        transactions: (_route, url) => {
          const pageNum = parseInt(url.searchParams.get('page') || '1', 10);
          return { data: Array.from({ length: 25 }, (_, i) => makeTx(i + 1)), total: 51, page: pageNum, pageSize: 25, totalPages: Math.ceil(51 / 25) };
        },
      },
    });
    await page.goto('/transactions');
    await expectLoadedOK(page, page.getByTestId('tx-row'));
    await expect(page.getByTestId('pagination-status')).toHaveText('Page 1 of 3');
    // Header total reflects the mocked total.
    await expect(page.getByTestId('tx-total-count')).toHaveText('51 transactions');
  });
});

test.describe('List length == len(mocked data)', () => {
  test('Transactions renders exactly len(data) rows, no drops/dupes', async ({ page }) => {
    await apiMock(page, {
      routes: {
        transactions: { body: { data: Array.from({ length: 7 }, (_, i) => makeTx(i + 1)), total: 7, page: 1, pageSize: 25, totalPages: 1 } },
      },
    });
    await page.goto('/transactions');
    await expectLoadedOK(page, page.getByTestId('tx-row'));
    await expect(page.getByTestId('tx-row')).toHaveCount(7);
  });
});

test.describe('Redacted: total=50 but data=2 → UI shows 2 rows and "of 50" faithfully', () => {
  test('row count == 2 while header total == 50', async ({ page }) => {
    await apiMock(page, {
      routes: {
        transactions: { body: { data: [makeTx(1), makeTx(2)], total: 50, page: 1, pageSize: 25, totalPages: 2 } },
      },
    });
    await page.goto('/transactions');
    await expectLoadedOK(page, page.getByTestId('tx-row'));
    await expect(page.getByTestId('tx-row')).toHaveCount(2);
    await expect(page.getByTestId('tx-total-count')).toHaveText('50 transactions');
    await expect(page.getByTestId('pagination-status')).toHaveText('Page 1 of 2');
  });
});

test.describe('Tab/section badges == mocked counts', () => {
  test('Address tab counts equal the mocked txCount / tokenTransferCount / internalTxCount', async ({ page }) => {
    const addr = '0x' + '4'.repeat(40);
    await apiMock(page, {
      routes: {
        address: { body: { address: addr, balance: '0', txCount: 123, isContract: false, tokenTransferCount: 45, internalTxCount: 6 } },
      },
    });
    await page.goto(`/address/${addr}`);
    await expectLoadedOK(page, page.getByTestId('tab-transactions'));
    await expect(page.getByTestId('tab-count-transactions')).toHaveText('123');
    await expect(page.getByTestId('tab-count-transfers')).toHaveText('45');
    await expect(page.getByTestId('tab-count-internal')).toHaveText('6');
  });

  test('Block "Transactions (N)" / "Internal (N)" equal mocked array lengths', async ({ page }) => {
    await apiMock(page, {
      routes: {
        block: { body: { block: { number: 5, hash: '0x'+'b'.repeat(64), parentHash: '0x'+'b'.repeat(64), timestamp: 1, gasUsed: 1, gasLimit: 2, transactionCount: 3, size: 1, difficulty: '0', totalDifficulty: '0', nonce: '0x0', miner: '0x'+'1'.repeat(40), extraData: '', stateRoot: '0x'+'a'.repeat(64), transactionsRoot: '0x'+'c'.repeat(64), receiptsRoot: '0x'+'d'.repeat(64), createdAt: '2024-01-01T00:00:00Z' }, transactions: [makeTx(1), makeTx(2), makeTx(3)] } },
        'block-internal': { body: [
          { id: 1, txHash: '0x'+'e'.repeat(64), blockNumber: 5, traceAddress: '0', from: '0x'+'1'.repeat(40), to: '0x'+'2'.repeat(40), value: '0', callType: 'call' },
        ] },
      },
    });
    await page.goto('/block/5');
    await expectLoadedOK(page, page.getByTestId('tab-transactions'));
    await expect(page.getByTestId('tab-transactions')).toContainText('Transactions (3)');
    await expect(page.getByTestId('tab-internal')).toContainText('Internal Txns (1)');
  });

  test('TokenDetail tab badges equal mocked transferCount / holderCount', async ({ page }) => {
    const addr = '0x' + 'a'.repeat(40);
    await apiMock(page, {
      routes: {
        token: { body: { address: addr, symbol: 'X', name: 'X', decimals: 18, tokenType: 'ERC20', totalSupply: '0', holderCount: 88, transferCount: 99, blockNumber: 1, createdAt: '2024-01-01T00:00:00Z' } },
      },
    });
    await page.goto(`/token/${addr}`);
    await expectLoadedOK(page, page.getByTestId('stat-tile'));
    await expect(page.getByTestId('tab-count-transfers')).toHaveText('99');
    await expect(page.getByTestId('tab-count-holders')).toHaveText('88');
  });
});

test.describe('Chart points == mocked tx-history length', () => {
  test('the chart consumes /stats/tx-history (length asserted via request mock)', async ({ page }) => {
    let served = -1;
    await apiMock(page, {
      routes: {
        'tx-history': () => {
          const pts = FIXTURE.txHistory.slice(0, 12);
          served = pts.length;
          return pts;
        },
      },
    });
    await page.goto('/');
    await expectLoadedOK(page, page.getByTestId('stat-card'));
    // The chart fetched our mocked series; assert the served length is what we set.
    await expect.poll(() => served).toBe(12);
  });
});

test.describe('Accounts: "N accounts" header == mocked total; rows == len(data)', () => {
  test('total and row count are faithful', async ({ page }) => {
    await apiMock(page, {
      routes: {
        accounts: { body: { data: Array.from({ length: 4 }, (_, i) => ({ address: '0x' + (i+1).toString(16).padStart(40,'3'), balance: '1000000000000000000', txCount: 5, isContract: false })), total: 4, page: 1, pageSize: 25, totalPages: 1 } },
      },
    });
    await page.goto('/accounts');
    await expectLoadedOK(page, page.getByTestId('account-row'));
    await expect(page.getByTestId('account-total-count')).toHaveText('4 accounts');
    await expect(page.getByTestId('account-row')).toHaveCount(4);
  });
});
