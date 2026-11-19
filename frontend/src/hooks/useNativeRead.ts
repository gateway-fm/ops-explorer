import { useCallback, useEffect, useState, useSyncExternalStore } from 'react';
import { appSession, ReadHTTPError } from '../lib/v2Session';

export interface NativeRead<T> {
  data?: T;
  loading: boolean;
  error?: string;
  stale?: boolean;
  refresh?: () => void;
}

// Every result belongs to its original app activation and exact endpoint.
// Route changes and account changes cannot display the prior result while a
// later request is pending. The same session owner fences transport and decode.
export function useNativeRead<T>(endpoint: string | null, decode: (raw: unknown) => T, refreshMs = 0): NativeRead<T> {
  const identity = useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity;
  const [attempt, setAttempt] = useState(0);
  const refresh = useCallback(() => setAttempt(value => value + 1), []);
  useEffect(() => { if (refreshMs <= 0 || !identity) return; const timer = window.setInterval(refresh, refreshMs); return () => window.clearInterval(timer); }, [refreshMs, refresh, identity]);
  const key = `${identity?.activation ?? ''}:${endpoint ?? ''}`;
  const [result, setResult] = useState<NativeRead<T> & { key: string }>({ key, loading: true });

  useEffect(() => {
    if (!endpoint || !identity) return;
    let active = true;
    let owner: ReturnType<typeof appSession.captureOperation> | undefined;
    try {
      owner = appSession.captureOperation();
      const operation = owner;
      void appSession.read<unknown>(endpoint, operation.signal).then(raw => {
        operation.check();
        const data = decode(raw);
        operation.check();
        if (active) setResult({ key, data, loading: false });
      }).catch(error => {
        if (!active || operation.signal.aborted) return;
        try { operation.check(); } catch { return; }
        setResult({ key, loading: false, error: error instanceof ReadHTTPError ? error.message : 'The read could not be completed. Try again.', stale: error instanceof ReadHTTPError && ['cursor_stale', 'history_pruned'].includes(error.code) });
      });
    } catch {
      setResult({ key, loading: false, error: 'Verify your app session before reading data.' });
    }
    return () => { active = false; owner?.dispose(); };
  }, [endpoint, key, identity, decode, attempt]);

  if (!endpoint) return { loading: false, refresh };
  if (!identity) return { loading: false, refresh, error: 'Verify your app session before reading data.' };
  if(result.key !== key)return { loading: true, refresh };
  const {key: _owner, ...view}=result;
  return { ...view, refresh };
}
