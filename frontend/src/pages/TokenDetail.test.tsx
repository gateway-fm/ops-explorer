import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API } from '../test/msw';
import TokenDetail from './TokenDetail';

describe('TokenDetail', () => {
  const addr = '0x' + 'a'.repeat(40);
  beforeEach(() => server.resetHandlers());

  function mount() {
    return renderWithProviders(<TokenDetail />, { path: '/token/:address', initialEntries: [`/token/${addr}`] });
  }

  it('renders hero + StatBar (4 tiles) + tab badges == mocked counts', async () => {
    server.use(
      http.get(`${API}/tokens/${addr}`, () =>
        HttpResponse.json({ address: addr, symbol: 'MK', name: 'Mock', decimals: 18, tokenType: 'ERC20', totalSupply: '1000000000000000000000000', holderCount: 88, transferCount: 99, blockNumber: 1, createdAt: '2024-01-01T00:00:00Z' }),
      ),
      http.get(`${API}/tokens/${addr}/transfers`, () => HttpResponse.json({ data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 })),
    );
    mount();
    await waitFor(() => expect(screen.getAllByTestId('stat-tile')).toHaveLength(4));
    expect(screen.getByTestId('tab-count-transfers')).toHaveTextContent('99');
    expect(screen.getByTestId('tab-count-holders')).toHaveTextContent('88');
  });

  it('renders the restricted interstitial on a 403', async () => {
    server.use(http.get(`${API}/tokens/${addr}`, () => HttpResponse.json({ error: 'forbidden' }, { status: 403 })));
    mount();
    await waitFor(() => expect(screen.getByTestId('restricted-state')).toBeInTheDocument());
    expect(screen.getByText('Token Restricted')).toBeInTheDocument();
  });

  it('renders not-found on a 404', async () => {
    server.use(http.get(`${API}/tokens/${addr}`, () => HttpResponse.json({ error: 'nope' }, { status: 404 })));
    mount();
    await waitFor(() => expect(screen.getByTestId('app-error')).toBeInTheDocument());
    expect(screen.getByText('Token not found.')).toBeInTheDocument();
  });
});
