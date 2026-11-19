import { describe, expect, it } from 'vitest';
import { Interface } from 'ethers';
import { decodeNativeEvent } from './nativeEvent';
import type { Log } from './v2ReadTypes';
const from = '0x' + 'a'.repeat(40), to = '0x' + 'b'.repeat(40);
function log(abi: string, args: unknown[]): Log {
  const iface = new Interface([abi]), event = iface.fragments[0];
  const encoded = iface.encodeEventLog(event, args);
  return {id: 'row', txHash: '0x' + 'c'.repeat(64), logIndex: 1, address: from, topic0: encoded.topics[0], topic1: encoded.topics[1] ?? null, topic2: encoded.topics[2] ?? null, topic3: encoded.topics[3] ?? null, data: encoded.data, blockNumber: '1', removed: false};
}
describe('native event decoding of authorized wire fields', () => {
  it('decodes a local Transfer without rounding and keeps a missing indexed position hidden', () => {
    const row = log('event Transfer(address indexed from,address indexed to,uint256 value)', [from, to, '900719925474099312345']);
    row.topic1 = null;
    const event = decodeNativeEvent(row)!;
    expect(event.name).toBe('Transfer'); expect(event.source).toBe('Known event format');
    expect(event.params[0]).toMatchObject({name: 'from', hidden: true});
    expect(String(event.params[1].value).toLowerCase()).toBe(to);
    expect(event.params[2].value).toBe(900719925474099312345n);
  });
  it('distinguishes an ERC721 indexed token ID from an ERC20 amount', () => {
    const row = log('event Transfer(address indexed from,address indexed to,uint256 indexed tokenId)', [from, to, '9007199254740993']);
    const event = decodeNativeEvent(row)!;
    expect(event.params[2]).toMatchObject({name: 'tokenId', indexed: true, value: 9007199254740993n});
  });
  it('decodes ERC1155 arrays as values instead of displaying their ABI offsets', () => {
    const row = log('event TransferBatch(address indexed operator,address indexed from,address indexed to,uint256[] ids,uint256[] values)', [from, from, to, [1n, 9007199254740993n], [2n, 3n]]);
    const event = decodeNativeEvent(row)!;
    expect(Array.from(event.params[3].value as bigint[])).toEqual([1n, 9007199254740993n]);
    expect(Array.from(event.params[4].value as bigint[])).toEqual([2n, 3n]);
  });
  it.each([
    ['event Approval(address indexed owner,address indexed spender,uint256 value)', [from, to, 8n], 'Approval'],
    ['event OwnershipTransferred(address indexed previousOwner,address indexed newOwner)', [from, to], 'OwnershipTransferred'],
    ['event ApprovalForAll(address indexed owner,address indexed operator,bool approved)', [from, to, false], 'ApprovalForAll'],
  ])('derives local signature hashes for %s', (abi, args, name) => {
    expect(decodeNativeEvent(log(abi as string, args as unknown[]))?.name).toBe(name);
  });
  it('uses an approved custom event ABI and retains indexed dynamic hashes', () => {
    const abi = 'event BankRecord(bytes32 indexed reference,address owner,int256 delta)';
    const row = log(abi, ['0x' + '1'.repeat(64), to, -12n]);
    expect(decodeNativeEvent(row)).toBeNull();
    const event = decodeNativeEvent(row, [abi])!;
    expect(event).toMatchObject({name: 'BankRecord', source: 'Approved ABI'});
    expect(event.params[2].value).toBe(-12n);
    const dynamic = log('event Named(string indexed label,uint256 amount)', ['secret', 5n]);
    expect(decodeNativeEvent(dynamic, ['event Named(string indexed label,uint256 amount)'])?.params[0]).toMatchObject({hash: true, value: dynamic.topic1});
  });
  it('falls back to raw for malformed data, unrecognized topics and an incompatible ABI layout', () => {
    const row = log('event Transfer(address indexed from,address indexed to,uint256 value)', [from, to, 7n]);
    expect(decodeNativeEvent({...row, data: '0x01'})).toBeNull();
    expect(decodeNativeEvent({...row, data: '0x'})).toBeNull();
    expect(decodeNativeEvent({...row, data: row.data + '00'.repeat(32)})).toBeNull();
    expect(decodeNativeEvent({...row, topic0: null})).toBeNull();
    expect(decodeNativeEvent(row, ['event Transfer(address from,address to,uint256 value)'])).toBeNull();
  });
  it.each([
    ['bool', true, '2'],
    ['uint8', 0n, '100'],
    ['int8', -128n, '80'],
  ])('keeps noncanonical indexed %s words raw instead of normalizing their meaning', (type, canonical, malformed) => {
    const abi = `event Checked(${type} indexed value)`;
    const row = log(abi, [canonical]);
    expect(decodeNativeEvent(row, [abi])?.params[0].value).toBe(canonical);
    expect(decodeNativeEvent({...row, topic1: '0x' + String(malformed).padStart(64, '0')}, [abi])).toBeNull();
  });
});
