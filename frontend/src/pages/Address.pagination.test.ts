import { describe, it, expect } from 'vitest';

import { nextPageSearchParams } from './Address.pagination';

// RD-1149 (Copilot review on #125): "Load more" must page the active feed in
// place. The previous per-panel setSearchParams({ cursor }) / ({ before }) calls
// replaced the whole query string and dropped tab=transactions, bouncing the
// user back to the default (details) tab. nextPageSearchParams centralizes the
// merge so the active tab — and any other param — is preserved.
describe('nextPageSearchParams (by-address load-more)', () => {
  it('preserves the active tab and prefers the opaque cursor', () => {
    const next = nextPageSearchParams(new URLSearchParams('tab=transactions'), 'op-cursor', '100');
    expect(next.get('tab')).toBe('transactions'); // no longer dropped
    expect(next.get('cursor')).toBe('op-cursor');
    expect(next.get('before')).toBeNull(); // cursor supersedes ?before=
  });

  it('falls back to ?before= (keeping the tab) when there is no cursor', () => {
    const next = nextPageSearchParams(new URLSearchParams('tab=transactions'), undefined, '100');
    expect(next.get('tab')).toBe('transactions');
    expect(next.get('before')).toBe('100');
    expect(next.get('cursor')).toBeNull();
  });

  it('clears a stale ?before= when advancing to a cursor', () => {
    const next = nextPageSearchParams(new URLSearchParams('tab=transfers&before=100'), 'c2', undefined);
    expect(next.get('cursor')).toBe('c2');
    expect(next.get('before')).toBeNull();
  });

  it('leaves unrelated params untouched', () => {
    const next = nextPageSearchParams(new URLSearchParams('tab=contract&sub=read'), 'c', undefined);
    expect(next.get('tab')).toBe('contract');
    expect(next.get('sub')).toBe('read');
  });

  it('is a no-op signal when neither cursor nor before is available', () => {
    // The caller guards this case, but the pure fn must still not invent params.
    const next = nextPageSearchParams(new URLSearchParams('tab=transactions'), undefined, undefined);
    expect(next.get('cursor')).toBeNull();
    expect(next.get('before')).toBeNull();
    expect(next.get('tab')).toBe('transactions');
  });
});
