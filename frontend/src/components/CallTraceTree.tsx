import { useMemo, useState } from 'react';
import { ChevronDown, ChevronRight, AlertTriangle } from 'lucide-react';
import type { Transaction, InternalTransaction } from '../lib/api';
import { buildTree, type TraceNode } from '../lib/callTrace';
import { AddressLink } from './AddressLink';
import { AddressLabel } from './AddressLabel';
import { formatWei, formatGas, getNetworkCurrency } from '../lib/utils';

// A few common 4-byte selectors so the trace reads more naturally. Anything
// unknown falls back to showing the raw selector.
const KNOWN_SELECTORS: Record<string, string> = {
  '0xa9059cbb': 'transfer(address,uint256)',
  '0x23b872dd': 'transferFrom(address,address,uint256)',
  '0x095ea7b3': 'approve(address,uint256)',
  '0x40c10f19': 'mint(address,uint256)',
  '0x42842e0e': 'safeTransferFrom(address,address,uint256)',
  '0x2e1a7d4d': 'withdraw(uint256)',
  '0xd0e30db0': 'deposit()',
};

const CALL_TYPE_STYLES: Record<string, string> = {
  call: 'bg-blue-100 text-blue-700',
  staticcall: 'bg-neutral-100 text-neutral-500',
  delegatecall: 'bg-amber-100 text-amber-700',
  create: 'bg-green-100 text-green-700',
  create2: 'bg-green-100 text-green-700',
};

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
      <div
        className={`flex items-start gap-2 py-2 pr-3 text-sm border-l-2 ${
          trace.error ? 'border-error-400 bg-error-50/40' : 'border-transparent hover:bg-primary-50/40'
        } transition-colors`}
        style={{ paddingLeft: `${depth * 1.5 + 0.5}rem` }}
      >
        {/* expand / collapse toggle (only when there are nested calls) */}
        <button
          onClick={() => hasChildren && setOpen((o) => !o)}
          className={`mt-0.5 shrink-0 text-neutral-400 ${hasChildren ? 'hover:text-neutral-700' : 'invisible'}`}
          aria-label={open ? 'Collapse' : 'Expand'}
        >
          {open ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
        </button>

        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0">
          <span className={`text-xs px-1.5 py-0.5 rounded font-medium uppercase ${CALL_TYPE_STYLES[callType] || 'bg-neutral-100 text-neutral-500'}`}>
            {isRoot ? `${callType} (top)` : callType}
          </span>

          <AddressLink address={trace.from} chars={6} className="text-neutral-600 hover:text-neutral-800" reason={fromReason} />
          <AddressLabel reason={fromReason} />
          {trace.to && (
            <>
              <span className="text-neutral-400">→</span>
              <AddressLink address={trace.to} chars={6} reason={toReason} />
              <AddressLabel reason={toReason} />
            </>
          )}

          {selector && (
            <span
              className="text-xs font-mono text-neutral-500 bg-neutral-100 px-1.5 py-0.5 rounded"
              title={KNOWN_SELECTORS[selector] ? `${KNOWN_SELECTORS[selector]}  (${selector})` : selector}
            >
              {KNOWN_SELECTORS[selector] ? KNOWN_SELECTORS[selector].split('(')[0] : selector}
            </span>
          )}

          {hasValue(trace.value) && (
            <span className="text-xs font-mono text-success-600 font-medium">
              {formatWei(trace.value)} {getNetworkCurrency()}
            </span>
          )}

          {trace.gasUsed != null && (
            <span className="text-xs text-neutral-400">gas {formatGas(Number(trace.gasUsed))}</span>
          )}

          {trace.error && (
            <span className="inline-flex items-center gap-1 text-xs text-error-600 font-medium">
              <AlertTriangle className="w-3 h-3" />
              {trace.error}
            </span>
          )}
        </div>
      </div>

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
