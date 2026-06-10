import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API } from '../test/msw';
import Tokens from './Tokens';

function token(addr: string, over: Record<string, unknown> = {}) {
  return { address: addr, symbol: 'TKN', name: 'Token', decimals: 18, tokenType: 'ERC20', totalSupply: '1000', holderCount: 5, transferCount: 10, blockNumber: 1, createdAt: '2024-01-01T00:00:00Z', ...over };
}

describe('Tokens', () => {
  beforeEach(() => server.resetHandlers());

  function mount() {
    return renderWithProviders(<Tokens />, { path: '/tokens', initialEntries: ['/tokens'] });
  }

  it('renders happy: rows == len(data), filter tabs present', async () => {
    server.use(
      http.get(`${API}/tokens`, () =>
        HttpResponse.json({ data: [token('0x' + 'a'.repeat(40)), token('0x' + 'b'.repeat(40), { tokenType: 'ERC721' })], total: 2, page: 1, pageSize: 25, totalPages: 1 }),
      ),
    );
    mount();
    await waitFor(() => expect(screen.getAllByTestId('token-row')).toHaveLength(2));
    // Filter tabs model role=tab + aria-selected (All selected initially).
    expect(screen.getByTestId('tab-all')).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('tab-ERC20')).toBeInTheDocument();
  });

  it('renders the empty state when no tokens are indexed', async () => {
    server.use(http.get(`${API}/tokens`, () => HttpResponse.json({ data: [], total: 0, page: 1, pageSize: 25, totalPages: 1 })));
    mount();
    await waitFor(() => expect(screen.getByTestId('app-empty')).toBeInTheDocument());
    expect(screen.getByText(/No tokens have been indexed/i)).toBeInTheDocument();
  });

  it('renders the error state when the request fails', async () => {
    server.use(http.get(`${API}/tokens`, () => HttpResponse.json({ error: 'boom' }, { status: 500 })));
    mount();
    await waitFor(() => expect(screen.getByTestId('app-error')).toBeInTheDocument());
  });
});
