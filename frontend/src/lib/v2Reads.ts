import type { Page, PageInfo } from './v2ReadTypes';

export type ReadItem = Record<string, unknown>;
const object = (v: unknown): v is ReadItem => !!v && typeof v === 'object' && !Array.isArray(v);
const quantity = (v: unknown): v is string => typeof v === 'string' && /^(0|[1-9][0-9]{0,77})$/.test(v);
const quantities = new Set(['number', 'blockNumber', 'blockTimestamp', 'timestamp', 'gasLimit', 'baseFeePerGas', 'transactionCount', 'difficulty', 'totalDifficulty', 'value', 'gasUsed', 'gasPrice', 'maxFeePerGas', 'maxPriorityFeePerGas', 'nonce', 'maxFeePerBlobGas', 'tokenTransferCount', 'tokenId', 'gas', 'totalSupply', 'holderCount', 'transferCount', 'balance', 'txCount', 'internalTxCount', 'firstSeen', 'lastSeen', 'totalBlocks', 'totalTransactions', 'totalAddresses', 'totalTokens', 'count', 'lastIndexedBlock', 'minFetchedBlock', 'maxFetchedBlock', 'behindBlocks', 'chainId']);
const numbers = new Set(['txIndex', 'logIndex', 'txType', 'status', 'decimals', 'optimizationRuns']);
function checkedValue(value: unknown, key = '', parent?: ReadItem): void {
  if (value === null) return; // actual nullable source fields stay null
  if (key === 'nonce' && parent?.parentHash) { if (typeof value !== 'string' || !/^0x[0-9a-f]{16}$/.test(value)) throw Error('Invalid block nonce'); return; }
  if (key === 'value' && typeof parent?.type === 'string' && typeof parent?.label === 'string') { if (typeof value !== 'string') throw Error('Invalid suggestion'); return; }
  if (quantities.has(key)) { if (!quantity(value)) throw Error('Invalid lossless quantity'); return; }
  if (typeof value === 'number') { if (!numbers.has(key) || !Number.isSafeInteger(value) || value < 0 || value > 0xffffffff) throw Error('Invalid numeric read field'); return; }
  if (Array.isArray(value)) { value.forEach(v => checkedValue(v)); return; }
  if (object(value)) { Object.entries(value).forEach(([k, v]) => checkedValue(v, k, value)); return; }
  if (typeof value !== 'string' && typeof value !== 'boolean') throw Error('Invalid read value');
}
export function readItem(value: unknown): ReadItem {
  if (!object(value)) throw Error('Invalid read record');
  checkedValue(value);
  return value;
}
export function readPage(value: unknown): Page<ReadItem> {
  if (!object(value) || Object.keys(value).sort().join() !== 'items,page' || !Array.isArray(value.items) || value.items.length > 1000 || !object(value.page)) throw Error('Invalid read page');
  const p = value.page;
  if (Object.keys(p).some(k => !['next', 'hasMore', 'scanLimited', 'total', 'population'].includes(k)) || p.population !== 'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS' || p.hasMore !== true && p.hasMore !== false && p.hasMore !== null || typeof p.scanLimited !== 'boolean' || p.next !== undefined && (typeof p.next !== 'string' || p.next.length < 1 || p.next.length > 8192) || p.total !== undefined && !quantity(p.total) || p.hasMore === true && p.next === undefined || p.hasMore === false && p.next !== undefined || p.hasMore === null && (!p.scanLimited || p.next === undefined)) throw Error('Invalid read continuation');
  return { items: value.items.map(readItem), page: p as unknown as PageInfo };
}
export function readTarget(path: string, filters: URLSearchParams, cursor?: string): string {
  const query = new URLSearchParams(filters); query.delete('cursor');
  if (cursor) query.set('cursor', cursor);
  return '/v2/explorer' + path + (query.size ? '?' + query.toString() : '');
}
export function continuationMessage(page: PageInfo): string {
  if (page.hasMore === false) return 'End of the available records.';
  if (page.scanLimited) return 'This page reached the read limit. Continue to look for more records.';
  return 'More records are available.';
}
