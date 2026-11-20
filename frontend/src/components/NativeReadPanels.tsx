import { NativeNames } from './NativeNames';
import { useEffect, useState, useSyncExternalStore } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { advancePosition, firstPosition, navigationOwner, restorePosition, savePosition, type NativePosition } from '../lib/nativeNavigation';
import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';
import { appSession } from '../lib/v2Session';
import { useNativeRead } from '../hooks/useNativeRead';
import { readTarget, continuationMessage } from '../lib/v2Reads';
import type { Page, PageInfo, Transaction } from '../lib/v2ReadTypes';
import { exactQuantity, exactUnits, exactTimestamp } from '../lib/nativePresentation';
import { getNetworkCurrency, formatHash } from '../lib/utils';
import { NativeAddress } from './NativeAddress';
import { AddressTransactionsTable } from './AddressPresentation';
import { StateMessage } from './StateMessage';
import { Loader2 } from 'lucide-react';

export function useNativePage<T>(path: string | null, decode: (raw: unknown) => Page<T>, limit = 25, filters = '', enabled = true) {
  const identity = useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity;
  const location = useLocation(), navigate = useNavigate();
  const owner = navigationOwner(identity, path, limit, filters);
  const pointers = (location.state as {nativePages?: Record<string, unknown>} | null)?.nativePages;
  const [position, setPosition] = useState<{key: string; value: NativePosition}>();
  const restored = owner ? restorePosition(owner, pointers?.[owner]) : undefined;
  const candidate = owner && (position?.key === location.key && position.value.owner === owner ? position.value : restored);
  const current = candidate && candidate.expires > Date.now() ? candidate : owner && identity ? firstPosition(owner, identity.expires) : undefined;
  const persist = (next: NativePosition) => {
    savePosition(next); setPosition({key: location.key, value: next});
    const state = location.state && typeof location.state === 'object' ? location.state : {};
    const entries = [...Object.entries(state.nativePages ?? {}).filter(([key,value]) => key !== next.owner && restorePosition(key, value)), [next.owner, next] as const].slice(-12);
    while (entries.length > 1 && JSON.stringify(entries).length > 64 * 1024) entries.shift();
    void navigate({pathname: location.pathname, search: location.search, hash: location.hash}, {replace: true, state: {...state, nativePages: Object.fromEntries(entries)}});
  };
  useEffect(() => {
    if (!enabled || !current) return;
    savePosition(current); setPosition({key: location.key, value: current});
    if (owner && (pointers?.[owner] === undefined || !restored)) persist(current);
  }, [owner, enabled, location.key, current?.id]);
  const query = new URLSearchParams(filters); query.sort(); query.set('limit', String(limit));
  const endpoint = path && enabled && current ? readTarget(path, query, current.cursors[current.index]) : null;
  const read = useNativeRead(endpoint, decode);
  return {...read, endpoint, index: current ? current.offset + current.index : 0, canPrevious: !!current?.index,
    trimmed: !!current?.offset,
    next: () => { if (current && read.data?.page.next) persist(advancePosition(current, read.data.page.next)); },
    previous: () => { if (current && current.index > 0) persist({...current, index: current.index - 1}); },
    reset: () => {if (owner && identity) persist(firstPosition(owner, identity.expires)); read.refresh?.();},
  };
}
export function NativeReadState({loading, error, stale, reset, refresh, loadingView, errorView}: {loading?: boolean; error?: string; stale?: boolean; reset?: () => void; refresh?: () => void; loadingView?: ReactNode; errorView?: ReactNode}) {
  if (error) return <div role="alert">{errorView ?? <StateMessage variant="error" title={error} action={stale && reset ? <button onClick={reset} className="btn-secondary">Restart from the first page</button> : refresh ? <button onClick={refresh} className="btn-secondary">Try again</button> : undefined}/>}</div>;
  return loading ? loadingView ?? <StateMessage variant="loading" title="Loading…" icon={<Loader2 className="h-4 w-4 animate-spin"/>}/> : null;
}
export function NativeContinuation({page, index, next, previous, canPrevious, trimmed, reset}: {page?: PageInfo; index: number; next: () => void; previous: () => void; canPrevious?: boolean; trimmed?: boolean; reset?: () => void}) {
  if (!page) return null;
  return <div className="border-t border-neutral-200 px-4 py-3 flex flex-wrap items-center justify-between gap-3 text-sm"><span className="text-xs text-neutral-500">Page {index + 1}. {continuationMessage(page)}{trimmed && ' Earlier navigation coordinates were released.'}</span><div className="flex flex-wrap items-center gap-3">{trimmed && reset && <button onClick={reset} className="text-primary hover:text-primary-600">Restart from the first page</button>}<div className="inline-flex items-center overflow-hidden rounded-full border border-neutral-200 bg-neutral-50"><button disabled={canPrevious === undefined ? !index : !canPrevious} onClick={previous} className="px-4 py-1.5 font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300">Previous</button><span className="h-5 w-px bg-neutral-200" aria-hidden/><button data-testid="load-more" disabled={!page.next} onClick={next} className="px-4 py-1.5 font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300">Continue</button></div></div></div>;
}
export function nativeFee(tx: Transaction): string {
  if (tx.gasUsed === undefined) return 'Unavailable';
  const wei = (BigInt(tx.gasUsed) * BigInt(tx.gasPrice)).toString();
  return wei.length <= 78 ? `${exactUnits(wei)} ${getNetworkCurrency()}` : `${BigInt(wei).toLocaleString('en-US')} wei`;
}
export function NativeTransactionsTable({items, footer, currentAddress}: {items: Transaction[]; footer?: ReactNode; currentAddress?: string}) {
  return <NativeNames items={items.flatMap(tx => [{address:tx.from,metadata:tx.addressMetadata}, ...(tx.to ? [{address:tx.to,metadata:tx.addressMetadata}] : [])])}><AddressTransactionsTable footer={footer}>{items.map(tx => <tr key={tx.hash} data-testid="tx-row" className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10"><td className="px-4 py-4 align-middle"><Link to={`/tx/${tx.hash}`} title={tx.hash} className="font-mono text-sm text-primary transition-colors hover:text-primary-600">{formatHash(tx.hash,8)}</Link>{tx.blockTimestamp && <div className="mt-0.5 text-xs text-neutral-500">{exactTimestamp(tx.blockTimestamp)}</div>}{tx.txCategories?.map(category=><span key={category} className="mr-1 inline-block text-xs text-neutral-500">{category.replaceAll('_',' ')}</span>)}<div className="text-xs text-neutral-500">{tx.status === 1 ? 'Success' : tx.status === 0 ? 'Failed' : 'Pending / unknown'}</div></td><td className="hidden px-4 py-4 align-middle text-sm tabular-nums md:table-cell">{tx.blockNumber === null ? 'Pending' : <Link className="text-primary" to={`/block/${tx.blockNumber}`}>{exactQuantity(tx.blockNumber)}</Link>}</td><td className="px-4 py-4 align-middle"><div className="flex flex-wrap items-center gap-x-2 gap-y-1"><NativeAddress address={tx.from} metadata={tx.addressMetadata}/><span className="text-neutral-400">{currentAddress ? tx.from === currentAddress ? 'OUT →' : 'IN →' : '→'}</span>{tx.to ? <NativeAddress address={tx.to} metadata={tx.addressMetadata}/> : <span className="text-xs italic text-primary">Contract creation</span>}</div></td><td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">{exactUnits(tx.value)}{tx.value !== null && ` ${getNetworkCurrency()}`}</td><td className="hidden px-4 py-4 text-right align-middle font-mono text-xs tabular-nums text-neutral-500 lg:table-cell">{nativeFee(tx)}</td></tr>)}</AddressTransactionsTable></NativeNames>;
}
export { NativeLogCards } from './NativeLogCards';
export { NativeTraceRows } from './NativeCallTrace';
export { NativeTransferRows } from './NativeTransferRows';
