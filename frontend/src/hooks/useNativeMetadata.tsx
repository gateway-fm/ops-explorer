import { createContext, useContext, useEffect, useMemo, useSyncExternalStore, type ReactNode } from 'react';
import { appSession, type AppOperation } from '../lib/v2Session';

// A route owns one coordinator, shared by names, transfers, hero and holdings.
// Neither the coordinator nor its results survive a route/activation boundary.
class MetadataPage {
  private values: Record<string, unknown> = {};
  private selected = new Set<string>();
  private queue: string[] = [];
  private pending = new Map<string, number>();
  private scheduled = false;
  private running = 0;
  private listeners = new Set<() => void>();
  private operation?: AppOperation;
  private disposed = false;
  private lifecycle = 0;
  attach() {
    const generation = ++this.lifecycle;
    return () => { queueMicrotask(() => { if (generation === this.lifecycle) this.dispose(); }); };
  }
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; };
  snapshot = () => this.values;
  add(paths: string[], priority: number) {
    if (this.disposed) return;
    for (const path of paths) {
      if (this.selected.has(path) || !/^\/v2\/explorer\/(?:tokens\/0x[0-9a-f]{40}|addresses\/0x[0-9a-f]{40}\/contract)$/.test(path) || path.includes('0x' + '0'.repeat(40))) continue;
      this.pending.set(path, Math.max(priority, this.pending.get(path) ?? -Infinity));
    }
    if (!this.pending.size || this.scheduled) return;
    this.scheduled = true;
    // Gather this render's consumers before choosing the page's targets. Hero
    // and exact asset units take priority over optional party names.
    queueMicrotask(() => {
      this.scheduled = false;
      if (this.disposed) return;
      const candidates = [...this.pending].sort(([a, ap], [b, bp]) => bp - ap || a.localeCompare(b));
      this.pending.clear();
      for (const [path] of candidates) {
        if (this.selected.size >= 25) break;
        if (this.selected.has(path)) continue;
        this.selected.add(path); this.queue.push(path);
      }
      if (!this.queue.length) return;
      if (!this.operation) { try { this.operation = appSession.captureOperation(); } catch { return; } }
      this.pump();
    });
  }
  private pump() {
    while (!this.disposed && this.running < 4 && this.queue.length) {
      const path = this.queue.shift()!, operation = this.operation!; this.running++;
      void (async () => {
        try {
          operation.check(); const value = await appSession.read<unknown>(path, operation.signal); operation.check();
          if (!this.disposed) { this.values = {...this.values, [path]: value}; this.listeners.forEach(listener => listener()); }
        } catch { /* Optional denied/missing metadata leaves the authorized row intact. */ }
        finally { this.running--; if (!this.disposed && !operation.signal.aborted) this.pump(); }
      })();
    }
  }
  dispose() { this.disposed = true; this.queue = []; this.pending.clear(); this.operation?.dispose(); this.listeners.clear(); this.values = {}; }
}
const MetadataContext = createContext<MetadataPage | null>(null);
export function NativeMetadataBoundary({children, ownerKey}: {children: ReactNode; ownerKey?: string}) {
  const identity = useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity;
  const page = useMemo(() => new MetadataPage(), [identity?.actor, identity?.profile, identity?.activation, ownerKey]);
  useEffect(() => page.attach(), [page]);
  return <MetadataContext.Provider value={page}>{children}</MetadataContext.Provider>;
}
export function useNativeMetadata<T>(paths: string[], decode: (raw: unknown, path?: string) => T, priority = 0): Record<string, T> {
  const identity = useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity;
  const shared = useContext(MetadataContext);
  // Focused standalone consumers still have the same bounded coordinator.
  const local = useMemo(() => new MetadataPage(), [identity?.actor, identity?.profile, identity?.activation]);
  const page = shared ?? local;
  useEffect(() => local.attach(), [local]);
  const targets = JSON.stringify([...new Set(paths)].slice(0, 25));
  useEffect(() => { if (identity) page.add(JSON.parse(targets) as string[], priority); }, [page, identity, targets, priority]);
  const raw = useSyncExternalStore(page.subscribe, page.snapshot);
  return useMemo(() => {
    if (!identity) return {};
    const result: Record<string, T> = {};
    for (const path of JSON.parse(targets) as string[]) {
      if (raw[path] === undefined) continue;
      try { result[path] = decode(raw[path], path); } catch { /* strict DTO failure is optional enrichment failure */ }
    }
    return result;
  }, [raw, identity, targets, decode]);
}
