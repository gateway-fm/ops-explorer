import { useEffect, useSyncExternalStore, type Context, type ReactNode } from 'react';
import { flushSync } from 'react-dom';
import { useQueryClient } from '@tanstack/react-query';
import type { AuthContextType } from './authContext';
import { appSession } from './v2Session';
import { PublicNetworkSetup } from '../pages/NetworkSetupV2';

export function V2AuthProvider({ children, context: Context }: { children: ReactNode; context: Context<AuthContextType | undefined> }) {
  const state = useSyncExternalStore(appSession.subscribe, appSession.snapshot);
  const queries = useQueryClient();
  useEffect(() => {
    appSession.onClear(() => { void queries.cancelQueries(); queries.clear(); });
    void appSession.check();
    const timer = window.setInterval(() => { appSession.tick(); }, 500);
    const focus = () => { void appSession.check(); };
    const notice = (event: StorageEvent) => { flushSync(() => appSession.notice(event)); };
    const hide = () => { flushSync(() => appSession.suspend()); };
    const show = (event: PageTransitionEvent) => { if (event.persisted) window.location.reload(); };
    window.addEventListener('focus', focus); window.addEventListener('storage', notice); window.addEventListener('pagehide', hide); window.addEventListener('pageshow', show);
    return () => { clearInterval(timer); window.removeEventListener('focus', focus); window.removeEventListener('storage', notice); window.removeEventListener('pagehide', hide); window.removeEventListener('pageshow', show); appSession.discard(); };
  }, [queries]);
  const identity = state.identity && state.identity.expires > Math.floor(Date.now() / 1000) ? state.identity : null;
  const login = () => { void appSession.login().catch(() => {}); };
  const value: AuthContextType = { isAuthenticated: !!identity, isLoading: state.loading, auth: { authenticated: !!identity, did: identity?.actor ?? null, expiresAt: identity?.expires ?? null }, logout: () => appSession.logout(), checkStatus: () => appSession.check() };
  return <Context.Provider value={value}>{identity ? children : <main className="max-w-3xl mx-auto p-8 space-y-5"><h1 className="text-2xl">Explorer</h1>{state.loading ? <p>Checking your session…</p> : <><p>{state.changed ? 'The app session changed. Confirm before showing another account’s data.' : 'Sign in to view private chain data.'}</p>{state.error && <p role="alert">{state.error}</p>}<button onClick={login}>Sign in</button>{state.changed && <button onClick={() => { void appSession.check(true); }}>Use the current app session</button>}<button onClick={() => { void appSession.check(); }}>Check again</button></>}<PublicNetworkSetup /></main>}</Context.Provider>;
}
