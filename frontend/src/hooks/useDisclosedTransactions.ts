import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { disclosureQueryOptions, isDisclosureReady } from '../lib/disclosureQuery';
import type { GrantBoundary, GrantSource, GrantWindow } from '../pages/Transactions.disclosure';
import { loadGrantWindow } from '../pages/Transactions.disclosure';

interface Continuation { source: GrantSource; cursor?: string }
interface FeedCursor { sources: Continuation[]; window?: GrantWindow; boundaryPage?: number }
interface BoundaryLookup { page: number; load: (page: number, signal: AbortSignal) => Promise<GrantBoundary> }

export function useDisclosedTransactions(enabled: boolean, viewer: string | null, page: number, window: GrantWindow | undefined, boundary?: BoundaryLookup) {
  const sources = useQuery({
    ...disclosureQueryOptions,
    queryKey: ['disclosed-transaction-sources', viewer, enabled],
    enabled,
    queryFn: async ({ signal }) => {
      const addresses = await api.getViewableAddresses(signal);
      signal.throwIfAborted();
      const unique = new Map((addresses.disclosed_addresses ?? []).map((address) => [`${address.grant_id}:${address.address_id}`, address]));
      const resolved = await Promise.allSettled([...unique.values()].map(async (address) => {
        const grant = await api.getGrantedAddress(address.grant_id, address.address_id, signal);
        signal.throwIfAborted();
        const methods = grant.scope_methods ?? [];
        if (methods.length && !methods.includes('transaction_history') && !methods.includes('full_disclosure')) return null;
        return { grantId: address.grant_id, addressId: address.address_id, level: grant.disclosure_level };
      }));
      signal.throwIfAborted();
      return {
        sources: resolved.flatMap((result) => result.status === 'fulfilled' && result.value ? [result.value] : []),
        incomplete: resolved.some((result) => result.status === 'rejected'),
      };
    },
  });

  const sourcesReady = enabled && isDisclosureReady(sources);
  const initial: Continuation[] = (sourcesReady ? sources.data?.sources ?? [] : []).map((source) => ({ source }));
  const feedEnabled = sourcesReady && !!window && initial.length > 0;
  const feed = useInfiniteQuery({
    ...disclosureQueryOptions,
    queryKey: ['disclosed-transactions', viewer, page, window, sources.data?.sources, sources.dataUpdatedAt, feedEnabled, boundary?.page],
    enabled: feedEnabled,
    initialPageParam: { sources: initial, boundaryPage: boundary?.page } as FeedCursor,
    queryFn: async ({ pageParam, signal }) => {
      let resolvedWindow = pageParam.window ?? window!;
      if (boundary && pageParam.boundaryPage !== undefined) {
        const result = await boundary.load(pageParam.boundaryPage, signal);
        signal.throwIfAborted();
        if (result.nextPage !== undefined) {
          return { rows: [], continuation: { sources: pageParam.sources, boundaryPage: result.nextPage } as FeedCursor };
        }
        resolvedWindow = { ...resolvedWindow, upper: result.upper };
      }
      // Promise.all rejects immediately, but leaves sibling requests running.
      // Cancel the whole batch on failure as well as on a query scope change.
      const batch = new AbortController();
      const cancel = () => batch.abort(signal.reason);
      signal.addEventListener('abort', cancel, { once: true });
      if (signal.aborted) cancel();
      try {
        const results = await Promise.all(pageParam.sources.map(async ({ source, cursor }) => ({
          source, ...await loadGrantWindow(source, resolvedWindow, api.getGrantedAddressTransactions, cursor, 5, batch.signal),
        })));
        signal.throwIfAborted();
        const nextSources = results.flatMap((result) => result.nextCursor ? [{ source: result.source, cursor: result.nextCursor }] : []);
        return {
          rows: results.flatMap((result) => result.rows),
          continuation: nextSources.length ? { sources: nextSources, window: resolvedWindow } as FeedCursor : undefined,
        };
      } catch (error) {
        batch.abort(error);
        throw error;
      } finally {
        signal.removeEventListener('abort', cancel);
      }
    },
    getNextPageParam: (last) => last.continuation,
  });

  // Keep current rows during online pagination, but never during a refresh,
  // paused access check, or after a denied continuation.
  const canShow = feedEnabled && feed.isSuccess && !feed.isRefetching && !feed.isPaused;
  return {
    rows: canShow ? feed.data?.pages.flatMap((batch) => batch.rows) ?? [] : [],
    loading: enabled && (sources.isPending || sources.fetchStatus !== 'idle' || (feedEnabled && (feed.isPending || feed.isRefetching || feed.isPaused))),
    error: enabled && (sources.error || sources.data?.incomplete || feed.error),
    hasMore: canShow && !!feed.hasNextPage,
    loadingMore: feed.isFetchingNextPage,
    loadMore: () => { if (canShow && feed.hasNextPage) return feed.fetchNextPage(); },
    // A successful metadata refresh creates a new feed key. Never restart the
    // old feed in parallel: its cached Full pages may predate a downgrade.
    retry: () => { if (enabled) void sources.refetch(); },
  };
}
