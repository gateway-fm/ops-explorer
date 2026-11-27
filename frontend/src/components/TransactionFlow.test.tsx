import { describe, it, expect } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../test/render';
import { TransactionFlow } from './TransactionFlow';
import type { FlowStep, TransactionFlow as Flow } from '../lib/api';

const SENDER = '0x' + '1'.repeat(40);
const VAULT = '0x' + '3'.repeat(40);
const USDC = '0x' + '4'.repeat(40);
const ORACLE = '0x' + '6'.repeat(40);

const participants: Flow['participants'] = [
  { address: SENDER, label: '0x1111…1111', kind: 'account' },
  { address: VAULT, label: 'Vault', kind: 'contract', verified: true },
  { address: USDC, label: 'USDC', kind: 'token' },
  { address: ORACLE, label: '0x6666…6666', kind: 'contract' },
];

const steps: FlowStep[] = [
  { index: 0, kind: 'call', depth: 0, from: SENDER, to: VAULT, method: 'deposit', decoded: 'verified', signature: 'deposit(address,uint256)', args: [{ name: 'amount', type: 'uint256', value: '5000000' }], text: 'The sender called deposit on Vault' },
  { index: 1, kind: 'call', depth: 1, from: VAULT, to: ORACLE, method: 'latestAnswer', readOnly: true, repeat: 30, text: 'Vault read latestAnswer from contract 0x6666…6666 (30 times in a row)' },
  { index: 2, kind: 'token_transfer', depth: 0, from: SENDER, to: VAULT, token: { address: USDC, symbol: 'USDC', raw: '5000000', amount: '5' }, method: 'Transfer', text: 'The sender sent 5 USDC to Vault' },
  { index: 3, kind: 'event', depth: 0, from: VAULT, to: VAULT, method: 'Deposited', decoded: 'verified', text: 'Vault reported Deposited' },
];

const flow: Flow = {
  txHash: '0x' + 'a'.repeat(64), status: 'success', currency: 'tTEST', hasCallData: true,
  participants, steps, totalSteps: 4, truncated: false,
  summary: 'The sender called deposit on Vault. It led to 31 internal calls.',
  notes: ['Events are shown inside the call that emitted them.'],
};

function mount(f: Flow = flow) {
  return renderWithProviders(<TransactionFlow flow={f} />);
}

describe('TransactionFlow', () => {
  it('shows the summary, notes and the first step', () => {
    mount();
    expect(screen.getByTestId('flow-summary')).toHaveTextContent('It led to 31 internal calls');
    expect(screen.getByText('Events are shown inside the call that emitted them.')).toBeInTheDocument();
    expect(screen.getByTestId('flow-position')).toHaveTextContent('Step 1 of 3');
    expect(screen.getByTestId('flow-step-text')).toHaveTextContent('The sender called deposit on Vault');
    expect(screen.getByText('deposit(address,uint256)')).toBeInTheDocument();
    expect(screen.getByTestId('flow-back')).toBeDisabled();
  });

  it('walks forward and back with buttons and arrow keys, highlighting the row', async () => {
    mount();
    await userEvent.click(screen.getByTestId('flow-next'));
    expect(screen.getByTestId('flow-step-text')).toHaveTextContent('The sender sent 5 USDC to Vault');
    const rows = screen.getAllByTestId('flow-row');
    expect(rows[1]).toHaveAttribute('data-active', 'true');

    screen.getByTestId('flow-diagram').focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(screen.getByTestId('flow-step-text')).toHaveTextContent('Vault reported Deposited');
    expect(screen.getByTestId('flow-next')).toBeDisabled();
    await userEvent.keyboard('{ArrowLeft}{ArrowLeft}');
    expect(screen.getByTestId('flow-position')).toHaveTextContent('Step 1 of 3');
  });

  it('hides technical steps by default and keeps the selected step when toggled', async () => {
    mount();
    expect(screen.getAllByTestId('flow-row')).toHaveLength(3);
    expect(screen.queryByText('0x6666…6666')).not.toBeInTheDocument(); // oracle lane hidden too
    await userEvent.click(screen.getByTestId('flow-next')); // the USDC transfer
    expect(screen.getByText('Show technical steps (1)')).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('flow-show-technical'));
    expect(screen.getAllByTestId('flow-row')).toHaveLength(4);
    expect(screen.getByTestId('flow-step-text')).toHaveTextContent('5 USDC');
    expect(screen.getByTestId('flow-position')).toHaveTextContent('Step 3 of 4');
  });

  it('selects a step by clicking its row, and offers a list view', async () => {
    mount();
    await userEvent.click(screen.getAllByTestId('flow-row')[2]);
    expect(screen.getByTestId('flow-step-text')).toHaveTextContent('Vault reported Deposited');
    await userEvent.click(screen.getByTestId('flow-view-list'));
    const list = screen.getByTestId('flow-list');
    expect(within(list).getAllByRole('button')).toHaveLength(3);
    expect(within(list).getByRole('button', { current: 'step' })).toHaveTextContent('Vault reported Deposited');
    await userEvent.click(within(list).getAllByRole('button')[0]);
    expect(screen.getByTestId('flow-position')).toHaveTextContent('Step 1 of 3');
  });

  it('draws a capped 250-step flow without trouble', () => {
    const many: FlowStep[] = Array.from({ length: 250 }, (_, i) => ({
      index: i, kind: 'call', depth: 1, from: VAULT, to: i % 2 ? USDC : ORACLE, method: `fn${i}`, decoded: 'verified', text: `Step ${i}`,
    }));
    const start = performance.now();
    mount({ ...flow, steps: many, totalSteps: 1200, truncated: true, notes: ['Showing the first 250 of 1200 steps.'] });
    expect(performance.now() - start).toBeLessThan(3000);
    expect(screen.getAllByTestId('flow-row')).toHaveLength(250);
    expect(screen.getByText('Showing the first 250 of 1200 steps.')).toBeInTheDocument();
  });
});
