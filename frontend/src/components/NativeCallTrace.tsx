import { useMemo, useState, useSyncExternalStore } from 'react';
import { appSession } from '../lib/v2Session';
import { nativeTraceForest, type NativeTraceNode } from '../lib/nativeTrace';
import type { InternalTransaction } from '../lib/v2ReadTypes';
import { exactQuantity, exactUnits } from '../lib/nativePresentation';
import { getNetworkCurrency } from '../lib/utils';
import { NativeAddress } from './NativeAddress';
import { TraceRowPresentation } from './TraceRowPresentation';
import { EmptyRow } from './AddressPresentation';

function TraceNode({node, depth = 0}: {node: NativeTraceNode; depth?: number}) {
  const [open, setOpen] = useState(true);
  const trace = node.trace;
  return <div data-testid="native-trace-node" data-trace-path={trace.traceAddress}>
    {node.parentUnavailable && <p className="px-4 pt-2 text-xs text-neutral-400">Parent call is not on this visible page · path {trace.traceAddress || 'unavailable'}</p>}
    <TraceRowPresentation depth={Math.min(depth, 8)} hasChildren={node.children.length > 0} open={open} onToggle={() => setOpen(v => !v)}
      callType={trace.callType.toLowerCase()} isRoot={trace.traceAddress === ''}
      from={<NativeAddress address={trace.from} metadata={trace.addressMetadata}/>}
      to={trace.to ? <NativeAddress address={trace.to} metadata={trace.addressMetadata}/> : undefined}
      selector={trace.input && trace.input.length >= 10 ? trace.input.slice(0, 10) : undefined}
      value={`${exactUnits(trace.value)}${trace.value !== null ? ` ${getNetworkCurrency()}` : ''}`}
      gasUsed={trace.gasUsed === undefined ? undefined : exactQuantity(trace.gasUsed)} error={trace.error}/>
    {open && node.children.map(child => <TraceNode key={child.trace.id} node={child} depth={depth + 1}/>)}
  </div>;
}
export function NativeTraceRows({items}: {items: InternalTransaction[]}) {
  const identity = useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity;
  const [raw, setRaw] = useState(false);
  const forest = useMemo(() => nativeTraceForest(items), [items]);
  return <div className="card"><div className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 border-b border-neutral-100"><div><span className="text-sm font-medium text-neutral-700">Call Trace</span><p className="text-xs text-neutral-400">Visible page; missing or hidden parents are not reconstructed.</p></div><button className="text-xs text-neutral-500 hover:text-neutral-700 bg-neutral-100 hover:bg-neutral-200 px-2 py-1 rounded" onClick={() => setRaw(v => !v)}>{raw ? 'Tree view' : 'Raw JSON'}</button></div>
    {raw ? <div className="code-block p-3 text-xs max-h-[32rem] overflow-auto m-3"><pre className="whitespace-pre-wrap break-all">{JSON.stringify(items, null, 2)}</pre></div> : <div className="py-1">{items.length ? forest.map(node => <TraceNode key={`${identity?.activation}:${node.trace.id}`} node={node}/>) : <EmptyRow>No internal calls on this visible page.</EmptyRow>}</div>}
  </div>;
}
