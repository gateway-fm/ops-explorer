import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { type ReactNode } from 'react';

// M1 (RD-1063): the Layout header pollers (/price 60s, /gas 15s) run on EVERY
// page, so in privacy mode they must be gated off (enabled: features().gasTracker).
// getConfig drives features(); set it before importing the module because
// mobileNavItems is built at module load.
const mockGetConfig = vi.fn();
vi.mock('../lib/runtimeConfig', () => ({
  getConfig: (key: string, fallback?: string) => mockGetConfig(key, fallback),
}));

const mockGetPrice = vi.fn();
const mockGetGasPrices = vi.fn();
const mockGetStats = vi.fn();
const mockSearchSuggestions = vi.fn();
vi.mock('../lib/api', () => ({
  api: {
    getPrice: () => mockGetPrice(),
    getGasPrices: () => mockGetGasPrices(),
    getStats: () => mockGetStats(),
    searchSuggestions: (...a: unknown[]) => mockSearchSuggestions(...a),
  },
}));

// Stub heavy children — irrelevant to the poller assertion.
vi.mock('./NavDropdown', () => ({ NavDropdown: () => null }));
vi.mock('./NetworkMenu', () => ({ MobileNetworkList: () => null, NetworkMenu: () => null }));
vi.mock('./ImpersonationBanner', () => ({ ImpersonationBanner: () => null }));
vi.mock('./MetaMask', () => ({ MetaMaskFox: () => null }));
vi.mock('../lib/metamask', () => ({ addNetworkToMetaMask: vi.fn() }));
vi.mock('../hooks/usePrivacyEnabled', () => ({ usePrivacyEnabled: () => false }));
vi.mock('./ui/tooltip', () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

function setPrivacy(on: boolean) {
  mockGetConfig.mockImplementation((key: string, fallback?: string) => {
    if (key === 'VITE_PRIVACY_MODE') return on ? 'true' : '';
    return fallback ?? '';
  });
}

async function renderLayout() {
  vi.resetModules();
  const { Layout } = await import('./Layout');
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <Layout />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('Layout header pollers (privacy gate, M1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetPrice.mockResolvedValue({ price: 1 });
    mockGetGasPrices.mockResolvedValue({ normal: { price: 2 } });
    mockGetStats.mockResolvedValue({});
  });

  it('privacy: does NOT call api.getPrice or api.getGasPrices', async () => {
    setPrivacy(true);
    await renderLayout();
    // Let any (erroneously enabled) queries flush.
    await new Promise((r) => setTimeout(r, 0));
    expect(mockGetPrice).not.toHaveBeenCalled();
    expect(mockGetGasPrices).not.toHaveBeenCalled();
  });

  it('standalone: calls api.getPrice and api.getGasPrices', async () => {
    setPrivacy(false);
    await renderLayout();
    await waitFor(() => expect(mockGetGasPrices).toHaveBeenCalled());
    await waitFor(() => expect(mockGetPrice).toHaveBeenCalled());
  });
});
