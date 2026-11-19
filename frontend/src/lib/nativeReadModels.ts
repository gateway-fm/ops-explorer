import { readItem, readPage, readObservedStats, searchSuggestionTarget, type ReadItem } from './v2Reads';
import type { AddressStats, Balance, Block, Code, Contract, InternalTransaction, Log, NativeBalance, Page, Token, TokenHolder, TokenTransfer, Transaction, SyncStatus, IndexerProgress, Catchup } from './v2ReadTypes';

const address = /^0x[0-9a-f]{40}$/;
const hash = /^0x[0-9a-f]{64}$/;
const hex = /^0x(?:[0-9a-f]{2})*$/;
const quantity = /^(0|[1-9][0-9]{0,77})$/;
const check = (condition: boolean): void => { if (!condition) throw Error('Invalid native read response'); };
const text = (item: ReadItem, key: string, pattern?: RegExp) => typeof item[key] === 'string' && (!pattern || pattern.test(item[key] as string));
const closed = (item:ReadItem, fields:string[]) => check(Object.keys(item).every(key=>fields.includes(key)));
const optionalText = (item:ReadItem, keys:string[], pattern?:RegExp) => keys.forEach(key=>check(item[key]===undefined||text(item,key,pattern)));
const optionalBool = (item:ReadItem, keys:string[]) => keys.forEach(key=>check(item[key]===undefined||typeof item[key]==='boolean'));
const decimal=/^(0|[1-9][0-9]*)(?:\.[0-9]+)?$/;

function block(raw: unknown): Block {
  const item = readItem(raw);
  closed(item,['source','number','hash','parentHash','timestamp','gasLimit','baseFeePerGas','transactionCount','difficulty','totalDifficulty','nonce','miner','extraData','stateRoot','transactionsRoot','receiptsRoot','createdAt']);
  check(item.source === undefined || item.source === 'INDEXER' || item.source === 'NODE');
  check(text(item, 'number', quantity) && text(item, 'hash', hash) && text(item, 'timestamp', quantity) && text(item, 'gasLimit', quantity));
  check(BigInt(item.number as string)<=9223372036854775807n && text(item,'parentHash',hash) && text(item,'miner',address) && text(item,'nonce',/^0x[0-9a-f]{16}$/) && text(item,'extraData',hex) && text(item,'difficulty',quantity) && text(item,'createdAt'));
  for(const key of ['stateRoot','transactionsRoot','receiptsRoot'])check(text(item,key,hash));
  optionalText(item,['baseFeePerGas','transactionCount','totalDifficulty'],quantity);
  return item as unknown as Block;
}
function transaction(raw: unknown): Transaction {
  const item = readItem(raw);
  check(item.source === undefined || item.source === 'INDEXER' || item.source === 'NODE');
  check(text(item, 'hash', hash) && text(item, 'from', address) && (item.to === null || text(item, 'to', address)) && (item.blockNumber === null || text(item, 'blockNumber', quantity)) && (item.value === null || text(item, 'value', quantity)) && text(item, 'gasPrice', quantity) && text(item, 'inputData', hex) && (item.status === null || item.status === 0 || item.status === 1));
  check(item.txCategories === undefined || Array.isArray(item.txCategories) && item.txCategories.every(value => typeof value === 'string'));
  return item as unknown as Transaction;
}
function transfer(raw: unknown): TokenTransfer {
  const item = readItem(raw);
  check(text(item, 'id') && text(item, 'txHash', hash) && text(item, 'tokenAddress', address) && text(item, 'from', address) && text(item, 'to', address) && (item.value === null || text(item, 'value', quantity)));
  return item as unknown as TokenTransfer;
}
function log(raw: unknown): Log {
  const item = readItem(raw);
  check(text(item, 'id') && text(item, 'txHash', hash) && text(item, 'address', address) && typeof item.logIndex === 'number' && text(item, 'data', hex));
  for (const field of ['topic0', 'topic1', 'topic2', 'topic3']) check(item[field] === null || text(item, field, hash));
  return item as unknown as Log;
}
function internal(raw: unknown): InternalTransaction {
  const item = readItem(raw);
  check(text(item, 'id') && text(item, 'txHash', hash) && text(item, 'from', address) && (item.to === null || text(item, 'to', address)) && text(item, 'traceAddress') && (item.value === null || text(item, 'value', quantity)));
  return item as unknown as InternalTransaction;
}
function page<T>(raw: unknown, decode: (item: unknown) => T): Page<T> {
  const value = readPage(raw);
  return { page: value.page, items: value.items.map(decode) };
}
export const decodeBlocks = (raw: unknown) => page(raw, block);
export const decodeBlock = (raw: unknown) => raw === null ? null : block(raw);
export const decodeTransactions = (raw: unknown) => page(raw, transaction);
export const decodeTransfers = (raw: unknown) => page(raw, transfer);
export const decodeLogs = (raw: unknown) => page(raw, log);
export const decodeInternalTransactions = (raw: unknown) => page(raw, internal);
export const decodeTransaction = (raw: unknown) => raw === null ? null : transaction(raw);
export const decodeObservedStats = readObservedStats;
export function decodeLatestBlock(raw: unknown): { number: string } {
  const item = readItem(raw); check(text(item, 'number', quantity)); return { number: item.number as string };
}
export function decodeAddressStats(raw: unknown): AddressStats | null {
  if (raw === null) return null;
  const item = readItem(raw);closed(item,['address','txCount','internalTxCount','tokenTransferCount','firstSeen','lastSeen','isContract','updatedAt']);
  check(text(item, 'address', address));optionalBool(item,['isContract']);optionalText(item,['txCount','internalTxCount','tokenTransferCount','firstSeen','lastSeen'],quantity);optionalText(item,['updatedAt']);return item as unknown as AddressStats;
}
export function decodeBalance(raw: unknown): NativeBalance | null {
  if (raw === null) return null;
  const item = readItem(raw); check(text(item, 'address', address) && text(item, 'balance', quantity)); return item as unknown as NativeBalance;
}
export function decodeCode(raw: unknown): Code | null {
  if (raw === null) return null;
  const item = readItem(raw); check(text(item, 'address', address) && text(item, 'code', hex)); return item as unknown as Code;
}
export function decodeContract(raw: unknown): Contract | null {
  if (raw === null) return null;
  const item = readItem(raw); check(text(item, 'address', address));
  closed(item,['address','bytecode','bytecodeHash','creator','creationTx','blockNumber','isVerified','contractName','compilerVersion','optimizationUsed','evmVersion','sourceCode','abi','createdAt','licenseType','constructorArgs','optimizationRuns']);
  optionalText(item,['bytecode','constructorArgs'],hex);optionalText(item,['bytecodeHash','creationTx'],hash);optionalText(item,['creator'],address);optionalText(item,['blockNumber'],quantity);optionalText(item,['contractName','compilerVersion','evmVersion','sourceCode','createdAt','licenseType']);optionalBool(item,['isVerified','optimizationUsed']);
  check(item.abi === undefined || Array.isArray(item.abi) && item.abi.length <= 1000);
  return item as unknown as Contract;
}
function token(raw: unknown): Token {
  const item = readItem(raw);
  closed(item,['address','symbol','name','decimals','tokenType','totalSupply','holderCount','transferCount','usdPrice','iconUrl','l1Address','blockNumber','creationTx','offChainUpdatedAt','createdAt']);
  check(text(item,'address',address) && text(item,'symbol') && text(item,'tokenType') && text(item,'blockNumber',quantity) && Number.isInteger(item.decimals) && Number(item.decimals) >= 0 && Number(item.decimals) <= 255);
  check(text(item,'createdAt'));optionalText(item,['name','iconUrl','offChainUpdatedAt']);optionalText(item,['l1Address'],address);optionalText(item,['creationTx'],hash);optionalText(item,['usdPrice'],decimal);
  for (const key of ['totalSupply','holderCount','transferCount']) check(item[key] === undefined || text(item,key,quantity));
  return item as unknown as Token;
}
export const decodeToken = (raw: unknown) => raw === null ? null : token(raw);
export const decodeTokens = (raw: unknown) => page(raw, token);
export const decodeAccounts = (raw: unknown) => page(raw, value => {
  const item = decodeAddressStats(value); check(item !== null); return item!;
});
export const decodeHolders = (raw: unknown) => page(raw, value => {
  const item = readItem(value); check(text(item,'address',address) && text(item,'balance',quantity));
  closed(item,['address','balance','percentage','isContract']);optionalText(item,['percentage'],decimal);optionalBool(item,['isContract']);
  return item as unknown as TokenHolder;
});
export const decodeBalances = (raw: unknown) => page(raw, value => {
  const item = readItem(value); check(text(item,'address',address) && text(item,'tokenAddress',address) && text(item,'balance',quantity) && text(item,'blockNumber',quantity));
  closed(item,['address','tokenAddress','balance','blockNumber']);
  return item as unknown as Balance;
});
export function decodeSyncStatus(raw: unknown): SyncStatus {
  const item = readItem(raw); check(text(item,'lastIndexedBlock',quantity) && typeof item.isSyncing === 'boolean' && text(item,'updatedAt')); return item as unknown as SyncStatus;
}
export function decodeIndexerProgress(raw: unknown): IndexerProgress {
  const item = readItem(raw); check(text(item,'minFetchedBlock',quantity) && text(item,'maxFetchedBlock',quantity) && typeof item.backfillComplete === 'boolean' && text(item,'updatedAt')); return item as unknown as IndexerProgress;
}
export function decodeCatchup(raw: unknown): Catchup {
  const item = readItem(raw); decodeSyncStatus(item.sync); decodeIndexerProgress(item.indexer); check(item.behindBlocks === undefined || text(item,'behindBlocks',quantity)); return item as unknown as Catchup;
}
export function decodeSearchSuggestions(raw: unknown) {
  return readPage(raw).items.map(item => {
    check(!!searchSuggestionTarget(item));
    return { type: item.type as string, value: item.value as string, label: item.value as string };
  });
}
