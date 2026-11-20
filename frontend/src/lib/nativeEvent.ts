import { AbiCoder, EventFragment, Interface, ParamType, type InterfaceAbi } from 'ethers';
import { KNOWN_EVENTS } from './eventDecoder';
import type { Log } from './v2ReadTypes';

export type EventDecodeInput = Pick<Log, 'topic0' | 'topic1' | 'topic2' | 'topic3' | 'data'>;
export interface NativeEventParameter { name: string; type: ParamType; indexed: boolean; value?: unknown; hidden?: boolean; hash?: boolean }
export interface NativeEvent { name: string; signature: string; source: 'Approved ABI' | 'Known event format'; params: NativeEventParameter[]; indexedCount: number }
const coder = AbiCoder.defaultAbiCoder();
const transfer = '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef';
// Derive the key from the local declaration rather than trusting a separately
// maintained hash table. A matching hash still describes a format, not an
// attestation that an emitting contract implements the corresponding standard.
const localEvents = new Map(Object.values(KNOWN_EVENTS).map(event => {
  const fragment = EventFragment.from('event ' + event.signature);
  return [fragment.topicHash, fragment] as const;
}));

function localFragment(log: EventDecodeInput): EventFragment | null {
  const known = log.topic0 && localEvents.get(log.topic0);
  if (!known) return null;
  // ERC721 and ERC20 share Transfer/Approval hashes but differ in indexing.
  // Never treat the third indexed token ID as an ERC20 data amount.
  if (log.data === '0x' && log.topic3 !== null && (log.topic0 === transfer || known.name === 'Approval')) {
    return EventFragment.from(`event ${known.name}(address indexed ${known.name === 'Transfer' ? 'from' : 'owner'}, address indexed ${known.name === 'Transfer' ? 'to' : 'approved'}, uint256 indexed tokenId)`);
  }
  return known;
}

export function decodeNativeEvent(log: EventDecodeInput, abi?: unknown[]): NativeEvent | null {
  if (!log.topic0 || log.data.length > (1 << 20)) return null;
  try {
    let fragment: EventFragment | null = null, source: NativeEvent['source'] = 'Known event format';
    if (abi?.length && abi.length <= 4096) {
      fragment = new Interface(abi as InterfaceAbi).getEvent(log.topic0);
      if (fragment) source = 'Approved ABI';
    }
    fragment ??= localFragment(log);
    if (!fragment || fragment.anonymous || fragment.topicHash !== log.topic0 || fragment.inputs.length > 64) return null;
    const indexedCount = fragment.inputs.filter(p => p.indexed).length;
    if (indexedCount > 3) return null;
    const topics = [log.topic1, log.topic2, log.topic3];
    if (topics.slice(indexedCount).some(t => t !== null)) return null;
    const plain = fragment.inputs.filter(p => !p.indexed);
    const values = coder.decode(plain, log.data);
    // Conservative raw fallback for malformed/noncanonical ABI layouts.
    if (coder.encode(plain, values).toLowerCase() !== log.data) return null;
    let topicIndex = 0, dataIndex = 0;
    const params = fragment.inputs.map((type, index): NativeEventParameter => {
      const base = {name: type.name || `arg${index}`, type, indexed: !!type.indexed};
      if (!type.indexed) return {...base, value: values[dataIndex++]};
      const topic = topics[topicIndex++];
      // Keep original positions. Filtering missing topics would move a later
      // participant into a hidden participant's field.
      if (topic === null) return {...base, hidden: true};
      const hash = type.baseType === 'array' || type.baseType === 'tuple' || type.type === 'string' || type.type === 'bytes';
      if (hash) return {...base, hash, value: topic};
      const decoded = coder.decode([type], topic);
      if (coder.encode([type], decoded).toLowerCase() !== topic.toLowerCase()) throw Error('Noncanonical indexed event value');
      return {...base, hash, value: decoded[0]};
    });
    return {name: fragment.name, signature: fragment.format('full'), source, params, indexedCount};
  } catch { return null; }
}
