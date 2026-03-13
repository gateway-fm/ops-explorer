import { useQuery } from '@tanstack/react-query';
import { api, type LinkedAddress } from '../lib/api';
import { useAuth } from '../lib/auth';
import { usePrivacyEnabled } from './usePrivacyEnabled';

export const LINKED_ADDRESSES_QUERY_KEY = ['linkedAddresses'] as const;

/**
 * Hook to fetch linked ETH addresses for the authenticated user.
 * Only fetches when privacy is enabled and the user is authenticated.
 */
export function useLinkedAddresses() {
  const privacyEnabled = usePrivacyEnabled();
  const { isAuthenticated } = useAuth();

  const enabled = privacyEnabled && isAuthenticated;

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: LINKED_ADDRESSES_QUERY_KEY,
    queryFn: () => api.eth.getLinkedAddresses(),
    enabled,
    retry: false,
    staleTime: 30000,
  });

  const addresses: LinkedAddress[] = data?.addresses ?? [];
  const userAddresses = addresses.filter(a => a.link_type === 'user');
  const systemAddresses = addresses.filter(a => a.link_type === 'system');

  return {
    addresses,
    userAddresses,
    systemAddresses,
    isLoading: enabled ? isLoading : false,
    error: enabled ? error : null,
    refetch,
    enabled,
  };
}
