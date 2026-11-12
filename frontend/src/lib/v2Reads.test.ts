import { describe, expect, it } from 'vitest';
import { continuationMessage, readItem, readPage, readTarget } from './v2Reads';
const page = { hasMore: null, next: 'opaque', scanLimited: true, population: 'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS' };
describe('lossless current-scope read pages', () => {
  it('preserves huge decimal values, masked null and a continued empty page without inventing totals', () => {
    const amount = '9007199254740993123456789';
    expect(readPage({ items: [{ value: amount, txHash: '0x1' }, { value: null }], page }).items[0].value).toBe(amount);
    const empty = readPage({ items: [], page }); expect(empty.page.next).toBe('opaque'); expect(empty.page.total).toBeUndefined();
    expect(continuationMessage(empty.page)).toContain('Continue'); expect(continuationMessage(empty.page)).not.toContain('End');
    expect(readTarget('/transactions', new URLSearchParams('category=token_transfer&cursor=old'), 'new')).toBe('/v2/explorer/transactions?category=token_transfer&cursor=new');
  });
  it('distinguishes block nonce hex and suggestion text from transaction quantities', () => {
    expect(readItem({ parentHash: '0x1', nonce: '0x0000000000000000' }).nonce).toBe('0x0000000000000000');
    expect(readItem({ type: 'token', value: 'Bank token', label: 'Bank' }).value).toBe('Bank token');
    expect(() => readItem({ value: 9007199254740992 })).toThrow();
    expect(() => readItem({ balance: '1e6' })).toThrow();
  });
  it('rejects legacy wrappers and false exhaustion/unsafe totals', () => {
    for (const value of [{ transactions: [], total: 1 }, { items: [], page: { ...page, total: 5 } }, { items: [], page: { ...page, hasMore: false } }, { items: [], page: { ...page, next: undefined } }]) expect(() => readPage(value)).toThrow();
    expect(readPage({ items: [], page: { hasMore: false, scanLimited: false, total: '0', population: page.population } }).page.total).toBe('0');
  });
});
