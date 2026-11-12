// V2 ordinary read wire. Chain quantities remain strings; pagination never
// infers a total or exhaustion from a short or empty visible page.
export type Quantity = string; // canonical /^(0|[1-9][0-9]*)$/, bounded uint256
export type Decimal = string; // bounded canonical nonnegative decimal, no exponent
export type Hex = string; // canonical lowercase 0x bytes; shape checked by field
export type Address = Hex; // exactly20bytes, including intentional masked zero
export type Hash = Hex; // exactly32bytes
export type OpaqueRowID = string; // HMAC row identity, NEVER raw SQL/global counter
export type VisibilityReason = "own_address"|"public_address"|"no_access"|
  "rbac_group_member"|"participant_override"|"visible_to_grant";
export type AddressMetadata = Record<Address,VisibilityReason>; // ONLY still-Full keys
export interface PageInfo {
  next?: string;
  hasMore: boolean|null;
  scanLimited: boolean;
  total?: Quantity;
  population: "LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS";
  coverage?: "OBSERVED_VISIBLE_PAGE";
}
export interface Page<T> { items:T[]; page:PageInfo }
// hasMore:true requires visible lookahead. False requires actual exhaustion.
// null+scanLimited+next permits short/empty items. Never fake0/false for unknown.
export interface Block {
  number:Quantity; hash:Hash; parentHash:Hash; timestamp:Quantity;
  gasLimit:Quantity; baseFeePerGas?:Quantity; transactionCount?:Quantity;
  difficulty:Quantity; totalDifficulty?:Quantity; nonce:Hex; miner:Address;
  extraData:Hex; stateRoot:Hash; transactionsRoot:Hash; receiptsRoot:Hash;
  createdAt:string;
  // gasUsed/bloom/blobGasUsed/size omitted; no raw global-volume leak.
}
export interface Transaction {
  hash:Hash; blockNumber:Quantity|null; blockTimestamp?:Quantity;
  txIndex:number|null; from:Address; to:Address|null; contractAddress?:Address;
  value:Quantity|null; gasUsed?:Quantity; gasPrice:Quantity; gasLimit?:Quantity;
  maxFeePerGas?:Quantity; maxPriorityFeePerGas?:Quantity; nonce?:Quantity;
  maxFeePerBlobGas?:Quantity; txType:0|1|2|3|4; inputData:Hex;
  status:0|1|null; postStateRoot?:Hash; error?:string; revertReason?:string;
  createdAt?:string; txCategories?:string[]; tokenTransferCount?:Quantity;
  addressMetadata?:AddressMetadata;
  // status:null for pending/pre-status/unknown, never inferred success.
}
export interface TokenTransfer {
  id:OpaqueRowID; txHash:Hash; logIndex:number; tokenAddress:Address;
  from:Address; to:Address; value:Quantity|null; blockNumber:Quantity;
  timestamp?:Quantity; transferType:string; tokenType:string; tokenId?:Quantity;
  isInternal:boolean; addressMetadata?:AddressMetadata;
}
export interface InternalTransaction {
  id:OpaqueRowID; txHash:Hash; blockNumber:Quantity; traceAddress:string;
  from:Address; to:Address|null; value:Quantity|null; gas?:Quantity; gasUsed?:Quantity;
  input?:Hex; output?:Hex; callType:string; error?:string; timestamp?:Quantity;
  addressMetadata?:AddressMetadata;
}
export interface Log {
  id:OpaqueRowID; txHash:Hash; logIndex:number; address:Address;
  topic0:Hash|null; topic1:Hash|null; topic2:Hash|null; topic3:Hash|null;
  data:Hex; blockNumber:Quantity; timestamp?:Quantity; removed:boolean;
  addressMetadata?:AddressMetadata;
}
export interface Token {
  address:Address; symbol:string; name?:string; decimals:number; tokenType:string;
  totalSupply?:Quantity; holderCount?:Quantity; transferCount?:Quantity;
  usdPrice?:Decimal; iconUrl?:string; l1Address?:Address; blockNumber:Quantity;
  creationTx?:Hash; offChainUpdatedAt?:string; createdAt:string;
}
export interface TokenHolder {
  address:Address; balance:Quantity; percentage?:Decimal; isContract?:boolean;
}
export interface Balance {
  address:Address; tokenAddress:Address; blockNumber:Quantity; balance:Quantity;
}
export interface AddressStats {
  address:Address; txCount?:Quantity; internalTxCount?:Quantity;
  tokenTransferCount?:Quantity; firstSeen?:Quantity; lastSeen?:Quantity;
  isContract?:boolean; updatedAt?:string;
}
export interface ChainStats {
  totalBlocks?:Quantity; totalTransactions?:Quantity; totalAddresses?:Quantity;
  totalTokens?:Quantity; avgBlockTime?:Decimal; privacyEnabled:true;
  observedTransactions:Quantity; observedBlocks:Quantity;
  observedParticipantAddresses:Quantity; page:PageInfo;
}
export interface TxHistoryPoint { timestamp:Quantity; count:Quantity }
export interface Contract {
  address:Address; bytecode?:Hex; bytecodeHash?:Hash; creator?:Address;
  creationTx?:Hash; blockNumber?:Quantity; isVerified?:boolean;
  contractName?:string; compilerVersion?:string; optimizationUsed?:boolean;
  evmVersion?:string; sourceCode?:string; abi?:unknown[]; createdAt?:string;
  licenseType?:string; constructorArgs?:Hex; optimizationRuns?:number;
  // Code is an owned StateEngine result; ABI ONLY current approved Git registry.
  // Creator/creation fields obey the actual source and current field masks.
}
export interface SearchSuggestion { type:'transaction'|'address'|'token'|'contract'; value:string }
export interface SyncStatus {
  lastIndexedBlock:Quantity; isSyncing:boolean; updatedAt:string;
}
export interface IndexerProgress {
  minFetchedBlock:Quantity; maxFetchedBlock:Quantity;
  backfillComplete:boolean; updatedAt:string;
  // No SQL id, transaction throughput, raw row totals or coverage certificate.
}
export interface Catchup { indexer:IndexerProgress; sync:SyncStatus; behindBlocks?:Quantity }
export interface ChainID { chainId:Quantity }
export interface LatestBlock { number:Quantity }
export interface NativeBalance { address:Address; balance:Quantity; blockNumber?:Quantity; blockHash?:Hash }
export interface Code { address:Address; code:Hex; blockNumber?:Quantity; blockHash?:Hash }
export interface IsContract { address:Address; isContract:boolean }
export interface Failure { error:{code:"invalid_request"|"unauthenticated"|"denied"|
  "not_found"|"cursor_stale"|"too_broad"|"unavailable"} }
// Optional numeric metrics are present ONLY for a qualified exact current scope
// or explicit relevant-owner admin dataset permission; unknown is omission.
// Percentage never uses hidden/global denominator for a partial visible set.
// Existing readprivacy/explorerprivacy masks still control each item first.
// Scalar/single routes return their named object directly, never {data:T}.
// List routes ALL return Page<T>, never legacy per-family list/count wrappers.
// Wallet writes and protected automatic-case DTOs are separately owned contracts.
