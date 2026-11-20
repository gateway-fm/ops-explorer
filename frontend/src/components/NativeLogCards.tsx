import { useMemo } from 'react';
import { decodeContract } from '../lib/nativeReadModels';
import { decodeNativeEvent, type NativeEvent } from '../lib/nativeEvent';
import type { Log } from '../lib/v2ReadTypes';
import { useNativeMetadata } from '../hooks/useNativeMetadata';
import { NativeAddress } from './NativeAddress';
import { EmptyRow } from './AddressPresentation';
import { LogCardPresentation } from './TransactionDetailPresentation';
import { DecodedEvent, EventTopics } from './LogPayloadPresentation';

const zeroAddress = '0x' + '0'.repeat(40);
const contractPath = (address: string) => `/v2/explorer/addresses/${address}/contract`;
function NativeLogCard({log, abi, local}: {log: Log; abi?: unknown[]; local: NativeEvent | null}) {
  const event = useMemo(() => abi ? decodeNativeEvent(log, abi) : local, [log, abi, local]);
  const renderAddress = (address: string) => <NativeAddress address={address} metadata={log.addressMetadata} full/>;
  return <LogCardPresentation index={log.logIndex} address={<NativeAddress address={log.address} metadata={log.addressMetadata} full/>}
    decoded={event && <DecodedEvent event={event} renderAddress={renderAddress}/>}
    topics={<EventTopics payload={log} event={event} renderAddress={renderAddress}/>}
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
