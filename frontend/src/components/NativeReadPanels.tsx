import { TransferMethodBadge } from './TransferMethodBadge';
import { useState, useSyncExternalStore } from 'react';
import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';
import { appSession } from '../lib/v2Session';
import { useNativeRead } from '../hooks/useNativeRead';
import { readTarget, continuationMessage } from '../lib/v2Reads';
import type { Page, PageInfo, Transaction, Log, InternalTransaction, TokenTransfer, Token } from '../lib/v2ReadTypes';
import { exactQuantity, exactUnits, exactTimestamp } from '../lib/nativePresentation';
import { getNetworkCurrency, formatHash } from '../lib/utils';
import { NativeAddress } from './NativeAddress';
import { AddressTransactionsTable, EmptyRow } from './AddressPresentation';
import { LogCardPresentation } from './TransactionDetailPresentation';
import { TraceRowPresentation } from './TraceRowPresentation';

export function useNativePage<T>(path: string | null, decode: (raw: unknown) => Page<T>, limit = 25, filters = '', enabled = true) {
  const activation = useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity?.activation;
  const owner = `${activation ?? ''}:${path ?? ''}:${limit}:${filters}`;
  const [position, setPosition] = useState({ owner, cursors: [''], index: 0 });
  const current = position.owner === owner ? position : { owner, cursors: [''], index: 0 };
  const query = new URLSearchParams(filters); query.set('limit', String(limit));
  const endpoint = path && enabled ? readTarget(path, query, current.cursors[current.index]) : null;
  const read = useNativeRead(endpoint, decode);
  return {...read, index: current.index,
    next: () => { if (read.data?.page.next) setPosition({owner, cursors: [...current.cursors.slice(0, current.index + 1), read.data.page.next], index: current.index + 1}); },
    previous: () => setPosition({...current, index: Math.max(0, current.index - 1)}),
    reset: () => {setPosition({owner, cursors: [''], index: 0}); read.refresh?.();},
  };
}
export function NativeReadState({loading, error, stale, reset, refresh}: {loading?: boolean; error?: string; stale?: boolean; reset?: () => void; refresh?: () => void}) {
  if (error) return <div role="alert" className="rounded-xl border border-error-200 bg-error-50 p-4 text-sm text-error-700">{error}{stale && reset && <button onClick={reset} className="ml-3 underline">Restart from the first page</button>}{!stale && refresh && <button onClick={refresh} className="ml-3 underline">Try again</button>}</div>;
  return loading ? <p role="status" className="p-6 text-sm text-neutral-500">Loading…</p> : null;
}
export function NativeContinuation({page, index, next, previous}: {page?: PageInfo; index: number; next: () => void; previous: () => void}) {
  if (!page) return null;
  return <div className="border-t border-neutral-200 px-4 py-3 flex flex-wrap items-center justify-between gap-3 text-sm"><span className="text-xs text-neutral-500">Page {index + 1}. {continuationMessage(page)}</span><div className="flex gap-4"><button disabled={!index} onClick={previous} className="text-primary disabled:text-neutral-400">Previous</button><button data-testid="load-more" disabled={!page.next} onClick={next} className="text-primary disabled:text-neutral-400">Continue</button></div></div>;
}
export function nativeFee(tx: Transaction): string {
  if (tx.gasUsed === undefined) return 'Unavailable';
  const wei = (BigInt(tx.gasUsed) * BigInt(tx.gasPrice)).toString();
  return wei.length <= 78 ? `${exactUnits(wei)} ${getNetworkCurrency()}` : `${BigInt(wei).toLocaleString('en-US')} wei`;
}
export function NativeTransactionsTable({items, footer, currentAddress}: {items: Transaction[]; footer?: ReactNode; currentAddress?: string}) {
  return <AddressTransactionsTable footer={footer}>{items.map(tx => <tr key={tx.hash} data-testid="tx-row" className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10"><td className="px-4 py-4 align-middle"><Link to={`/tx/${tx.hash}`} title={tx.hash} className="font-mono text-sm text-primary transition-colors hover:text-primary-600">{formatHash(tx.hash,8)}</Link>{tx.blockTimestamp && <div className="mt-0.5 text-xs text-neutral-500">{exactTimestamp(tx.blockTimestamp)}</div>}{tx.txCategories?.map(category=><span key={category} className="mr-1 inline-block text-xs text-neutral-500">{category.replaceAll('_',' ')}</span>)}<div className="text-xs text-neutral-500">{tx.status === 1 ? 'Success' : tx.status === 0 ? 'Failed' : 'Pending / unknown'}</div></td><td className="hidden px-4 py-4 align-middle text-sm tabular-nums md:table-cell">{tx.blockNumber === null ? 'Pending' : <Link className="text-primary" to={`/block/${tx.blockNumber}`}>{exactQuantity(tx.blockNumber)}</Link>}</td><td className="px-4 py-4 align-middle"><div className="flex flex-wrap items-center gap-x-2 gap-y-1"><NativeAddress address={tx.from} metadata={tx.addressMetadata}/><span className="text-neutral-400">{currentAddress ? tx.from === currentAddress ? 'OUT →' : 'IN →' : '→'}</span>{tx.to ? <NativeAddress address={tx.to} metadata={tx.addressMetadata}/> : <span className="text-xs italic text-primary">Contract creation</span>}</div></td><td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">{exactUnits(tx.value)}{tx.value !== null && ` ${getNetworkCurrency()}`}</td><td className="hidden px-4 py-4 text-right align-middle font-mono text-xs tabular-nums text-neutral-500 lg:table-cell">{nativeFee(tx)}</td></tr>)}</AddressTransactionsTable>;
}
export function NativeLogCards({items}: {items: Log[]}) {
  if (!items.length) return <EmptyRow>No logs on this visible page.</EmptyRow>;
  return <div className="space-y-4">{items.map(log => <LogCardPresentation key={log.id} index={log.logIndex} address={<NativeAddress address={log.address} metadata={log.addressMetadata} full/>} data={log.data} topics={<div className="flex flex-col gap-1"><span className="text-neutral-500 text-sm">Topics</span><div className="space-y-2 pl-2">{[log.topic0,log.topic1,log.topic2,log.topic3].map((topic,index) => topic && <div key={index} className="flex items-start gap-2"><span className="text-neutral-400 text-xs font-mono w-4 shrink-0">[{index}]</span><span className="font-mono text-xs text-neutral-700 break-all">{topic}</span></div>)}</div></div>}/>)}</div>;
}
export function NativeTraceRows({items}: {items: InternalTransaction[]}) {
  return <div className="card"><div className="flex items-center justify-between px-4 py-3 border-b border-neutral-100"><span className="text-sm font-medium text-neutral-700">Internal calls</span><span className="text-xs text-neutral-400">Visible page; parent calls may be on another page.</span></div><div className="py-1">{items.length ? items.map(trace => <TraceRowPresentation key={trace.id} depth={0} callType={trace.callType.toLowerCase()} from={<NativeAddress address={trace.from} metadata={trace.addressMetadata}/>} to={trace.to ? <NativeAddress address={trace.to} metadata={trace.addressMetadata}/> : undefined} selector={trace.input && trace.input.length >= 10 ? trace.input.slice(0,10) : undefined} value={`${exactUnits(trace.value)}${trace.value !== null ? ` ${getNetworkCurrency()}` : ''}`} gasUsed={trace.gasUsed === undefined ? undefined : exactQuantity(trace.gasUsed)} error={trace.error}/>) : <EmptyRow>No internal calls on this visible page.</EmptyRow>}</div></div>;
}
export function NativeTransferRows({items,token}: {items: TokenTransfer[];token?:Token}) {
  return <div className="space-y-3">{items.length ? items.map(transfer => <div key={transfer.id} data-testid="transfer-row" className="flex items-center gap-1.5 flex-wrap text-sm"><TransferMethodBadge method={transfer.transferType||'transfer'}/><span className="text-neutral-500">From</span><NativeAddress address={transfer.from} metadata={transfer.addressMetadata}/><span className="text-neutral-500">To</span><NativeAddress address={transfer.to} metadata={transfer.addressMetadata}/><span className="text-neutral-500">For</span><span className="font-mono text-success-600 font-medium">{token?.address===transfer.tokenAddress?exactUnits(transfer.value,token.decimals):exactQuantity(transfer.value)}{transfer.value !== null && (token?.address===transfer.tokenAddress?` ${token.symbol}`:' base units')}</span><NativeAddress address={transfer.tokenAddress} metadata={transfer.addressMetadata} token/><Link title={transfer.txHash} className="font-mono text-primary" to={`/tx/${transfer.txHash}`}>{formatHash(transfer.txHash,6)}</Link></div>) : <EmptyRow>No token transfers on this visible page.</EmptyRow>}</div>;
}
