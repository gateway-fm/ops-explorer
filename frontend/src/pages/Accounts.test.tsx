import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API } from '../test/msw';
import { Accounts } from './Accounts';

function account(i: number) {
  return { address: '0x' + (i + 1).toString(16).padStart(40, '3'), balance: String((10 - i) * 1e18), txCount: 100 - i, isContract: i % 3 === 0 };
}

describe('Accounts', () => {
  beforeEach(() => server.resetHandlers());

  function mount() {
    return renderWithProviders(<Accounts />, { path: '/accounts', initialEntries: ['/accounts'] });
  }

  it('renders happy: row count == len(data); "N accounts" header == mocked total', async () => {
    server.use(
      http.get(`${API}/accounts`, () =>
        HttpResponse.json({ data: [account(0), account(1), account(2)], total: 42, page: 1, pageSize: 25, totalPages: 2 }),
      ),
    );
    mount();
    await waitFor(() => expect(screen.getAllByTestId('account-row')).toHaveLength(3));
    expect(screen.getByTestId('account-total-count')).toHaveTextContent('42 accounts');
    // 2 totalPages → pagination renders with "Page 1 of 2".
    expect(screen.getByTestId('pagination-status')).toHaveTextContent('Page 1 of 2');
  });

  it('renders the loading state initially', () => {
    server.use(http.get(`${API}/accounts`, () => new Promise(() => {})));
    mount();
    expect(screen.getByTestId('app-loading')).toBeInTheDocument();
  });

  it('renders the error state on failure', async () => {
    server.use(http.get(`${API}/accounts`, () => HttpResponse.json({ error: 'boom' }, { status: 500 })));
    mount();
    await waitFor(() => expect(screen.getByTestId('app-error')).toBeInTheDocument());
  });
});
