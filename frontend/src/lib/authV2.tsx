import { useEffect, useSyncExternalStore, type Context, type ReactNode } from 'react';
import { flushSync } from 'react-dom';
import { useQueryClient } from '@tanstack/react-query';
import type { AuthContextType } from './authContext';
import { appSession } from './v2Session';
import { NativeAuthenticationPrompt } from '../components/NativeAuthenticationPrompt';

export function V2AuthProvider({ children, context: Context, gate }: { children: ReactNode; context: Context<AuthContextType | undefined>; gate?: (content: ReactNode) => ReactNode }) {
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
  const value: AuthContextType = { isAuthenticated: !!identity, isLoading: state.loading, auth: { authenticated: !!identity, did: identity?.actor ?? null, expiresAt: identity?.expires ?? null }, logout: () => appSession.logout(), checkStatus: () => appSession.check() };
  const fallback = <NativeAuthenticationPrompt state={state} />;
  return <Context.Provider value={value}>{identity ? children : gate ? gate(fallback) : fallback}</Context.Provider>;
}
