import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { onlineManager, useQueryClient } from '@tanstack/react-query';
import { Link, Route, Routes } from 'react-router-dom';
import { renderWithProviders } from '../test/render';
import { API, http, HttpResponse, server } from '../test/msw';
import { GrantedTransactionDetail } from './GrantedTransactionDetail';
import { GrantedAddressPage } from './GrantedAddressPage';

const viewing = vi.hoisted(() => ({ isActive: false }));
vi.mock('../lib/auth', () => ({ useAuth: () => ({ isAuthenticated: true, isLoading: false, auth: { did: 'did:viewer' } }) }));
vi.mock('../hooks/useImpersonation', () => ({ useImpersonation: () => viewing, getImpersonationTokenHeader: () => null }));

const detail = { tx_hash: '0x123', block_number: 20, tx_index: 2, block_timestamp: 1700000000, from: '0xprivate-sender', to: '0xdef', value: '0', gas_used: 21000, gas_price: 2, tx_type: 0, input_data: '0xprivate-input', status: 1 };

function Navigation() {
  const client = useQueryClient();
  return <>
    <button onClick={() => void client.invalidateQueries({ queryKey: ['grantedAddress'] })}>Refresh grant</button>
    <button onClick={() => void client.invalidateQueries({ queryKey: ['grantedTransaction'] })}>Refresh detail</button>
    <Link to="/grant/g/a/tx/0x123">Reopen detail</Link>
    <Routes>
      <Route path="/grant/:grantId/:addressId" element={<GrantedAddressPage />} />
      <Route path="/grant/:grantId/:addressId/tx/:hash" element={<GrantedTransactionDetail />} />
    </Routes>
  </>;
}

describe('GrantedTransactionDetail', () => {
  beforeEach(() => { viewing.isActive = false; });
  it.each(['downgrade', 'scope restriction'])('revalidates a recently visited Full detail after a grant %s without displaying cached fields', async (change) => {
    let restricted = false;
    let finishRevalidation!: () => void;
    const revalidation = new Promise<void>((resolve) => { finishRevalidation = resolve; });
    const getDetail = vi.fn(async () => {
      if (!restricted) return HttpResponse.json(detail);
      await revalidation;
      return new HttpResponse(null, { status: 403 });
    });
    server.use(
      http.get(`${API}/privacy/grant/g/a/transactions/0x123`, getDetail),
      http.get(`${API}/privacy/grant/g/a`, () => HttpResponse.json({ grant_id: 'g', display_address: restricted ? 'Changed grant' : 'Original grant', disclosure_level: restricted && change === 'downgrade' ? 'pseudonymous' : 'full', scope_methods: restricted && change === 'scope restriction' ? ['activity_logs'] : ['transaction_history'] })),
      http.get(`${API}/privacy/grant/g/a/transactions`, () => HttpResponse.json({ transactions: [], address_labels: {} })),
      http.get(`${API}/privacy/grant/g/activity`, () => HttpResponse.json({ logs: [], total: 0 })),
    );
    // The shared test client keeps data fresh indefinitely: returning must still
    // check access even when ordinary cache freshness would suppress a request.
    renderWithProviders(<Navigation />, { initialEntries: ['/grant/g/a/tx/0x123'] });
    await screen.findByText(detail.input_data);
    await userEvent.click(screen.getByRole('link', { name: 'Back to disclosed transactions' }));
    await screen.findByText('Original grant');
    restricted = true;
    await userEvent.click(screen.getByRole('button', { name: 'Refresh grant' }));
    await screen.findByText('Changed grant');
    await userEvent.click(screen.getByRole('link', { name: 'Reopen detail' }));
    expect(screen.queryByText(detail.input_data)).not.toBeInTheDocument();
    expect(screen.queryByText(detail.from)).not.toBeInTheDocument();
    await waitFor(() => expect(getDetail).toHaveBeenCalledTimes(2));
    await act(async () => { finishRevalidation(); });
    await screen.findByText('Transaction details not available');
    expect(screen.queryByText(detail.input_data)).not.toBeInTheDocument();
  });

  it('hides cached Full fields during a detail refresh and after denial', async () => {
    let finishRefresh!: () => void;
    const refresh = new Promise<void>((resolve) => { finishRefresh = resolve; });
    const getDetail = vi.fn()
      .mockImplementationOnce(() => HttpResponse.json(detail))
      .mockImplementation(async () => { await refresh; return new HttpResponse(null, { status: 403 }); });
    server.use(http.get(`${API}/privacy/grant/g/a/transactions/0x123`, getDetail));
    renderWithProviders(<Navigation />, { initialEntries: ['/grant/g/a/tx/0x123'] });
    await screen.findByText(detail.input_data);
    await userEvent.click(screen.getByRole('button', { name: 'Refresh detail' }));
    expect(screen.queryByText(detail.input_data)).not.toBeInTheDocument();
    expect(screen.getByTestId('app-loading')).toBeInTheDocument();
    await act(async () => { finishRefresh(); });
    await screen.findByText('Transaction details not available');
  });

  it('keeps Full fields hidden while an access check is paused offline', async () => {
    const getDetail = vi.fn()
      .mockImplementationOnce(() => HttpResponse.json(detail))
      .mockImplementation(() => new HttpResponse(null, { status: 403 }));
    server.use(http.get(`${API}/privacy/grant/g/a/transactions/0x123`, getDetail));
    renderWithProviders(<Navigation />, { initialEntries: ['/grant/g/a/tx/0x123'] });
    await screen.findByText(detail.input_data);
    try {
      onlineManager.setOnline(false);
      await userEvent.click(screen.getByRole('button', { name: 'Refresh detail' }));
      expect(screen.queryByText(detail.input_data)).not.toBeInTheDocument();
      expect(screen.getByTestId('app-loading')).toBeInTheDocument();
      expect(getDetail).toHaveBeenCalledTimes(1);
    } finally {
      act(() => onlineManager.setOnline(true));
    }
    await screen.findByText('Transaction details not available');
  });

  it('cancels an in-flight Full request during View-as and rechecks access when it ends', async () => {
    let finishOriginal!: () => void;
    let originalSignal: AbortSignal | undefined;
    const original = new Promise<void>((resolve) => { finishOriginal = resolve; });
    const getDetail = vi.fn()
      .mockImplementationOnce(async ({ request }: { request: Request }) => {
        originalSignal = request.signal;
        await original;
        return HttpResponse.json(detail);
      })
      .mockImplementation(() => new HttpResponse(null, { status: 403 }));
    server.use(http.get(`${API}/privacy/grant/g/a/transactions/0x123`, getDetail));
    function SwitchView() {
      const [active, setActive] = useState(false);
      return <>
        <button onClick={() => { viewing.isActive = !active; setActive(!active); }}>Toggle View-as</button>
        <Navigation />
      </>;
    }
    renderWithProviders(<SwitchView />, { initialEntries: ['/grant/g/a/tx/0x123'] });
    await waitFor(() => expect(getDetail).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole('button', { name: 'Toggle View-as' }));
    expect(originalSignal?.aborted).toBe(true);
    expect(screen.queryByText(detail.input_data)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Toggle View-as' }));
    await waitFor(() => expect(getDetail).toHaveBeenCalledTimes(2));
    await act(async () => { finishOriginal(); });
    await screen.findByText('Transaction details not available');
    expect(screen.queryByText(detail.input_data)).not.toBeInTheDocument();
  });

  it('shows old-backend detail unavailability and a grant link without querying regular detail', async () => {
    const regular = vi.fn(() => HttpResponse.json({}));
    server.use(
      http.get(`${API}/privacy/grant/g/a/transactions/0x123`, () => new HttpResponse(null, { status: 404 })),
      http.get(`${API}/transactions/0x123`, regular),
    );
    renderWithProviders(<GrantedTransactionDetail />, { path: '/grant/:grantId/:addressId/tx/:hash', initialEntries: ['/grant/g/a/tx/0x123'] });
    await screen.findByText('Transaction details not available');
    expect(screen.getByRole('link', { name: 'Back to disclosed transactions' })).toHaveAttribute('href', '/grant/g/a');
    expect(regular).not.toHaveBeenCalled();
  });

  it('renders the grant detail fields and their disclosure context', async () => {
    server.use(http.get(`${API}/privacy/grant/g/a/transactions/0x123`, () => HttpResponse.json({ tx_hash: '0x123', block_number: 20, tx_index: 2, block_timestamp: 1700000000, from: '0xabc', to: '0xdef', value: '0', gas_used: 21000, gas_price: 2, tx_type: 0, input_data: '0x', status: 1 })));
    renderWithProviders(<GrantedTransactionDetail />, { path: '/grant/:grantId/:addressId/tx/:hash', initialEntries: ['/grant/g/a/tx/0x123'] });
    await screen.findByText('0x123');
    expect(screen.getByText('Disclosed')).toBeInTheDocument();
    expect(screen.getByText('21,000')).toBeInTheDocument();
  });
});
