import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { type ReactNode } from 'react';

// getConfig drives features().gasTracker (privacy → false). features() reads it
// at render time, so a per-test return value is enough — no module reset needed.
const mockGetConfig = vi.fn();
vi.mock('../lib/runtimeConfig', () => ({
  getConfig: (key: string, fallback?: string) => mockGetConfig(key, fallback),
}));

const mockGetGasPrices = vi.fn();
vi.mock('../lib/api', () => ({
  api: { getGasPrices: () => mockGetGasPrices() },
}));

import { GasTracker } from './GasTracker';

function setPrivacy(on: boolean) {
  mockGetConfig.mockImplementation((key: string) =>
    key === 'VITE_PRIVACY_MODE' && on ? 'true' : '',
  );
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
  return render(<GasTracker />, { wrapper });
}

describe('GasTracker (privacy gate)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetGasPrices.mockResolvedValue({
      slow: { price: 1 }, normal: { price: 2 }, fast: { price: 3 },
      updatedAt: new Date().toISOString(),
    });
  });

  it('privacy: renders FeatureUnavailable and never calls api.getGasPrices', async () => {
    setPrivacy(true);
    renderPage();
    expect(screen.getByText(/Gas Tracker is not available in this deployment/i)).toBeInTheDocument();
    // Give any (erroneously subscribed) query a tick to fire.
    await Promise.resolve();
    expect(mockGetGasPrices).not.toHaveBeenCalled();
  });

  it('standalone: renders the real page and calls api.getGasPrices', async () => {
    setPrivacy(false);
    renderPage();
    await waitFor(() => expect(mockGetGasPrices).toHaveBeenCalled());
    expect(screen.queryByText(/not available in this deployment/i)).not.toBeInTheDocument();
  });
});
