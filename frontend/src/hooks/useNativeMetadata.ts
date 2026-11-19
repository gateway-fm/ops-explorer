import { useEffect, useState, useSyncExternalStore } from 'react';
import { appSession } from '../lib/v2Session';

// Metadata is optional enrichment, never a substitute for the authorized row.
// One visible page owns at most 25 distinct reads and four concurrent requests.
export function useNativeMetadata<T>(paths: string[], decode: (raw: unknown) => T): Record<string, T> {
  const identity = useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity;
  const targets = JSON.stringify([...new Set(paths)].slice(0, 25));
  const key = `${identity?.activation ?? ''}:${targets}`;
  const [result, setResult] = useState<{key: string; values: Record<string, T>}>({key, values: {}});
  useEffect(() => {
    if (!identity) return;
    const selected = JSON.parse(targets) as string[];
    if (!selected.length) return;
    let active = true, next = 0;
    let owner: ReturnType<typeof appSession.captureOperation>;
    try { owner = appSession.captureOperation(); } catch { return; }
    async function work() {
      while (active && next < selected.length && !owner.signal.aborted) {
        const path = selected[next++];
        try {
          owner.check();
          const raw = await appSession.read<unknown>(path, owner.signal);
          owner.check();
          const value = decode(raw);
          owner.check();
          if (active) setResult(previous => ({key, values: {...(previous.key === key ? previous.values : {}), [path]: value}}));
        } catch {
          // A denied/missing/invalid optional record leaves the row unchanged.
          // Context replacement retires all remaining work, not just this read.
          try { owner.check(); } catch { return; }
        }
      }
    }
    for (let i = 0; i < Math.min(4, selected.length); i++) void work();
    return () => { active = false; owner.dispose(); };
  }, [identity, targets, key, decode]);
  return identity && result.key === key ? result.values : {};
}
