import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';

/**
 * MSW node server for RTL/vitest tests.
 *
 * Lets page/component tests mock `fetch('/api/...')` DECLARATIVELY instead of
 * hand-rolling `vi.mock('../lib/api')` per file (PLAN §6). The default handlers
 * return shape-accurate JSON (mirroring frontend/src/lib/api.ts); individual
 * tests narrow with `server.use(...)`.
 *
 * In happy-dom, same-origin `/api` resolves against the jsdom base URL
 * (http://localhost/), so handlers match the absolute form.
 */

// happy-dom's default document URL is http://localhost:3000, so same-origin
// `/api` fetches resolve against that origin. Handlers match the absolute form.
const API = 'http://localhost:3000/api';

export function makeTx(i: number, over: Record<string, unknown> = {}) {
  return {
    hash: '0x' + i.toString(16).padStart(64, 'e'),
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
    txCategories: ['coin_transfer'],
    tokenTransferCount: 0,
    ...over,
  };
}

export function makeBlock(n: number, over: Record<string, unknown> = {}) {
  return {
    number: n,
    hash: '0x' + n.toString(16).padStart(64, 'b'),
    parentHash: '0x' + (n - 1).toString(16).padStart(64, 'b'),
    timestamp: 1_700_000_000 + n * 12,
    gasUsed: 12_000_000,
    gasLimit: 30_000_000,
    transactionCount: 2,
    size: 1234,
    difficulty: '0',
    totalDifficulty: '0',
    nonce: '0x0',
    miner: '0x' + '1'.repeat(40),
    extraData: '',
    stateRoot: '0x' + 'a'.repeat(64),
    transactionsRoot: '0x' + 'c'.repeat(64),
    receiptsRoot: '0x' + 'd'.repeat(64),
    createdAt: '2024-01-01T00:00:00Z',
    ...over,
  };
}

export const defaultHandlers = [
  http.get(`${API}/stats`, () =>
    HttpResponse.json({ totalBlocks: 1000, totalTransactions: 5000, totalAddresses: 200, avgBlockTime: 12, privacyEnabled: false }),
  ),
  http.get(`${API}/auth/status`, () => HttpResponse.json({ authenticated: false })),
  http.get(`${API}/stats/tx-history`, () => HttpResponse.json([])),
  http.get(`${API}/gas`, () => HttpResponse.json({ slow: null, normal: null, fast: null, updatedAt: '' })),
  http.get(`${API}/price`, () => HttpResponse.json({ price: 0, currency: 'USD', change24h: 0, lastUpdated: '' })),
  http.get(`${API}/blocks`, () => HttpResponse.json({ data: [makeBlock(1000), makeBlock(999)], hasMore: false })),
  http.get(`${API}/blocks/latest`, () => HttpResponse.json(makeBlock(1000))),
  http.get(`${API}/transactions`, () =>
    HttpResponse.json({ data: [makeTx(1), makeTx(2)], total: 2, page: 1, pageSize: 25, totalPages: 1 }),
  ),
  http.get(`${API}/search/suggestions`, () => HttpResponse.json({ query: '', suggestions: [] })),
];

export const server = setupServer(...defaultHandlers);

export { http, HttpResponse, API };
