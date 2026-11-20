import { useMemo } from 'react';
import { decodeNativeEvent, type EventDecodeInput } from '../lib/nativeEvent';
import { displayDisclosureTime, type DataEvent, type DisclosureLevel } from '../lib/v2Disclosure';
import { AddressLabel } from './AddressLabel';
import { DecodedEvent, EventTopics } from './LogPayloadPresentation';
import { LogCardPresentation } from './TransactionDetailPresentation';

// Case access does not grant ordinary address or contract access. Keep addresses
// inert, including those interpreted from the already-disclosed payload.
const inertAddress = (address: string) => <span className="font-mono break-all">{address}</span>;
const levels: Record<DisclosureLevel, string> = { full: 'Full', pseudonymous: 'Pseudonymous', redacted: 'Redacted' };

function DisclosureEventCard({event, level}: {event: DataEvent; level: DisclosureLevel}) {
  const payload = useMemo<EventDecodeInput | undefined>(() => {
    if (event.payload !== 'disclosed' || !event.topics || event.data === undefined) return undefined;
    return {topic0: event.topics[0] ?? null, topic1: event.topics[1] ?? null, topic2: event.topics[2] ?? null, topic3: event.topics[3] ?? null, data: event.data};
  }, [event]);
  // Local format interpretation only: never fetch an ordinary registry ABI or
  // claim that a signature match verifies the emitting contract's behavior.
  const decoded = useMemo(() => payload ? decodeNativeEvent(payload) : null, [payload]);
  return <article aria-label={`Disclosed event ${event.logIndex}`} data-testid="disclosure-event-card" data-event-id={event.eventId}><LogCardPresentation
    index={event.logIndex}
    address={inertAddress(event.emitter)}
    provenance={<><AddressLabel reason="disclosure_grant"/><span className="badge badge-neutral">{levels[level]}</span></>}
    context={<div className="space-y-2 border-b border-neutral-100 pb-3 text-xs text-neutral-500">
      <dl className="flex flex-wrap gap-x-5 gap-y-2"><div><dt>Block</dt><dd className="font-mono">{event.blockNumber}</dd></div><div><dt>Transaction index</dt><dd className="font-mono">{event.transactionIndex}</dd></div><div><dt>Time</dt><dd>{displayDisclosureTime(event.blockTime)}</dd></div></dl>
      {payload && <details><summary className="cursor-pointer text-primary">Transaction details</summary><dl className="mt-2 space-y-2"><div><dt>Transaction hash</dt><dd className="break-all font-mono">{event.transactionHash}</dd></div><div><dt>Block hash</dt><dd className="break-all font-mono">{event.blockHash}</dd></div></dl></details>}
    </div>}
    decoded={decoded && <DecodedEvent event={decoded} renderAddress={inertAddress}/>}
    topics={payload ? <EventTopics payload={payload} event={decoded} renderAddress={inertAddress} showEmpty/> : <p className="text-sm text-neutral-500">Payload withheld</p>}
    data={payload?.data ?? ''}
    showEmptyData
  /></article>;
}

export function DisclosureEventCards({rows, level}: {rows: DataEvent[]; level: DisclosureLevel}) {
  return <div className="space-y-4">{rows.map(event => <DisclosureEventCard key={event.eventId} event={event} level={level}/>)}</div>;
}
