import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { useAuth } from '../lib/auth';

/**
 * Hook to get all viewable addresses for the authenticated user.
 * Uses TanStack React Query for caching and consistency.
 */
export function useViewableAddresses() {
  const { isAuthenticated } = useAuth();

  const { data: rawData, isLoading: loading, error, refetch } = useQuery({
    queryKey: ['viewableAddresses'],
    queryFn: () => api.getViewableAddresses(),
    enabled: isAuthenticated,
    retry: false,
    staleTime: 30000,
  });

  const data = rawData ? {
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

  return { data, loading, error, refetch, isConnected: isAuthenticated };
}
