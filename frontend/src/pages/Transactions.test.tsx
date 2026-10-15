import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../test/render';
import { API, http, HttpResponse, makeTx, server } from '../test/msw';
import { Transactions } from './Transactions';

const session = vi.hoisted(() => ({ isActive: false, token: null as string | null }));
vi.mock('../lib/auth', () => ({ useAuth: () => ({ isAuthenticated: true, auth: { did: 'did:viewer' } }) }));
vi.mock('../hooks/useImpersonation', () => ({ useImpersonation: () => session, getImpersonationTokenHeader: () => session.token }));

const hash = '0x' + 'a'.repeat(64);
const grantTx = { tx_hash: hash, block_number: 900, tx_index: 0, block_timestamp: 1700000000, from: 'Address-A', to: 'External-B', value: 'hidden', gas_used: 21000, status: 1, direction: 'out' };

function grantHandlers(level = 'full', methods = ['transaction_history']) {
  return [
    http.get(`${API}/privacy/viewable-addresses`, () => HttpResponse.json({ own_addresses: [], disclosed_addresses: [{ grant_id: 'g1', address_id: 'a1', disclosure_level: level }] })),
    http.get(`${API}/privacy/grant/g1/a1`, () => HttpResponse.json({ grant_id: 'g1', disclosure_level: level, scope_methods: methods })),
    http.get(`${API}/privacy/grant/g1/a1/transactions`, () => HttpResponse.json({ transactions: [{ ...grantTx, ...(level === 'full' ? {} : { tx_hash: undefined }) }], disclosure_level: level })),
  ];
}

describe('Transactions disclosure composition', () => {
  beforeEach(() => {
    session.isActive = false;
    session.token = null;
    server.use(...grantHandlers(), http.get(`${API}/addresses/:address/contract`, () => HttpResponse.json({ isVerified: false })));
  });

  it('shows grant-only transactions with a disclosure badge and grant detail route', async () => {
    server.use(http.get(`${API}/transactions`, () => HttpResponse.json({ data: [], page: 1, totalPages: 0, total: 0 })));
    renderWithProviders(<Transactions />);
    await screen.findByText('Disclosed');
    expect(screen.getAllByTestId('tx-row')).toHaveLength(1);
    expect(screen.getByRole('link', { name: /0xaaaa/ })).toHaveAttribute('href', `/grant/g1/a1/tx/${hash}`);
    expect(screen.getByTestId('tx-total-count')).toHaveTextContent('0 regular transactions');
    expect(screen.getByTestId('disclosed-count')).toHaveTextContent('1 disclosed shown');
    expect(screen.queryByText('No transactions yet')).not.toBeInTheDocument();
  });

  it('adds older disclosures to the final page and preserves regular pagination', async () => {
    server.use(http.get(`${API}/transactions`, ({ request }) => {
      const page = Number(new URL(request.url).searchParams.get('page') || '1');
      return HttpResponse.json({ data: [makeTx(page, { blockNumber: page === 1 ? 1000 : 950 })], total: 26, totalPages: 2, page });
    }));
    renderWithProviders(<Transactions />, { initialEntries: ['/transactions?page=2'] });
    await screen.findByText('Disclosed');
    expect(screen.getAllByTestId('tx-row')).toHaveLength(2);
    expect(screen.getByTestId('pagination-status')).toHaveTextContent('Page 2 of 2');
  });

  it('returns an out-of-range URL to the last regular page instead of reopening an unbounded grant window', async () => {
    server.use(http.get(`${API}/transactions`, ({ request }) => {
      const page = Number(new URL(request.url).searchParams.get('page') || '1');
      return HttpResponse.json({ data: page === 1 ? [makeTx(1)] : [], total: 1, totalPages: 1, page });
    }));
    renderWithProviders(<Transactions />, { initialEntries: ['/transactions?page=99'] });
    await screen.findByText('Disclosed');
    expect(screen.getAllByTestId('tx-row')).toHaveLength(2);
  });

  it('links pseudonymous rows only to the grant address and renders hidden value', async () => {
    server.use(...grantHandlers('pseudonymous'));
    renderWithProviders(<Transactions />);
    await screen.findByText('Disclosed');
    const row = screen.getAllByTestId('tx-row').find((r) => within(r).queryByText('Disclosed'))!;
    expect(within(row).getByRole('link', { name: 'Hidden transaction' })).toHaveAttribute('href', '/grant/g1/a1');
    expect(within(row).getByText('hidden')).toBeInTheDocument();
    expect(within(row).queryByRole('link', { name: 'Address-A' })).not.toBeInTheDocument();
  });

  it('replaces a masked regular duplicate with the consented Full disclosure and grant detail link', async () => {
    server.use(
      http.get(`${API}/transactions`, () => HttpResponse.json({ data: [makeTx(1, { hash, blockNumber: 900, from: '[PRIVATE]', value: '' })], total: 1, totalPages: 1, page: 1 })),
      http.get(`${API}/privacy/grant/g1/a1/transactions`, () => HttpResponse.json({ transactions: [{ ...grantTx, value: '5' }], disclosure_level: 'full' })),
    );
    renderWithProviders(<Transactions />);
    await screen.findByText('Disclosed');
    expect(screen.getAllByTestId('tx-row')).toHaveLength(1);
    expect(screen.getByRole('link', { name: /0xaaaa/ })).toHaveAttribute('href', `/grant/g1/a1/tx/${hash}`);
    expect(screen.getByText('Address-A')).toBeInTheDocument();
  });

  it('excludes activity-only grants before fetching their transaction feed', async () => {
    const feed = vi.fn(() => HttpResponse.json({ transactions: [grantTx] }));
    server.use(...grantHandlers('full', ['activity_logs']), http.get(`${API}/privacy/grant/g1/a1/transactions`, feed));
    renderWithProviders(<Transactions />);
    await screen.findAllByTestId('tx-row');
    await waitFor(() => expect(screen.queryByText('Loading disclosures…')).not.toBeInTheDocument());
    expect(feed).not.toHaveBeenCalled();
    expect(screen.queryByText('Disclosed')).not.toBeInTheDocument();
  });

  it('keeps the ordinary detail link for a participant-visible transaction even with a matching Full grant', async () => {
    const visible = makeTx(1, { hash, blockNumber: 900, value: '5' });
    visible.addressMetadata = { [visible.from]: 'participant_override' };
    const feed = vi.fn(() => HttpResponse.json({ transactions: [{ ...grantTx, value: '5' }], disclosure_level: 'full' }));
    server.use(
      http.get(`${API}/transactions`, () => HttpResponse.json({ data: [visible], total: 1, totalPages: 1, page: 1 })),
      http.get(`${API}/privacy/grant/g1/a1/transactions`, feed),
    );
    renderWithProviders(<Transactions />);
    await waitFor(() => expect(feed).toHaveBeenCalled());
    await waitFor(() => expect(screen.queryByText('Loading disclosures…')).not.toBeInTheDocument());
    expect(screen.getAllByTestId('tx-row')).toHaveLength(1);
    expect(screen.getByRole('link', { name: /0xaaaa/ })).toHaveAttribute('href', `/tx/${hash}`);
    expect(screen.queryByText('Disclosed')).not.toBeInTheDocument();
  });

  it('does not query grants or render their cached rows during View-as', async () => {
    session.isActive = true;
    session.token = 'view-as';
    const addresses = vi.fn(() => HttpResponse.json({ disclosed_addresses: [] }));
    server.use(http.get(`${API}/privacy/viewable-addresses`, addresses));
    renderWithProviders(<Transactions />);
    await screen.findAllByTestId('tx-row');
    expect(addresses).not.toHaveBeenCalled();
    expect(screen.queryByText('Disclosed')).not.toBeInTheDocument();
  });

  it('removes already-loaded grant rows immediately when View-as starts', async () => {
    function SwitchViewer() {
      const [, render] = useState(0);
      return <><button onClick={() => { session.isActive = true; session.token = 'view-as'; render(1); }}>Start View-as</button><Transactions /></>;
    }
    renderWithProviders(<SwitchViewer />);
    await screen.findByText('Disclosed');
    await userEvent.click(screen.getByRole('button', { name: 'Start View-as' }));
    expect(screen.queryByText('Disclosed')).not.toBeInTheDocument();
  });

  it('suppresses grants while a View-as URL is being restored', async () => {
    const addresses = vi.fn(() => HttpResponse.json({ disclosed_addresses: [] }));
    server.use(http.get(`${API}/privacy/viewable-addresses`, addresses));
    renderWithProviders(<Transactions />, { initialEntries: ['/transactions?as=restoring'] });
    await screen.findAllByTestId('tx-row');
    expect(addresses).not.toHaveBeenCalled();
  });

  it('resumes after bounded empty grant pages rather than declaring the feed empty', async () => {
    let calls = 0;
    server.use(
      http.get(`${API}/transactions`, () => HttpResponse.json({ data: [], total: 0, totalPages: 0, page: 1 })),
      http.get(`${API}/privacy/grant/g1/a1/transactions`, () => {
        calls++;
        return HttpResponse.json(calls <= 5 ? { transactions: [], next_cursor: `cursor-${calls}` } : { transactions: [grantTx] });
      }),
    );
    renderWithProviders(<Transactions />);
    await userEvent.click(await screen.findByRole('button', { name: 'Load more disclosed transactions' }));
    await screen.findByText('Disclosed');
    expect(calls).toBe(6);
    expect(screen.queryByRole('button', { name: 'Load more disclosed transactions' })).not.toBeInTheDocument();
  });

  it.each([false, true])('hides all Full pages after a downgrade, including an empty continuation (%s), and revalidates before retrying', async (empty) => {
    let calls = 0;
    let level = 'full';
    const events: string[] = [];
    server.use(
      http.get(`${API}/transactions`, () => HttpResponse.json({ data: [], total: 0, totalPages: 0, page: 1 })),
      http.get(`${API}/privacy/grant/g1/a1`, () => {
        events.push('resolve');
        return HttpResponse.json({ grant_id: 'g1', disclosure_level: level, scope_methods: ['transaction_history'] });
      }),
      http.get(`${API}/privacy/grant/g1/a1/transactions`, ({ request }) => {
        events.push('feed');
        calls++;
        if (level === 'full') {
          return HttpResponse.json({ transactions: calls === 1 ? [{ ...grantTx, from: 'Full-address', value: '1234' }] : [], disclosure_level: level, next_cursor: `cursor-${calls}` });
        }
        const continuing = new URL(request.url).searchParams.has('cursor');
        return HttpResponse.json({ transactions: empty && continuing ? [] : [{ ...grantTx, tx_hash: undefined }], disclosure_level: level });
      }),
    );
    renderWithProviders(<Transactions />);
    await screen.findByRole('link', { name: /0xaaaa/ });
    level = 'pseudonymous';
    await userEvent.click(screen.getByRole('button', { name: 'Load more disclosed transactions' }));
    await screen.findByRole('alert');
    expect(screen.queryByRole('link', { name: /0xaaaa/ })).not.toBeInTheDocument();
    expect(screen.queryByText('Full-address')).not.toBeInTheDocument();
    expect(screen.queryByText('Disclosed')).not.toBeInTheDocument();
    events.length = 0;
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await screen.findByRole('link', { name: 'Hidden transaction' });
    expect(events).toEqual(['resolve', 'feed']);
    expect(screen.queryByRole('link', { name: /0xaaaa/ })).not.toBeInTheDocument();
    expect(screen.queryByText('Full-address')).not.toBeInTheDocument();
    expect(screen.getAllByTestId('tx-row')).toHaveLength(1);
  });

  it('removes stale grant rows when continued access is denied and shows a recoverable error', async () => {
    let calls = 0;
    server.use(http.get(`${API}/privacy/grant/g1/a1/transactions`, () => {
      calls++;
      if (calls > 5) return new HttpResponse(null, { status: 403 });
      return HttpResponse.json({ transactions: calls === 1 ? [grantTx] : [], next_cursor: `cursor-${calls}` });
    }));
    renderWithProviders(<Transactions />);
    await screen.findByText('Disclosed');
    await userEvent.click(screen.getByRole('button', { name: 'Load more disclosed transactions' }));
    await screen.findByRole('alert');
    expect(screen.queryByText('Disclosed')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });
});
