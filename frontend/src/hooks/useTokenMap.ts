import { useQueries } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Token } from '../lib/api';

/**
 * Fetches token metadata for a set of token addresses.
 * Returns a map of lowercase address -> Token.
 * react-query caches each token individually — per user session, no cross-user leakage.
 */
export function useTokenMap(tokenAddresses: string[]): Record<string, Token> {
  const unique = [...new Set(tokenAddresses.map(a => a.toLowerCase()))];

  const results = useQueries({
    queries: unique.map(addr => ({
      queryKey: ['token', addr],
      queryFn: () => api.getToken(addr),
      staleTime: 5 * 60 * 1000,
      retry: 1,
    })),
  });

  const map: Record<string, Token> = {};
  results.forEach((r, i) => {
    if (r.data) {
      map[unique[i]] = r.data;
    }
  });
  return map;
}
