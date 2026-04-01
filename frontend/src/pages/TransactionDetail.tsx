import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { api } from '../lib/api';
import type { TokenTransfer, Log, TxCategory } from '../lib/api';
import { formatWei, formatGas, formatTimestamp } from '../lib/utils';
import { formatTokenValue } from '../lib/formatToken';
import { useTokenMap } from '../hooks/useTokenMap';
import { AddressLink, TokenAddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';

import { PageHeader } from '../components/PageHeader';
import { CopyButton } from '../components/CopyButton';
import { decodeEvent, getMethodId, fetchEventSignature, decodeEventWithSignature, KNOWN_EVENTS } from '../lib/eventDecoder';
import type { DecodedEvent } from '../lib/eventDecoder';

// Category display configuration
const CATEGORY_CONFIG: Record<TxCategory, { label: string; className: string }> = {
  contract_creation: { label: 'Contract Creation', className: 'bg-purple-100 text-purple-700' },
  contract_call: { label: 'Contract Call', className: 'bg-blue-100 text-blue-700' },
  coin_transfer: { label: 'Coin Transfer', className: 'bg-green-100 text-green-700' },
  token_transfer: { label: 'Token Transfer', className: 'bg-orange-100 text-orange-700' },
  system_transaction: { label: 'System Transaction', className: 'bg-neutral-100 text-neutral-500' },
};

function TxCategoryBadges({ categories }: { categories?: TxCategory[] }) {
  if (!categories || categories.length === 0) {
    return <span className="text-neutral-400">-</span>;
  }

  return (
    <div className="flex flex-wrap gap-2">
      {categories.map((category) => {
        const config = CATEGORY_CONFIG[category];
        if (!config) return null;
        return (
          <span
            key={category}
            className={`inline-block text-xs px-2 py-0.5 rounded font-medium ${config.className}`}
          >
            {config.label}
          </span>
        );
      })}
    </div>
  );
}

export function TransactionDetail() {
  const { hash } = useParams<{ hash: string }>();
  const [activeTab, setActiveTab] = useState<'overview' | 'logs'>('overview');
  const [showMore, setShowMore] = useState(false);

  const { data: tx, isLoading, error } = useQuery({
    queryKey: ['transaction', hash],
    queryFn: () => api.getTransaction(hash!),
    enabled: !!hash,
  });

  const { data: transfers } = useQuery({
    queryKey: ['transaction-transfers', hash],
    queryFn: () => api.getTransactionTransfers(hash!),
    enabled: !!hash,
  });

  const { data: logs } = useQuery({
    queryKey: ['transaction-logs', hash],
    queryFn: () => api.getTransactionLogs(hash!),
    enabled: !!hash,
  });

  const tokenMap = useTokenMap((transfers || []).map(t => t.tokenAddress));

  // Batch check logic removed; visibility metadata is now attached directly to API responses

  if (isLoading) return <div className="text-neutral-400">Loading...</div>;
  if (error && error instanceof Error && (error.message.includes('403') || error.message.includes('500'))) {
    return (
      <div className="space-y-6">
        <PageHeader title="Transaction" />
        <div className="card">
          <div className="flex flex-col items-center justify-center py-16 space-y-4">
            <div className="w-16 h-16 rounded-full bg-neutral-100 flex items-center justify-center border border-neutral-200">
              <svg className="w-8 h-8 text-neutral-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v.01M12 12a1.5 1.5 0 001.5-1.5c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5M19.5 12c0 4.14-3.36 7.5-7.5 7.5S4.5 16.14 4.5 12 7.86 4.5 12 4.5s7.5 3.36 7.5 7.5z" /></svg>
            </div>
            <h2 className="text-xl font-semibold text-neutral-900">Transaction Restricted</h2>
            <p className="text-neutral-500 text-center max-w-md text-sm">
              This transaction involves private addresses. Sign in to view it if you are a participant.
            </p>
          </div>
        </div>
      </div>
    );
  }
  if (error || !tx) return <div className="text-error-600">Transaction not found</div>;

  const fromReason = tx.addressMetadata?.[tx.from?.toLowerCase()];
  const toReason = tx.to ? tx.addressMetadata?.[tx.to.toLowerCase()] : undefined;

  const hasLogs = logs && logs.length > 0;

  return (
    <div className="space-y-6">
      <PageHeader title="Transaction Details" />

      {/* Tabs - only show when logs exist */}
      {hasLogs && (
        <div className="tabs">
          <button
            onClick={() => setActiveTab('overview')}
            className={activeTab === 'overview' ? 'tab-active' : 'tab'}
          >
            Overview
          </button>
          <button
            onClick={() => setActiveTab('logs')}
            className={activeTab === 'logs' ? 'tab-active' : 'tab'}
          >
            Logs ({logs?.length || 0})
          </button>
        </div>
      )}

      {/* Overview Tab */}
      {activeTab === 'overview' && (
        <div className="card">
          <div className="divide-y divide-neutral-100">
            <InfoRow
              label="Transaction Hash"
              value={
                <span className="flex items-center gap-1">
                  <span className="font-mono text-sm break-all text-neutral-900">{tx.hash}</span>
                  <CopyButton text={tx.hash} />
                </span>
              }
            />
            <InfoRow
              label="Status"
              value={
                <div className="flex items-center gap-2 flex-wrap">
                  <span className={`badge ${tx.status === 1 ? 'badge-success' : 'badge-error'}`}>
                    {tx.status === 1 ? 'Success' : 'Failed'}
                  </span>
                  {tx.status !== 1 && tx.error && (
                    <span className="text-sm text-error-600">{tx.error}</span>
                  )}
                  {tx.status !== 1 && tx.revertReason && (
                    <span className="text-sm text-error-600 font-mono">Revert: {tx.revertReason}</span>
                  )}
                </div>
              }
            />
            {tx.txCategories && tx.txCategories.length > 0 && (
              <InfoRow
                label="Transaction Action"
                value={<TxCategoryBadges categories={tx.txCategories} />}
              />
            )}
            <InfoRow
              label="Block"
              value={
                <Link to={`/block/${tx.blockNumber}`} className="text-primary hover:text-primary-600 transition-colors">
                  {tx.blockNumber}
                </Link>
              }
            />
            {tx.blockTimestamp && (
              <InfoRow label="Timestamp" value={formatTimestamp(tx.blockTimestamp)} />
            )}
            <InfoRow
              label="From"
              value={
                <span className="flex items-center gap-1">
                  <AddressLink address={tx.from} full className="text-sm" reason={fromReason} />
                  <AddressLabel reason={fromReason} />
                  <CopyButton text={tx.from} />
                </span>
              }
            />
            <InfoRow
              label="To"
              value={
                tx.to ? (
                  <span className="flex items-center gap-1">
                    <AddressLink address={tx.to} full className="text-sm" reason={toReason} />
                    <AddressLabel reason={toReason} />
                    {tx.to !== '[PRIVATE]' && <CopyButton text={tx.to} />}
                  </span>
                ) : tx.contractAddress ? (
                  <span className="flex items-center gap-1 text-sm">
                    <span className="text-neutral-500">[Contract </span>
                    <AddressLink address={tx.contractAddress} full />
                    <span className="text-neutral-500"> Created]</span>
                    <CopyButton text={tx.contractAddress} />
                  </span>
                ) : (
                  <span className="text-neutral-500">Contract Creation</span>
                )
              }
            />

            {/* ERC-20 Tokens Transferred - Etherscan style, inline with other fields */}
            {transfers && transfers.length > 0 && (
              <div className="info-row">
                <div className="info-label flex items-start gap-1">
                  <span>ERC-20 Tokens Transferred</span>
                  <span className="text-xs text-neutral-400">({transfers.length})</span>
                </div>
                <div className="flex flex-col gap-2">
                  {transfers.map((transfer) => {
                    const tokenInfo = tokenMap[transfer.tokenAddress.toLowerCase()];
                    return (
                      <TokenTransferRow
                        key={`${transfer.txHash}-${transfer.logIndex}`}
                        transfer={transfer}
                        tokenDecimals={tokenInfo?.decimals}
                        tokenSymbol={tokenInfo?.symbol}
                      />
                    );
                  })}
                </div>
              </div>
            )}

            <InfoRow
              label="Value"
              value={
                tx.value === '' || tx.value == null
                  ? <span className="text-neutral-400 italic">hidden</span>
                  : `${formatWei(tx.value)} ETH`
              }
            />
            <InfoRow
              label="Transaction Fee"
              value={`${formatWei((BigInt(tx.gasUsed) * BigInt(tx.gasPrice)).toString())} ETH`}
            />
            <InfoRow label="Gas Price" value={`${tx.gasPrice} wei`} />
          </div>

          {/* Show More / Show Less toggle */}
          <button
            onClick={() => setShowMore(!showMore)}
            className="w-full px-4 py-3 flex items-center justify-center gap-2 text-sm text-neutral-500 hover:text-neutral-700 hover:bg-neutral-50 transition-colors border-t border-neutral-100"
          >
            {showMore ? (
              <>
                <ChevronUp className="w-4 h-4" />
                Show Less
              </>
            ) : (
              <>
                <ChevronDown className="w-4 h-4" />
                Show More
              </>
            )}
          </button>

          {showMore && (
            <div className="divide-y divide-neutral-100 border-t border-neutral-100">
              <InfoRow
                label="Transaction Type"
                value={
                  <span className="font-mono text-sm">
                    {tx.txType === 0 ? '0 (Legacy)' :
                     tx.txType === 1 ? '1 (Access List)' :
                     tx.txType === 2 ? '2 (EIP-1559)' :
                     tx.txType === 126 ? '126 (System Deposit)' :
                     tx.txType ?? '-'}
                  </span>
                }
              />
              <InfoRow
                label="Nonce"
                value={
                  tx.nonce == null
                    ? <span className="text-neutral-400 italic">hidden</span>
                    : <span className="font-mono text-sm">{tx.nonce}</span>
                }
              />
              <InfoRow
                label="Position in Block"
                value={<span className="font-mono text-sm">{tx.txIndex}</span>}
              />
              <InfoRow label="Gas Limit" value={tx.gasLimit != null ? formatGas(tx.gasLimit) : '-'} />
              <InfoRow label="Gas Used" value={formatGas(tx.gasUsed)} />
              {tx.maxFeePerGas != null && (
                <InfoRow label="Max Fee Per Gas" value={`${tx.maxFeePerGas} wei`} />
              )}
              {tx.maxPriorityFeePerGas != null && (
                <InfoRow label="Max Priority Fee Per Gas" value={`${tx.maxPriorityFeePerGas} wei`} />
              )}
            </div>
          )}

          {/* Input Data */}
          {tx.inputData && tx.inputData !== '' && (
            <div className="border-t border-neutral-100 px-4 py-3">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-neutral-500">Input Data</span>
                  <span className="text-neutral-400 text-xs">({tx.inputData.length / 2} bytes)</span>
                </div>
                <button
                  onClick={() => navigator.clipboard.writeText('0x' + tx.inputData)}
                  className="text-xs text-neutral-500 hover:text-neutral-700 bg-neutral-100 hover:bg-neutral-200 px-2 py-1 rounded transition-colors"
                >
                  Copy
                </button>
              </div>
              <div className="code-block p-2 text-xs max-h-64 overflow-y-auto">
                <div className="break-all whitespace-pre-wrap">0x{tx.inputData}</div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Logs Tab */}
      {activeTab === 'logs' && logs && (
        <div className="space-y-4">
          {logs.map((log) => (
            <LogCard key={`${log.txHash}-${log.logIndex}`} log={log} />
          ))}
        </div>
      )}
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="info-row">
      <div className="info-label">{label}</div>
      <div className="info-value">{value}</div>
    </div>
  );
}

function TokenTransferRow({ transfer, tokenDecimals, tokenSymbol }: { transfer: TokenTransfer; tokenDecimals?: number; tokenSymbol?: string }) {
  const formattedValue = formatTokenValue(transfer.value, tokenDecimals ?? 18);
  const fromReason = transfer.addressMetadata?.[transfer.from?.toLowerCase()];
  const toReason = transfer.addressMetadata?.[transfer.to?.toLowerCase()];

  return (
    <div className="flex items-center gap-1.5 flex-wrap text-sm">
      <span className="text-neutral-500">From</span>
      <AddressLink address={transfer.from} reason={fromReason} />
      <AddressLabel reason={fromReason} />
      <span className="text-neutral-500">To</span>
      <AddressLink address={transfer.to} reason={toReason} />
      <AddressLabel reason={toReason} />
      <span className="text-neutral-500">For</span>
      <span className="font-mono text-success-600 font-medium">{formattedValue}{tokenSymbol ? ` ${tokenSymbol}` : ''}</span>
      <TokenAddressLink address={transfer.tokenAddress} />
    </div>
  );
}

type TopicDecodeFormat = 'hex' | 'address' | 'number' | 'text';

// Decode a 32-byte hex topic to different formats
function decodeTopic(hex: string, format: TopicDecodeFormat): string {
  // Remove 0x prefix if present
  const cleanHex = hex.startsWith('0x') ? hex.slice(2) : hex;

  switch (format) {
    case 'hex':
      return hex;
    case 'address': {
      // Address is last 20 bytes (40 hex chars), with 0x prefix
      const addressHex = cleanHex.slice(-40);
      return `0x${addressHex}`;
    }
    case 'number':
      try {
        const num = BigInt('0x' + cleanHex);
        return num.toString();
      } catch {
        return 'Invalid number';
      }
    case 'text':
      try {
        // Convert hex to bytes, then decode as UTF-8
        const bytes = [];
        for (let i = 0; i < cleanHex.length; i += 2) {
          const byte = parseInt(cleanHex.slice(i, i + 2), 16);
          if (byte !== 0) bytes.push(byte); // Skip null bytes
        }
        const text = new TextDecoder('utf-8', { fatal: false }).decode(new Uint8Array(bytes));
        // Check if result is printable
        if (/^[\x20-\x7E\s]*$/.test(text) && text.trim().length > 0) {
          return text.trim();
        }
        return '(non-printable)';
      } catch {
        return '(decode error)';
      }
    default:
      return hex;
  }
}

function TopicRow({ index, value }: { index: number; value: string }) {
  const [format, setFormat] = useState<TopicDecodeFormat>('hex');

  // Topic 0 is always the event signature hash - no decode options
  const showDecodeOptions = index > 0;
  const decodedValue = showDecodeOptions ? decodeTopic(value, format) : value;

  // Check if decoded address looks valid (for linking)
  const isAddress = format === 'address' && decodedValue.startsWith('0x') && decodedValue.length === 42;

  return (
    <div className="flex items-start gap-2">
      <span className="text-neutral-400 text-xs font-mono w-4 shrink-0">[{index}]</span>
      <div className="flex-1 min-w-0">
        {isAddress ? (
          <AddressLink address={decodedValue} full className="text-xs" />
        ) : (
          <span className="font-mono text-xs text-neutral-700 break-all">{decodedValue}</span>
        )}
      </div>
      {showDecodeOptions && (
        <select
          value={format}
          onChange={(e) => setFormat(e.target.value as TopicDecodeFormat)}
          className="text-xs bg-neutral-100 border border-neutral-200 rounded px-1.5 py-0.5 text-neutral-600 hover:bg-neutral-200 transition-colors cursor-pointer shrink-0"
        >
          <option value="hex">Hex</option>
          <option value="address">Address</option>
          <option value="number">Number</option>
          <option value="text">Text</option>
        </select>
      )}
    </div>
  );
}

function DecodedEventSection({ decoded, methodId }: { decoded: DecodedEvent; methodId: string }) {
  return (
    <div className="bg-primary-50 border border-primary-200 rounded-lg p-3 space-y-2">
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-primary-700 bg-primary-100 px-2 py-0.5 rounded">
          Decoded
        </span>
        <span className="text-xs text-neutral-500">Method ID: {methodId}</span>
      </div>

      <div className="font-mono text-sm text-neutral-800">
        <span className="text-primary-600 font-semibold">{decoded.name}</span>
        <span className="text-neutral-500">(</span>
        {decoded.params.map((param, i) => (
          <span key={param.name}>
            {i > 0 && <span className="text-neutral-400">, </span>}
            <span className="text-neutral-500">{param.type}</span>
            {param.indexed && <span className="text-warning-600 text-xs ml-0.5">indexed</span>}
            <span className="text-neutral-600"> {param.name}</span>
          </span>
        ))}
        <span className="text-neutral-500">)</span>
      </div>

      <div className="space-y-1 pt-1 border-t border-primary-200">
        {decoded.params.map((param) => (
          <div key={param.name} className="flex items-start gap-2 text-sm">
            <span className="text-neutral-500 min-w-[80px] shrink-0">{param.name}:</span>
            {param.type === 'address' && param.value ? (
              <AddressLink address={param.value} full className="text-xs" />
            ) : (
              <span className="font-mono text-xs text-neutral-700 break-all">
                {param.value || '(empty)'}
              </span>
            )}
            {param.indexed && (
              <span className="text-xs text-warning-600 bg-warning-50 px-1 rounded shrink-0">indexed</span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function LogCard({ log }: { log: Log }) {
  const topics = [
    { index: 0, value: log.topic0 },
    { index: 1, value: log.topic1 },
    { index: 2, value: log.topic2 },
    { index: 3, value: log.topic3 },
  ].filter((t) => t.value !== null) as { index: number; value: string }[];

  // Try to decode the event from local database first
  const localDecoded = decodeEvent(log.topic0, log.topic1, log.topic2, log.topic3, log.data || '');
  const methodId = log.topic0 ? getMethodId(log.topic0) : '';

  // Check if we need to fetch from 4byte.directory
  const needsFetch = !localDecoded && !!log.topic0 && !KNOWN_EVENTS[log.topic0.toLowerCase()];

  // Fetch from 4byte.directory for unknown events
  const { data: fetchedSignature, isLoading: isFetchingSignature } = useQuery({
    queryKey: ['event-signature', log.topic0],
    queryFn: () => fetchEventSignature(log.topic0!, topics.length),
    enabled: needsFetch,
    staleTime: Infinity, // Signatures don't change
    retry: 1,
  });

  // Decode with fetched signature if available
  const decoded = localDecoded || (fetchedSignature && fetchedSignature.name
    ? decodeEventWithSignature(fetchedSignature, log.topic1, log.topic2, log.topic3, log.data || '')
    : null);

  return (
    <div className="card p-4 space-y-3">
      <div className="flex items-center gap-2">
        <span className="badge badge-neutral font-mono">
          {log.logIndex}
        </span>
        <span className="text-neutral-500 text-sm">Log Index</span>
      </div>

      <div className="space-y-3">
        <div className="flex flex-col sm:flex-row gap-1 sm:gap-2">
          <span className="text-neutral-500 text-sm w-20 shrink-0">Address</span>
          <span className="flex items-center gap-1">
            <AddressLink address={log.address} full className="text-sm" reason={log.addressMetadata?.[log.address?.toLowerCase()]} />
            <AddressLabel reason={log.addressMetadata?.[log.address?.toLowerCase()]} />
          </span>
        </div>

        {/* Decoded Event Section - shown when event is recognized */}
        {decoded && (
          <DecodedEventSection decoded={decoded} methodId={methodId} />
        )}

        {/* Loading indicator while fetching signature */}
        {isFetchingSignature && (
          <div className="bg-neutral-50 border border-neutral-200 rounded-lg p-3 flex items-center gap-2">
            <div className="w-4 h-4 border-2 border-neutral-300 border-t-primary-500 rounded-full animate-spin" />
            <span className="text-xs text-neutral-500">Looking up event signature...</span>
          </div>
        )}

        {topics.length > 0 && (
          <div className="flex flex-col gap-1">
            <span className="text-neutral-500 text-sm">Topics</span>
            <div className="space-y-2 pl-2">
              {topics.map((topic) => (
                <TopicRow key={topic.index} index={topic.index} value={topic.value} />
              ))}
            </div>
          </div>
        )}

        {log.data && log.data !== '' && (
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <span className="text-neutral-500 text-sm">Data</span>
              <span className="text-neutral-400 text-xs">({log.data.length / 2} bytes)</span>
            </div>
            <div className="code-block p-2 text-xs max-h-64 overflow-y-auto">
              <div className="break-all whitespace-pre-wrap">0x{log.data}</div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
