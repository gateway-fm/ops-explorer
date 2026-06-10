import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from './ui/tooltip';
import { CallTraceTree } from './CallTraceTree';
import type { Transaction, InternalTransaction } from '../lib/api';

// Renders the trace tree (PLAN §6): one trace-row per call (root + children),
// and the Raw JSON toggle.

const tx: Transaction = {
  hash: '0x' + 'a'.repeat(64), blockNumber: 1, txIndex: 0, from: '0x' + '1'.repeat(40), to: '0x' + '2'.repeat(40),
  value: '0', gasUsed: 21000, gasPrice: 1, inputData: '', status: 1, createdAt: '2024-01-01T00:00:00Z',
};

const internalTxs: InternalTransaction[] = [
  { id: 1, txHash: tx.hash, blockNumber: 1, traceAddress: '0', from: '0x' + '2'.repeat(40), to: '0x' + '3'.repeat(40), value: '0', callType: 'call' },
  { id: 2, txHash: tx.hash, blockNumber: 1, traceAddress: '0_0', from: '0x' + '3'.repeat(40), to: '0x' + '4'.repeat(40), value: '0', callType: 'staticcall' },
];

function r() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter>
          <CallTraceTree tx={tx} internalTxs={internalTxs} />
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

describe('CallTraceTree', () => {
  it('renders the call count and a trace-row per node (root + 2 internal)', () => {
    r();
    expect(screen.getByText(/2 internal calls/)).toBeInTheDocument();
    // root + 2 children = 3 rows.
    expect(screen.getAllByTestId('trace-row')).toHaveLength(3);
  });

  it('toggles to Raw JSON view', async () => {
    r();
    await userEvent.click(screen.getByRole('button', { name: 'Raw JSON' }));
    expect(screen.getByRole('button', { name: 'Tree view' })).toBeInTheDocument();
    // Raw view drops the per-node rows.
    expect(screen.queryAllByTestId('trace-row')).toHaveLength(0);
  });
});
