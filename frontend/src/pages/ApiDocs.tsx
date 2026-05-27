import { useState, useEffect, useRef, useCallback } from 'react';
import { Copy, Check, ChevronDown, ChevronRight, Play, Loader2, BookOpen, AlertTriangle } from 'lucide-react';
import { PageHeader } from '../components/PageHeader';
import { getConfig } from '../lib/runtimeConfig';

const BASE_URL = getConfig('VITE_PUBLIC_API_URL', 'http://localhost:8082');

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Param {
  name: string;
  type: string;
  required?: boolean;
  description: string;
  default?: string;
}

interface Endpoint {
  method: 'GET';
  path: string;
  description: string;
  params?: Param[];
  exampleResponse: unknown;
}

interface EndpointGroup {
  id: string;
  label: string;
  endpoints: Endpoint[];
}

// ---------------------------------------------------------------------------
// Endpoint data
// ---------------------------------------------------------------------------

const endpointGroups: EndpointGroup[] = [
  {
    id: 'blocks',
    label: 'Blocks',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/blocks',
        description: 'List blocks with optional pagination.',
        params: [
          { name: 'limit', type: 'int', description: 'Number of blocks to return.', default: '25' },
          { name: 'before', type: 'int', description: 'Cursor — return blocks before this block number.' },
        ],
        exampleResponse: {
          blocks: [
            { number: 1000, hash: '0xabc...', timestamp: 1700000000, transactionCount: 12 },
          ],
        },
      },
      {
        method: 'GET',
        path: '/api/v1/blocks/latest',
        description: 'Get the latest indexed block.',
        exampleResponse: {
          number: 1000,
          hash: '0xabc...',
          timestamp: 1700000000,
          transactionCount: 12,
          gasUsed: '21000',
          gasLimit: '30000000',
        },
      },
      {
        method: 'GET',
        path: '/api/v1/blocks/{number}',
        description: 'Get a block by its number.',
        params: [
          { name: 'number', type: 'int', required: true, description: 'Block number (path parameter).' },
        ],
        exampleResponse: {
          number: 1000,
          hash: '0xabc...',
          parentHash: '0xdef...',
          timestamp: 1700000000,
          transactionCount: 12,
          gasUsed: '21000',
          gasLimit: '30000000',
          miner: '0x1234...',
        },
      },
    ],
  },
  {
    id: 'transactions',
    label: 'Transactions',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/transactions',
        description: 'List transactions with pagination.',
        params: [
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.', default: '25' },
        ],
        exampleResponse: {
          transactions: [
            { hash: '0xabc...', from: '0x1234...', to: '0x5678...', value: '1000000000000000000', blockNumber: 1000 },
          ],
          total: 50000,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/transactions/{hash}',
        description: 'Get a transaction by its hash.',
        params: [
          { name: 'hash', type: 'string', required: true, description: 'Transaction hash, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          hash: '0xabc...',
          from: '0x1234...',
          to: '0x5678...',
          value: '1000000000000000000',
          gasUsed: '21000',
          gasPrice: '20000000000',
          blockNumber: 1000,
          status: 1,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/transactions/{hash}/logs',
        description: 'Get event logs emitted by a transaction.',
        params: [
          { name: 'hash', type: 'string', required: true, description: 'Transaction hash, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          logs: [
            { logIndex: 0, address: '0xabc...', topics: ['0xddf...'], data: '0x...' },
          ],
        },
      },
      {
        method: 'GET',
        path: '/api/v1/transactions/{hash}/transfers',
        description: 'Get token transfers within a transaction.',
        params: [
          { name: 'hash', type: 'string', required: true, description: 'Transaction hash, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          transfers: [
            { from: '0x1234...', to: '0x5678...', value: '1000000', tokenAddress: '0xtoken...' },
          ],
        },
      },
      {
        method: 'GET',
        path: '/api/v1/transactions/{hash}/internal',
        description: 'Get internal transactions (traces) for a transaction.',
        params: [
          { name: 'hash', type: 'string', required: true, description: 'Transaction hash, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          internalTransactions: [
            { from: '0x1234...', to: '0x5678...', value: '500000000000000000', type: 'CALL' },
          ],
        },
      },
    ],
  },
  {
    id: 'addresses',
    label: 'Addresses',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/addresses/{address}',
        description: 'Get information about an address.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Address, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          address: '0x1234...',
          balance: '1000000000000000000',
          transactionCount: 42,
          isContract: false,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/addresses/{address}/transactions',
        description: 'Get transactions for an address.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Address, 0x-prefixed (path parameter).' },
          { name: 'limit', type: 'int', description: 'Number of transactions to return.' },
          { name: 'before', type: 'int', description: 'Cursor — return transactions before this index.' },
        ],
        exampleResponse: {
          transactions: [
            { hash: '0xabc...', from: '0x1234...', to: '0x5678...', value: '1000000000000000000', blockNumber: 1000 },
          ],
        },
      },
      {
        method: 'GET',
        path: '/api/v1/addresses/{address}/transfers',
        description: 'Get token transfers for an address.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Address, 0x-prefixed (path parameter).' },
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
        ],
        exampleResponse: {
          transfers: [
            { from: '0x1234...', to: '0x5678...', value: '1000000', tokenAddress: '0xtoken...' },
          ],
          total: 100,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/addresses/{address}/balances',
        description: 'Get token balances for an address.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Address, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          balances: [
            { tokenAddress: '0xtoken...', symbol: 'USDC', decimals: 6, balance: '1000000' },
          ],
        },
      },
      {
        method: 'GET',
        path: '/api/v1/addresses/{address}/internal',
        description: 'Get internal transactions for an address.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Address, 0x-prefixed (path parameter).' },
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
        ],
        exampleResponse: {
          internalTransactions: [
            { from: '0x1234...', to: '0x5678...', value: '500000000000000000', type: 'CALL', blockNumber: 1000 },
          ],
          total: 50,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/addresses/{address}/logs',
        description: 'Get event logs for an address.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Address, 0x-prefixed (path parameter).' },
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
        ],
        exampleResponse: {
          logs: [
            { logIndex: 0, transactionHash: '0xabc...', topics: ['0xddf...'], data: '0x...' },
          ],
          total: 200,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/addresses/{address}/contract',
        description: 'Get contract details for an address (ABI, source, compiler info).',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Contract address, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          address: '0x1234...',
          name: 'MyToken',
          compiler: 'solc 0.8.20',
          verified: true,
          abi: ['...'],
        },
      },
    ],
  },
  {
    id: 'tokens',
    label: 'Tokens',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/tokens',
        description: 'List tokens with optional type filter.',
        params: [
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
          { name: 'type', type: 'string', description: 'Token standard filter: erc20 or erc721.' },
        ],
        exampleResponse: {
          tokens: [
            { address: '0xtoken...', name: 'USD Coin', symbol: 'USDC', decimals: 6, totalSupply: '1000000000000' },
          ],
          total: 150,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/tokens/{address}',
        description: 'Get details for a specific token.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Token contract address, 0x-prefixed (path parameter).' },
        ],
        exampleResponse: {
          address: '0xtoken...',
          name: 'USD Coin',
          symbol: 'USDC',
          decimals: 6,
          totalSupply: '1000000000000',
          holdersCount: 5000,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/tokens/{address}/holders',
        description: 'Get holders of a token.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Token contract address (path parameter).' },
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
        ],
        exampleResponse: {
          holders: [
            { address: '0x1234...', balance: '500000000', share: 0.05 },
          ],
          total: 5000,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/tokens/{address}/transfers',
        description: 'Get transfers of a specific token.',
        params: [
          { name: 'address', type: 'string', required: true, description: 'Token contract address (path parameter).' },
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
        ],
        exampleResponse: {
          transfers: [
            { from: '0x1234...', to: '0x5678...', value: '1000000', transactionHash: '0xabc...' },
          ],
          total: 10000,
        },
      },
    ],
  },
  {
    id: 'token-transfers',
    label: 'Token Transfers',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/token-transfers',
        description: 'List all token transfers across the chain.',
        params: [
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
        ],
        exampleResponse: {
          transfers: [
            { from: '0x1234...', to: '0x5678...', value: '1000000', tokenAddress: '0xtoken...', transactionHash: '0xabc...' },
          ],
          total: 500000,
        },
      },
    ],
  },
  {
    id: 'accounts',
    label: 'Accounts',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/accounts',
        description: 'Get top accounts by balance.',
        params: [
          { name: 'page', type: 'int', description: 'Page number.' },
          { name: 'pageSize', type: 'int', description: 'Results per page.' },
        ],
        exampleResponse: {
          accounts: [
            { address: '0x1234...', balance: '50000000000000000000000', transactionCount: 15000 },
          ],
          total: 100000,
        },
      },
    ],
  },
  {
    id: 'search',
    label: 'Search',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/search',
        description: 'Search for blocks, transactions, or addresses.',
        params: [
          { name: 'q', type: 'string', required: true, description: 'Search query (block number, tx hash, or address).' },
        ],
        exampleResponse: {
          results: [
            { type: 'address', value: '0x1234...', label: '0x1234...' },
          ],
        },
      },
      {
        method: 'GET',
        path: '/api/v1/search/suggestions',
        description: 'Autocomplete search suggestions.',
        params: [
          { name: 'q', type: 'string', required: true, description: 'Partial search query.' },
        ],
        exampleResponse: {
          suggestions: [
            { type: 'address', value: '0x1234...', label: '0x1234...' },
            { type: 'block', value: '1000', label: 'Block #1000' },
          ],
        },
      },
    ],
  },
  {
    id: 'stats',
    label: 'Stats',
    endpoints: [
      {
        method: 'GET',
        path: '/api/v1/stats',
        description: 'Get chain statistics (total blocks, transactions, addresses).',
        exampleResponse: {
          totalBlocks: 1000000,
          totalTransactions: 50000000,
          totalAddresses: 2000000,
          averageBlockTime: 2.1,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/stats/tx-history',
        description: 'Get transaction count history over time.',
        params: [
          { name: 'interval', type: 'int', description: 'Time interval in seconds between data points.' },
          { name: 'limit', type: 'int', description: 'Number of data points to return.' },
        ],
        exampleResponse: {
          history: [
            { timestamp: 1700000000, count: 1200 },
            { timestamp: 1700003600, count: 1350 },
          ],
        },
      },
      {
        method: 'GET',
        path: '/api/v1/gas',
        description: 'Get current gas prices (slow, normal, fast).',
        exampleResponse: {
          slow: { price: 0.1, baseFee: 0.08, priorityFee: 0.02 },
          normal: { price: 0.5, baseFee: 0.08, priorityFee: 0.42 },
          fast: { price: 1.2, baseFee: 0.08, priorityFee: 1.12 },
          updatedAt: '2025-01-01T00:00:00Z',
        },
      },
      {
        method: 'GET',
        path: '/api/v1/chain-info',
        description: 'Get chain metadata — chain ID, client version, sync status, etc.',
        exampleResponse: {
          chainId: '0x3E9',
          chainIdDecimal: 1001,
          clientVersion: 'Geth/v1.13.0',
          protocolVersion: '68',
          networkId: '1001',
          isSyncing: false,
          latestBlock: 1000000,
          gasPrice: '20000000000',
          peerCount: 25,
        },
      },
      {
        method: 'GET',
        path: '/api/v1/sync',
        description: 'Get indexer sync status.',
        exampleResponse: {
          latestChainBlock: 1000000,
          syncStatus: { lastIndexedBlock: 999998 },
          blocksRemaining: 2,
        },
      },
    ],
  },
];

// ---------------------------------------------------------------------------
// JSON Syntax Highlighting
// ---------------------------------------------------------------------------

function JsonSyntax({ data }: { data: unknown }) {
  const json = JSON.stringify(data, null, 2);

  // Tokenize JSON into spans without dangerouslySetInnerHTML
  const tokens: { text: string; className?: string }[] = [];
  const re = /("(?:\\.|[^"\\])*")\s*:|:\s*("(?:\\.|[^"\\])*")|:\s*(\d+(?:\.\d+)?)|:\s*(true|false|null)|[^":]+|./g;
  let match: RegExpExecArray | null;
  let lastIndex = 0;

  while ((match = re.exec(json)) !== null) {
    // Add any skipped text as plain
    if (match.index > lastIndex) {
      tokens.push({ text: json.slice(lastIndex, match.index) });
    }
    if (match[1]) {
      // Key
      tokens.push({ text: match[1], className: 'text-purple-600' });
      tokens.push({ text: ':' });
    } else if (match[2]) {
      // String value
      tokens.push({ text: ': ' });
      tokens.push({ text: match[2], className: 'text-green-600' });
    } else if (match[3]) {
      // Number value
      tokens.push({ text: ': ' });
      tokens.push({ text: match[3], className: 'text-blue-600' });
    } else if (match[4]) {
      // Boolean/null
      tokens.push({ text: ': ' });
      tokens.push({ text: match[4], className: 'text-amber-600' });
    } else {
      tokens.push({ text: match[0] });
    }
    lastIndex = re.lastIndex;
  }
  if (lastIndex < json.length) {
    tokens.push({ text: json.slice(lastIndex) });
  }

  return (
    <pre className="text-xs leading-relaxed overflow-x-auto">
      {tokens.map((t, i) =>
        t.className ? <span key={i} className={t.className}>{t.text}</span> : t.text
      )}
    </pre>
  );
}

// ---------------------------------------------------------------------------
// Copy button
// ---------------------------------------------------------------------------

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const copy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      onClick={copy}
      className="p-1.5 rounded-md text-neutral-400 hover:text-neutral-600 hover:bg-neutral-200 transition-colors"
      title="Copy"
    >
      {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
    </button>
  );
}

// ---------------------------------------------------------------------------
// Build curl command
// ---------------------------------------------------------------------------

function buildCurl(endpoint: Endpoint): string {
  let path = endpoint.path;
  const queryParams: string[] = [];

  endpoint.params?.forEach((p) => {
    if (path.includes(`{${p.name}}`)) {
      // Replace path params with example values
      const example = p.type === 'int' ? '1' : p.name === 'hash' ? '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef' : p.name === 'address' ? '0x1234567890abcdef1234567890abcdef12345678' : 'example';
      path = path.replace(`{${p.name}}`, example);
    } else {
      const val = p.default || (p.type === 'int' ? '1' : 'value');
      queryParams.push(`${p.name}=${val}`);
    }
  });

  const qs = queryParams.length > 0 ? `?${queryParams.join('&')}` : '';
  return `curl "${BASE_URL}${path}${qs}"`;
}

// ---------------------------------------------------------------------------
// Try It panel
// ---------------------------------------------------------------------------

function TryIt({ endpoint }: { endpoint: Endpoint }) {
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [response, setResponse] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<number | null>(null);

  const execute = async () => {
    setLoading(true);
    setResponse(null);
    setStatus(null);

    let path = endpoint.path;
    const queryParams: string[] = [];

    endpoint.params?.forEach((p) => {
      const val = values[p.name] || '';
      if (!val) return;
      if (path.includes(`{${p.name}}`)) {
        path = path.replace(`{${p.name}}`, encodeURIComponent(val));
      } else {
        queryParams.push(`${encodeURIComponent(p.name)}=${encodeURIComponent(val)}`);
      }
    });

    // Also replace any remaining path params with empty string fallback
    path = path.replace(/\{[^}]+\}/g, '');

    const qs = queryParams.length > 0 ? `?${queryParams.join('&')}` : '';
    const url = `${BASE_URL}${path}${qs}`;

    // Aikido SAST (severity 15 / low SSRF): `path` is composed from the
    // OpenAPI spec + user-supplied path parameters. The user input is
    // URL-encoded in the substitution loop above, but to defend against
    // a malicious or compromised spec that re-introduces protocol-relative
    // segments (e.g. "//evil.com"), confirm the resolved URL still belongs
    // to BASE_URL's origin before we fetch.
    try {
      const resolved = new URL(url);
      const trusted = new URL(BASE_URL);
      if (resolved.origin !== trusted.origin) {
        setResponse(`Error: refusing to fetch off-origin URL ${resolved.origin}`);
        setLoading(false);
        return;
      }
    } catch {
      setResponse('Error: invalid request URL');
      setLoading(false);
      return;
    }

    try {
      const res = await fetch(url);
      setStatus(res.status);
      const text = await res.text();
      try {
        const json = JSON.parse(text);
        setResponse(JSON.stringify(json, null, 2));
      } catch {
        setResponse(text);
      }
    } catch (err) {
      setResponse(`Error: ${err instanceof Error ? err.message : 'Request failed'}`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="border-t border-neutral-100">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 px-4 py-2.5 w-full text-left text-sm font-medium text-primary hover:bg-primary-50 transition-colors"
      >
        <Play className="w-3.5 h-3.5" />
        Try it
        {open ? <ChevronDown className="w-3.5 h-3.5 ml-auto" /> : <ChevronRight className="w-3.5 h-3.5 ml-auto" />}
      </button>

      {open && (
        <div className="px-4 pb-4 space-y-3">
          {endpoint.params && endpoint.params.length > 0 && (
            <div className="space-y-2">
              {endpoint.params.map((p) => (
                <div key={p.name} className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-3">
                  <label className="text-xs font-medium text-neutral-600 sm:w-28 shrink-0 font-mono">
                    {p.name}
                    {p.required && <span className="text-red-500 ml-0.5">*</span>}
                  </label>
                  <input
                    type="text"
                    value={values[p.name] || ''}
                    onChange={(e) => setValues({ ...values, [p.name]: e.target.value })}
                    placeholder={p.default ? `Default: ${p.default}` : p.type}
                    className="input text-sm py-1.5 px-3 font-mono"
                  />
                </div>
              ))}
            </div>
          )}

          <button
            onClick={execute}
            disabled={loading}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-primary hover:bg-primary-600 rounded-lg transition-colors disabled:opacity-50"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Send Request
          </button>

          {response !== null && (
            <div className="relative">
              {status !== null && (
                <div className={`text-xs font-medium mb-1 ${status >= 200 && status < 300 ? 'text-green-600' : status === 429 ? 'text-amber-600' : 'text-red-600'}`}>
                  Status: {status}
                </div>
              )}
              <div className="bg-neutral-900 rounded-lg p-4 overflow-x-auto">
                <pre className="text-xs text-neutral-100 whitespace-pre-wrap break-words">{response}</pre>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Endpoint Card
// ---------------------------------------------------------------------------

function EndpointCard({ endpoint }: { endpoint: Endpoint }) {
  const [expanded, setExpanded] = useState(false);
  const curl = buildCurl(endpoint);

  return (
    <div className="card overflow-hidden">
      {/* Header */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 px-4 py-3 hover:bg-neutral-50 transition-colors text-left"
      >
        <span className="shrink-0 px-2 py-0.5 rounded text-xs font-bold uppercase bg-green-100 text-green-700 border border-green-200">
          {endpoint.method}
        </span>
        <span className="font-mono text-sm text-neutral-800 truncate">{endpoint.path}</span>
        <ChevronDown className={`w-4 h-4 text-neutral-400 ml-auto shrink-0 transition-transform ${expanded ? 'rotate-180' : ''}`} />
      </button>

      {expanded && (
        <div>
          {/* Description */}
          <div className="px-4 pb-3">
            <p className="text-sm text-neutral-600">{endpoint.description}</p>
          </div>

          {/* Parameters */}
          {endpoint.params && endpoint.params.length > 0 && (
            <div className="px-4 pb-4">
              <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Parameters</h4>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-neutral-200">
                      <th className="text-left py-2 pr-4 text-xs font-medium text-neutral-500">Name</th>
                      <th className="text-left py-2 pr-4 text-xs font-medium text-neutral-500">Type</th>
                      <th className="text-left py-2 pr-4 text-xs font-medium text-neutral-500">Required</th>
                      <th className="text-left py-2 text-xs font-medium text-neutral-500">Description</th>
                    </tr>
                  </thead>
                  <tbody>
                    {endpoint.params.map((p) => (
                      <tr key={p.name} className="border-b border-neutral-100 last:border-b-0">
                        <td className="py-2 pr-4 font-mono text-xs text-neutral-800">{p.name}</td>
                        <td className="py-2 pr-4 text-xs text-neutral-500">{p.type}</td>
                        <td className="py-2 pr-4 text-xs">
                          {p.required ? (
                            <span className="text-red-600 font-medium">Yes</span>
                          ) : (
                            <span className="text-neutral-400">No</span>
                          )}
                        </td>
                        <td className="py-2 text-xs text-neutral-600">
                          {p.description}
                          {p.default && <span className="text-neutral-400 ml-1">(default: {p.default})</span>}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Curl Example */}
          <div className="px-4 pb-4">
            <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Example Request</h4>
            <div className="relative bg-neutral-900 rounded-lg p-4">
              <div className="absolute top-2 right-2">
                <CopyButton text={curl} />
              </div>
              <pre className="text-xs text-neutral-100 font-mono overflow-x-auto pr-8">{curl}</pre>
            </div>
          </div>

          {/* Example Response */}
          <div className="px-4 pb-4">
            <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Example Response</h4>
            <div className="relative bg-neutral-50 rounded-lg border border-neutral-200 p-4">
              <div className="absolute top-2 right-2">
                <CopyButton text={JSON.stringify(endpoint.exampleResponse, null, 2)} />
              </div>
              <JsonSyntax data={endpoint.exampleResponse} />
            </div>
          </div>

          {/* Try It */}
          <TryIt endpoint={endpoint} />
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Sidebar
// ---------------------------------------------------------------------------

function Sidebar({ groups, activeGroup, onSelect }: { groups: EndpointGroup[]; activeGroup: string; onSelect: (id: string) => void }) {
  return (
    <nav className="space-y-1">
      {groups.map((g) => (
        <button
          key={g.id}
          onClick={() => onSelect(g.id)}
          className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
            activeGroup === g.id
              ? 'bg-primary-50 text-primary-700 font-medium border border-primary-200'
              : 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900'
          }`}
        >
          {g.label}
          <span className="ml-1.5 text-xs text-neutral-400">({g.endpoints.length})</span>
        </button>
      ))}
      <button
        onClick={() => onSelect('rate-limiting')}
        className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
          activeGroup === 'rate-limiting'
            ? 'bg-primary-50 text-primary-700 font-medium border border-primary-200'
            : 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900'
        }`}
      >
        Rate Limiting
      </button>
    </nav>
  );
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function ApiDocs() {
  const [activeGroup, setActiveGroup] = useState(endpointGroups[0].id);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const sectionRefs = useRef<Record<string, HTMLDivElement | null>>({});

  const scrollToSection = useCallback((id: string) => {
    setActiveGroup(id);
    setMobileNavOpen(false);
    const el = sectionRefs.current[id];
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, []);

  // Track active section on scroll
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveGroup(entry.target.id);
          }
        }
      },
      { rootMargin: '-80px 0px -60% 0px', threshold: 0 },
    );

    const ids = [...endpointGroups.map((g) => g.id), 'rate-limiting'];
    ids.forEach((id) => {
      const el = sectionRefs.current[id];
      if (el) observer.observe(el);
    });

    return () => observer.disconnect();
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader title="Public API" subtitle="REST API for accessing blockchain data">
        <div className="flex items-center gap-2">
          <BookOpen className="w-4 h-4 text-neutral-500" />
          <span className="text-sm text-neutral-500">v1</span>
        </div>
      </PageHeader>

      {/* Base URL */}
      <div className="card p-4 sm:p-5">
        <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
          <span className="text-sm font-medium text-neutral-500 shrink-0">Base URL</span>
          <div className="flex items-center gap-2 flex-1 min-w-0">
            <code className="text-sm font-mono text-neutral-800 bg-neutral-100 px-3 py-1.5 rounded-lg border border-neutral-200 truncate">
              {BASE_URL}
            </code>
            <CopyButton text={BASE_URL} />
          </div>
        </div>
        <p className="text-sm text-neutral-500 mt-3 leading-relaxed">
          All endpoints return JSON. No authentication is required. Responses are paginated where applicable.
        </p>
      </div>

      {/* Mobile section selector */}
      <div className="lg:hidden">
        <button
          onClick={() => setMobileNavOpen(!mobileNavOpen)}
          className="w-full flex items-center justify-between px-4 py-3 card text-sm font-medium text-neutral-700"
        >
          <span>Sections: {endpointGroups.find((g) => g.id === activeGroup)?.label || 'Rate Limiting'}</span>
          <ChevronDown className={`w-4 h-4 transition-transform ${mobileNavOpen ? 'rotate-180' : ''}`} />
        </button>
        {mobileNavOpen && (
          <div className="card mt-1 p-2 overflow-hidden">
            <Sidebar groups={endpointGroups} activeGroup={activeGroup} onSelect={scrollToSection} />
          </div>
        )}
      </div>

      {/* Layout: sidebar + content */}
      <div className="flex gap-6">
        {/* Desktop Sidebar */}
        <div className="hidden lg:block w-52 shrink-0">
          <div className="sticky top-4">
            <div className="card p-3">
              <h3 className="text-xs font-semibold text-neutral-400 uppercase tracking-wider px-3 mb-2">Endpoints</h3>
              <Sidebar groups={endpointGroups} activeGroup={activeGroup} onSelect={scrollToSection} />
            </div>
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0 space-y-10">
          {endpointGroups.map((group) => (
            <div
              key={group.id}
              id={group.id}
              ref={(el) => { sectionRefs.current[group.id] = el; }}
            >
              <h2 className="text-lg font-semibold text-neutral-900 mb-4">{group.label}</h2>
              <div className="space-y-3">
                {group.endpoints.map((ep) => (
                  <EndpointCard key={ep.path} endpoint={ep} />
                ))}
              </div>
            </div>
          ))}

          {/* Rate Limiting Section */}
          <div
            id="rate-limiting"
            ref={(el) => { sectionRefs.current['rate-limiting'] = el; }}
          >
            <h2 className="text-lg font-semibold text-neutral-900 mb-4">Rate Limiting</h2>

            <div className="card p-5 space-y-4">
              <div className="flex items-start gap-3">
                <div className="p-2 rounded-lg bg-amber-50 border border-amber-200 shrink-0">
                  <AlertTriangle className="w-5 h-5 text-amber-600" />
                </div>
                <div>
                  <h3 className="font-semibold text-neutral-900 mb-1">Default Rate Limit</h3>
                  <p className="text-sm text-neutral-600 leading-relaxed">
                    The API enforces a rate limit of <strong>100 requests per minute</strong> per IP address.
                    If you exceed this limit, you will receive a <code className="text-xs font-mono bg-neutral-100 px-1.5 py-0.5 rounded border border-neutral-200">429 Too Many Requests</code> response.
                  </p>
                </div>
              </div>

              {/* Rate limit headers */}
              <div>
                <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Response Headers</h4>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-neutral-200">
                        <th className="text-left py-2 pr-4 text-xs font-medium text-neutral-500">Header</th>
                        <th className="text-left py-2 text-xs font-medium text-neutral-500">Description</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr className="border-b border-neutral-100">
                        <td className="py-2 pr-4 font-mono text-xs text-neutral-800">X-RateLimit-Limit</td>
                        <td className="py-2 text-xs text-neutral-600">Maximum requests allowed per window (e.g. 100).</td>
                      </tr>
                      <tr className="border-b border-neutral-100">
                        <td className="py-2 pr-4 font-mono text-xs text-neutral-800">X-RateLimit-Remaining</td>
                        <td className="py-2 text-xs text-neutral-600">Number of requests remaining in the current window.</td>
                      </tr>
                      <tr>
                        <td className="py-2 pr-4 font-mono text-xs text-neutral-800">X-RateLimit-Reset</td>
                        <td className="py-2 text-xs text-neutral-600">Unix timestamp (seconds) when the rate limit window resets.</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              {/* 429 example */}
              <div>
                <h4 className="text-xs font-semibold text-neutral-500 uppercase tracking-wider mb-2">Example 429 Response</h4>
                <div className="bg-neutral-50 rounded-lg border border-neutral-200 p-4">
                  <JsonSyntax
                    data={{
                      error: 'Too Many Requests',
                      message: 'Rate limit exceeded. Please wait before making more requests.',
                      retryAfter: 42,
                    }}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
