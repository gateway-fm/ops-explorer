import { describe, expect, it, vi } from 'vitest';
import type { PseudonymizedTransaction, Transaction } from '../lib/api';
import { grantWindow, inGrantWindow, mergeTransactions, loadGrantWindow } from './Transactions.disclosure';

const regular = (block: number, index: number, hash = `${block}-${index}`) => ({
  hash, blockNumber: block, txIndex: index,
} as Transaction);
const disclosed = (block: number, index?: number, hash?: string) => ({
  block_number: block, tx_index: index, tx_hash: hash, from: 'Address-A', to: 'External-B',
  block_timestamp: 1, gas_used: 21000, status: 1, direction: 'out', value: 'hidden',
} as PseudonymizedTransaction);
const source = { grantId: 'g', addressId: 'a', level: 'pseudonymous' };

describe('disclosed transaction composition', () => {
  it('gives a grant-only viewer an unbounded first window and keeps older grants on the final regular page', () => {
    expect(grantWindow([], undefined, true)).toEqual({});
    const tail = grantWindow([regular(20, 2)], regular(30, 0), true);
    expect(inGrantWindow(disclosed(1, 0), tail)).toBe(true);
    expect(inGrantWindow(disclosed(30, 0), tail)).toBe(false);
  });

  it('partitions a block split between regular pages without losing or repeating boundary rows', () => {
    const first = grantWindow([regular(50, 9), regular(50, 5)], undefined, false);
    const second = grantWindow([regular(50, 4), regular(40, 0)], regular(50, 5), false);
    expect([8, 5, 4, 0].map((i) => inGrantWindow(disclosed(50, i), first))).toEqual([true, true, false, false]);
    expect([8, 5, 4, 0].map((i) => inGrantWindow(disclosed(50, i), second))).toEqual([false, false, true, true]);
    expect(inGrantWindow(disclosed(50), first)).toBe(true);
    expect(inGrantWindow(disclosed(50), second)).toBe(false);
  });

  it('prefers regular rows, suppresses hash/position duplicates, sorts, and preserves distinct hidden rows', () => {
    const grants = [
      { source, tx: disclosed(12, 2) },
      { source, tx: disclosed(10, 1) },
      { source, tx: disclosed(9, undefined, 'SAME') },
      { source: { ...source, grantId: 'other' }, tx: disclosed(12, 2) },
      { source, tx: disclosed(8, 2) },
      { source, tx: disclosed(8, 1) },
      { source, tx: { ...disclosed(7), from: 'Mine' } },
    ];
    const rows = mergeTransactions([regular(10, 1, 'same')], grants);
    expect(rows.map((row) => [row.blockNumber, row.txIndex])).toEqual([[12, 2], [10, 1], [8, 2], [8, 1]]);
    expect(rows[1].kind).toBe('regular');
  });

  it('keeps a resumable cursor after bounded empty batches and resumes without dropping rows', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce({ transactions: [], next_cursor: 'c1' })
      .mockResolvedValueOnce({ transactions: [], next_cursor: 'c2' })
      .mockResolvedValueOnce({ transactions: [disclosed(4, 0)] });
    const first = await loadGrantWindow(source, {}, fetch, undefined, 2);
    expect(first.rows).toEqual([]);
    expect(first.nextCursor).toBe('c2');
    const resumed = await loadGrantWindow(source, {}, fetch, first.nextCursor, 2);
    expect(resumed.rows).toHaveLength(1);
    expect(resumed.nextCursor).toBeUndefined();
    expect(fetch.mock.calls[2][4]).toBe('c2');
  });

  it('uses a Full grant to reveal a duplicate masked regular row while preserving visible own rows', () => {
    const masked = { ...regular(20, 0, 'shared'), from: '[PRIVATE]', to: '0xmine', value: '' };
    const full = { source: { ...source, level: 'full' }, tx: { ...disclosed(20, 0, 'shared'), from: '0xsubject', to: '0xmine', value: '5' } };
    const rows = mergeTransactions([masked], [full]);
    expect(rows).toHaveLength(1);
    expect(rows[0].kind).toBe('disclosed');
    expect(rows[0].tx.from).toBe('0xsubject');
    const visible = { ...masked, from: '0xsubject', value: '5' };
    expect(mergeTransactions([visible], [full])[0].kind).toBe('regular');
    expect(mergeTransactions([masked], [{ ...full, source }])[0].kind).toBe('regular');
  });

  it('fetches the upper boundary block inclusively and stops only after crossing the floor', async () => {
    const fetch = vi.fn().mockResolvedValue({
      transactions: [disclosed(50, 8), disclosed(50, 4), disclosed(40, 1), disclosed(39, 9)],
      next_cursor: 'older',
    });
    const result = await loadGrantWindow(source, { upper: { block: 50, index: 5 }, lower: { block: 40, index: 0 } }, fetch);
    expect(fetch).toHaveBeenCalledWith('g', 'a', 100, 51, undefined);
    expect(result.rows.map((row) => row.tx.block_number)).toEqual([50, 40]);
    expect(result.nextCursor).toBeUndefined();
  });
});
