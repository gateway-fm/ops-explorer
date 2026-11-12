import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { appSession } from '../lib/v2Session';
import { ReadPanel } from './ExplorerDataV2';
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it('continues an empty scoped page and keeps a large amount exact in the rendered result', async () => {
  vi.spyOn(appSession, 'captureOperation').mockImplementation(() => {
    const abort = new AbortController();
    return { identity: { actor: 'fixture', profile: 'fixture', activation: 'fixture', expires: 1 }, signal: abort.signal, check: () => {}, request: vi.fn(), dispose: () => abort.abort() };
  });
  const amount = '9007199254740993123456789';
  const read = vi.spyOn(appSession, 'read').mockResolvedValueOnce({ items: [], page: { next: 'opaque-next', hasMore: null, scanLimited: true, population: 'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS' } }).mockResolvedValueOnce({ items: [{ value: amount, status: null }], page: { hasMore: false, scanLimited: false, population: 'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS' } });
  render(<MemoryRouter><ReadPanel path="/transactions" title="Transactions" collection /></MemoryRouter>);
  await screen.findByText('No records on this page.'); expect(screen.getByText('Total unavailable')).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
  expect((await screen.findAllByText(amount)).every(node => node.textContent === amount)).toBe(true); await waitFor(() => expect(read.mock.calls[1][0]).toBe('/v2/explorer/transactions?limit=25&cursor=opaque-next'));
  expect(screen.getByText('End of the available records.')).toBeTruthy(); expect(screen.queryByText('Total: 0')).toBeNull();
});
function captureScope() {
  vi.spyOn(appSession, 'captureOperation').mockImplementation(() => {
    const abort = new AbortController();
    return { identity: { actor: 'fixture', profile: 'fixture', activation: 'fixture', expires: 1 }, signal: abort.signal, check: () => {}, request: vi.fn(), dispose: () => abort.abort() };
  });
}
it.each(['/transactions/0x' + 'a'.repeat(64), '/tokens/0x' + 'a'.repeat(40), '/addresses/0x' + 'a'.repeat(40) + '/contract'])('shows an unavailable singleton without claiming a read failure: %s', async path => {
  captureScope(); vi.spyOn(appSession, 'read').mockResolvedValue(null);
  render(<MemoryRouter><ReadPanel path={path} title="Record" /></MemoryRouter>);
  expect(await screen.findByText('This record is unavailable.')).toBeTruthy();
  expect(screen.queryByRole('alert')).toBeNull();
  expect(screen.queryByText('The read could not be completed. Try again.')).toBeNull();
});
it('links the actual Core search type/value shapes to their respective detail routes', async () => {
  captureScope();
  const tx = '0x' + 'a'.repeat(64), token = '0x' + 'b'.repeat(40), address = '0x' + 'c'.repeat(40), contract = '0x' + 'd'.repeat(40);
  vi.spyOn(appSession, 'read').mockResolvedValue({ items: [{ type: 'transaction', value: tx }, { type: 'token', value: token }, { type: 'address', value: address }, { type: 'contract', value: contract }], page: { hasMore: false, scanLimited: false, population: 'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS' } });
  render(<MemoryRouter><ReadPanel path="/search/suggestions" title="Search" collection /></MemoryRouter>);
  await screen.findAllByText(tx);
  for (const [value, target] of [[tx, '/transactions/' + tx], [token, '/tokens/' + token], [address, '/addresses/' + address + '/stats'], [contract, '/addresses/' + contract + '/contract']]) {
    expect(screen.getAllByRole('link', { name: value }).every(link => link.getAttribute('href') === target)).toBe(true);
  }
  expect(screen.queryByRole('alert')).toBeNull();
});
