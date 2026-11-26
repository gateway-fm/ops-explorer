import { describe, it, expect } from 'vitest';
import { directSuggestion, suggestionPath } from './searchDirect';

describe('directSuggestion', () => {
  it('recognises a full address and tx hash', () => {
    const addr = '0x071e11A4403dc38a35b600C6775766f6DF404c30';
    expect(directSuggestion(addr)).toEqual({ type: 'address', value: addr, label: addr });
    const hash = '0x' + 'ab'.repeat(32);
    expect(directSuggestion(hash)?.type).toBe('transaction');
  });

  it('keeps block numbers exact past 2^53 and never uses exponent form', () => {
    expect(directSuggestion('9007199254740993')?.value).toBe('9007199254740993');
    const big = '1' + '0'.repeat(21);
    expect(directSuggestion(big)?.value).toBe(big);
    expect(directSuggestion('00042')?.value).toBe('42');
    expect(directSuggestion('0')?.value).toBe('0');
  });

  it('ignores partial or non-hex input', () => {
    expect(directSuggestion('0x071e11')).toBeNull();
    expect(directSuggestion('0x' + 'g'.repeat(40))).toBeNull();
    expect(directSuggestion('Tick')).toBeNull();
  });
});

describe('suggestionPath', () => {
  it('routes every suggestion type', () => {
    expect(suggestionPath({ type: 'block', value: '7', label: '' })).toBe('/block/7');
    expect(suggestionPath({ type: 'transaction', value: '0xab', label: '' })).toBe('/tx/0xab');
    expect(suggestionPath({ type: 'address', value: '0xcd', label: '' })).toBe('/address/0xcd');
    expect(suggestionPath({ type: 'token', value: '0xef', label: '' })).toBe('/token/0xef');
  });
});
