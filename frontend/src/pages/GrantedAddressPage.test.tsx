import { beforeEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useQueryClient } from '@tanstack/react-query';
import { renderWithProviders } from '../test/render';
import { API, http, HttpResponse, server } from '../test/msw';
import { GrantedAddressPage } from './GrantedAddressPage';

const viewing = vi.hoisted(() => ({ isActive: false }));
vi.mock('../lib/auth', () => ({ useAuth: () => ({ isAuthenticated: true, isLoading: false, auth: { did: 'did:viewer' } }) }));
vi.mock('../hooks/useImpersonation', () => ({ useImpersonation: () => viewing, getImpersonationTokenHeader: () => null }));

const transaction = { tx_hash: '0x1234567890123', block_number: 1, block_timestamp: 1700000000, from: '0xabc', to: '0xdef', value: '0', gas_used: 21000, status: 1, direction: 'out' };

function Refresh({ queryKey }: { queryKey: string }) {
  const queryClient = useQueryClient();
  return <button onClick={() => void queryClient.invalidateQueries({ queryKey: [queryKey] })}>Refresh access</button>;
}

describe('GrantedAddressPage', () => {
  beforeEach(() => { viewing.isActive = false; });
  it('omits unavailable statistics and routes Full transaction links through the grant detail', async () => {
    server.use(
      http.get(`${API}/privacy/grant/g/a`, () => HttpResponse.json({ grant_id: 'g', display_address: '0xabc', disclosure_level: 'full' })),
      http.get(`${API}/privacy/grant/g/a/transactions`, () => HttpResponse.json({ transactions: [{ tx_hash: '0x1234567890123', block_number: 1, block_timestamp: 1700000000, from: '0xabc', to: '0xdef', value: '0', gas_used: 21000, status: 1, direction: 'out' }], address_labels: {} })),
    );
    renderWithProviders(<GrantedAddressPage />, { path: '/grant/:grantId/:addressId', initialEntries: ['/grant/g/a'] });
    expect(await screen.findByRole('link', { name: '0x12345678...' })).toHaveAttribute('href', '/grant/g/a/tx/0x1234567890123');
    expect(screen.getAllByText('Not available')).toHaveLength(3);
  });
  it('hides all grant data under View-as and makes no grant requests', async () => {
    viewing.isActive = true;
    const resolve = vi.fn(() => HttpResponse.json({}));
    server.use(http.get(`${API}/privacy/grant/g/a`, resolve));
    renderWithProviders(<GrantedAddressPage />, { path: '/grant/:grantId/:addressId', initialEntries: ['/grant/g/a'] });
    await screen.findByText('Disclosures are not available during View-as');
    expect(resolve).not.toHaveBeenCalled();
  });

  it('hides cached transactions after a denied refetch and lets the viewer retry', async () => {
    let allowed = true;
    server.use(
      http.get(`${API}/privacy/grant/g/a`, () => HttpResponse.json({ grant_id: 'g', display_address: '0xabc', disclosure_level: 'full' })),
      http.get(`${API}/privacy/grant/g/a/transactions`, () => allowed ? HttpResponse.json({ transactions: [transaction], address_labels: {} }) : new HttpResponse(null, { status: 403 })),
    );
    renderWithProviders(<><Refresh queryKey="grantedAddressTransactions" /><GrantedAddressPage /></>, { path: '/grant/:grantId/:addressId', initialEntries: ['/grant/g/a'] });
    await screen.findByRole('link', { name: '0x12345678...' });
    allowed = false;
    await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
    await screen.findByText('Disclosed transactions are not available');
    expect(screen.queryByRole('link', { name: '0x12345678...' })).not.toBeInTheDocument();
    allowed = true;
    await userEvent.click(screen.getByRole('button', { name: 'Retry transactions' }));
    await screen.findByRole('link', { name: '0x12345678...' });
  });

  it('hides cached activity after denied access and retries through the activity endpoint', async () => {
    let allowed = true;
    server.use(
      http.get(`${API}/privacy/grant/g/a`, () => HttpResponse.json({ grant_id: 'g', display_address: 'Address-A', disclosure_level: 'pseudonymous', scope_methods: ['activity_logs'] })),
      http.get(`${API}/privacy/grant/g/activity`, () => allowed ? HttpResponse.json({ logs: [{ method: 'eth_blockNumber', status_code: 200, timestamp: '2026-10-14T00:00:00Z' }], total: 1 }) : new HttpResponse(null, { status: 404 })),
    );
    renderWithProviders(<><Refresh queryKey="grantActivityLogs" /><GrantedAddressPage /></>, { path: '/grant/:grantId/:addressId', initialEntries: ['/grant/g/a'] });
    await screen.findByText('eth_blockNumber');
    allowed = false;
    await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
    await screen.findByText('Disclosed activity logs are not available');
    expect(screen.queryByText('eth_blockNumber')).not.toBeInTheDocument();
    allowed = true;
    await userEvent.click(screen.getByRole('button', { name: 'Retry activity logs' }));
    await screen.findByText('eth_blockNumber');
  });

  it('moves to an available tab when the selected tab is removed from scope', async () => {
    let methods = ['full_disclosure'];
    server.use(
      http.get(`${API}/privacy/grant/g/a`, () => HttpResponse.json({ grant_id: 'g', display_address: '0xabc', disclosure_level: 'full', scope_methods: methods })),
      http.get(`${API}/privacy/grant/g/a/transactions`, () => HttpResponse.json({ transactions: [transaction], address_labels: {} })),
      http.get(`${API}/privacy/grant/g/activity`, () => HttpResponse.json({ logs: [{ method: 'eth_blockNumber', status_code: 200, timestamp: '2026-10-14T00:00:00Z' }], total: 1 })),
    );
    renderWithProviders(<><Refresh queryKey="grantedAddress" /><GrantedAddressPage /></>, { path: '/grant/:grantId/:addressId', initialEntries: ['/grant/g/a'] });
    await userEvent.click(await screen.findByRole('tab', { name: 'Activity Logs' }));
    await screen.findByText('eth_blockNumber');
    methods = ['transaction_history'];
    await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
    await screen.findByRole('link', { name: '0x12345678...' });
    expect(screen.queryByText('eth_blockNumber')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Activity Logs' })).not.toBeInTheDocument();
  });

  it('revalidates transaction data after resolving a narrower disclosure level', async () => {
    let level = 'full';
    server.use(
      http.get(`${API}/privacy/grant/g/a`, () => HttpResponse.json({ grant_id: 'g', display_address: level === 'full' ? '0xabc' : 'Address-A', disclosure_level: level })),
      http.get(`${API}/privacy/grant/g/a/transactions`, () => HttpResponse.json({ transactions: [level === 'full' ? transaction : { ...transaction, tx_hash: undefined, from: 'Address-A', to: 'External-B', value: 'hidden' }], address_labels: {} })),
    );
    renderWithProviders(<><Refresh queryKey="grantedAddress" /><GrantedAddressPage /></>, { path: '/grant/:grantId/:addressId', initialEntries: ['/grant/g/a'] });
    await screen.findByRole('link', { name: '0x12345678...' });
    level = 'pseudonymous';
    await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
    await screen.findByText('External-B');
    expect(screen.queryByText('0xdef')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: '0x12345678...' })).not.toBeInTheDocument();
  });
});
