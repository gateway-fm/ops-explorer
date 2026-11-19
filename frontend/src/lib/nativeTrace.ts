import type { InternalTransaction } from './v2ReadTypes';

export interface NativeTraceNode { trace: InternalTransaction; path: number[]; children: NativeTraceNode[]; parentUnavailable: boolean }
function pathOf(value: string): number[] | null {
  if (value === '') return [];
  if (!/^(0|[1-9][0-9]*)(,(0|[1-9][0-9]*))*$/.test(value)) return null;
  const parts = value.split(',').map(Number);
  return parts.length <= 256 && parts.every(n => Number.isSafeInteger(n) && n <= 0xffffffff) ? parts : null;
}
function compare(a: NativeTraceNode, b: NativeTraceNode) {
  for (let i = 0; i < Math.min(a.path.length, b.path.length); i++) if (a.path[i] !== b.path[i]) return a.path[i] - b.path[i];
  return a.path.length - b.path.length;
}
export function nativeTraceForest(items: InternalTransaction[]): NativeTraceNode[] {
  // A block/address page may contain many transactions with identical paths.
  const counts = new Map<string, number>();
  const transactions = new Map<string, number>();
  const key = (trace: InternalTransaction) => `${trace.txHash}:${trace.traceAddress}`;
  for (const trace of items) {
    counts.set(key(trace), (counts.get(key(trace)) ?? 0) + 1);
    if (!transactions.has(trace.txHash)) transactions.set(trace.txHash, transactions.size);
  }
  const nodes = items.map(trace => ({trace, path: pathOf(trace.traceAddress) ?? [], children: [], parentUnavailable: trace.traceAddress !== ''} as NativeTraceNode));
  const unique = new Map(nodes.filter(node => pathOf(node.trace.traceAddress) !== null && counts.get(key(node.trace)) === 1).map(node => [key(node.trace), node]));
  const roots: NativeTraceNode[] = [];
  for (const node of nodes) {
    const parentPath = node.path.slice(0, -1).join(',');
    const parent = node.path.length ? unique.get(`${node.trace.txHash}:${parentPath}`) : undefined;
    // Only an actual, unambiguous immediate parent can own this child. Missing
    // parents are not synthesized from transaction data or private placeholders.
    if (parent && counts.get(key(node.trace)) === 1) { parent.children.push(node); node.parentUnavailable = false; }
    else roots.push(node);
  }
  const sort = (rows: NativeTraceNode[]) => { rows.sort((a, b) => transactions.get(a.trace.txHash)! - transactions.get(b.trace.txHash)! || compare(a, b)); rows.forEach(n => sort(n.children)); };
  sort(roots); return roots;
}
