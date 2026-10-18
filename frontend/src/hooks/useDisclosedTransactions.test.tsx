import type { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider, onlineManager } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { deferred } from '../test/deferred';
import { useDisclosedTransactions } from './useDisclosedTransactions';

const requests = vi.hoisted(() => ({ addresses: vi.fn(), metadata: vi.fn(), feed: vi.fn() }));
vi.mock('../lib/api', () => ({ api: {
  getViewableAddresses: requests.addresses,
  getGrantedAddress: requests.metadata,
  getGrantedAddressTransactions: requests.feed,
} }));

const addresses = { disclosed_addresses: [{ grant_id: 'g', address_id: 'a' }] };
const metadata = { grant_id: 'g', disclosure_level: 'full', scope_methods: ['transaction_history'] };
const feed = { disclosure_level: 'full', transactions: [{ tx_hash: 'private-hash', from: 'private-address', block_number: 10, tx_index: 0 }] };
const noGrants = { disclosed_addresses: [] };
const window = {};

function setup(boundary?: Parameters<typeof useDisclosedTransactions>[4]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return { client, ...renderHook(({ enabled, viewer }) => useDisclosedTransactions(enabled, viewer, boundary ? 3 : 1, window, boundary), {
    wrapper, initialProps: { enabled: true, viewer: 'did:admin' },
  }) };
}

beforeEach(() => {
  requests.addresses.mockReset().mockResolvedValue(addresses);
  requests.metadata.mockReset().mockResolvedValue(metadata);
  requests.feed.mockReset().mockResolvedValue(feed);
});
afterEach(() => { onlineManager.setOnline(true); });

describe('disclosure query authorization lifecycle', () => {
  it('retains the resolved boundary across grant cursor continuations', async () => {
    const lookup = vi.fn().mockResolvedValue({ upper: { block: 10, index: 0 } });
    let calls = 0;
    requests.feed.mockImplementation(() => {
      calls++;
      return {
        disclosure_level: 'full',
        transactions: calls === 1 || calls === 6 ? [
          { ...feed.transactions[0], tx_hash: `above-${calls}`, block_number: 20 },
          { ...feed.transactions[0], tx_hash: `below-${calls}`, block_number: calls === 1 ? 5 : 4 },
        ] : [],
        ...(calls <= 5 ? { next_cursor: `cursor-${calls}` } : {}),
      };
    });
    const { result } = setup({ page: 2, load: lookup });
    await waitFor(() => expect(result.current.hasMore).toBe(true));
    expect(result.current.rows.map((row) => row.tx.tx_hash)).toEqual(['below-1']);
    await act(async () => { await result.current.loadMore(); });
    await waitFor(() => expect(result.current.rows).toHaveLength(2));
    expect(result.current.rows.map((row) => row.tx.tx_hash)).toEqual(['below-1', 'below-6']);
    expect(lookup).toHaveBeenCalledTimes(1);
  });

  it('cancels a pending boundary lookup before it can start grant requests', async () => {
    const pending = deferred<{ upper: { block: number; index: number } }>();
    const lookup = vi.fn().mockReturnValue(pending.promise);
    const { result, rerender } = setup({ page: 2, load: lookup });
    await waitFor(() => expect(lookup).toHaveBeenCalledTimes(1));
    const signal = lookup.mock.calls[0][1] as AbortSignal;
    rerender({ enabled: false, viewer: 'did:admin' });
    expect(signal.aborted).toBe(true);
    await act(async () => { pending.resolve({ upper: { block: 10, index: 0 } }); });
    expect(requests.feed).not.toHaveBeenCalled();
    expect(result.current.rows).toEqual([]);
  });

  it('exposes a failed boundary lookup as a retryable disclosure error', async () => {
    const lookup = vi.fn().mockRejectedValueOnce(new Error('API error: 503')).mockResolvedValue({});
    const { result } = setup({ page: 2, load: lookup });
    await waitFor(() => expect(result.current.error).toBeTruthy());
    expect(requests.feed).not.toHaveBeenCalled();
    act(() => result.current.retry());
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    expect(lookup).toHaveBeenCalledTimes(2);
  });

  it('cancels the other grant batches when one fails, including their late cursor continuations', async () => {
    requests.addresses.mockResolvedValue({ disclosed_addresses: [...addresses.disclosed_addresses, { grant_id: 'other', address_id: 'b' }] });
    const pending = deferred<typeof feed & { next_cursor: string }>();
    requests.feed.mockImplementation((grantId: string) => grantId === 'g' ? Promise.reject(new Error('API error: 403')) : pending.promise);
    const { result, rerender } = setup();
    await waitFor(() => expect(result.current.error).toBeTruthy());
    expect(requests.feed).toHaveBeenCalledTimes(2);
    const signal = requests.feed.mock.calls[1].at(-1) as AbortSignal;
    expect(signal.aborted).toBe(true);
    rerender({ enabled: false, viewer: 'did:admin' });
    await act(async () => { pending.resolve({ ...feed, next_cursor: 'more' }); });
    expect(requests.feed).toHaveBeenCalledTimes(2);
    expect(result.current.rows).toEqual([]);
  });

  it.each(['addresses', 'metadata', 'feed'] as const)('cancels pending %s work and prevents later grant requests during View-as', async (stage) => {
    const pending = deferred<unknown>();
    requests[stage].mockReturnValueOnce(pending.promise);
    const { result, rerender } = setup();
    await waitFor(() => expect(requests[stage]).toHaveBeenCalledTimes(1));
    const signal = requests[stage].mock.calls[0].at(-1) as AbortSignal | undefined;
    rerender({ enabled: false, viewer: 'did:admin' });
    expect(signal?.aborted).toBe(true);
    // Deliberately deliver a response even after cancellation: continuations
    // must check their signal before starting the next request or cursor page.
    await act(async () => {
      pending.resolve({ addresses, metadata, feed: { ...feed, next_cursor: 'more' } }[stage]);
      await pending.promise;
    });
    expect(requests.metadata).toHaveBeenCalledTimes(stage === 'addresses' ? 0 : 1);
    expect(requests.feed).toHaveBeenCalledTimes(stage === 'feed' ? 1 : 0);
    expect(result.current.rows).toEqual([]);
    requests.addresses.mockResolvedValue(noGrants);
    rerender({ enabled: true, viewer: 'did:admin' });
    await waitFor(() => expect(requests.addresses).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toEqual([]);
  });

  it('requires fresh authorization on leaving View-as even when the old cache was fresh', async () => {
    const { result, rerender } = setup();
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    rerender({ enabled: false, viewer: 'did:admin' });
    const pending = deferred<typeof noGrants>();
    requests.addresses.mockReturnValueOnce(pending.promise);
    rerender({ enabled: true, viewer: 'did:admin' });
    expect(result.current.rows).toEqual([]);
    await waitFor(() => expect(requests.addresses).toHaveBeenCalledTimes(2));
    expect(requests.feed).toHaveBeenCalledTimes(1);
    await act(async () => { pending.resolve(noGrants); });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toEqual([]);
  });

  it.each(['disclosed-transaction-sources', 'disclosed-transactions'])('hides Full rows while %s revalidation is paused offline', async (key) => {
    const { client, result } = setup();
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    requests.addresses.mockResolvedValue(noGrants);
    requests.feed.mockRejectedValue(new Error('API error: 403'));
    act(() => {
      onlineManager.setOnline(false);
      void client.invalidateQueries({ queryKey: [key] });
    });
    await waitFor(() => expect(result.current.rows).toEqual([]));
    expect(result.current.loading).toBe(true);
    act(() => onlineManager.setOnline(true));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toEqual([]);
  });

  it('cancels source resolution when the authenticated account changes', async () => {
    const pending = deferred<typeof addresses>();
    requests.addresses.mockReturnValueOnce(pending.promise);
    const { result, rerender } = setup();
    await waitFor(() => expect(requests.addresses).toHaveBeenCalledTimes(1));
    const signal = requests.addresses.mock.calls[0][0] as AbortSignal | undefined;
    requests.addresses.mockResolvedValue(noGrants);
    rerender({ enabled: true, viewer: 'did:other' });
    expect(signal?.aborted).toBe(true);
    await act(async () => { pending.resolve(addresses); });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(requests.metadata).not.toHaveBeenCalled();
    expect(result.current.rows).toEqual([]);
  });
});
