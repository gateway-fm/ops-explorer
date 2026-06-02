import type { InternalTransaction, Transaction } from './api';

export interface TraceNode {
  trace: InternalTransaction;
  depth: number;
  children: TraceNode[];
}

// Stored internal txs are sub-calls only (the indexer strips the root frame),
// so we synthesize the root node from the transaction itself and hang the
// indexed calls underneath it as a tree keyed by their comma-separated
// trace_address (e.g. "0", "2,0").
export function buildRoot(tx: Transaction): InternalTransaction {
  const isCreate = !tx.to && !!tx.contractAddress;
  return {
    id: -1,
    txHash: tx.hash,
    blockNumber: tx.blockNumber,
    traceAddress: '',
    from: tx.from,
    to: tx.to ?? tx.contractAddress ?? null,
    value: tx.value ?? '0',
    gasUsed: tx.gasUsed,
    input: tx.inputData ? (tx.inputData.startsWith('0x') ? tx.inputData : `0x${tx.inputData}`) : undefined,
    callType: isCreate ? 'create' : 'call',
    error: tx.status === 1 ? undefined : (tx.error || tx.revertReason || 'reverted'),
    addressMetadata: tx.addressMetadata,
  };
}

// Numeric pre-order comparison: compares trace_address segment-by-segment as
// numbers, with a shorter (ancestor) path sorting before its descendants. This
// guarantees every parent is visited before its children.
export function compareTraceAddr(a: string, b: string): number {
  const pa = a.split(',').map(Number);
  const pb = b.split(',').map(Number);
  for (let i = 0; i < Math.min(pa.length, pb.length); i++) {
    if (pa[i] !== pb[i]) return pa[i] - pb[i];
  }
  return pa.length - pb.length;
}

export function buildTree(tx: Transaction, internalTxs: InternalTransaction[]): TraceNode {
  const root: TraceNode = { trace: buildRoot(tx), depth: 0, children: [] };
  const nodes = new Map<string, TraceNode>([['', root]]);

  // Pre-order sort guarantees every parent is inserted before its children.
  const sorted = [...internalTxs].sort((a, b) => compareTraceAddr(a.traceAddress, b.traceAddress));
  for (const itx of sorted) {
    const parts = itx.traceAddress.split(',');
    const node: TraceNode = { trace: itx, depth: parts.length, children: [] };
    nodes.set(itx.traceAddress, node);
    const parentKey = parts.slice(0, -1).join(',');
    (nodes.get(parentKey) ?? root).children.push(node);
  }
  return root;
}
