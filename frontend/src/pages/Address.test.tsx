import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import type { ReactNode } from 'react';

// Contract under test (REQ-5.6): when the address-info request is rejected with
// a 403, pages/Address.tsx renders the "Address restricted" privacy card rather
// than the normal address view or a generic "not found".
//
// pages/Address.tsx:247 currently triggers this via
// error.message.includes('403') (the fragile substring match the audit flagged
// — see the TODO in lib/api.test.ts; the structured-status refactor is a UX
// follow-up, out of scope here). This test pins the current behavior so the
// privacy interstitial cannot silently regress.

// The api client method getAddress is made to reject with a 403-style Error
// (matching fetchAPI's "API error: 403"). Other queries are harmless.
const mockGetAddress = vi.fn();

vi.mock('../lib/api', () => {
  const empty = () => Promise.resolve({ data: [], hasMore: false });
  return {
    api: {
      getAddress: () => mockGetAddress(),
      getContract: () => Promise.resolve(null),
      getAddressTokenBalances: () => Promise.resolve([]),
      getToken: () => Promise.resolve(null),
      getAddressTransactions: empty,
      getAddressTransfers: empty,
      getAddressInternalTxs: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 }),
      getAddressLogs: () => Promise.resolve({ data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 }),
    },
  };
});

// useAuth provides a stable unauthenticated context.
vi.mock('../lib/auth', () => ({
  useAuth: () => ({ isAuthenticated: false, did: undefined, loading: false, refresh: vi.fn() }),
}));

// runtimeConfig drives feature flags; default everything to its fallback.
vi.mock('../lib/runtimeConfig', () => ({
  getConfig: (_key: string, fallback: string) => fallback,
}));

// useTokenMap hits the network otherwise; stub it.
vi.mock('../hooks/useTokenMap', () => ({
  useTokenMap: () => ({}),
}));

// Tooltip provider (Radix needs a provider); render children inline.
vi.mock('../components/ui/tooltip', () => ({
  Tooltip: ({ children }: { children: ReactNode }) => children,
  TooltipTrigger: ({ children }: { children: ReactNode; asChild?: boolean }) => children,
  TooltipContent: ({ children }: { children: ReactNode }) => <span>{children}</span>,
}));

import { Address } from './Address';

function renderAt(addr: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/address/${addr}`]}>
        <Routes>
          <Route path="/address/:address" element={<Address />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('Address page — restricted card', () => {
  beforeEach(() => {
    mockGetAddress.mockReset();
  });

  it('renders the "Address restricted" card on a 403', async () => {
    mockGetAddress.mockRejectedValue(new Error('API error: 403'));

    renderAt('0x407d73d8a49eeb85d32cf465507dd71d507100c1');

    await waitFor(() => {
      expect(screen.getByText('Address restricted')).toBeInTheDocument();
    });
    // The explanatory copy mentions privacy controls.
    expect(
      screen.getByText(/protected by privacy controls/i),
    ).toBeInTheDocument();
  });

  it('does NOT render the restricted card on a 404 (shows not-found instead)', async () => {
    // Contract: only 403/500 trigger the restricted interstitial; a 404 falls
    // through to the generic "Address not found" branch.
    mockGetAddress.mockRejectedValue(new Error('API error: 404'));

    renderAt('0x0000000000000000000000000000000000000000');

    await waitFor(() => {
      expect(screen.getByText(/Address not found/i)).toBeInTheDocument();
    });
    expect(screen.queryByText('Address restricted')).not.toBeInTheDocument();
  });
});
