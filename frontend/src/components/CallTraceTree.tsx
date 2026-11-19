import { useMemo, useState } from 'react';
import { TraceRowPresentation } from './TraceRowPresentation';
import type { Transaction, InternalTransaction } from '../lib/api';
import { buildTree, type TraceNode } from '../lib/callTrace';
import { AddressLink } from './AddressLink';
import { AddressLabel } from './AddressLabel';
import { formatWei, formatGas, getNetworkCurrency } from '../lib/utils';

function selectorOf(input?: string): string | null {
  if (!input) return null;
  const hex = input.startsWith('0x') ? input : `0x${input}`;
  if (hex.length < 10) return null; // need 0x + 8 nibbles
  return hex.slice(0, 10).toLowerCase();
}

function hasValue(value: string | number | null | undefined): boolean {
  if (value === '' || value == null) return false;
  try {
    return BigInt(value) > 0n;
  } catch {
    return false;
  }
}

function TraceRow({ node }: { node: TraceNode }) {
  const [open, setOpen] = useState(true);
  const { trace, depth, children } = node;
  const hasChildren = children.length > 0;
  const isRoot = depth === 0;
  const callType = (trace.callType || 'call').toLowerCase();
  const selector = selectorOf(trace.input);
  const fromReason = trace.addressMetadata?.[trace.from?.toLowerCase()];
  const toReason = trace.to ? trace.addressMetadata?.[trace.to.toLowerCase()] : undefined;

  return (
    <div>
      <TraceRowPresentation depth={depth} hasChildren={hasChildren} open={open} onToggle={() => hasChildren && setOpen(o => !o)} callType={callType} isRoot={isRoot} from={<><AddressLink address={trace.from} chars={6} className="text-neutral-600 hover:text-neutral-800" reason={fromReason} /><AddressLabel reason={fromReason} /></>} to={trace.to && <><AddressLink address={trace.to} chars={6} reason={toReason} /><AddressLabel reason={toReason} /></>} selector={selector} value={hasValue(trace.value) ? `${formatWei(trace.value)} ${getNetworkCurrency()}` : undefined} gasUsed={trace.gasUsed == null ? undefined : formatGas(Number(trace.gasUsed))} error={trace.error} />

      {hasChildren && open && children.map((child) => (
        <TraceRow key={child.trace.traceAddress} node={child} />
      ))}
    </div>
  );
}

export function CallTraceTree({ tx, internalTxs }: { tx: Transaction; internalTxs: InternalTransaction[] }) {
  const [raw, setRaw] = useState(false);
  const root = useMemo(() => buildTree(tx, internalTxs), [tx, internalTxs]);

  return (
    <div className="card">
      <div className="flex items-center justify-between px-4 py-3 border-b border-neutral-100">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-neutral-700">Call Trace</span>
          <span className="text-xs text-neutral-400">({internalTxs.length} internal {internalTxs.length === 1 ? 'call' : 'calls'})</span>
        </div>
        <button
          onClick={() => setRaw((r) => !r)}
          className="text-xs text-neutral-500 hover:text-neutral-700 bg-neutral-100 hover:bg-neutral-200 px-2 py-1 rounded transition-colors"
        >
          {raw ? 'Tree view' : 'Raw JSON'}
        </button>
      </div>

      {raw ? (
        <div className="code-block p-3 text-xs max-h-[32rem] overflow-auto m-3">
          <pre className="whitespace-pre-wrap break-all">{JSON.stringify(internalTxs, null, 2)}</pre>
        </div>
      ) : (
        <div className="py-1">
          <TraceRow node={root} />
        </div>
      )}
    </div>
  );
}
