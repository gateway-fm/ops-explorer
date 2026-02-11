import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';

// Cache of contract names - persists across component renders
const contractNameCache: Record<string, string | null> = {};

export function useContractName(address: string | undefined | null) {
  const normalizedAddress = address?.toLowerCase();

  // Check cache first
  const cachedName = normalizedAddress ? contractNameCache[normalizedAddress] : undefined;

  const { data: contract } = useQuery({
    queryKey: ['contractName', normalizedAddress],
    queryFn: async () => {
      if (!normalizedAddress) return null;
      try {
        const contract = await api.getContract(normalizedAddress);
        // Cache the result
        contractNameCache[normalizedAddress] = contract?.isVerified ? contract.contractName || null : null;
        return contract;
      } catch {
        // Not a contract or error - cache as null
        contractNameCache[normalizedAddress] = null;
        return null;
      }
    },
    enabled: !!normalizedAddress && cachedName === undefined,
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: false,
  });

  // Return cached name if available, otherwise query result
  if (cachedName !== undefined) {
    return cachedName;
  }

  return contract?.isVerified ? contract.contractName || null : null;
}

// Simple function to get cached name (no fetching)
export function getCachedContractName(address: string | undefined | null): string | null {
  if (!address) return null;
  return contractNameCache[address.toLowerCase()] ?? null;
}

// Function to set a contract name in cache (useful when we already have the data)
export function setCachedContractName(address: string, name: string | null): void {
  contractNameCache[address.toLowerCase()] = name;
}
