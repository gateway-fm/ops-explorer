import { test, expect, apiMock, expectLoadedOK, makeTx, makeBlock } from '../../fixtures/api-mock';

// S6 Block, S7 Tx, S8 Address, S9 Contract.

test.describe('S6 Block', () => {
  test('list → click row → BlockDetail; tabs switch; show more', async ({ page }) => {
    await apiMock(page);
    await page.goto('/blocks');
    await expectLoadedOK(page, page.getByTestId('block-row'));

    await page.getByTestId('block-row').first().getByRole('link').first().click();
    await expect(page).toHaveURL(/\/block\/\d+$/);
    await expectLoadedOK(page, page.getByTestId('page-header-title'));

    // Three tabs, Details active by default.
    await expect(page.getByTestId('tab-details')).toHaveAttribute('aria-selected', 'true');
    await page.getByTestId('tab-transactions').click();
    await expect(page.getByTestId('tab-transactions')).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByTestId('tx-row').first()).toBeVisible();

    // Back to Details → Show More expands extra info rows.
    await page.getByTestId('tab-details').click();
    const before = await page.getByTestId('info-row').count();
    await page.getByTestId('show-more-toggle').click();
    expect(await page.getByTestId('info-row').count()).toBeGreaterThan(before);
  });

  test('block prev/next chevrons navigate', async ({ page }) => {
    await apiMock(page);
    await page.goto('/block/500');
    await expectLoadedOK(page, page.getByTestId('page-header-title'));
    // Block Height row has prev/next links to 499 / 501.
    await expect(page.getByRole('link', { name: 'Previous block' })).toHaveAttribute('href', /\/block\/499$/);
    await expect(page.getByRole('link', { name: 'Next block' })).toHaveAttribute('href', /\/block\/501$/);
  });
});

test.describe('S7 Transaction', () => {
  test('overview rows + inline transfers; Logs/Trace tabs only render with data', async ({ page }) => {
    const hash = '0x' + 'a'.repeat(64);
    await apiMock(page, {
      routes: {
        transaction: { body: makeTx(1, { hash, txCategories: ['contract_call'], inputData: 'deadbeef' }) },
        'tx-transfers': {
          body: [{
            id: 1, txHash: hash, logIndex: 0, tokenAddress: '0x' + 'f'.repeat(40),
            from: '0x' + '1'.repeat(40), to: '0x' + '2'.repeat(40), value: '1000000000000000000',
            blockNumber: 1, transferType: 'transfer', tokenType: 'ERC20', isInternal: false,
          }],
        },
        'tx-logs': {
          body: [{
            id: 1, txHash: hash, logIndex: 0, address: '0x' + '3'.repeat(40),
            topic0: '0x' + 'd'.repeat(64), topic1: null, topic2: null, topic3: null,
            data: '0x', blockNumber: 1,
          }],
        },
        'tx-internal': {
          body: [{
            id: 1, txHash: hash, blockNumber: 1, traceAddress: '0', from: '0x' + '1'.repeat(40),
            to: '0x' + '2'.repeat(40), value: '0', callType: 'call',
          }],
        },
      },
    });
    await page.goto(`/tx/${hash}`);
    await expectLoadedOK(page, page.getByTestId('page-header-title'));

    // Overview info rows + a tx-status badge + the inline transfer row.
    await expect(page.getByTestId('tx-status')).toBeVisible();
    await expect(page.getByTestId('info-row').first()).toBeVisible();
    await expect(page.getByTestId('transfer-row').first()).toBeVisible();

    // Tabs render because logs+internal are present. Switch to Logs then Trace.
    await expect(page.getByTestId('tab-overview')).toHaveAttribute('aria-selected', 'true');
    await page.getByTestId('tab-logs').click();
    await expect(page.getByTestId('tab-logs')).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByText('Log Index')).toBeVisible();

    await page.getByTestId('tab-trace').click();
    await expect(page.getByTestId('tab-trace')).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByTestId('trace-row').first()).toBeVisible();
  });

  test('no tabs render when there are no logs and no internal txns', async ({ page }) => {
    const hash = '0x' + 'b'.repeat(64);
    await apiMock(page, { routes: { transaction: { body: makeTx(2, { hash }) } } });
    await page.goto(`/tx/${hash}`);
    await expectLoadedOK(page, page.getByTestId('page-header-title'));
    await expect(page.getByTestId('tab-logs')).toHaveCount(0);
    await expect(page.getByTestId('tab-trace')).toHaveCount(0);
  });
});

test.describe('S8 Address', () => {
  test('EOA hero + tabs URL-driven and deep-linkable', async ({ page }) => {
    const addr = '0x' + '4'.repeat(40);
    await apiMock(page);
    await page.goto(`/address/${addr}`);
    await expectLoadedOK(page, page.getByText(addr));

    // EOA badge present, Contract tab absent for an EOA.
    await expect(page.getByText('EOA')).toBeVisible();
    await expect(page.getByTestId('tab-contract')).toHaveCount(0);

    // Tabs are URL-driven: clicking switches and updates ?tab=.
    await page.getByTestId('tab-transactions').click();
    await expect(page).toHaveURL(/\?tab=transactions/);
    await expect(page.getByTestId('tx-row').first()).toBeVisible();

    // Deep-link directly to the transfers tab.
    await page.goto(`/address/${addr}?tab=transfers`);
    await expect(page.getByTestId('tab-transfers')).toHaveAttribute('aria-selected', 'true');
  });
});

test.describe('S9 Contract', () => {
  const addr = '0x' + '5'.repeat(40);

  function contractMock(hasAbi: boolean) {
    return {
      address: { body: { address: addr, balance: '0', txCount: 1, isContract: true, tokenTransferCount: 0, internalTxCount: 0 } },
      token: { status: 404, body: { error: 'not a token' } },
      contract: {
        body: {
          address: addr, bytecode: '0x6080', creator: '0x' + '1'.repeat(40),
          creationTx: '0x' + 'e'.repeat(64), blockNumber: 10, isVerified: hasAbi,
          contractName: hasAbi ? 'MockContract' : undefined,
          sourceCode: hasAbi ? 'contract Mock {}' : undefined,
          abi: hasAbi ? [{ type: 'function', name: 'totalSupply', inputs: [], outputs: [{ name: '', type: 'uint256' }], stateMutability: 'view' }] : [],
          createdAt: '2024-01-01T00:00:00Z',
        },
      },
    } as const;
  }

  test('Contract overview shows verification/creator; Read/Write disabled without ABI', async ({ page }) => {
    await apiMock(page, { routes: contractMock(false) });
    await page.goto(`/address/${addr}?tab=contract`);
    await expectLoadedOK(page, page.getByTestId('tab-contract'));

    await expect(page.getByTestId('tab-contract')).toHaveAttribute('aria-selected', 'true');
    // Read/Write sub-tabs are disabled when hasAbi=false.
    await expect(page.getByTestId('tab-read')).toBeDisabled();
    await expect(page.getByTestId('tab-write')).toBeDisabled();
  });

  test('with ABI → Read enabled, expand fn → Query; Write → wallet gate', async ({ page }) => {
    await apiMock(page, { routes: contractMock(true) });
    await page.goto(`/address/${addr}?tab=contract`);
    await expectLoadedOK(page, page.getByTestId('verified-badge'));

    await expect(page.getByTestId('tab-read')).toBeEnabled();
    await page.getByTestId('tab-read').click();
    await expect(page.getByTestId('contract-read-fn').first()).toBeVisible();
    await page.getByTestId('contract-read-fn').first().click();
    await expect(page.getByTestId('contract-read-submit')).toBeVisible();

    // Write tab gates on a wallet connect.
    await page.getByTestId('tab-write').click();
    await expect(page.getByTestId('wallet-connect')).toBeVisible();
  });
});
