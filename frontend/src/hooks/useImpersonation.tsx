import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';

/**
 * RD-928 "View as user" — admin-driven impersonation for tier-2 admins.
 *
 * Design notes:
 *   - The browser only ever sees the opaque session token, never the target
 *     DID directly. The token lives in the URL as ?as=<token> so refreshes
 *     and shareable links inside the org are explicit and revocable, while
 *     DIDs are kept out of history / referrers / clipboard paste.
 *   - One session at a time per tab. start() while a session is already
 *     active throws — the UI is expected to surface that as "exit first".
 *   - An expiry watcher fires stop() automatically when the token TTL
 *     passes, so a banner cannot linger beyond the BFF-honoured window.
 */

export interface ImpersonationSession {
  token: string;
  targetDID: string;
  expiresAt: Date;
}

export interface ImpersonationContextValue {
  session: ImpersonationSession | null;
  /** Convenience aliases — common access pattern, avoids `?.` everywhere. */
  token: string | null;
  targetDID: string | null;
  expiresAt: Date | null;
  isActive: boolean;
  /** Last error message thrown by start/stop; null if none. */
  error: string | null;
  /**
   * Mint a session for the given target. Throws if a session is already
   * active (no chained impersonation), or if the backend rejects (cross-org
   * or non-admin produces a 404 here, mirrored from the privacy-proxy).
   */
  start: (targetDID: string) => Promise<void>;
  /** Revoke + clear current session. Safe to call when no session. */
  stop: () => Promise<void>;
}

const ImpersonationContext = createContext<ImpersonationContextValue | undefined>(undefined);

const URL_PARAM = 'as';
const START_ENDPOINT = '/api/impersonation/start';
const STOP_ENDPOINT = (token: string) => `/api/impersonation/${encodeURIComponent(token)}`;

interface StartResponse {
  token: string;
  expires_at: string;
  target_did: string;
}

function readTokenFromURL(): string | null {
  if (typeof window === 'undefined') return null;
  const params = new URLSearchParams(window.location.search);
  return params.get(URL_PARAM);
}

function writeTokenToURL(token: string | null) {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (token) {
    url.searchParams.set(URL_PARAM, token);
  } else {
    url.searchParams.delete(URL_PARAM);
  }
  window.history.replaceState({}, '', url.toString());
}

/**
 * ImpersonationProvider wires the session into a React context. Mounting it
 * at the root (above Routes) is intentional — the ?as= URL param must be
 * read before any route component renders, because route components use the
 * fetch wrapper that needs the header.
 */
export function ImpersonationProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<ImpersonationSession | null>(null);
  const [error, setError] = useState<string | null>(null);
  const expiryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearExpiryTimer = useCallback(() => {
    if (expiryTimerRef.current) {
      clearTimeout(expiryTimerRef.current);
      expiryTimerRef.current = null;
    }
  }, []);

  const internalStop = useCallback(
    async (token: string | null, opts: { silent?: boolean } = {}) => {
      clearExpiryTimer();
      if (token) {
        try {
          await fetch(STOP_ENDPOINT(token), { method: 'DELETE', credentials: 'include' });
        } catch (err) {
          // Best-effort: revocation failure should not leave the UI stuck in
          // view-as mode. We still clear the local session.
          if (!opts.silent) {
            console.warn('impersonation: stop request failed', err);
          }
        }
      }
      setSession(null);
      writeTokenToURL(null);
    },
    [clearExpiryTimer]
  );

  const stop = useCallback(async () => {
    setError(null);
    await internalStop(session?.token ?? null);
  }, [internalStop, session]);

  const start = useCallback(
    async (targetDID: string) => {
      if (session) {
        const msg = 'Already viewing as another user — stop the current view-as session first.';
        setError(msg);
        throw new Error(msg);
      }
      setError(null);
      const resp = await fetch(START_ENDPOINT, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target_did: targetDID }),
      });
      if (!resp.ok) {
        // Mirror the BFF response shape. 404 is "target not visible" (could
        // be cross-org or simply nonexistent); 403 is "not allowed". We keep
        // the wording generic to avoid information leak.
        let msg: string;
        switch (resp.status) {
          case 404:
            msg = 'User not found or not visible from your org.';
            break;
          case 403:
            msg = 'You are not authorised to use view-as for this user.';
            break;
          case 401:
            msg = 'You must be signed in to use view-as.';
            break;
          default:
            msg = `Failed to start view-as session (${resp.status}).`;
        }
        setError(msg);
        throw new Error(msg);
      }
      const data: StartResponse = await resp.json();
      const next: ImpersonationSession = {
        token: data.token,
        targetDID: data.target_did,
        expiresAt: new Date(data.expires_at),
      };
      setSession(next);
      writeTokenToURL(next.token);
    },
    [session]
  );

  // Schedule an auto-stop when the token TTL passes. Re-runs on every
  // session change so successive view-as sessions all get their own timer.
  useEffect(() => {
    clearExpiryTimer();
    if (!session) return;
    const ms = session.expiresAt.getTime() - Date.now();
    if (ms <= 0) {
      // Already expired — clean up immediately. The setState happens via
      // internalStop, which is the documented external-state-sync action
      // for this effect (clearing a server-bound session that's gone stale).
      // eslint-disable-next-line react-hooks/set-state-in-effect -- intentional clear of an externally-expired session
      void internalStop(session.token, { silent: true });
      return;
    }
    expiryTimerRef.current = setTimeout(() => {
      void internalStop(session.token, { silent: true });
    }, ms);
    return clearExpiryTimer;
  }, [session, internalStop, clearExpiryTimer]);

  // On first mount, restore from ?as=<token>. We do a probe DELETE-style
  // request — instead, we just trust the URL token and let the next API
  // call surface the 401 if the BFF has discarded it. That keeps cold-mount
  // fast and avoids a wasted round-trip in the happy path; a stale token
  // self-cleans on its first real use.
  useEffect(() => {
    const tok = readTokenFromURL();
    if (!tok) return;
    // We don't know the targetDID yet — the BFF response on next API call
    // doesn't carry it back. We display a generic label until the user
    // either acts or refreshes via a navigation that fetches /api/impersonation
    // session info. For now, render with a placeholder DID; the banner UI
    // copes with that.
    // eslint-disable-next-line react-hooks/set-state-in-effect -- restoring session from URL on mount
    setSession({
      token: tok,
      targetDID: '',
      expiresAt: new Date(Date.now() + 60 * 60 * 1000), // optimistic upper bound
    });
    // Best-effort: ping the BFF to load the real session metadata. The
    // endpoint is optional; if it doesn't exist we keep the placeholder and
    // let the next real request surface a 401 to clear state.
    void (async () => {
      try {
        const resp = await fetch(`/api/impersonation/${encodeURIComponent(tok)}`, {
          method: 'GET',
          credentials: 'include',
        });
        if (resp.ok) {
          const data = (await resp.json()) as { target_did: string; expires_at: string };
          setSession({
            token: tok,
            targetDID: data.target_did,
            expiresAt: new Date(data.expires_at),
          });
        } else if (resp.status === 401 || resp.status === 404) {
          setSession(null);
          writeTokenToURL(null);
        }
      } catch {
        // network blip — leave the placeholder so the user isn't kicked
        // out of view-as on a transient error.
      }
    })();
    // We only want this to run on the very first mount; an empty dep array
    // is correct here.
  }, []);

  const value = useMemo<ImpersonationContextValue>(
    () => ({
      session,
      token: session?.token ?? null,
      targetDID: session?.targetDID ?? null,
      expiresAt: session?.expiresAt ?? null,
      isActive: session !== null,
      error,
      start,
      stop,
    }),
    [session, error, start, stop]
  );

  return <ImpersonationContext.Provider value={value}>{children}</ImpersonationContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useImpersonation(): ImpersonationContextValue {
  const ctx = useContext(ImpersonationContext);
  if (ctx === undefined) {
    throw new Error('useImpersonation must be used within an ImpersonationProvider');
  }
  return ctx;
}

// Module-level singleton reference to the current token, so the fetch
// wrapper (non-hook context) can attach the header without requiring every
// fetch call site to be inside a React component. The provider keeps this
// in sync with `session.token`.
let currentToken: string | null = null;

/** Internal helper used by the fetch wrapper. Not exported in barrel files. */
// eslint-disable-next-line react-refresh/only-export-components
export function getImpersonationTokenHeader(): string | null {
  return currentToken;
}

/**
 * Subscribe-style hook used by ImpersonationProvider to mirror its token
 * into the module-level variable. Wrapped in useEffect rather than direct
 * assignment so React's render purity is preserved.
 */
export function ImpersonationTokenMirror() {
  const { token } = useImpersonation();
  useEffect(() => {
    currentToken = token;
    return () => {
      // No-op on unmount: ImpersonationProvider lives for the app lifetime.
    };
  }, [token]);
  return null;
}
