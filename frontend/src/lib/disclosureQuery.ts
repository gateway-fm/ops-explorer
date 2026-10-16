import type { FetchStatus, QueryStatus } from '@tanstack/react-query';

// Cache freshness is not an authorization check. All disclosure observers use
// a key that changes when disabled, consume AbortSignal, and discard inactive
// data so account/View-as transitions cannot reuse an earlier authorization.
export const disclosureQueryOptions = {
  retry: false,
  staleTime: 0,
  gcTime: 0,
  refetchOnMount: 'always',
} as const;

export function isDisclosureReady(query: { status: QueryStatus; fetchStatus: FetchStatus }) {
  // Paused requests are awaiting authorization too, even though isFetching is false.
  return query.status === 'success' && query.fetchStatus === 'idle';
}
