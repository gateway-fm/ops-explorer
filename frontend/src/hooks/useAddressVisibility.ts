import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { disclosureQueryOptions, isDisclosureReady } from '../lib/disclosureQuery';
import { useAuth } from '../lib/auth';
import { useImpersonation } from './useImpersonation';
import { useSearchParams } from 'react-router-dom';

/**
 * Hook to get all viewable addresses for the authenticated user.
 * Uses TanStack React Query for caching and consistency.
 */
export function useViewableAddresses() {
  const { isAuthenticated, isLoading: authLoading, auth } = useAuth();
  const { isActive, token } = useImpersonation();
  const [searchParams] = useSearchParams();
  const enabled = !authLoading && isAuthenticated && !isActive && !searchParams.has('as');

  const query = useQuery({
    ...disclosureQueryOptions,
    queryKey: ['viewableAddresses', isAuthenticated ? auth.did : null, token, enabled],
    queryFn: ({ signal }) => api.getViewableAddresses(signal),
    enabled,
  });
  const { data: rawData, error, refetch } = query;
  const loading = query.isPending || query.fetchStatus !== 'idle';

  // Disabled queries retain their cache, and in-flight requests may finish
  // after a scope change. Gate returned data as well as issuing requests.
  const data = enabled && isDisclosureReady(query) && rawData ? {
    ownAddresses: rawData.own_addresses?.map(a => a.address) || [],
    disclosedAddresses: rawData.disclosed_addresses?.map(a => ({
      address: a.address,
      addressId: a.address_id,
      ownerDid: a.owner_did,
      grantId: a.grant_id,
      disclosure_level: a.disclosure_level,
      expires_at: a.expires_at,
    })) || [],
  } : null;

  return { data, loading: enabled && loading, error: enabled ? error : null, refetch: () => { if (enabled) void refetch(); }, isConnected: isAuthenticated };
}
