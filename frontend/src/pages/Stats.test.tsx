import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { type ReactNode } from 'react';

const mockGetConfig = vi.fn();
vi.mock('../lib/runtimeConfig', () => ({
  getConfig: (key: string, fallback?: string) => mockGetConfig(key, fallback),
}));

const mockGetStats = vi.fn();
const mockGetChartLine = vi.fn();
vi.mock('../lib/api', () => ({
  api: {
    getStats: () => mockGetStats(),
    getChartLine: (...args: unknown[]) => mockGetChartLine(...args),
  },
}));

// recharts renders nothing meaningful in happy-dom (no layout); stub it so the
// standalone render doesn't warn/throw on a zero-size ResponsiveContainer.
vi.mock('recharts', () => {
  const Passthrough = ({ children }: { children?: ReactNode }) => <div>{children}</div>;
  const Noop = () => null;
  return {
    ResponsiveContainer: Passthrough,
    AreaChart: Passthrough,
    Area: Noop,
    XAxis: Noop,
    YAxis: Noop,
    Tooltip: Noop,
  };
});

import Stats from './Stats';

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
  return render(<Stats />, { wrapper });
}

describe('Stats (privacy gate)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetStats.mockResolvedValue({ totalBlocks: 1, totalTransactions: 2, totalAddresses: 3, avgBlockTime: 1 });
    mockGetChartLine.mockResolvedValue({ chart: [] });
  });

  it('privacy: renders FeatureUnavailable and never calls api.getChartLine', async () => {
    setPrivacy(true);
    renderPage();
    expect(screen.getByText(/Charts is not available in this deployment/i)).toBeInTheDocument();
    await Promise.resolve();
    expect(mockGetChartLine).not.toHaveBeenCalled();
    expect(mockGetStats).not.toHaveBeenCalled();
  });

  it('standalone: renders the real page and calls api.getChartLine', async () => {
    setPrivacy(false);
    renderPage();
    await waitFor(() => expect(mockGetChartLine).toHaveBeenCalled());
    expect(screen.queryByText(/not available in this deployment/i)).not.toBeInTheDocument();
  });
});
