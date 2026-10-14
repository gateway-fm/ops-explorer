import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../test/render';
import { API, http, HttpResponse, server } from '../test/msw';
import { GrantedTransactionDetail } from './GrantedTransactionDetail';

vi.mock('../lib/auth', () => ({ useAuth: () => ({ isAuthenticated: true, isLoading: false, auth: { did: 'did:viewer' } }) }));
vi.mock('../hooks/useImpersonation', () => ({ useImpersonation: () => ({ isActive: false }), getImpersonationTokenHeader: () => null }));

describe('GrantedTransactionDetail', () => {
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
