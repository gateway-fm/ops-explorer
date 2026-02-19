import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';

export function usePrivacyEnabled(): boolean {
  const { data } = useQuery({
    queryKey: ['stats'],
    queryFn: api.getStats,
    staleTime: 30000,
  });
  return data?.privacyEnabled ?? false;
}
