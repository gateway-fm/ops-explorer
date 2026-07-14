import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// RD-1149: the by-address feed helpers must prefer the opaque keyset cursor over
// the legacy ?before= block bound. These tests pin the outbound URL the client
// builds — cursor wins when both are supplied, and ?before= is the fallback for
// older proxies that don't return a nextCursor.

vi.mock('./runtimeConfig', () => ({
  getConfig: (_key: string, fallback: string) => fallback, // API_BASE = "/api"
}));
vi.mock('../hooks/useImpersonation', () => ({
  getImpersonationTokenHeader: () => null,
}));

import { api } from './api';

// Records the URL fetch was called with and returns an empty paginated body.
function captureFetch() {
  const fn = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ data: [], hasMore: false }),
    text: async () => '{"data":[],"hasMore":false}',
  } as Response);
  vi.stubGlobal('fetch', fn);
  return fn;
}

function calledURL(fn: ReturnType<typeof vi.fn>): string {
  return String(fn.mock.calls[0][0]);
}

describe('by-address pagination: cursor preference', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('getAddressTransactions sends ?cursor= (not ?before=) when a cursor is given', async () => {
    const fn = captureFetch();
    await api.getAddressTransactions('0xabc', 25, 100, 'op-cursor');
    const url = calledURL(fn);
    expect(url).toContain('cursor=op-cursor');
    expect(url).not.toContain('before=');
  });

  it('getAddressTransactions falls back to ?before= when no cursor is given', async () => {
    const fn = captureFetch();
    await api.getAddressTransactions('0xabc', 25, 100);
    const url = calledURL(fn);
    expect(url).toContain('before=100');
    expect(url).not.toContain('cursor=');
  });

  it('getAddressTransfers sends ?cursor= (not ?before=) when a cursor is given', async () => {
    const fn = captureFetch();
    await api.getAddressTransfers('0xabc', 25, 100, 'xfer-cursor');
    const url = calledURL(fn);
    expect(url).toContain('cursor=xfer-cursor');
    expect(url).not.toContain('before=');
  });

  it('getAddressTransfers falls back to ?before= when no cursor is given', async () => {
    const fn = captureFetch();
    await api.getAddressTransfers('0xabc', 25, 100);
    const url = calledURL(fn);
    expect(url).toContain('before=100');
    expect(url).not.toContain('cursor=');
  });

  it('getGrantedAddressTransactions sends ?cursor= when a cursor is given', async () => {
    const fn = captureFetch();
    await api.getGrantedAddressTransactions('g1', 'a1', 25, 100, 'grant-cursor');
    const url = calledURL(fn);
    expect(url).toContain('cursor=grant-cursor');
    expect(url).not.toContain('before=');
  });
});
