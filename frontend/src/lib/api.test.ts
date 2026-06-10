import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Contract under test: the error-normalization behavior of the BFF client's
// fetchAPI wrapper (api.ts:442-453). On a non-OK response it throws an Error
// whose message carries the HTTP status, and on an OK response it returns the
// parsed JSON body. This is the contract the REQ-5.6 "Address restricted" page
// depends on (pages/Address.tsx keys off error.message.includes('403')).
//
// TODO(REQ-5.6 / audit §4.2): fetchAPI currently throws a plain Error with the
// status only embedded in the message string ("API error: 403"). The desired
// end state is a STRUCTURED error (e.g. { status: 403 }) so consumers can do
// err.status === 403 instead of fragile substring matching. That refactor is a
// UX follow-up (out of scope for the test-coverage workstream); these tests pin
// the CURRENT contract so the refactor can be made safely later.

// runtimeConfig + useImpersonation are imported at module load; stub them so the
// client constructs a deterministic base URL and no impersonation header.
vi.mock('./runtimeConfig', () => ({
  getConfig: (_key: string, fallback: string) => fallback,
}));
vi.mock('../hooks/useImpersonation', () => ({
  getImpersonationTokenHeader: () => null,
}));

import { api } from './api';

function mockFetchOnce(status: number, body: unknown, ok = status >= 200 && status < 300) {
  return vi.fn().mockResolvedValueOnce({
    ok,
    status,
    json: async () => body,
    text: async () => (typeof body === 'string' ? body : JSON.stringify(body)),
  } as Response);
}

describe('fetchAPI error normalization', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('returns parsed JSON on a 200 response', async () => {
    vi.stubGlobal('fetch', mockFetchOnce(200, { totalBlocks: 42 }));
    const stats = await api.getStats();
    expect(stats).toEqual({ totalBlocks: 42 });
  });

  // The error message must carry each status so the restricted-page substring
  // check (and any logging) can read it.
  for (const status of [401, 403, 404, 500]) {
    it(`throws an Error carrying the status ${status}`, async () => {
      vi.stubGlobal('fetch', mockFetchOnce(status, { error: 'nope' }));
      await expect(api.getStats()).rejects.toThrow(String(status));
    });
  }

  it('throws an Error (not a non-Error) so `instanceof Error` holds', async () => {
    vi.stubGlobal('fetch', mockFetchOnce(403, {}));
    await expect(api.getStats()).rejects.toBeInstanceOf(Error);
  });

  it('the 403 message contains "403" so the restricted-page check matches', async () => {
    vi.stubGlobal('fetch', mockFetchOnce(403, {}));
    let caught: unknown;
    try {
      await api.getStats();
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(Error);
    // This is exactly the predicate pages/Address.tsx uses.
    expect((caught as Error).message.includes('403')).toBe(true);
  });
});
