import { getConfig } from './runtimeConfig';
import { getImpersonationTokenHeader } from '../hooks/useImpersonation';

const API_BASE = getConfig('VITE_API_URL', '/api');

/**
 * Headers attached to every BFF request. The impersonation token is read
 * here (rather than at fetch call sites) so any view-as session
 * transparently scopes the entire frontend without per-call wiring.
 *
 * The token never lives in URL params on outbound BFF calls — only the
 * X-Impersonate-Token header. Carrying it in the URL would expose it to
 * server logs / proxy headers; the header is the only sanctioned channel.
 */
function defaultHeaders(extra?: HeadersInit): HeadersInit {
  const headers: Record<string, string> = {};
  const token = getImpersonationTokenHeader();
  if (token) {
    headers['X-Impersonate-Token'] = token;
  }
  if (extra) {
    // Merge: explicit per-call headers win over our defaults.
    if (extra instanceof Headers) {
      extra.forEach((v, k) => (headers[k] = v));
    } else if (Array.isArray(extra)) {
      for (const [k, v] of extra) headers[k] = v;
    } else {
      Object.assign(headers, extra);
    }
  }
  return headers;
}

export interface Block {
  number: number;
  hash: string;
  parentHash: string;
  timestamp: number;
  gasUsed: number;
  gasLimit: number;
  baseFeePerGas?: number;
  transactionCount: number;
  // Additional block fields (Blockscout parity)
  size: number;
  difficulty: string;
  totalDifficulty: string;
  nonce: string;
  miner: string;
  extraData: string;
  stateRoot: string;
  transactionsRoot: string;
  receiptsRoot: string;
  createdAt: string;
}

export interface Transaction {
  hash: string;
  blockNumber: number;
  blockTimestamp?: number;  // Unix timestamp from block
  txIndex: number;
  from: string;
  to: string | null;
  contractAddress?: string | null;  // address of created contract (if contract creation)
  value: string | number;  // API may return string or number
  gasUsed: number;
  gasPrice: number;
  inputData: string;
  status: number;
  createdAt: string;
  nonce?: number;
  txType?: number;
  gasLimit?: number;
  maxFeePerGas?: number;
  maxPriorityFeePerGas?: number;
  error?: string;
  revertReason?: string;
  // Transaction categories
  txCategories?: TxCategory[];
  tokenTransferCount?: number;
  addressMetadata?: Record<string, VisibilityReason>;
}

// Transaction category types
export type TxCategory = 'coin_transfer' | 'contract_call' | 'contract_creation' | 'token_transfer' | 'system_transaction';

export interface AddressInfo {
  address: string;
  balance: string | number;  // API may return string or number
  txCount: number;
  isContract: boolean;
  // Both populated by the indexer's address_stats aggregate. Default to 0 if
  // the row hasn't been built yet for this address.
  tokenTransferCount: number;
  internalTxCount: number;
}

export interface Contract {
  address: string;
  bytecode: string;
  bytecodeHash?: string;
  creator: string;
  creationTx: string;
  blockNumber: number;
  isVerified: boolean;
  contractName?: string;
  compilerVersion?: string;
  optimizationUsed?: boolean;
  evmVersion?: string;
  sourceCode?: string;
  abi?: AbiFragment[];
  createdAt: string;
  licenseType?: string;
  constructorArgs?: string;
  optimizationRuns?: number;
}

// ABI types for contract interaction
export interface AbiInput {
  name: string;
  type: string;
  indexed?: boolean;
  internalType?: string;
}

export interface AbiOutput {
  name: string;
  type: string;
  internalType?: string;
}

export interface AbiFragment {
  type: 'function' | 'event' | 'constructor' | 'fallback' | 'receive' | 'error';
  name?: string;
  inputs?: AbiInput[];
  outputs?: AbiOutput[];
  stateMutability?: 'pure' | 'view' | 'nonpayable' | 'payable';
  anonymous?: boolean;
  constant?: boolean;
  payable?: boolean;
}

export interface ChainStats {
  totalBlocks: number;
  totalTransactions: number;
  totalAddresses: number;
  avgBlockTime: number;
  privacyEnabled?: boolean;
}

export interface RowAddressInfo {
  isContract: boolean;
  name?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  nextCursor?: string;
  hasMore: boolean;
  addressInfo?: Record<string, RowAddressInfo>;
}

export interface SearchSuggestion {
  type: 'block' | 'transaction' | 'address';
  value: string;
  label: string;
}

export interface SearchSuggestionsResponse {
  query: string;
  suggestions: SearchSuggestion[];
}

export interface TokenTransfer {
  id: number;
  txHash: string;
  logIndex: number;
  tokenAddress: string;
  from: string;
  to: string;
  value: string | number;
  blockNumber: number;
  timestamp?: number;
  transferType: string;
  tokenType: string;
  tokenId?: string;
  isInternal: boolean;
  addressMetadata?: Record<string, VisibilityReason>;
}

export interface Balance {
  address: string;
  tokenAddress: string;
  blockNumber: number;
  balance: string;
}

export interface Log {
  id: number;
  txHash: string;
  logIndex: number;
  address: string;
  topic0: string | null;
  topic1: string | null;
  topic2: string | null;
  topic3: string | null;
  data: string;
  blockNumber: number;
  addressMetadata?: Record<string, VisibilityReason>;
}

export interface InternalTransaction {
  id: number;
  txHash: string;
  blockNumber: number;
  traceAddress: string;
  from: string;
  to: string | null;
  value: string | number;
  gas?: number;
  gasUsed?: number;
  input?: string;
  output?: string;
  callType: string;
  error?: string;
  timestamp?: number;
  addressMetadata?: Record<string, VisibilityReason>;
}

export interface TxHistoryPoint {
  timestamp: number;
  count: number;
}

export interface Token {
  address: string;
  symbol: string;
  name?: string;
  decimals: number;
  tokenType: string;
  totalSupply?: string;
  holderCount: number;
  transferCount: number;
  usdPrice?: number;
  iconUrl?: string;
  blockNumber: number;
  creationTx?: string;
  createdAt: string;
}

export interface TokenHolder {
  address: string;
  balance: string | number;
  percentage: number;
  isContract: boolean;
  addressMetadata?: Record<string, VisibilityReason>;
}

export interface NFTItem {
  tokenId: string;
  owner: string;
  tokenUri?: string;
}

export interface ChainInfo {
  chainId: string;
  chainIdDecimal: number;
  networkId: string;
  clientVersion: string;
  protocolVersion: string;
  latestBlock: number;
  gasPrice: string;
  peerCount: number;
  isSyncing: boolean;
  genesisHash: string;
  updatedAt: string;
}

export interface SyncStatus {
  syncStatus: {
    id: number;
    lastIndexedBlock: number;
    lastVerifiedBlock?: number;
    lastFinalizedBlock?: number;
    isSyncing: boolean;
    updatedAt: string;
  };
  latestChainBlock: number;
  blocksRemaining: number;
  isSynced: boolean;
}

export interface PriceData {
  price: number;
  currency: string;
  change24h: number;
  marketCap?: number;
  volume24h?: number;
  lastUpdated: string;
}

export interface GasPrice {
  price: number;      // Gwei
  priceWei: string;   // Wei
  baseFee?: number;   // Gwei
  priorityFee?: number; // Gwei
}

export interface GasPrices {
  slow: GasPrice | null;
  normal: GasPrice | null;
  fast: GasPrice | null;
  updatedAt: string;
}

export interface AccountListItem {
  address: string;
  balance: string | number;
  txCount: number;
  isContract: boolean;
}

export interface OffsetPaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

// ETH address linking types
export interface LinkedAddress {
  address: string;
  verified_at: string;
  link_type: 'user' | 'system';
  ens_name?: string;
}

// Privacy types
export type VisibilityLevel = 'full' | 'pseudonymous' | 'redacted' | 'hidden';
export type VisibilityReason = 'own_address' | 'disclosure_grant' | 'rbac_group_member' | 'public_address' | 'no_access' | 'participant_override';

export interface AddressVisibility {
  address: string;
  visible: boolean;
  level: VisibilityLevel;
  reason: VisibilityReason;
  pseudonym?: string;
  grant_id?: string;
  expires_at?: string;
}

export interface OwnAddress {
  address: string;
  ens_name?: string;
}

export interface DisclosedAddress {
  address: string;
  address_id: string;
  owner_did: string;
  disclosure_level: string;
  grant_id: string;
  expires_at?: string;
  ens_name?: string;
}

export interface ViewableAddressesResponse {
  viewer_wallet: string;
  viewer_did?: string;
  own_addresses: OwnAddress[];
  disclosed_addresses: DisclosedAddress[];
}

export interface GrantedAddressResponse {
  display_address: string;
  disclosure_level: string;
  grant_id: string;
  balance: string;
  tx_count: number;
  is_contract: boolean;
  scope_methods?: string[]; // Grant scope methods (e.g. "transaction_history", "activity_logs")
}

export interface PseudonymizedTransaction {
  tx_hash?: string;
  block_number: number;
  block_timestamp: number;
  from: string;
  to?: string;
  value: string | number;
  gas_used: number;
  status: number;
  direction: 'in' | 'out' | 'self';
}

export interface PseudonymizedTransactionsResponse {
  transactions: PseudonymizedTransaction[];
  disclosure_level: string;
  address_labels: Record<string, string>;
  has_more: boolean;
}

export interface ActivityLogEntry {
  method: string;
  status_code: number;
  timestamp: string;
}

export interface ActivityLogsResponse {
  logs: ActivityLogEntry[];
  total: number;
  limit: number;
  offset: number;
}

export interface ChartLineInfo {
  id: string;
  title: string;
  description: string;
  units?: string;
  section: string;
}

export interface ChartDataPoint {
  date: string;
  value: number;
}

export interface ChartLineResponse {
  info: ChartLineInfo;
  chart: ChartDataPoint[];
}

export interface ChartCounter {
  id: string;
  title: string;
  value: string;
  units?: string;
  description: string;
}

async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${endpoint}`, {
    credentials: 'include',
    ...options,
    headers: defaultHeaders(options?.headers),
  });

  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export const api = {
  getStats: () => fetchAPI<ChainStats>('/stats'),

  // Chart endpoints
  getChartLines: () => fetchAPI<ChartLineInfo[]>('/charts/lines'),
  getChartLine: (id: string, from?: string, to?: string) => {
    const params = new URLSearchParams();
    if (from) params.set('from', from);
    if (to) params.set('to', to);
    const qs = params.toString();
    return fetchAPI<ChartLineResponse>(`/charts/lines/${id}${qs ? '?' + qs : ''}`);
  },
  getChartCounters: () => fetchAPI<ChartCounter[]>('/charts/counters'),

  getBlocks: (limit = 25, before?: number) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set('before', String(before));
    return fetchAPI<PaginatedResponse<Block>>(`/blocks?${params}`);
  },

  getBlock: (number: number) =>
    fetchAPI<{ block: Block; transactions: Transaction[] }>(`/blocks/${number}`),

  getBlockInternalTxs: (number: number) =>
    fetchAPI<InternalTransaction[]>(`/blocks/${number}/internal`),

  getLatestBlock: () => fetchAPI<Block>('/blocks/latest'),

  getTransactions: (limit = 25, before?: number) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set('before', String(before));
    return fetchAPI<PaginatedResponse<Transaction>>(`/transactions?${params}`);
  },

  getTransactionsPaginated: (page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<Transaction>>(`/transactions?${params}`);
  },

  getTransaction: (hash: string) => fetchAPI<Transaction>(`/transactions/${hash}`),

  getTransactionTransfers: (hash: string) => fetchAPI<TokenTransfer[]>(`/transactions/${hash}/transfers`),

  getTransactionLogs: (hash: string) => fetchAPI<Log[]>(`/transactions/${hash}/logs`),

  getTransactionInternalTxs: (hash: string) => fetchAPI<InternalTransaction[]>(`/transactions/${hash}/internal`),

  getAddress: (address: string) => fetchAPI<AddressInfo>(`/addresses/${address}`),

  getAddressTransactions: (address: string, limit = 25, before?: number) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set('before', String(before));
    return fetchAPI<PaginatedResponse<Transaction>>(`/addresses/${address}/transactions?${params}`);
  },

  getContract: (address: string) => fetchAPI<Contract>(`/addresses/${address}/contract`),

  getAddressTransfers: (address: string, limit = 25, before?: number) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set('before', String(before));
    return fetchAPI<PaginatedResponse<TokenTransfer>>(`/addresses/${address}/transfers?${params}`);
  },

  getAddressInternalTxs: (address: string, page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<InternalTransaction>>(`/addresses/${address}/internal?${params}`);
  },

  getAddressLogs: (address: string, page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<Log>>(`/addresses/${address}/logs?${params}`);
  },

  getAddressTokenBalances: (address: string) =>
    fetchAPI<Balance[]>(`/addresses/${address}/balances`),

  search: (query: string) =>
    fetchAPI<{ type: string; data: unknown }>(`/search?q=${encodeURIComponent(query)}`),

  searchSuggestions: (query: string) =>
    fetchAPI<SearchSuggestionsResponse>(`/search/suggestions?q=${encodeURIComponent(query)}`),

  getTransactionHistory: (interval = 60, limit = 30) => {
    const params = new URLSearchParams({ interval: String(interval), limit: String(limit) });
    return fetchAPI<TxHistoryPoint[]>(`/stats/tx-history?${params}`);
  },

  getAccounts: (page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<AccountListItem>>(`/accounts?${params}`);
  },

  // Token endpoints
  getTokens: (page = 1, pageSize = 25, type?: string, search?: string) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (type) params.set('type', type);
    if (search) params.set('search', search);
    return fetchAPI<OffsetPaginatedResponse<Token>>(`/tokens?${params}`);
  },

  getToken: (address: string) => fetchAPI<Token>(`/tokens/${address}`),

  getTokenHolders: (address: string, page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<TokenHolder>>(`/tokens/${address}/holders?${params}`);
  },

  getTokenTransfers: (address: string, page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<TokenTransfer>>(`/tokens/${address}/transfers?${params}`);
  },

  getTokenInventory: (address: string, page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<NFTItem>>(`/tokens/${address}/inventory?${params}`);
  },

  getNftItem: async (address: string, tokenId: string): Promise<NFTItem | null> => {
    const params = new URLSearchParams({ tokenId, page: '1', pageSize: '1' });
    const res = await fetchAPI<OffsetPaginatedResponse<NFTItem>>(`/tokens/${address}/inventory?${params}`);
    return res.data?.[0] ?? null;
  },

  getAllTokenTransfers: (page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<TokenTransfer>>(`/token-transfers?${params}`);
  },

  // Chain info
  getChainInfo: () => fetchAPI<ChainInfo>('/chain-info'),

  // Sync status
  getSyncStatus: () => fetchAPI<SyncStatus>('/sync'),

  // Price
  getPrice: () => fetchAPI<PriceData>('/price'),

  // Gas prices
  getGasPrices: () => fetchAPI<GasPrices>('/gas'),

  // Contract ABI
  updateContractABI: async (address: string, abi: AbiFragment[]) => {
    const res = await fetch(`${API_BASE}/addresses/${address}/abi`, {
      method: 'POST',
      headers: defaultHeaders({ 'Content-Type': 'application/json' }),
      credentials: 'include',
      body: JSON.stringify({ abi }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `API error: ${res.status}`);
    }
    return res.json() as Promise<{ success: boolean; address: string }>;
  },

  // Sourcify integration
  checkSourcify: (address: string, chainId?: string) => {
    const params = chainId ? `?chainId=${chainId}` : '';
    return fetchAPI<{ address: string; chainId: string; isVerified: boolean; status: string }>(
      `/addresses/${address}/sourcify/check${params}`
    );
  },

  fetchFromSourcify: async (address: string, chainId?: string) => {
    const params = chainId ? `?chainId=${chainId}` : '';
    const res = await fetch(`${API_BASE}/addresses/${address}/sourcify${params}`, {
      credentials: 'include',
      headers: defaultHeaders(),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `API error: ${res.status}`);
    }
    return res.json() as Promise<{
      success: boolean;
      address: string;
      contractName: string;
      compilerVersion: string;
      abiLength: number;
    }>;
  },

  // Privacy endpoints
  getViewableAddresses: () =>
    fetchAPI<ViewableAddressesResponse>('/privacy/viewable-addresses'),

  getGrantedAddress: (grantId: string, addressId: string) =>
    fetchAPI<GrantedAddressResponse>(`/privacy/grant/${grantId}/${addressId}`),

  getGrantedAddressTransactions: (grantId: string, addressId: string, limit = 25, before?: number) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set('before', String(before));
    return fetchAPI<PseudonymizedTransactionsResponse>(`/privacy/grant/${grantId}/${addressId}/transactions?${params}`);
  },

  getGrantActivityLogs: (grantId: string, limit = 25, offset = 0) =>
    fetchAPI<ActivityLogsResponse>(`/privacy/grant/${grantId}/activity?limit=${limit}&offset=${offset}`),

  // ETH address linking endpoints (proxied through backend to privacy-proxy)
  eth: {
    getLinkedAddresses: () =>
      fetchAPI<{ addresses: LinkedAddress[] }>('/eth/addresses'),

    createLinkChallenge: () =>
      fetchAPI<{ nonce: string; message: string }>('/eth/link/challenge', {
        method: 'POST',
      }),

    verifyLink: (nonce: string, address: string, signature: string) =>
      fetchAPI<{ message: string; address: string }>('/eth/link/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nonce, address, signature }),
      }),

    unlinkAddress: async (address: string) => {
      const res = await fetch(`${API_BASE}/eth/addresses/${encodeURIComponent(address.toLowerCase())}`, {
        method: 'DELETE',
        credentials: 'include',
        headers: defaultHeaders(),
      });
      if (!res.ok) {
        throw new Error(`API error: ${res.status}`);
      }
      return res.json() as Promise<{ success: boolean }>;
    },
  },

  // Auth endpoints
  auth: {
    status: () =>
      fetchAPI<{ authenticated: boolean; did?: string; expires_at?: number }>('/auth/status'),

    logout: async () => {
      const res = await fetch(`${API_BASE}/auth/logout`, {
        method: 'POST',
        credentials: 'include',
      });
      if (!res.ok) throw new Error(`API error: ${res.status}`);
      return res.json() as Promise<{ logged_out: boolean }>;
    },
  },

  // Contract verification
  verifyContract: async (data: {
    address: string;
    sourceCode?: string;
    sourceFiles?: Record<string, string>;
    mainContractFile?: string;
    contractName: string;
    compilerVersion: string;
    evmVersion?: string;
    optimizationUsed: boolean;
    optimizationRuns: number;
    constructorArgs?: string;
    libraries?: Record<string, string>;
    licenseType?: string;
  }) => {
    const res = await fetch(`${API_BASE}/verify`, {
      method: 'POST',
      headers: defaultHeaders({ 'Content-Type': 'application/json' }),
      credentials: 'include',
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `API error: ${res.status}`);
    }
    return res.json() as Promise<{
      success: boolean;
      matchType?: string;
      contractName?: string;
      compilerVersion?: string;
      abi?: unknown;
      address?: string;
      error?: string;
    }>;
  },

  verifyContractStandardJSON: async (data: {
    address: string;
    compilerVersion: string;
    contractName: string;
    contractFile?: string;
    standardInput: unknown;
    constructorArgs?: string;
    licenseType?: string;
  }) => {
    const res = await fetch(`${API_BASE}/verify/standard-json`, {
      method: 'POST',
      headers: defaultHeaders({ 'Content-Type': 'application/json' }),
      credentials: 'include',
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `API error: ${res.status}`);
    }
    return res.json() as Promise<{
      success: boolean;
      matchType?: string;
      contractName?: string;
      compilerVersion?: string;
      abi?: unknown;
      address?: string;
      error?: string;
    }>;
  },

  getCompilerVersions: () =>
    fetchAPI<{ versions: { version: string; path: string; longVersion?: string }[] }>('/verify/compilers'),
};
