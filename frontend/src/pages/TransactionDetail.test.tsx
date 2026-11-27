import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useNavigate } from 'react-router-dom';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API, makeTx } from '../test/msw';
import { TransactionDetail } from './TransactionDetail';

// Focus: the tab-VISIBILITY logic (Flow always; Logs/Trace only when data exists),
// plus restricted + not-found states (PLAN §6).

describe('TransactionDetail', () => {
  const hash = '0x' + 'a'.repeat(64);
  beforeEach(() => server.resetHandlers());

  function mount(h = hash) {
    return renderWithProviders(<TransactionDetail />, { path: '/tx/:hash', initialEntries: [`/tx/${h}`] });
  }

  function txEndpoints(over: { logs?: unknown[]; internal?: unknown[]; transfers?: unknown[]; onFlow?: () => void } = {}) {
    return [
      http.get(`${API}/transactions/${hash}`, () => HttpResponse.json(makeTx(1, { hash }))),
      http.get(`${API}/transactions/${hash}/transfers`, () => HttpResponse.json(over.transfers ?? [])),
      http.get(`${API}/transactions/${hash}/logs`, () => HttpResponse.json(over.logs ?? [])),
      http.get(`${API}/transactions/${hash}/internal`, () => HttpResponse.json(over.internal ?? [])),
      http.get(`${API}/transactions/${hash}/flow`, () => {
        over.onFlow?.();
        return HttpResponse.json({
          txHash: hash, status: 'success', currency: 'ETH', hasCallData: false, totalSteps: 1, truncated: false,
          summary: 'The sender sent 1 ETH to 0x2222…2222.',
          participants: [
            { address: '0x' + '1'.repeat(40), label: '0x1111…1111', kind: 'account' },
            { address: '0x' + '2'.repeat(40), label: '0x2222…2222', kind: 'account' },
          ],
          steps: [{ index: 0, kind: 'value_transfer', depth: 0, from: '0x' + '1'.repeat(40), to: '0x' + '2'.repeat(40), value: '1', amount: '1', text: 'The sender sent 1 ETH to 0x2222…2222' }],
        });
      }),
    ];
  }

  it('shows only Overview and Flow when there are neither logs nor internal txns', async () => {
    server.use(...txEndpoints());
    mount();
    await waitFor(() => expect(screen.getByTestId('tx-status')).toBeInTheDocument());
    expect(screen.getByTestId('tab-flow')).toBeInTheDocument();
    expect(screen.queryByTestId('tab-logs')).not.toBeInTheDocument();
    expect(screen.queryByTestId('tab-trace')).not.toBeInTheDocument();
  });

  it('loads the flow only when the Flow tab opens', async () => {
    let flowCalls = 0;
    server.use(...txEndpoints({ onFlow: () => { flowCalls++; } }));
    mount();
    await waitFor(() => expect(screen.getByTestId('tx-status')).toBeInTheDocument());
    expect(flowCalls).toBe(0);
    await userEvent.click(screen.getByTestId('tab-flow'));
    await waitFor(() => expect(screen.getByTestId('flow-step-text')).toHaveTextContent('The sender sent 1 ETH'));
    expect(flowCalls).toBe(1);
  });

  it('shows an error state when the flow cannot be loaded', async () => {
    // The first matching handler wins, so the failing flow goes before the defaults.
    server.use(http.get(`${API}/transactions/${hash}/flow`, () => HttpResponse.json({}, { status: 500 })), ...txEndpoints());
    mount();
    await waitFor(() => expect(screen.getByTestId('tx-status')).toBeInTheDocument());
    await userEvent.click(screen.getByTestId('tab-flow'));
    await waitFor(() => expect(screen.getByText('The flow for this transaction is not available')).toBeInTheDocument());
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

  it('draws transfers a page at a time and looks up only the tokens on screen', async () => {
    const tokenOf = (i: number) => '0x' + i.toString(16).padStart(40, '0');
    const transfers = Array.from({ length: 120 }, (_, i) => ({
      txHash: hash, logIndex: i, blockNumber: 1, tokenAddress: tokenOf(i + 1),
      from: '0x' + '1'.repeat(40), to: '0x' + '2'.repeat(40), value: '1', tokenType: 'ERC20',
    }));
    const looked = new Set<string>();
    server.use(
      ...txEndpoints({ transfers }),
      http.get(`${API}/tokens/:addr`, ({ params }) => {
        looked.add(String(params.addr).toLowerCase());
        return HttpResponse.json({ address: params.addr, symbol: 'T', decimals: 0 });
      }),
      http.get(`${API}/addresses/:addr/contract`, () => new HttpResponse(null, { status: 404 })),
    );
    mount();
    await waitFor(() => expect(screen.getAllByTestId('transfer-row')).toHaveLength(50));
    expect(screen.getByTestId('transfers-show-more')).toHaveTextContent('Show 50 more (70 not shown)');
    await waitFor(() => expect(looked.size).toBe(50));
    expect(looked.has(tokenOf(51))).toBe(false);

    await userEvent.click(screen.getByTestId('transfers-show-more'));
    await userEvent.click(screen.getByTestId('transfers-show-more'));
    expect(screen.getAllByTestId('transfer-row')).toHaveLength(120);
    expect(screen.queryByTestId('transfers-show-more')).not.toBeInTheDocument();
  });

  const TRANSFER_TOPIC = '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef';
  const manyLogs = (n: number, h = hash) => Array.from({ length: n }, (_, i) => ({
    id: i, txHash: h, logIndex: i, address: '0x' + '3'.repeat(40), topic0: TRANSFER_TOPIC,
    topic1: '0x' + '0'.repeat(24) + '1'.repeat(40), topic2: '0x' + '0'.repeat(24) + '2'.repeat(40), topic3: null,
    data: '0x' + '0'.repeat(63) + '1', blockNumber: 1,
  }));
  const noContract = () => http.get(`${API}/addresses/:addr/contract`, () => new HttpResponse(null, { status: 404 }));

  it('draws logs a page at a time', async () => {
    server.use(...txEndpoints({ logs: manyLogs(70) }), noContract());
    mount();
    await userEvent.click(await screen.findByTestId('tab-logs'));
    await waitFor(() => expect(screen.getAllByTestId('log-card')).toHaveLength(50));
    await userEvent.click(screen.getByTestId('logs-show-more'));
    expect(screen.getAllByTestId('log-card')).toHaveLength(70);
    expect(screen.queryByTestId('logs-show-more')).not.toBeInTheDocument();
  });

  it('draws internal calls a page at a time', async () => {
    const internal = Array.from({ length: 120 }, (_, i) => ({
      id: i, txHash: hash, blockNumber: 1, traceAddress: String(i), from: '0x' + '1'.repeat(40), to: '0x' + '2'.repeat(40), value: '0', callType: 'call',
    }));
    server.use(...txEndpoints({ internal }), noContract());
    mount();
    await userEvent.click(await screen.findByTestId('tab-trace'));
    await waitFor(() => expect(screen.getAllByTestId('trace-row')).toHaveLength(50));
    expect(screen.getByTestId('trace-show-more')).toHaveTextContent('Show 50 more (71 not shown)');
    await userEvent.click(screen.getByTestId('trace-show-more'));
    await userEvent.click(screen.getByTestId('trace-show-more'));
    expect(screen.getAllByTestId('trace-row')).toHaveLength(121);
  });

  it('starts the next transaction on one page again', async () => {
    const other = '0x' + 'b'.repeat(64);
    function GoToOther() {
      const navigate = useNavigate();
      return <button type="button" onClick={() => navigate(`/tx/${other}`)}>other</button>;
    }
    server.use(
      ...txEndpoints({ logs: manyLogs(120) }),
      http.get(`${API}/transactions/${other}`, () => HttpResponse.json(makeTx(2, { hash: other }))),
      http.get(`${API}/transactions/${other}/transfers`, () => HttpResponse.json([])),
      http.get(`${API}/transactions/${other}/logs`, () => HttpResponse.json(manyLogs(120, other))),
      http.get(`${API}/transactions/${other}/internal`, () => HttpResponse.json([])),
      noContract(),
    );
    renderWithProviders(<><TransactionDetail /><GoToOther /></>, { path: '/tx/:hash', initialEntries: [`/tx/${hash}`] });
    await userEvent.click(await screen.findByTestId('tab-logs'));
    await userEvent.click(await screen.findByTestId('logs-show-more'));
    expect(screen.getAllByTestId('log-card')).toHaveLength(100);

    await userEvent.click(screen.getByRole('button', { name: 'other' }));
    await waitFor(() => expect(screen.getByTestId('tab-overview')).toHaveAttribute('aria-selected', 'true'));
    await userEvent.click(await screen.findByTestId('tab-logs'));
    await waitFor(() => expect(screen.getAllByTestId('log-card')).toHaveLength(50));
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
