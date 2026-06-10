import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API, makeBlock, makeTx } from '../test/msw';
import { BlockDetail } from './BlockDetail';

describe('BlockDetail', () => {
  beforeEach(() => server.resetHandlers());

  function mount(n = 5) {
    return renderWithProviders(<BlockDetail />, { path: '/block/:number', initialEntries: [`/block/${n}`] });
  }

  it('renders happy: title + tabs with counts equal to mocked array lengths', async () => {
    server.use(
      http.get(`${API}/blocks/5`, () =>
        HttpResponse.json({ block: makeBlock(5, { transactionCount: 3 }), transactions: [makeTx(1), makeTx(2), makeTx(3)] }),
      ),
      http.get(`${API}/blocks/5/internal`, () =>
        HttpResponse.json([{ id: 1, txHash: '0x' + 'e'.repeat(64), blockNumber: 5, traceAddress: '0', from: '0x' + '1'.repeat(40), to: '0x' + '2'.repeat(40), value: '0', callType: 'call' }]),
      ),
    );
    mount(5);

    await waitFor(() => expect(screen.getByTestId('page-header-title')).toHaveTextContent('Block #5'));
    // Counter assertion in the tab labels.
    expect(screen.getByTestId('tab-transactions')).toHaveTextContent('Transactions (3)');
    expect(screen.getByTestId('tab-internal')).toHaveTextContent('Internal Txns (1)');
  });

  it('switches to the Transactions tab and renders the rows', async () => {
    server.use(
      http.get(`${API}/blocks/7`, () => HttpResponse.json({ block: makeBlock(7), transactions: [makeTx(1), makeTx(2)] })),
      http.get(`${API}/blocks/7/internal`, () => HttpResponse.json([])),
    );
    mount(7);
    await waitFor(() => expect(screen.getByTestId('tab-transactions')).toBeInTheDocument());

    await userEvent.click(screen.getByTestId('tab-transactions'));
    expect(screen.getByTestId('tab-transactions')).toHaveAttribute('aria-selected', 'true');
    await waitFor(() => expect(screen.getAllByTestId('tx-row').length).toBe(2));
  });

  it('renders the error state when the block is not found', async () => {
    server.use(http.get(`${API}/blocks/999`, () => HttpResponse.json({ error: 'not found' }, { status: 404 })));
    mount(999);
    await waitFor(() => expect(screen.getByTestId('app-error')).toBeInTheDocument());
    expect(screen.getByText('Block not found')).toBeInTheDocument();
  });
});
