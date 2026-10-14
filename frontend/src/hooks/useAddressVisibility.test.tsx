import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useViewableAddresses } from './useAddressVisibility';
import { renderWithProviders } from '../test/render';
import { API, http, HttpResponse, server } from '../test/msw';

const viewer = vi.hoisted(() => ({ authenticated: true, did: 'did:viewer', isActive: false, token: null as string | null }));
vi.mock('../lib/auth', () => ({ useAuth: () => ({ isAuthenticated: viewer.authenticated, auth: { did: viewer.did } }) }));
vi.mock('./useImpersonation', () => ({ useImpersonation: () => viewer, getImpersonationTokenHeader: () => viewer.token }));

function Harness() {
  const [, render] = useState(0);
  const { data } = useViewableAddresses();
  return <>
    <button onClick={() => { viewer.isActive = true; viewer.token = 'scoped'; render(1); }}>View as</button>
    <button onClick={() => { viewer.did = 'did:other'; render(2); }}>Switch account</button>
    <button onClick={() => { viewer.authenticated = false; render(3); }}>Sign out</button>
    <div data-testid="grants">{data?.disclosedAddresses.map((grant) => grant.grantId).join(',')}</div>
  </>;
}

describe('viewable address grant isolation', () => {
  it('removes cached grants immediately on View-as and does not fetch the admin list', async () => {
    Object.assign(viewer, { authenticated: true, did: 'did:viewer', isActive: false, token: null });
    const list = vi.fn(() => HttpResponse.json({ disclosed_addresses: [{ grant_id: 'grant-admin' }] }));
    server.use(http.get(`${API}/privacy/viewable-addresses`, list));
    renderWithProviders(<Harness />);
    await screen.findByText('grant-admin');
    await userEvent.click(screen.getByRole('button', { name: 'View as' }));
    expect(screen.getByTestId('grants')).toBeEmptyDOMElement();
    expect(list).toHaveBeenCalledTimes(1);
  });

  it('keeps an in-flight admin response hidden when View-as starts', async () => {
    Object.assign(viewer, { authenticated: true, did: 'did:viewer', isActive: false, token: null });
    let finish!: () => void;
    const pending = new Promise<void>((resolve) => { finish = resolve; });
    let started = false;
    server.use(http.get(`${API}/privacy/viewable-addresses`, async () => { started = true; await pending; return HttpResponse.json({ disclosed_addresses: [{ grant_id: 'late-admin-grant' }] }); }));
    renderWithProviders(<Harness />);
    await waitFor(() => expect(started).toBe(true));
    await userEvent.click(screen.getByRole('button', { name: 'View as' }));
    const delivered = new Promise<void>((resolve) => server.events.once('response:mocked', () => resolve()));
    await act(async () => { finish(); await delivered; });
    expect(screen.getByTestId('grants')).toBeEmptyDOMElement();
  });

  it('isolates accounts and clears grants on sign-out', async () => {
    Object.assign(viewer, { authenticated: true, did: 'did:viewer', isActive: false, token: null });
    server.use(http.get(`${API}/privacy/viewable-addresses`, () => HttpResponse.json({ disclosed_addresses: [{ grant_id: `grant-${viewer.did}` }] })));
    renderWithProviders(<Harness />);
    await screen.findByText('grant-did:viewer');
    await userEvent.click(screen.getByRole('button', { name: 'Switch account' }));
    expect(screen.queryByText('grant-did:viewer')).not.toBeInTheDocument();
    await screen.findByText('grant-did:other');
    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    expect(screen.getByTestId('grants')).toBeEmptyDOMElement();
  });
});
