import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { GrantSource, GrantWindow } from '../pages/Transactions.disclosure';
import { loadGrantWindow } from '../pages/Transactions.disclosure';

interface Continuation { source: GrantSource; cursor?: string }

export function useDisclosedTransactions(enabled: boolean, viewer: string | null, page: number, window: GrantWindow | undefined) {
  const sources = useQuery({
    queryKey: ['disclosed-transaction-sources', viewer],
    enabled,
    retry: false,
    queryFn: async () => {
      const addresses = await api.getViewableAddresses();
      const unique = new Map((addresses.disclosed_addresses ?? []).map((address) => [`${address.grant_id}:${address.address_id}`, address]));
      const resolved = await Promise.allSettled([...unique.values()].map(async (address) => {
        const grant = await api.getGrantedAddress(address.grant_id, address.address_id);
        const methods = grant.scope_methods ?? [];
        if (methods.length && !methods.includes('transaction_history') && !methods.includes('full_disclosure')) return null;
        return { grantId: address.grant_id, addressId: address.address_id, level: grant.disclosure_level };
      }));
      return {
        sources: resolved.flatMap((result) => result.status === 'fulfilled' && result.value ? [result.value] : []),
        incomplete: resolved.some((result) => result.status === 'rejected'),
      };
    },
  });

  const initial: Continuation[] = (sources.data?.sources ?? []).map((source) => ({ source }));
  const feed = useInfiniteQuery({
    queryKey: ['disclosed-transactions', viewer, page, window, sources.data?.sources, sources.dataUpdatedAt],
    enabled: enabled && !!window && initial.length > 0,
    initialPageParam: initial,
    retry: false,
    queryFn: async ({ pageParam }) => {
      const results = await Promise.all(pageParam.map(async ({ source, cursor }) => ({
        source, ...await loadGrantWindow(source, window!, api.getGrantedAddressTransactions, cursor),
      })));
      return {
        rows: results.flatMap((result) => result.rows),
        continuation: results.flatMap((result) => result.nextCursor ? [{ source: result.source, cursor: result.nextCursor }] : []),
      };
    },
    getNextPageParam: (last) => last.continuation.length ? last.continuation : undefined,
  });

  // Disabled React Query observers retain cached data. Gate the render result
  // as well, so activating View-as or signing out removes grant rows immediately.
  // A denied refresh/continuation must not keep formerly authorized rows visible.
  const canShow = enabled && !sources.error && !feed.error && !sources.isFetching && !feed.isRefetching;
  return {
    rows: canShow ? feed.data?.pages.flatMap((batch) => batch.rows) ?? [] : [],
    loading: enabled && (sources.isFetching || (!!window && initial.length > 0 && (feed.isLoading || feed.isRefetching))),
    error: enabled && (sources.error || sources.data?.incomplete || feed.error),
    hasMore: canShow && !!feed.hasNextPage,
    loadingMore: feed.isFetchingNextPage,
    loadMore: () => feed.fetchNextPage(),
    retry: () => { void sources.refetch(); if (window && initial.length) void feed.refetch(); },
  };
}
