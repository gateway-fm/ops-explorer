import { useState, type ReactNode } from 'react';
import type { ParamType } from 'ethers';
import type { EventDecodeInput, NativeEvent } from '../lib/nativeEvent';

// Pure presentation of already-authorized fields. The caller owns address
// navigation; this module never fetches metadata or infers permissions.
export type AddressRenderer = (address: string) => ReactNode;
function EventValue({value, type, renderAddress, depth = 0}: {value: unknown; type: ParamType; renderAddress: AddressRenderer; depth?: number}) {
  if (depth > 8 || typeof value === 'string' && value.length > 4096 || Array.isArray(value) && value.length > 64) return <span>Large value; see raw data</span>;
  if (type.baseType === 'array' && Array.isArray(value) && type.arrayChildren) return <span>[{value.map((item, i) => <span key={i}>{i > 0 && ', '}<EventValue value={item} type={type.arrayChildren!} renderAddress={renderAddress} depth={depth + 1}/></span>)}]</span>;
  if (type.baseType === 'tuple' && Array.isArray(value) && type.components) return <span>({type.components.map((part, i) => <span key={i}>{i > 0 && ', '}{part.name && `${part.name}: `}<EventValue value={value[i]} type={part} renderAddress={renderAddress} depth={depth + 1}/></span>)})</span>;
  if (type.type === 'address' && typeof value === 'string') return <>{renderAddress(value.toLowerCase())}</>;
  return <span className="font-mono break-all">{typeof value === 'bigint' ? value.toLocaleString('en-US') : typeof value === 'string' || typeof value === 'boolean' ? String(value) : 'Unavailable'}</span>;
}
export function DecodedEvent({event, renderAddress}: {event: NativeEvent; renderAddress: AddressRenderer}) {
  return <div className="bg-primary-50 border border-primary-200 rounded-lg p-3 space-y-2" data-testid="decoded-event">
    <div className="flex flex-wrap items-center gap-2"><span className="text-xs font-medium text-primary-700 bg-primary-100 px-2 py-0.5 rounded">Decoded</span><span className="text-xs text-neutral-500">{event.source}</span></div>
    <p className="font-mono text-sm text-primary-600 font-semibold break-all" title={event.signature}>{event.name}</p>
    <div className="space-y-2 pt-2 border-t border-primary-200">{event.params.map((param, i) => <div key={i} className="flex flex-wrap items-start gap-2 text-xs" data-testid="decoded-param">
      <span className="text-neutral-500 min-w-20">{param.name}:</span>
      {param.hidden ? <span className="text-neutral-400 italic">Hidden</span> : param.hash ? <span className="font-mono break-all">{String(param.value)} <span className="text-neutral-500">(indexed hash)</span></span> : <EventValue value={param.value} type={param.type} renderAddress={renderAddress}/>}
      <span className="text-neutral-400">{param.type.type}{param.indexed && ' · indexed'}</span>
    </div>)}</div>
  </div>;
}
function TopicRow({index, value, renderAddress}: {index: number; value: string | null; renderAddress: AddressRenderer}) {
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
    {value === null ? <span className="text-neutral-400 italic">Hidden</span> : format === 'address' ? address ? <>{renderAddress(address)}</> : <span>Not an address</span> : <span className="font-mono text-neutral-700 break-all">{text}</span>}
  </div>{index > 0 && value !== null && <select aria-label={`Topic ${index} format`} value={format} onChange={e => setFormat(e.target.value)} className="text-xs bg-neutral-100 border border-neutral-200 rounded px-1.5 py-0.5 text-neutral-600 shrink-0"><option value="hex">Hex</option><option value="address">Address</option><option value="number">Number</option><option value="text">Text</option></select>}</div>;
}
export function EventTopics({payload, event, renderAddress, showEmpty = false}: {payload: EventDecodeInput; event: NativeEvent | null; renderAddress: AddressRenderer; showEmpty?: boolean}) {
  const topics = [payload.topic0, payload.topic1, payload.topic2, payload.topic3];
  const last = Math.max(event?.indexedCount ?? 0, topics.reduce((found, value, i) => value === null ? found : i, -1));
  return <div className="flex flex-col gap-1"><span className="text-neutral-500 text-sm">Topics</span><div className="space-y-2 pl-2">{showEmpty && topics.every(topic => topic === null) ? <span className="text-xs text-neutral-500">No topics</span> : topics.slice(0, last + 1).map((value, i) => <TopicRow key={`${i}:${value}`} index={i} value={value} renderAddress={renderAddress}/>)}</div></div>;
}
