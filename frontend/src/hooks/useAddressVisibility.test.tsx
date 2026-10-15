import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { onlineManager } from '@tanstack/react-query';
import { deferred } from '../test/deferred';
import { useViewableAddresses } from './useAddressVisibility';
import { renderWithProviders } from '../test/render';
import { API, http, HttpResponse, server } from '../test/msw';

const viewer = vi.hoisted(() => ({ authenticated: true, did: 'did:viewer', isActive: false, token: null as string | null }));
vi.mock('../lib/auth', () => ({ useAuth: () => ({ isAuthenticated: viewer.authenticated, auth: { did: viewer.did } }) }));
vi.mock('./useImpersonation', () => ({ useImpersonation: () => viewer, getImpersonationTokenHeader: () => viewer.token }));

function Harness() {
  const [, render] = useState(0);
  const { data, loading, refetch } = useViewableAddresses();
  return <>
    <button onClick={() => { viewer.isActive = true; viewer.token = 'scoped'; render((n) => n + 1); }}>View as</button>
    <button onClick={() => { viewer.isActive = false; viewer.token = null; render((n) => n + 1); }}>Stop View as</button>
    <button onClick={refetch}>Refresh</button>
    <button onClick={() => { viewer.did = 'did:other'; render(2); }}>Switch account</button>
    <button onClick={() => { viewer.authenticated = false; render(3); }}>Sign out</button>
    <div data-testid="grants">{data?.disclosedAddresses.map((grant) => grant.grantId).join(',')}</div>
    {loading && <div>Checking access</div>}
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
    let signal: AbortSignal | undefined;
    server.use(http.get(`${API}/privacy/viewable-addresses`, async ({ request }) => { signal = request.signal; await pending; return HttpResponse.json({ disclosed_addresses: [{ grant_id: 'late-admin-grant' }] }); }));
    renderWithProviders(<Harness />);
    await waitFor(() => expect(signal).toBeDefined());
    await userEvent.click(screen.getByRole('button', { name: 'View as' }));
    expect(signal?.aborted).toBe(true);
    await act(async () => { finish(); await pending; });
    expect(screen.getByTestId('grants')).toBeEmptyDOMElement();
  });

  it('requires a new address check after View-as before showing any cached grants', async () => {
    Object.assign(viewer, { authenticated: true, did: 'did:viewer', isActive: false, token: null });
    const pending = deferred<void>();
    const list = vi.fn()
      .mockImplementationOnce(() => HttpResponse.json({ disclosed_addresses: [{ grant_id: 'grant-admin' }] }))
      .mockImplementation(async () => { await pending.promise; return HttpResponse.json({ disclosed_addresses: [] }); });
    server.use(http.get(`${API}/privacy/viewable-addresses`, list));
    renderWithProviders(<Harness />);
    await screen.findByText('grant-admin');
    await userEvent.click(screen.getByRole('button', { name: 'View as' }));
    await userEvent.click(screen.getByRole('button', { name: 'Stop View as' }));
    expect(screen.getByTestId('grants')).toBeEmptyDOMElement();
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    await act(async () => { pending.resolve(); });
    await waitFor(() => expect(screen.queryByText('Checking access')).not.toBeInTheDocument());
    expect(screen.getByTestId('grants')).toBeEmptyDOMElement();
  });

  it('hides cached dashboard grants during offline-paused revalidation', async () => {
    Object.assign(viewer, { authenticated: true, did: 'did:viewer', isActive: false, token: null });
    const list = vi.fn()
      .mockImplementationOnce(() => HttpResponse.json({ disclosed_addresses: [{ grant_id: 'grant-admin' }] }))
      .mockImplementation(() => new HttpResponse(null, { status: 403 }));
    server.use(http.get(`${API}/privacy/viewable-addresses`, list));
    renderWithProviders(<Harness />);
    await screen.findByText('grant-admin');
    try {
      onlineManager.setOnline(false);
      await userEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      expect(screen.getByTestId('grants')).toBeEmptyDOMElement();
      expect(screen.getByText('Checking access')).toBeInTheDocument();
      expect(list).toHaveBeenCalledTimes(1);
    } finally {
      act(() => onlineManager.setOnline(true));
    }
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByText('Checking access')).not.toBeInTheDocument());
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
