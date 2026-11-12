import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { appSession } from '../lib/v2Session';
import { readObservedHistory, readObservedStats } from '../lib/v2Reads';
import { ReadPanel } from './ExplorerDataV2';

afterEach(() => { cleanup(); vi.restoreAllMocks(); });
const page = { coverage: 'OBSERVED_VISIBLE_PAGE', population: 'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS', hasMore: null, next: 'opaque-next', scanLimited: true };
const lastPage = { coverage: 'OBSERVED_VISIBLE_PAGE', population: 'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS', hasMore: false, scanLimited: false };
function captureScope() {
  vi.spyOn(appSession, 'captureOperation').mockImplementation(() => {
    const abort = new AbortController();
    return { identity: { actor: 'fixture', profile: 'fixture', activation: 'fixture', expires: 1 }, signal: abort.signal, check: () => {}, request: vi.fn(), dispose: () => abort.abort() };
  });
}
it('keeps an empty observed page distinct from a global zero and continues to counted data', async () => {
  captureScope();
  const read = vi.spyOn(appSession, 'read').mockResolvedValueOnce({ privacyEnabled: true, observedTransactions: '0', observedBlocks: '0', observedParticipantAddresses: '0', page }).mockResolvedValueOnce({ privacyEnabled: true, observedTransactions: '2', observedBlocks: '1', observedParticipantAddresses: '1', page: lastPage });
  render(<MemoryRouter><ReadPanel path="/stats" title="Observed activity" summary="stats" /></MemoryRouter>);
  await screen.findByText('Transactions on this page');
  expect(screen.getByText(/not the whole chain or all retained history/)).toBeTruthy();
  expect(screen.queryByText('Total: 0')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
  await screen.findByText('2');
  await waitFor(() => expect(read.mock.calls[1][0]).toBe('/v2/explorer/stats?limit=25&cursor=opaque-next'));
  expect(screen.getByText('Blocks represented')).toBeTruthy();
  expect(screen.getByText('Visible participant addresses')).toBeTruthy();
});
it('renders actual UTC count bars for the page and does not merge a continued bucket', async () => {
  captureScope();
  const read = vi.spyOn(appSession, 'read').mockResolvedValueOnce({ items: [{ timestamp: '0', count: '1' }, { timestamp: '3600', count: '2' }], page }).mockResolvedValueOnce({ items: [{ timestamp: '3600', count: '1' }], page: lastPage });
  render(<MemoryRouter><ReadPanel path="/stats/tx-history" title="History" collection filters="chart" summary="chart" /></MemoryRouter>);
  const chart = await screen.findByRole('img', { name: 'Visible transaction counts on this page' });
  expect(chart.querySelectorAll('rect')).toHaveLength(2);
  expect(chart.textContent).toContain('1970-01-01 01:00:00 UTC: 2 transactions on this page');
  expect(screen.getByText(/missing buckets are not reported as zero/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
  await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  await waitFor(() => expect(screen.getByRole('img').querySelectorAll('rect')).toHaveLength(1));
  expect(screen.getByRole('img').textContent).toContain('1970-01-01 01:00:00 UTC: 1 transactions on this page');
});
it('rejects ambiguous/global coverage and never fills empty visible history with zero buckets', async () => {
  expect(() => readObservedStats({ privacyEnabled: true, observedTransactions: '0', observedBlocks: '0', observedParticipantAddresses: '0', totalTransactions: '0', page })).toThrow();
  expect(() => readObservedHistory({ items: [{ timestamp: '0', count: '0' }], page })).toThrow();
  expect(() => readObservedHistory({ items: [], page: { ...page, coverage: 'COMPLETE_AT_CUT' } })).toThrow();
  captureScope(); vi.spyOn(appSession, 'read').mockResolvedValue({ items: [], page });
  render(<MemoryRouter><ReadPanel path="/stats/tx-history" title="History" collection filters="chart" summary="chart" /></MemoryRouter>);
  await screen.findByText('No visible transactions on this page. Continue if another page is available.');
  expect(screen.queryByRole('img')).toBeNull();
  expect((screen.getByRole('button', { name: 'Continue' }) as HTMLButtonElement).disabled).toBe(false);
});
