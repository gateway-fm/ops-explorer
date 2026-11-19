import { useMemo, useState } from 'react';
import type { ParamType } from 'ethers';
import { decodeContract } from '../lib/nativeReadModels';
import { decodeNativeEvent, type NativeEvent } from '../lib/nativeEvent';
import type { AddressMetadata, Log } from '../lib/v2ReadTypes';
import { useNativeMetadata } from '../hooks/useNativeMetadata';
import { NativeAddress } from './NativeAddress';
import { EmptyRow } from './AddressPresentation';
import { LogCardPresentation } from './TransactionDetailPresentation';

const zeroAddress = '0x' + '0'.repeat(40);
const contractPath = (address: string) => `/v2/explorer/addresses/${address}/contract`;
function EventValue({value, type, metadata, depth = 0}: {value: unknown; type: ParamType; metadata?: AddressMetadata; depth?: number}) {
  if (depth > 8 || typeof value === 'string' && value.length > 4096 || Array.isArray(value) && value.length > 64) return <span>Large value; see raw data</span>;
  if (type.baseType === 'array' && Array.isArray(value) && type.arrayChildren) return <span>[{value.map((item, i) => <span key={i}>{i > 0 && ', '}<EventValue value={item} type={type.arrayChildren!} metadata={metadata} depth={depth + 1}/></span>)}]</span>;
  if (type.baseType === 'tuple' && Array.isArray(value) && type.components) return <span>({type.components.map((part, i) => <span key={i}>{i > 0 && ', '}{part.name && `${part.name}: `}<EventValue value={value[i]} type={part} metadata={metadata} depth={depth + 1}/></span>)})</span>;
  if (type.type === 'address' && typeof value === 'string') return <NativeAddress address={value.toLowerCase()} metadata={metadata} full/>;
  return <span className="font-mono break-all">{typeof value === 'bigint' ? value.toLocaleString('en-US') : typeof value === 'string' || typeof value === 'boolean' ? String(value) : 'Unavailable'}</span>;
}
function DecodedEvent({event, metadata}: {event: NativeEvent; metadata?: AddressMetadata}) {
  return <div className="bg-primary-50 border border-primary-200 rounded-lg p-3 space-y-2" data-testid="decoded-event">
    <div className="flex flex-wrap items-center gap-2"><span className="text-xs font-medium text-primary-700 bg-primary-100 px-2 py-0.5 rounded">Decoded</span><span className="text-xs text-neutral-500">{event.source}</span></div>
    <p className="font-mono text-sm text-primary-600 font-semibold break-all" title={event.signature}>{event.name}</p>
    <div className="space-y-2 pt-2 border-t border-primary-200">{event.params.map((param, i) => <div key={i} className="flex flex-wrap items-start gap-2 text-xs" data-testid="decoded-param">
      <span className="text-neutral-500 min-w-20">{param.name}:</span>
      {param.hidden ? <span className="text-neutral-400 italic">Hidden</span> : param.hash ? <span className="font-mono break-all">{String(param.value)} <span className="text-neutral-500">(indexed hash)</span></span> : <EventValue value={param.value} type={param.type} metadata={metadata}/>}
      <span className="text-neutral-400">{param.type.type}{param.indexed && ' · indexed'}</span>
    </div>)}</div>
  </div>;
}
function TopicRow({index, value, metadata}: {index: number; value: string | null; metadata?: AddressMetadata}) {
  const [format, setFormat] = useState('hex');
  let text = value;
  if (value !== null && format === 'number') text = BigInt(value).toLocaleString('en-US');
  if (value !== null && format === 'text') {
    try {
      const bytes = Uint8Array.from(value.slice(2).match(/../g) ?? [], pair => parseInt(pair, 16));
      const decoded = new TextDecoder('utf-8', {fatal: true}).decode(bytes).replace(/\0+$/, '');
      text = /^[\x20-\x7e\s]+$/.test(decoded) ? decoded : '(non-printable)';
    } catch { text = '(non-printable)'; }
  }
  const address = value !== null && /^0x0{24}[0-9a-f]{40}$/.test(value) ? '0x' + value.slice(-40) : null;
  return <div className="flex items-start gap-2"><span className="text-neutral-400 text-xs font-mono w-4 shrink-0">[{index}]</span><div className="flex-1 min-w-0 text-xs">
    {value === null ? <span className="text-neutral-400 italic">Hidden</span> : format === 'address' ? address ? <NativeAddress address={address} metadata={metadata} full/> : <span>Not an address</span> : <span className="font-mono text-neutral-700 break-all">{text}</span>}
  </div>{index > 0 && value !== null && <select aria-label={`Topic ${index} format`} value={format} onChange={e => setFormat(e.target.value)} className="text-xs bg-neutral-100 border border-neutral-200 rounded px-1.5 py-0.5 text-neutral-600 shrink-0"><option value="hex">Hex</option><option value="address">Address</option><option value="number">Number</option><option value="text">Text</option></select>}</div>;
}
function NativeLogCard({log, abi, local}: {log: Log; abi?: unknown[]; local: NativeEvent | null}) {
  const event = useMemo(() => abi ? decodeNativeEvent(log, abi) : local, [log, abi, local]);
  const topics = [log.topic0, log.topic1, log.topic2, log.topic3];
  const last = Math.max(event?.indexedCount ?? 0, topics.reduce((found, value, i) => value === null ? found : i, -1));
  return <LogCardPresentation index={log.logIndex} address={<NativeAddress address={log.address} metadata={log.addressMetadata} full/>}
    decoded={event && <DecodedEvent event={event} metadata={log.addressMetadata}/>}
    topics={<div className="flex flex-col gap-1"><span className="text-neutral-500 text-sm">Topics</span><div className="space-y-2 pl-2">{topics.slice(0, last + 1).map((value, i) => <TopicRow key={`${i}:${value}`} index={i} value={value} metadata={log.addressMetadata}/>)}</div></div>}
    data={log.data}/>;
}
export function NativeLogCards({items}: {items: Log[]}) {
  const localEvents = useMemo(() => new Map(items.map(log => [log.id, decodeNativeEvent(log)])), [items]);
  // Common local formats need no extra API read. Only an unknown or incompatible
  // format asks for that visible emitter's currently authorized registry ABI.
  const contracts = useNativeMetadata(items.filter(log => !localEvents.get(log.id) && log.topic0 && log.address !== zeroAddress && !['no_access', 'participant_override'].includes(log.addressMetadata?.[log.address] ?? '')).map(log => contractPath(log.address)), decodeContract);
  if (!items.length) return <EmptyRow>No logs on this visible page.</EmptyRow>;
  return <div className="space-y-4">{items.map(log => {
    const contract = contracts[contractPath(log.address)];
    return <NativeLogCard key={log.id} log={log} local={localEvents.get(log.id) ?? null} abi={contract?.address === log.address ? contract.abi : undefined}/>;
  })}</div>;
}
