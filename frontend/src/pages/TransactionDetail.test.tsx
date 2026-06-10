import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API, makeTx } from '../test/msw';
import { TransactionDetail } from './TransactionDetail';

// Focus: the tab-VISIBILITY logic (tabs only render when logs/internal exist),
// plus restricted + not-found states (PLAN §6).

describe('TransactionDetail', () => {
  const hash = '0x' + 'a'.repeat(64);
  beforeEach(() => server.resetHandlers());

  function mount(h = hash) {
    return renderWithProviders(<TransactionDetail />, { path: '/tx/:hash', initialEntries: [`/tx/${h}`] });
  }

  function txEndpoints(over: { logs?: unknown[]; internal?: unknown[] } = {}) {
    return [
      http.get(`${API}/transactions/${hash}`, () => HttpResponse.json(makeTx(1, { hash }))),
      http.get(`${API}/transactions/${hash}/transfers`, () => HttpResponse.json([])),
      http.get(`${API}/transactions/${hash}/logs`, () => HttpResponse.json(over.logs ?? [])),
      http.get(`${API}/transactions/${hash}/internal`, () => HttpResponse.json(over.internal ?? [])),
    ];
  }

  it('renders no tabs when there are neither logs nor internal txns', async () => {
    server.use(...txEndpoints());
    mount();
    await waitFor(() => expect(screen.getByTestId('tx-status')).toBeInTheDocument());
    expect(screen.queryByTestId('tab-logs')).not.toBeInTheDocument();
    expect(screen.queryByTestId('tab-trace')).not.toBeInTheDocument();
  });

  it('renders Logs + Trace tabs only when their data is present, and switches', async () => {
    server.use(
      ...txEndpoints({
        logs: [{ id: 1, txHash: hash, logIndex: 0, address: '0x' + '3'.repeat(40), topic0: '0x' + 'd'.repeat(64), topic1: null, topic2: null, topic3: null, data: '0x', blockNumber: 1 }],
        internal: [{ id: 1, txHash: hash, blockNumber: 1, traceAddress: '0', from: '0x' + '1'.repeat(40), to: '0x' + '2'.repeat(40), value: '0', callType: 'call' }],
      }),
    );
    mount();
    await waitFor(() => expect(screen.getByTestId('tab-logs')).toBeInTheDocument());
    expect(screen.getByTestId('tab-trace')).toBeInTheDocument();

    await userEvent.click(screen.getByTestId('tab-trace'));
    expect(screen.getByTestId('tab-trace')).toHaveAttribute('aria-selected', 'true');
    await waitFor(() => expect(screen.getAllByTestId('trace-row').length).toBeGreaterThan(0));
  });

  it('renders the restricted interstitial on a 403', async () => {
    server.use(http.get(`${API}/transactions/${hash}`, () => HttpResponse.json({ error: 'forbidden' }, { status: 403 })));
    mount();
    await waitFor(() => expect(screen.getByTestId('restricted-state')).toBeInTheDocument());
    expect(screen.getByText('Transaction Restricted')).toBeInTheDocument();
  });

  it('renders the not-found error on a generic failure', async () => {
    server.use(http.get(`${API}/transactions/${hash}`, () => HttpResponse.json({ error: 'nope' }, { status: 404 })));
    mount();
    await waitFor(() => expect(screen.getByTestId('app-error')).toBeInTheDocument());
    expect(screen.getByText('Transaction not found')).toBeInTheDocument();
  });
});
