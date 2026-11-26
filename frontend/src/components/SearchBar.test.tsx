import { describe, it, expect, vi, beforeEach, onTestFinished } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import type { ReactNode } from 'react';

// SearchBar query classification + navigation (PLAN §6). The regex routing is
// pure client logic; we assert it lands on the right route. Suggestions fetch
// is stubbed so a debounce timer never hits the network here.
vi.mock('../lib/api', () => ({
  api: { searchSuggestions: vi.fn().mockResolvedValue({ query: '', suggestions: [] }) },
}));

import { SearchBar } from './SearchBar';
import { deferred } from '../test/deferred';
import { api } from '../lib/api';

function LocationProbe() {
  const loc = useLocation();
  return <div data-testid="loc">{loc.pathname}</div>;
}

function renderBar(children?: ReactNode) {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <SearchBar />
      <LocationProbe />
      {children}
      <Routes>
        <Route path="*" element={null} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('SearchBar classification + navigation', () => {
  // mockReset, not just clearAllMocks: a queued `...Once` value that a test never
  // consumed (e.g. it navigated before the debounced lookup fired) would
  // otherwise answer the next test's first lookup.
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.searchSuggestions).mockReset().mockResolvedValue({ query: '', suggestions: [] });
  });

  it('navigates to /block/:n for an all-digits query', async () => {
    const user = userEvent.setup();
    renderBar();
    await user.type(screen.getByTestId('search-input'), '12345{Enter}');
    expect(screen.getByTestId('loc')).toHaveTextContent('/block/12345');
  });

  it('navigates to /tx/:hash for a 66-char 0x hash', async () => {
    const user = userEvent.setup();
    renderBar();
    const hash = '0x' + 'a'.repeat(64);
    await user.type(screen.getByTestId('search-input'), `${hash}{Enter}`);
    expect(screen.getByTestId('loc')).toHaveTextContent(`/tx/${hash}`);
  });

  it('navigates to /address/:addr for a 42-char 0x address', async () => {
    const user = userEvent.setup();
    renderBar();
    const addr = '0x' + 'b'.repeat(40);
    await user.type(screen.getByTestId('search-input'), `${addr}{Enter}`);
    expect(screen.getByTestId('loc')).toHaveTextContent(`/address/${addr}`);
  });

  it('does not navigate for an unclassifiable query (stays on /)', async () => {
    const user = userEvent.setup();
    renderBar();
    await user.type(screen.getByTestId('search-input'), 'hello world{Enter}');
    expect(screen.getByTestId('loc')).toHaveTextContent('/');
  });
  it('navigates to /token/:addr when a token suggestion is picked', async () => {
    const token = '0x' + 'c'.repeat(40);
    vi.mocked(api.searchSuggestions).mockResolvedValueOnce({
      query: 'Tick',
      suggestions: [{ type: 'token', value: token, label: 'TICK (Tickr)' }],
    });
    const user = userEvent.setup();
    renderBar();
    await user.type(screen.getByTestId('search-input'), 'Tick');
    const item = await screen.findByTestId('search-suggestion');
    expect(item).toHaveAttribute('data-suggestion-type', 'token');
    await user.click(item);
    expect(screen.getByTestId('loc')).toHaveTextContent(`/token/${token}`);
  });

  it('offers a complete address straight away, even when the lookup fails', async () => {
    vi.mocked(api.searchSuggestions).mockRejectedValueOnce(new Error('deadline exceeded'));
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    onTestFinished(() => consoleError.mockRestore());
    const addr = '0x071e11A4403dc38a35b600C6775766f6DF404c30';
    const user = userEvent.setup();
    renderBar();
    await user.click(screen.getByTestId('search-input'));
    await user.paste(addr);
    const item = await screen.findByTestId('search-suggestion');
    expect(item).toHaveAttribute('data-suggestion-type', 'address');
    await user.click(item);
    expect(screen.getByTestId('loc')).toHaveTextContent(`/address/${addr}`);
  });
  // A lookup that settles after the query changed must not replace the newer
  // suggestions (revmux PR #199 round 1).
  it('ignores a slow lookup for an earlier query', async () => {
    const slow = deferred<{ query: string; suggestions: { type: 'token'; value: string; label: string }[] }>();
    const addr = '0x' + 'd'.repeat(40);
    vi.mocked(api.searchSuggestions)
      .mockReturnValueOnce(slow.promise)
      .mockResolvedValueOnce({ query: addr, suggestions: [] });
    const user = userEvent.setup();
    renderBar();
    const input = screen.getByTestId('search-input');
    await user.type(input, 'Tick');
    await waitFor(() => expect(api.searchSuggestions).toHaveBeenCalledWith('Tick'));
    await user.clear(input);
    await user.paste(addr);
    await waitFor(() => expect(api.searchSuggestions).toHaveBeenCalledWith(addr));
    await act(async () => slow.resolve({ query: 'Tick', suggestions: [{ type: 'token', value: '0x' + 'e'.repeat(40), label: 'STALE' }] }));
    const items = await screen.findAllByTestId('search-suggestion');
    expect(items).toHaveLength(1);
    expect(items[0]).toHaveTextContent(addr);
  });

  it('does not bring back an earlier address when its lookup fails late', async () => {
    let rejectSlow!: (e: Error) => void;
    const slowPromise = new Promise<never>((_, reject) => { rejectSlow = reject; });
    const a = '0x' + '1'.repeat(40);
    const b = '0x' + '2'.repeat(40);
    vi.mocked(api.searchSuggestions)
      .mockReturnValueOnce(slowPromise)
      .mockResolvedValueOnce({ query: b, suggestions: [] });
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    onTestFinished(() => consoleError.mockRestore());
    const user = userEvent.setup();
    renderBar();
    const input = screen.getByTestId('search-input');
    await user.click(input);
    await user.paste(a);
    await waitFor(() => expect(api.searchSuggestions).toHaveBeenCalledWith(a));
    await user.clear(input);
    await user.paste(b);
    await waitFor(() => expect(api.searchSuggestions).toHaveBeenCalledWith(b));
    await act(async () => rejectSlow(new Error('deadline exceeded')));
    const items = await screen.findAllByTestId('search-suggestion');
    expect(items).toHaveLength(1);
    expect(items[0]).toHaveTextContent(b);
  });
});
