import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SessionState } from './v2Session';
import { V2App } from '../V2App';

const session = vi.hoisted(() => {
  let state: SessionState = { identity: null, loading: false, changed: false, error: null };
  const listeners = new Set<() => void>();
  return {
    snapshot: () => state,
    subscribe: (listener: () => void) => { listeners.add(listener); return () => listeners.delete(listener); },
    set: (next: SessionState) => { state = next; listeners.forEach(listener => listener()); },
    check: vi.fn(async () => {}), login: vi.fn(async () => {}), logout: vi.fn(async () => {}),
    tick: vi.fn(), onClear: vi.fn(), notice: vi.fn(), suspend: vi.fn(), discard: vi.fn(),
    read: vi.fn(), captureOperation: vi.fn(), disclosure: vi.fn(),
  };
});
vi.mock('./v2Session', async importOriginal => ({ ...await importOriginal<typeof import('./v2Session')>(), appSession: session }));
const anonymous: SessionState = {identity:null,loading:false,changed:false,error:null};
const expired = {actor:'ops:p1:'+'A'.repeat(43),profile:'azure',activation:'11111111-1111-1111-1111-111111111111',expires:1};
beforeEach(() => {
  vi.stubGlobal('localStorage',{getItem:()=>null,setItem:()=>{},removeItem:()=>{}});
  session.set(anonymous);
  vi.clearAllMocks(); session.login.mockResolvedValue(); session.check.mockResolvedValue();
  vi.stubGlobal('fetch',vi.fn(async()=>{throw Error('Unexpected public request');}));
  window.history.replaceState({},'','/');
});
function expectNoPrivateDispatch() {
  expect(session.read).not.toHaveBeenCalled();
  expect(session.captureOperation).not.toHaveBeenCalled();
  expect(session.disclosure).not.toHaveBeenCalled();
  expect(screen.queryByRole('heading',{name:'Latest Transactions'})).not.toBeInTheDocument();
  expect(screen.queryByTestId('stat-card')).not.toBeInTheDocument();
}

describe('native Explorer entry uses the original signed-out presentation', () => {
  it.each([
    [anonymous,'anonymous','Sign in to Explorer'],
    [{...anonymous,loading:true},'loading','Checking your session'],
    [{...anonymous,reason:'expired'},'expired','Session expired'],
    [{...anonymous,identity:expired},'expired','Session expired'],
    [{...anonymous,changed:true},'changed','Your account changed'],
    [{...anonymous,error:'Cannot verify app session. Try again.'},'error','Unable to verify your session'],
  ] as const)('keeps private reads and network diagnostics off the %s entry', async(state,kind,title) => {
    session.set(state); render(<V2App/>);
    expect(screen.getByRole('heading',{name:title,exact:true})).toBeInTheDocument();
    const entry=screen.getByTestId('explorer-auth-entry');
    expect(entry).toHaveAttribute('data-auth-state',kind);
    if(kind==='loading') {
      expect(within(entry).queryByRole('button')).not.toBeInTheDocument();
      expect(screen.getByRole('button',{name:'Checking session…'})).toBeDisabled();
    } else expect(within(entry).getAllByRole('button').every(button=>button.className.includes('btn-'))).toBe(true);
    fireEvent.click(screen.getByTestId('mobile-menu-trigger'));
    expect(screen.queryByPlaceholderText(/Search/)).not.toBeInTheDocument();
    expect(screen.queryByText('Genesis hash')).not.toBeInTheDocument();
    expectNoPrivateDispatch(); expect(fetch).not.toHaveBeenCalled();
  });
  it('requires explicit acceptance of a changed account, once, with no private dispatch before verification', async()=>{
    session.set({...anonymous,changed:true}); render(<V2App/>);session.check.mockClear();
    let finish!:()=>void; session.check.mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));
    const button=screen.getByRole('button',{name:'Use current session'});
    fireEvent.click(button); fireEvent.click(button);
    expect(session.check).toHaveBeenCalledTimes(1); expect(session.check).toHaveBeenCalledWith(true);
    expect(button).toBeDisabled();expectNoPrivateDispatch();
    await act(async()=>{finish();});
  });
  it('treats unavailable shared state as a verification error rather than a known account change', async()=>{
    const message='Shared app state is unavailable. Allow local storage, then verify the session again.';
    session.set({...anonymous,changed:true,error:message});render(<V2App/>);session.check.mockClear();
    expect(screen.getByTestId('explorer-auth-entry')).toHaveAttribute('data-auth-state','error');
    expect(screen.getByRole('heading',{name:'Unable to verify your session',exact:true})).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent(message);
    expect(screen.queryByRole('button',{name:'Use current session'})).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button',{name:'Check again',exact:true}));
    await waitFor(()=>expect(session.check).toHaveBeenCalledTimes(1));
    expect(session.check).toHaveBeenCalledWith();expectNoPrivateDispatch();
  });
  it('shows a usable sign-in failure without swallowing it or making private reads',async()=>{
    session.login.mockRejectedValueOnce(Error('Unavailable'));render(<V2App/>);
    fireEvent.click(screen.getByTestId('entry-sign-in'));
    expect(await screen.findByRole('alert')).toHaveTextContent('Sign in could not start. Please try again.');
    expect(screen.getByTestId('entry-sign-in')).toBeEnabled();expectNoPrivateDispatch();
  });
  it('makes only approved public network setup reachable while signed out',async()=>{
    const setup={protocol:'ops-v2',source:'approved-config',chainId:'0x7a69',chainIdDecimal:'31337',genesisHash:'0x'+'a'.repeat(64),privacyProxyPublicUrl:'https://ops.test'};
    vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify(setup),{headers:{'Content-Type':'application/json'}})));
    window.history.replaceState({},'','/chain-info');render(<V2App/>);
    expect(await screen.findByText(setup.genesisHash)).toBeInTheDocument();
    expect(screen.getByRole('heading',{name:'Network setup',exact:true})).toBeInTheDocument();
    expect(screen.queryByTestId('explorer-auth-entry')).not.toBeInTheDocument();
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch).toHaveBeenCalledWith('/api/chain-info',expect.objectContaining({cache:'no-store',redirect:'error'}));
    expectNoPrivateDispatch();
  });
});
