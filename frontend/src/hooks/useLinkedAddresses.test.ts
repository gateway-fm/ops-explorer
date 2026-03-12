import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { useLinkedAddresses } from './useLinkedAddresses';

// Mock usePrivacyEnabled
const mockPrivacyEnabled = vi.fn(() => true);
vi.mock('./usePrivacyEnabled', () => ({
  usePrivacyEnabled: () => mockPrivacyEnabled(),
}));

// Mock useAuth
const mockAuth = vi.fn(() => ({
  isAuthenticated: true,
  isLoading: false,
  auth: { authenticated: true, did: 'did:example:123', expiresAt: null },
  logout: vi.fn(),
  checkStatus: vi.fn(),
}));
vi.mock('../lib/auth', () => ({
  useAuth: () => mockAuth(),
  AuthProvider: ({ children }: { children: ReactNode }) => children,
}));

// Mock api
const mockGetLinkedAddresses = vi.fn();
vi.mock('../lib/api', () => ({
  api: {
    eth: {
      getLinkedAddresses: () => mockGetLinkedAddresses(),
    },
    getStats: vi.fn(),
  },
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useLinkedAddresses', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPrivacyEnabled.mockReturnValue(true);
    mockAuth.mockReturnValue({
      isAuthenticated: true,
      isLoading: false,
      auth: { authenticated: true, did: 'did:example:123', expiresAt: null },
      logout: vi.fn(),
      checkStatus: vi.fn(),
    });
  });

  it('returns addresses when privacy enabled and authenticated', async () => {
    mockGetLinkedAddresses.mockResolvedValue({
      addresses: [
        { address: '0xabc', verified_at: '2024-01-01T00:00:00Z', link_type: 'user' },
        { address: '0xdef', verified_at: '2024-01-02T00:00:00Z', link_type: 'system' },
      ],
    });

    const { result } = renderHook(() => useLinkedAddresses(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.addresses).toHaveLength(2);
    expect(result.current.userAddresses).toHaveLength(1);
    expect(result.current.systemAddresses).toHaveLength(1);
    expect(result.current.userAddresses[0].address).toBe('0xabc');
    expect(result.current.systemAddresses[0].address).toBe('0xdef');
    expect(result.current.enabled).toBe(true);
  });

  it('returns empty when not privacy enabled', async () => {
    mockPrivacyEnabled.mockReturnValue(false);

    const { result } = renderHook(() => useLinkedAddresses(), {
      wrapper: createWrapper(),
    });

    // Should not be loading and should not fetch
    expect(result.current.isLoading).toBe(false);
    expect(result.current.addresses).toHaveLength(0);
    expect(result.current.enabled).toBe(false);
    expect(mockGetLinkedAddresses).not.toHaveBeenCalled();
  });

  it('returns empty when not authenticated', async () => {
    mockAuth.mockReturnValue({
      isAuthenticated: false,
      isLoading: false,
      auth: { authenticated: false, did: null as string | null, expiresAt: null as number | null },
      logout: vi.fn(),
      checkStatus: vi.fn(),
    });

    const { result } = renderHook(() => useLinkedAddresses(), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.addresses).toHaveLength(0);
    expect(result.current.enabled).toBe(false);
    expect(mockGetLinkedAddresses).not.toHaveBeenCalled();
  });

  it('handles API errors', async () => {
    mockGetLinkedAddresses.mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useLinkedAddresses(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
    });

    expect(result.current.addresses).toHaveLength(0);
  });
});
