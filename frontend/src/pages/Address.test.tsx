import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
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
const mockGetBalances = vi.fn();
const mockUseTokenMap = vi.fn<(addresses: string[]) => Record<string, unknown>>(() => ({}));

vi.mock('../lib/api', () => {
  const empty = () => Promise.resolve({ data: [], hasMore: false });
  return {
    api: {
      getAddress: () => mockGetAddress(),
      getContract: () => Promise.resolve(null),
      getAddressTokenBalances: () => mockGetBalances(),
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
  useTokenMap: (addresses: string[]) => mockUseTokenMap(addresses),
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
    mockGetBalances.mockReset().mockResolvedValue([]);
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

describe('Address page — overview', () => {
  const addr = '0x407d73d8a49eeb85d32cf465507dd71d507100c1';
  beforeEach(() => {
    mockGetAddress.mockReset().mockResolvedValue({
      address: addr, balance: '0', txCount: 0, isContract: false, tokenTransferCount: 0, internalTxCount: 0,
    });
    mockGetBalances.mockReset();
    // Every looked-up token is a $2 ERC-20 with no decimals.
    mockUseTokenMap.mockReset().mockImplementation((addresses) =>
      Object.fromEntries(addresses.map((a) => [a.toLowerCase(), { address: a, symbol: 'TKN', decimals: 0, usdPrice: 2, tokenType: 'ERC20' }])),
    );
  });

  const holdings = (n: number) => Array.from({ length: n }, (_, i) => ({
    address: addr, tokenAddress: '0x' + (i + 1).toString(16).padStart(40, '0'), blockNumber: 1, balance: '1',
  }));

  it('bounds the token metadata lookups to a few holdings and hides the partial USD total', async () => {
    mockGetBalances.mockResolvedValue(holdings(25));

    renderAt(addr);

    await waitFor(() => expect(screen.getByText('+22 more')).toBeInTheDocument());
    const largest = Math.max(...mockUseTokenMap.mock.calls.map(([a]) => a.length));
    expect(largest).toBeGreaterThanOrEqual(3);
    expect(largest).toBeLessThanOrEqual(10);
    expect(screen.getByTestId('address-overview')).not.toHaveTextContent(/Total/);
  });

  it('shows the USD total when every holding was looked up', async () => {
    mockGetBalances.mockResolvedValue(holdings(5));

    renderAt(addr);

    await waitFor(() => expect(screen.getByText('+2 more')).toBeInTheDocument());
    expect(screen.getByTestId('address-overview')).toHaveTextContent('Total $10');
  });

  it('shows an unavailable state when the balances request fails', async () => {
    mockGetBalances.mockRejectedValue(new Error('API error: 503'));

    renderAt(addr);

    await waitFor(() => {
      expect(screen.getByTestId('address-overview')).toHaveTextContent(/token balances unavailable/i);
    });
    fireEvent.click(screen.getByTestId('tab-tokens'));
    expect(screen.getByText(/token balances are unavailable/i)).toBeInTheDocument();
  });
});
