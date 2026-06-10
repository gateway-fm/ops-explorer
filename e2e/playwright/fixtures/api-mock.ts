import { test as base, expect, type Page, type Route } from '@playwright/test';

/**
 * apiMock — a typed, deterministic browser→/api mock for the `ui-mock`
 * Playwright project.
 *
 * WHY: the new basic-UI suite must run with NO backend (decision in
 * PLAN_UI_E2E_TESTS.md §1). The browser hits same-origin `/api`
 * (frontend/src/lib/api.ts), so `page.route('**​/api/**', …)` intercepts
 * everything. This fixture serves response shapes derived from the TS types
 * the frontend decodes into (frontend/src/lib/api.ts) so a contract drift
 * surfaces as a failing render, not a silently-wrong mock.
 *
 * INVARIANTS:
 *  - Every catalog route returns shape-accurate JSON.
 *  - Any UNMOCKED `/api/**` route fulfils 404 — a missing mock fails loudly
 *    rather than hanging on `networkidle` or silently rendering empty.
 *  - External origins (4byte, ipfs, NFT hosts, featured-networks.json) are
 *    stubbed so a test never touches the network.
 *  - `window.__runtimeConfig` + a fake `window.ethereum` are injected before
 *    any app code runs, so privacy flag / brand / wallet flows are
 *    deterministic.
 *
 * Auth needs no crypto: the BFF doesn't verify JWT signatures, so we set an
 * unsigned `explorer_auth` cookie (header.<b64url-json>.sig, future exp) AND
 * mock GET /api/auth/status. Tests flip auth via `apiMock(page, { authenticated })`.
 */

// ----- Response types (mirror frontend/src/lib/api.ts) -----

export interface MockBlock {
  number: number;
  hash: string;
  parentHash: string;
  timestamp: number;
  gasUsed: number;
  gasLimit: number;
  baseFeePerGas?: number;
  transactionCount: number;
  size: number;
  difficulty: string;
  totalDifficulty: string;
  nonce: string;
  miner: string;
  extraData: string;
  stateRoot: string;
  transactionsRoot: string;
  receiptsRoot: string;
  createdAt: string;
}

export type TxCategory =
  | 'coin_transfer'
  | 'contract_call'
  | 'contract_creation'
  | 'token_transfer'
  | 'system_transaction';

export type VisibilityReason =
  | 'own_address'
  | 'disclosure_grant'
  | 'rbac_group_member'
  | 'public_address'
  | 'no_access'
  | 'participant_override';

export interface MockTransaction {
  hash: string;
  blockNumber: number;
  blockTimestamp?: number;
  txIndex: number;
  from: string;
  to: string | null;
  contractAddress?: string | null;
  value: string | number;
  gasUsed: number;
  gasPrice: number;
  inputData: string;
  status: number;
  createdAt: string;
  nonce?: number;
  txType?: number;
  gasLimit?: number;
  txCategories?: TxCategory[];
  tokenTransferCount?: number;
  addressMetadata?: Record<string, VisibilityReason>;
}

interface MockOverrides {
  /** Drive auth state. Sets cookie + /api/auth/status. Default: false. */
  authenticated?: boolean;
  did?: string;
  /** Privacy flag surfaced via /api/stats.privacyEnabled. Default: true. */
  privacyEnabled?: boolean;
  /** window.__runtimeConfig overrides (brand, currency, privacy build flag). */
  runtimeConfig?: Record<string, string>;
  /**
   * Per-route JSON/handler overrides keyed by a stable catalog key (e.g.
   * 'stats', 'block', 'transaction'). A function receives the matched Route
   * and the parsed URL so a test can branch on path segments / query.
   */
  routes?: Partial<Record<string, RouteValue>>;
}

type RouteValue =
  | { status?: number; body?: unknown; contentType?: string }
  | ((route: Route, url: URL) => unknown | Promise<unknown>);

// ----- Deterministic factory builders -----

const ZERO_HASH = '0x' + '0'.repeat(64);

export function makeBlock(n: number, over: Partial<MockBlock> = {}): MockBlock {
  return {
    number: n,
    hash: '0x' + n.toString(16).padStart(64, 'b'),
    parentHash: '0x' + (n - 1).toString(16).padStart(64, 'b'),
    timestamp: 1_700_000_000 + n * 12,
    gasUsed: 12_000_000,
    gasLimit: 30_000_000,
    baseFeePerGas: 7,
    transactionCount: 2,
    size: 1234,
    difficulty: '0',
    totalDifficulty: '0',
    nonce: '0x0000000000000000',
    miner: '0x1111111111111111111111111111111111111111',
    extraData: '',
    stateRoot: '0x' + 'a'.repeat(64),
    transactionsRoot: '0x' + 'c'.repeat(64),
    receiptsRoot: '0x' + 'd'.repeat(64),
    createdAt: '2024-01-01T00:00:00Z',
    ...over,
  };
}

export function makeTx(i: number, over: Partial<MockTransaction> = {}): MockTransaction {
  const hash = '0x' + i.toString(16).padStart(64, 'e');
  return {
    hash,
    blockNumber: 1000 - i,
    blockTimestamp: 1_700_000_000 + i,
    txIndex: 0,
    from: '0x' + i.toString(16).padStart(40, '1'),
    to: '0x' + i.toString(16).padStart(40, '2'),
    value: '1000000000000000000',
    gasUsed: 21000,
    gasPrice: 20_000_000_000,
    inputData: '',
    status: 1,
    createdAt: '2024-01-01T00:00:00Z',
    nonce: i,
    txType: 2,
    gasLimit: 21000,
    txCategories: ['coin_transfer'],
    tokenTransferCount: 0,
    ...over,
  };
}

export const FIXTURE = {
  stats: {
    totalBlocks: 1_234_567,
    totalTransactions: 7_654_321,
    totalAddresses: 42_000,
    avgBlockTime: 12.5,
    privacyEnabled: true,
  },
  txHistory: Array.from({ length: 24 }, (_, i) => ({
    timestamp: 1_700_000_000 + i * 3600,
    count: 100 + i,
  })),
  gas: {
    slow: { price: 1.2, priceWei: '1200000000', baseFee: 1, priorityFee: 0.2 },
    normal: { price: 2.5, priceWei: '2500000000', baseFee: 2, priorityFee: 0.5 },
    fast: { price: 5.0, priceWei: '5000000000', baseFee: 4, priorityFee: 1 },
    updatedAt: '2024-01-01T00:00:00Z',
  },
  chainInfo: {
    chainId: '0x3e9',
    chainIdDecimal: 1001,
    networkId: '1001',
    clientVersion: 'mock/v1.0.0',
    protocolVersion: '0x41',
    latestBlock: 1000,
    gasPrice: '2500000000',
    peerCount: 3,
    isSyncing: false,
    genesisHash: ZERO_HASH,
    updatedAt: '2024-01-01T00:00:00Z',
  },
};

// ----- Cookie helper (unsigned explorer_auth) -----

function b64url(obj: unknown): string {
  return Buffer.from(JSON.stringify(obj))
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}

/** Build an unsigned `header.<payload>.sig` JWT-shaped token with future exp. */
export function makeAuthToken(did: string): string {
  const header = b64url({ alg: 'none', typ: 'JWT' });
  const payload = b64url({ did, exp: Math.floor(Date.now() / 1000) + 3600 });
  return `${header}.${payload}.sig`;
}

// ----- The catalog -----

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}

/**
 * Apply the mock to a page. Returns nothing; call before `page.goto`.
 *
 * Routing strategy: a single `**​/api/**` handler classifies by pathname so
 * unmocked endpoints reliably 404. Overrides are consulted first by catalog
 * key, then the built-in catalog responds, else 404.
 */
export async function apiMock(page: Page, over: MockOverrides = {}): Promise<void> {
  const authenticated = over.authenticated ?? false;
  const did = over.did ?? 'did:privado:e2e_user';
  const privacyEnabled = over.privacyEnabled ?? true;
  const routes = over.routes ?? {};

  // 1) Inject runtime config + fake wallet BEFORE app code runs.
  const runtimeConfig: Record<string, string> = {
    VITE_BRAND_NAME: 'Gateway Explorer',
    VITE_NETWORK_CURRENCY: 'ETH',
    VITE_PRIVACY_MODE: privacyEnabled ? 'true' : 'false',
    ...over.runtimeConfig,
  };
  await page.addInitScript((cfg) => {
    (window as unknown as { __runtimeConfig: Record<string, string> }).__runtimeConfig = cfg;
    // Minimal EIP-1193 stub so wallet/write flows are deterministic and never
    // pop a real extension. accountsChanged/chainChanged are no-ops.
    const eth = {
      isMetaMask: true,
      _accounts: [] as string[],
      request: async ({ method }: { method: string; params?: unknown[] }) => {
        switch (method) {
          case 'eth_chainId':
            return '0x3e9';
          case 'eth_accounts':
            return eth._accounts;
          case 'eth_requestAccounts':
            eth._accounts = ['0xabc0000000000000000000000000000000000abc'];
            return eth._accounts;
          case 'wallet_revokePermissions':
            eth._accounts = [];
            return null;
          default:
            return null;
        }
      },
      on: () => {},
      removeListener: () => {},
    };
    (window as unknown as { ethereum: typeof eth }).ethereum = eth;
  }, runtimeConfig);

  // 2) Auth cookie (unsigned). Set for both localhost + 127.0.0.1 hosts.
  if (authenticated) {
    const token = makeAuthToken(did);
    await page.context().addCookies([
      { name: 'explorer_auth', value: token, domain: 'localhost', path: '/' },
      { name: 'explorer_auth', value: token, domain: '127.0.0.1', path: '/' },
    ]);
  }

  // 3) Stub external origins (must NOT hit the network).
  await page.route('**/featured-networks.json', (route) => json(route, []));
  await page.route('https://www.4byte.directory/**', (route) =>
    json(route, { count: 0, results: [] }),
  );
  await page.route('https://ipfs.io/**', (route) =>
    route.fulfill({ status: 200, contentType: 'image/png', body: '' }),
  );
  await page.route('https://**/*.png', (route) =>
    route.fulfill({ status: 200, contentType: 'image/png', body: '' }),
  );

  // 4) The single /api classifier.
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace(/.*\/api/, ''); // strip any base prefix

    // Helper: resolve a catalog override if present.
    const tryOverride = async (key: string): Promise<boolean> => {
      const v = routes[key];
      if (v === undefined) return false;
      if (typeof v === 'function') {
        const body = await v(route, url);
        await json(route, body);
      } else {
        await route.fulfill({
          status: v.status ?? 200,
          contentType: v.contentType ?? 'application/json',
          body: typeof v.body === 'string' ? v.body : JSON.stringify(v.body ?? {}),
        });
      }
      return true;
    };

    // --- auth ---
    if (path === '/auth/status') {
      if (await tryOverride('auth/status')) return;
      return json(route, authenticated ? { authenticated: true, did } : { authenticated: false });
    }
    if (path === '/auth/logout') {
      return json(route, { logged_out: true });
    }

    // --- stats / charts / chain ---
    if (path === '/stats') {
      if (await tryOverride('stats')) return;
      return json(route, { ...FIXTURE.stats, privacyEnabled });
    }
    if (path === '/stats/tx-history') {
      if (await tryOverride('tx-history')) return;
      return json(route, FIXTURE.txHistory);
    }
    if (path === '/gas') {
      if (await tryOverride('gas')) return;
      return json(route, FIXTURE.gas);
    }
    if (path === '/chain-info') return json(route, FIXTURE.chainInfo);
    if (path === '/price') return json(route, { price: 0, currency: 'USD', change24h: 0, lastUpdated: '2024-01-01T00:00:00Z' });
    if (path === '/sync') {
      return json(route, {
        syncStatus: { id: 1, lastIndexedBlock: 1000, isSyncing: false, updatedAt: '2024-01-01T00:00:00Z' },
        latestChainBlock: 1000, blocksRemaining: 0, isSynced: true,
      });
    }
    if (path.startsWith('/charts/')) return json(route, path === '/charts/counters' || path === '/charts/lines' ? [] : { info: {}, chart: [] });

    // --- blocks ---
    if (path === '/blocks/latest') {
      if (await tryOverride('block-latest')) return;
      return json(route, makeBlock(1000));
    }
    const blockInternal = path.match(/^\/blocks\/(\d+)\/internal$/);
    if (blockInternal) {
      if (await tryOverride('block-internal')) return;
      return json(route, []);
    }
    const blockDetail = path.match(/^\/blocks\/(\d+)$/);
    if (blockDetail) {
      if (await tryOverride('block')) return;
      const n = parseInt(blockDetail[1], 10);
      return json(route, { block: makeBlock(n), transactions: [makeTx(1, { blockNumber: n }), makeTx(2, { blockNumber: n })] });
    }
    if (path === '/blocks') {
      if (await tryOverride('blocks')) return;
      const data = Array.from({ length: 10 }, (_, i) => makeBlock(1000 - i));
      return json(route, { data, hasMore: true });
    }

    // --- transactions ---
    const txTransfers = path.match(/^\/transactions\/(0x[0-9a-fA-F]+)\/transfers$/);
    if (txTransfers) {
      if (await tryOverride('tx-transfers')) return;
      return json(route, []);
    }
    const txLogs = path.match(/^\/transactions\/(0x[0-9a-fA-F]+)\/logs$/);
    if (txLogs) {
      if (await tryOverride('tx-logs')) return;
      return json(route, []);
    }
    const txInternal = path.match(/^\/transactions\/(0x[0-9a-fA-F]+)\/internal$/);
    if (txInternal) {
      if (await tryOverride('tx-internal')) return;
      return json(route, []);
    }
    const txDetail = path.match(/^\/transactions\/(0x[0-9a-fA-F]+)$/);
    if (txDetail) {
      if (await tryOverride('transaction')) return;
      return json(route, makeTx(1, { hash: txDetail[1] }));
    }
    if (path === '/transactions' || path === '/transactions/paginated') {
      if (await tryOverride('transactions')) return;
      // Offset shape (getTransactionsPaginated) — also fine for the list poll.
      const pageSize = parseInt(url.searchParams.get('pageSize') || '25', 10);
      const pageNum = parseInt(url.searchParams.get('page') || '1', 10);
      const total = 50;
      const data = Array.from({ length: Math.min(pageSize, total) }, (_, i) => makeTx(i + 1 + (pageNum - 1) * pageSize));
      return json(route, { data, total, page: pageNum, pageSize, totalPages: Math.ceil(total / pageSize) });
    }
    if (path === '/token-transfers') {
      if (await tryOverride('token-transfers')) return;
      return json(route, { data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 });
    }

    // --- addresses ---
    const addrContractUml = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)\/contract\/uml$/);
    if (addrContractUml) {
      if (await tryOverride('address-uml')) return;
      return route.fulfill({ status: 200, contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg"></svg>' });
    }
    const addrContract = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)\/contract$/);
    if (addrContract) {
      if (await tryOverride('contract')) return;
      return json(route, { error: 'not a contract' }, 404);
    }
    const addrTxs = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)\/transactions$/);
    if (addrTxs) {
      if (await tryOverride('address-transactions')) return;
      return json(route, { data: [makeTx(1), makeTx(2)], hasMore: false });
    }
    const addrTransfers = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)\/transfers$/);
    if (addrTransfers) {
      if (await tryOverride('address-transfers')) return;
      return json(route, { data: [], hasMore: false });
    }
    const addrInternal = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)\/internal$/);
    if (addrInternal) {
      if (await tryOverride('address-internal')) return;
      return json(route, { data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 });
    }
    const addrLogs = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)\/logs$/);
    if (addrLogs) {
      if (await tryOverride('address-logs')) return;
      return json(route, { data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 });
    }
    const addrBalances = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)\/balances$/);
    if (addrBalances) {
      if (await tryOverride('address-balances')) return;
      return json(route, []);
    }
    const addrInfo = path.match(/^\/addresses\/(0x[0-9a-fA-F]+)$/);
    if (addrInfo) {
      if (await tryOverride('address')) return;
      return json(route, {
        address: addrInfo[1],
        balance: '5000000000000000000',
        txCount: 2,
        isContract: false,
        tokenTransferCount: 0,
        internalTxCount: 0,
      });
    }

    // --- tokens ---
    const tokenHolders = path.match(/^\/tokens\/(0x[0-9a-fA-F]+)\/holders$/);
    if (tokenHolders) {
      if (await tryOverride('token-holders')) return;
      return json(route, { data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 });
    }
    const tokenTransfers = path.match(/^\/tokens\/(0x[0-9a-fA-F]+)\/transfers$/);
    if (tokenTransfers) {
      if (await tryOverride('token-detail-transfers')) return;
      return json(route, { data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 });
    }
    const tokenInventory = path.match(/^\/tokens\/(0x[0-9a-fA-F]+)\/inventory$/);
    if (tokenInventory) {
      if (await tryOverride('token-inventory')) return;
      return json(route, { data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 });
    }
    const tokenDetail = path.match(/^\/tokens\/(0x[0-9a-fA-F]+)$/);
    if (tokenDetail) {
      if (await tryOverride('token')) return;
      return json(route, {
        address: tokenDetail[1], symbol: 'MOCK', name: 'Mock Token', decimals: 18,
        tokenType: 'ERC20', totalSupply: '1000000000000000000000000', holderCount: 3,
        transferCount: 7, blockNumber: 1, createdAt: '2024-01-01T00:00:00Z',
      });
    }
    if (path === '/tokens') {
      if (await tryOverride('tokens')) return;
      const data = [
        { address: '0x' + 'a'.repeat(40), symbol: 'AAA', name: 'Token A', decimals: 18, tokenType: 'ERC20', totalSupply: '1000', holderCount: 5, transferCount: 10, blockNumber: 1, createdAt: '2024-01-01T00:00:00Z' },
        { address: '0x' + 'b'.repeat(40), symbol: 'BBB', name: 'Token B', decimals: 18, tokenType: 'ERC721', totalSupply: '100', holderCount: 2, transferCount: 4, blockNumber: 2, createdAt: '2024-01-01T00:00:00Z' },
      ];
      return json(route, { data, total: 2, page: 1, pageSize: 25, totalPages: 1 });
    }

    // --- accounts ---
    if (path === '/accounts') {
      if (await tryOverride('accounts')) return;
      const data = Array.from({ length: 10 }, (_, i) => ({
        address: '0x' + (i + 1).toString(16).padStart(40, '3'),
        balance: String((10 - i) * 1e18), txCount: 100 - i, isContract: i % 3 === 0,
      }));
      return json(route, { data, total: 25, page: 1, pageSize: 25, totalPages: 1 });
    }

    // --- search ---
    if (path === '/search/suggestions') {
      if (await tryOverride('search-suggestions')) return;
      return json(route, { query: url.searchParams.get('q') ?? '', suggestions: [] });
    }
    if (path === '/search') return json(route, { type: 'unknown', data: null });

    // --- privacy ---
    if (path === '/privacy/viewable-addresses') {
      if (await tryOverride('viewable-addresses')) return;
      return json(route, { viewer_wallet: '0x0', viewer_did: did, own_addresses: [], disclosed_addresses: [] });
    }
    if (path.startsWith('/privacy/')) {
      if (await tryOverride('privacy')) return;
      return json(route, {}, 404);
    }

    // --- eth linking ---
    if (path.startsWith('/eth/')) {
      if (await tryOverride('eth')) return;
      if (path === '/eth/addresses') return json(route, { addresses: [] });
      return json(route, {}, 404);
    }

    // --- impersonation (literal paths) ---
    if (path === '/impersonation/start') {
      if (await tryOverride('impersonation-start')) return;
      return json(route, { token: 'mock-imp-token', expires_at: new Date(Date.now() + 3600_000).toISOString(), target_did: did, org_id: 'org1' });
    }
    if (path.startsWith('/impersonation/')) {
      if (await tryOverride('impersonation')) return;
      // DELETE (stop) or GET (restore) — both 200 ok by default.
      return json(route, { stopped: true });
    }

    // --- contract verification compilers ---
    if (path === '/verify/compilers') return json(route, { versions: [] });

    // DEFAULT: unmocked /api route → 404 so a missing mock fails loudly.
    return route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: `[apiMock] no mock for ${path}` }),
    });
  });
}

// ----- Shared "loaded OK" assertion (PLAN §4) -----

/**
 * Assert a page rendered cleanly: a key element is visible AND the tree did
 * not crash (header brand present) AND no error / restricted state AND no
 * stuck loading skeleton. This is the per-page contract every scenario uses.
 */
export async function expectLoadedOK(page: Page, keyLocator: ReturnType<Page['locator']>) {
  await expect(keyLocator.first()).toBeVisible();
  // Tree didn't crash: the Layout header brand is present.
  await expect(page.getByTestId('header-brand')).toBeVisible();
  // No generic error, no privacy-restricted interstitial.
  await expect(page.getByTestId('app-error')).toHaveCount(0);
  await expect(page.getByTestId('restricted-state')).toHaveCount(0);
  // No stuck loading state.
  await expect(page.getByTestId('app-loading')).toHaveCount(0);
}

// ----- Fixture wiring -----

export const test = base.extend({});
export { expect };
