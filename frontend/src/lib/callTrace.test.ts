import { describe, it, expect } from 'vitest';
import { buildTree, buildRoot, compareTraceAddr } from './callTrace';
import type { InternalTransaction, Transaction } from './api';

function tx(overrides: Partial<Transaction> = {}): Transaction {
  return {
    hash: '0xtx',
    blockNumber: 1,
    txIndex: 0,
    from: '0xfrom',
    to: '0xto',
    value: '1000',
    gasUsed: 21000,
    gasPrice: 1,
    inputData: '',
    status: 1,
    createdAt: '',
    ...overrides,
  };
}

function itx(traceAddress: string, overrides: Partial<InternalTransaction> = {}): InternalTransaction {
  return {
    id: 0,
    txHash: '0xtx',
    blockNumber: 1,
    traceAddress,
    from: '0xa',
    to: '0xb',
    value: '0',
    callType: 'call',
    ...overrides,
  };
}

describe('compareTraceAddr', () => {
  it('orders ancestors before descendants (pre-order)', () => {
    const input = ['3', '2,0', '0', '2', '1'];
    expect([...input].sort(compareTraceAddr)).toEqual(['0', '1', '2', '2,0', '3']);
  });

  it('compares segments numerically, not lexicographically', () => {
    // lexicographic would put "10" before "2"
    expect([...['10', '2', '1']].sort(compareTraceAddr)).toEqual(['1', '2', '10']);
  });

  it('nests deeply in the right order', () => {
    const input = ['1', '0,1', '0', '0,0', '0,0,0'];
    expect([...input].sort(compareTraceAddr)).toEqual(['0', '0,0', '0,0,0', '0,1', '1']);
  });
});

describe('buildRoot', () => {
  it('synthesizes a call root from a normal tx', () => {
    const root = buildRoot(tx({ from: '0xee', to: '0xcc', value: '5', inputData: 'deadbeef' }));
    expect(root.traceAddress).toBe('');
    expect(root.callType).toBe('call');
    expect(root.from).toBe('0xee');
    expect(root.to).toBe('0xcc');
    expect(root.value).toBe('5');
    expect(root.input).toBe('0xdeadbeef'); // 0x prefix added
    expect(root.error).toBeUndefined();
  });

  it('treats a contract-creation tx as a create frame pointing at the new contract', () => {
    const root = buildRoot(tx({ to: null, contractAddress: '0xnew' }));
    expect(root.callType).toBe('create');
    expect(root.to).toBe('0xnew');
  });

  it('carries an error for a failed tx', () => {
    expect(buildRoot(tx({ status: 0, revertReason: 'boom' })).error).toBe('boom');
    expect(buildRoot(tx({ status: 0 })).error).toBe('reverted');
  });

  it('does not double-prefix input that already has 0x', () => {
    expect(buildRoot(tx({ inputData: '0xabcd' })).input).toBe('0xabcd');
  });
});

describe('buildTree', () => {
  it('builds the synthesized root with no children when there are no internal txs', () => {
    const root = buildTree(tx(), []);
    expect(root.depth).toBe(0);
    expect(root.trace.traceAddress).toBe('');
    expect(root.children).toHaveLength(0);
  });

  it('nests internal calls under the synthesized root by trace_address', () => {
    // shape from the TraceDemo.run() fixture: 0, 1, 2, 2,0, 3
    const root = buildTree(tx(), [
      itx('0'),
      itx('1', { callType: 'staticcall' }),
      itx('2'),
      itx('2,0'),
      itx('3', { callType: 'staticcall', error: 'execution reverted' }),
    ]);

    expect(root.children.map((c) => c.trace.traceAddress)).toEqual(['0', '1', '2', '3']);
    expect(root.children.every((c) => c.depth === 1)).toBe(true);

    const child2 = root.children.find((c) => c.trace.traceAddress === '2')!;
    expect(child2.children).toHaveLength(1);
    expect(child2.children[0].trace.traceAddress).toBe('2,0');
    expect(child2.children[0].depth).toBe(2);

    const child3 = root.children.find((c) => c.trace.traceAddress === '3')!;
    expect(child3.trace.error).toBe('execution reverted');
  });

  it('produces the same tree regardless of input ordering', () => {
    const shuffled = buildTree(tx(), [itx('2,0'), itx('3'), itx('0'), itx('2'), itx('1')]);
    expect(shuffled.children.map((c) => c.trace.traceAddress)).toEqual(['0', '1', '2', '3']);
    const child2 = shuffled.children.find((c) => c.trace.traceAddress === '2')!;
    expect(child2.children[0].trace.traceAddress).toBe('2,0');
  });

  it('handles multiple levels of nesting', () => {
    const root = buildTree(tx(), [itx('0'), itx('0,0'), itx('0,0,0'), itx('0,1')]);
    const c0 = root.children[0];
    expect(c0.trace.traceAddress).toBe('0');
    expect(c0.children.map((c) => c.trace.traceAddress)).toEqual(['0,0', '0,1']);
    const c00 = c0.children[0];
    expect(c00.children[0].trace.traceAddress).toBe('0,0,0');
    expect(c00.children[0].depth).toBe(3);
  });
});
