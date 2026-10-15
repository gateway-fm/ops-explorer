import { describe, expect, it, vi } from 'vitest';
import type { PseudonymizedTransaction, Transaction } from '../lib/api';
import { grantWindow, inGrantWindow, mergeTransactions, loadGrantWindow, loadGrantBoundary } from './Transactions.disclosure';
import { deferred } from '../test/deferred';

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
    const tail = grantWindow([regular(20, 2)], regular(30, 0), true)!;
    expect(inGrantWindow(disclosed(1, 0), tail)).toBe(true);
    expect(inGrantWindow(disclosed(30, 0), tail)).toBe(false);
  });

  it('partitions a block split between regular pages without losing or repeating boundary rows', () => {
    const first = grantWindow([regular(50, 9), regular(50, 5)], undefined, false)!;
    const second = grantWindow([regular(50, 4), regular(40, 0)], regular(50, 5), false)!;
    expect([8, 5, 4, 0].map((i) => inGrantWindow(disclosed(50, i), first))).toEqual([true, true, false, false]);
    expect([8, 5, 4, 0].map((i) => inGrantWindow(disclosed(50, i), second))).toEqual([false, false, true, true]);
    expect(inGrantWindow(disclosed(50), first)).toBe(true);
    expect(inGrantWindow(disclosed(50), second)).toBe(false);
  });

  it.each([
    { name: 'leading empty pages', pages: [[], [], [regular(50, 5)], [regular(40, 0)]] },
    { name: 'an empty page within a split block', pages: [[regular(50, 9)], [], [regular(50, 4)], [regular(40, 0)]] },
    { name: 'trailing empty pages', pages: [[regular(50, 5)], [], [], []] },
    { name: 'all filtered pages', pages: [[], [], [], []] },
  ])('assigns every disclosure exactly once with $name', async ({ pages }) => {
    const candidates = [disclosed(60, 0), ...[9, 8, 5, 4, 0].map((i) => disclosed(50, i)), disclosed(50), disclosed(40, 0), disclosed(1, 0)];
    const counts = candidates.map(() => 0);
    for (const [index, page] of pages.entries()) {
      const window = grantWindow(page, undefined, index === pages.length - 1);
      if (!window) continue;
      const boundary = await loadGrantBoundary(index, async (previousPage) => pages[previousPage - 1], new AbortController().signal);
      expect(boundary.nextPage).toBeUndefined();
      candidates.forEach((tx, i) => { if (inGrantWindow(tx, { ...window, upper: boundary.upper })) counts[i]++; });
    }
    expect(counts).toEqual(candidates.map(() => 1));
  });

  it('does not continue a boundary search after cancellation, even if its request resolves', async () => {
    const pending = deferred<Transaction[]>();
    const read = vi.fn(() => pending.promise);
    const controller = new AbortController();
    const result = loadGrantBoundary(3, read, controller.signal);
    const rejected = expect(result).rejects.toMatchObject({ name: 'AbortError' });
    controller.abort();
    pending.resolve([]);
    await rejected;
    expect(read).toHaveBeenCalledTimes(1);
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

  it('rejects a level change within a batch before returning any earlier Full rows', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce({ transactions: [disclosed(4, 0, 'full-hash')], disclosure_level: 'full', next_cursor: 'c1' })
      .mockResolvedValueOnce({ transactions: [], disclosure_level: 'redacted' });
    await expect(loadGrantWindow({ ...source, level: 'full' }, {}, fetch)).rejects.toThrow('Disclosure level changed');
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
    expect(fetch).toHaveBeenCalledWith('g', 'a', 100, 51, undefined, undefined);
    expect(result.rows.map((row) => row.tx.block_number)).toEqual([50, 40]);
    expect(result.nextCursor).toBeUndefined();
  });

  it('preserves participant-visible regular rows when a matching Full grant exists', () => {
    const visible: Transaction = {
      ...regular(20, 0, 'shared'), from: '0xsubject', to: '0xmine', value: '5',
      addressMetadata: { '0xsubject': 'participant_override' },
    };
    const full = { source: { ...source, level: 'full' }, tx: disclosed(20, 0, 'shared') };
    expect(mergeTransactions([visible], [full])).toEqual([
      expect.objectContaining({ kind: 'regular', tx: visible }),
    ]);
  });
});
