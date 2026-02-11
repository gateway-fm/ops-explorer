const API_BASE = import.meta.env.VITE_API_URL || '/api';

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
  // Transaction categories
  txCategories?: TxCategory[];
  tokenTransferCount?: number;
}

// Transaction category types
export type TxCategory = 'coin_transfer' | 'contract_call' | 'contract_creation' | 'token_transfer';

export interface AddressInfo {
  address: string;
  balance: string | number;  // API may return string or number
  txCount: number;
  isContract: boolean;
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
  sourceCode?: string;
  abi?: AbiFragment[];
  createdAt: string;
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
}

export interface PaginatedResponse<T> {
  data: T[];
  nextCursor?: string;
  hasMore: boolean;
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

async function fetchAPI<T>(endpoint: string): Promise<T> {
  const res = await fetch(`${API_BASE}${endpoint}`);

  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export const api = {
  getStats: () => fetchAPI<ChainStats>('/stats'),

  getBlocks: (limit = 25, before?: number) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set('before', String(before));
    return fetchAPI<PaginatedResponse<Block>>(`/blocks?${params}`);
  },

  getBlock: (number: number) =>
    fetchAPI<{ block: Block; transactions: Transaction[] }>(`/blocks/${number}`),

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

  getAddress: (address: string) => fetchAPI<AddressInfo>(`/addresses/${address}`),

  getAddressTransactions: (address: string, limit = 25, before?: number) => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set('before', String(before));
    return fetchAPI<PaginatedResponse<Transaction>>(`/addresses/${address}/transactions?${params}`);
  },

  getContract: (address: string) => fetchAPI<Contract>(`/addresses/${address}/contract`),

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
  getTokens: (page = 1, pageSize = 25, type?: string) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (type) params.set('type', type);
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

  getAllTokenTransfers: (page = 1, pageSize = 25) => {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    return fetchAPI<OffsetPaginatedResponse<TokenTransfer>>(`/token-transfers?${params}`);
  },

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
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ abi }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `API error: ${res.status}`);
    }
    return res.json() as Promise<{ success: boolean; address: string }>;
  },

  // Sourcify integration
  checkSourcify: (address: string, chainId = '1') =>
    fetchAPI<{ address: string; chainId: string; isVerified: boolean; status: string }>(
      `/addresses/${address}/sourcify/check?chainId=${chainId}`
    ),

  fetchFromSourcify: async (address: string, chainId = '1') => {
    const res = await fetch(`${API_BASE}/addresses/${address}/sourcify?chainId=${chainId}`);
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

  // Contract verification
  verifySourcify: async (data: {
    address: string;
    chainId: string;
    sourceCode: string;
    contractName: string;
    compilerVersion: string;
    optimizationUsed: boolean;
    runs: number;
  }) => {
    const res = await fetch(`${API_BASE}/verify`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `API error: ${res.status}`);
    }
    return res.json() as Promise<{ success: boolean; status?: string; address?: string; error?: string }>;
  },
};
