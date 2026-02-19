import { useQuery } from '@tanstack/react-query';
import { api, type AddressVisibility } from '../lib/api';
import { useAuth } from '../lib/auth';

/**
 * Hook to check visibility of a single address.
 * Uses TanStack React Query for caching and consistency with the codebase.
 * SSO-only authentication via Privado.
 */
export function useAddressVisibility(address: string | null | undefined) {
  const { ssoAuth } = useAuth();
  const isAuthenticated = ssoAuth.authenticated;

  const { data: visibility = null, isLoading: loading, error } = useQuery({
    queryKey: ['addressVisibility', address],
    queryFn: () => api.checkAddressVisibility(address!),
    enabled: !!address && isAuthenticated,
    retry: false,
    staleTime: 30000,
  });

  return { visibility, loading, error, isConnected: isAuthenticated };
}

/**
 * Hook to check visibility of multiple addresses at once.
 * Uses TanStack React Query for caching and consistency.
 */
export function useBatchAddressVisibility(addresses: string[]) {
  const { ssoAuth } = useAuth();
  const isAuthenticated = ssoAuth.authenticated;

  const addressKey = addresses.join(',');

  const { data, isLoading: loading, error } = useQuery({
    queryKey: ['batchAddressVisibility', addressKey],
    queryFn: () => api.batchCheckAddresses(addresses),
    enabled: addresses.length > 0 && isAuthenticated,
    retry: false,
    staleTime: 30000,
  });

  const visibilities = data?.results ?? {};

  return { visibilities, loading, error, isConnected: isAuthenticated };
}

/**
 * Hook to get all viewable addresses for the authenticated user.
 * Uses TanStack React Query for caching and consistency.
 */
export function useViewableAddresses() {
  const { ssoAuth } = useAuth();
  const isAuthenticated = ssoAuth.authenticated;

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

/**
 * Helper to check if an address is visible based on cached visibility data
 */
export function isAddressVisible(
  address: string,
  visibilities: Record<string, AddressVisibility>,
): boolean {
  const vis = visibilities[address.toLowerCase()];
  return vis ? vis.visible : true; // Fail open
}

/**
 * Helper to get display name for an address based on visibility
 */
export function getAddressDisplayName(
  address: string,
  visibilities: Record<string, AddressVisibility>,
  truncate = true
): string {
  const vis = visibilities[address.toLowerCase()];

  if (!vis || !vis.visible) {
    return '[PRIVATE]';
  }

  if (vis.level === 'pseudonymous' && vis.pseudonym) {
    return vis.pseudonym;
  }

  if (truncate) {
    return `${address.slice(0, 6)}...${address.slice(-4)}`;
  }

  return address;
}
