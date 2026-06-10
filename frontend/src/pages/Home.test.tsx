import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API, makeBlock, makeTx } from '../test/msw';
import { Home } from './Home';

// Home renders happy / empty (PLAN §6) via MSW. The chart child fetches
// tx-history through the same server; default handler returns [].

describe('Home', () => {
  beforeEach(() => server.resetHandlers());

  it('renders 4 StatCards reflecting mocked /api/stats and the latest lists', async () => {
    server.use(
      http.get(`${API}/stats`, () =>
        HttpResponse.json({ totalBlocks: 4321, totalTransactions: 9999, totalAddresses: 12, avgBlockTime: 2, privacyEnabled: false }),
      ),
      http.get(`${API}/blocks`, () => HttpResponse.json({ data: [makeBlock(1000), makeBlock(999)], hasMore: false })),
      http.get(`${API}/transactions`, () => HttpResponse.json({ data: [makeTx(1)], total: 1, page: 1, pageSize: 25, totalPages: 1 })),
    );

    renderWithProviders(<Home />);

    await waitFor(() => expect(screen.getAllByTestId('stat-card')).toHaveLength(4), { timeout: 3000 });
    // Counter assertion: the mocked totals are rendered (locale-formatted).
    expect(screen.getByText('4,321')).toBeInTheDocument();
    expect(screen.getByText('9,999')).toBeInTheDocument();
    // Lists render rows.
    await waitFor(() => expect(screen.getAllByTestId('block-row').length).toBeGreaterThan(0), { timeout: 3000 });
    expect(screen.getAllByTestId('tx-row').length).toBe(1);
  });

  it('shows empty states when lists are empty', async () => {
    server.use(
      http.get(`${API}/blocks`, () => HttpResponse.json({ data: [], hasMore: false })),
      http.get(`${API}/transactions`, () => HttpResponse.json({ data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 })),
    );

    renderWithProviders(<Home />);

    await waitFor(() => expect(screen.getByText('No blocks yet')).toBeInTheDocument());
    expect(screen.getByText('No transactions yet')).toBeInTheDocument();
    expect(screen.queryAllByTestId('block-row')).toHaveLength(0);
  });
});
