import { useQuery } from '@tanstack/react-query';

import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import type { TokenTransfer } from '../lib/api';
import { formatHash, formatTimeAgo } from '../lib/utils';
import { formatTokenValue } from '../lib/formatToken';
import { useTokenMap } from '../hooks/useTokenMap';
import { AddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';
import { PageHeader } from '../components/PageHeader';

import { ArrowRight } from 'lucide-react';

const ZERO = '0x0000000000000000000000000000000000000000';

// Token-standard filter tabs. `value` is the API `type` query param ('' = all).
const FILTERS: { label: string; value: string }[] = [
  { label: 'All', value: '' },
  { label: 'ERC-20', value: 'ERC20' },
  { label: 'ERC-721', value: 'ERC721' },
];

function methodOf(t: TokenTransfer): string {
  if (t.from === ZERO) return 'mint';
  if (t.to === ZERO) return 'burn';
  return 'transfer';
}

export default function TokenTransfers() {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = parseInt(searchParams.get('page') || '1', 10);
  const type = searchParams.get('type') || '';

  const { data, isLoading, error } = useQuery({
    queryKey: ['allTokenTransfers', page, type],
    queryFn: () => api.getAllTokenTransfers(page, 25, type || undefined),
  });

  const transfers = data?.data || [];
  const tokenMap = useTokenMap(transfers.map((t) => t.tokenAddress));
  const totalPages = data?.totalPages || 1;
  const total = data?.total ?? 0;

  const handlePageChange = (newPage: number) => {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(newPage));
    setSearchParams(params);
  };

  const handleFilterChange = (value: string) => {
    const params = new URLSearchParams();
    if (value) params.set('type', value);
    params.set('page', '1');
    setSearchParams(params);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Token Transfers"
        subtitle={
          total > 0
            ? `${total.toLocaleString()} transfer${total === 1 ? '' : 's'} across all tokens`
            : 'All ERC-20 and ERC-721 transfers on this network'
        }
      />

      {/* Token-standard filter */}
      <div className="flex items-center gap-1 border-b border-neutral-200">
        {FILTERS.map((f) => {
          const isActive = type === f.value;
          return (
            <button
              key={f.value || 'all'}
              onClick={() => handleFilterChange(f.value)}
              className={[
                'relative -mb-px px-4 py-2.5 text-sm font-medium transition-colors',
                isActive ? 'text-primary' : 'text-neutral-500 hover:text-neutral-800',
              ].join(' ')}
            >
              {f.label}
              {isActive && (
                <span className="absolute inset-x-3 -bottom-px h-0.5 rounded-full bg-primary" />
              )}
            </button>
          );
        })}
      </div>

      <div className="card">
        {isLoading ? (
          <div className="p-8 text-center text-neutral-400">Loading token transfers...</div>
        ) : error ? (
          <div className="p-8 text-center text-error-600">Error loading token transfers</div>
        ) : transfers.length === 0 ? (
          <div className="text-center text-neutral-400 py-8">No token transfers found</div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Tx Hash</th>
                    <th>Method</th>
                    <th>Block</th>
                    <th>From</th>
                    <th></th>
                    <th>To</th>
                    <th>Token</th>
                    <th className="text-right">Token ID</th>
                    <th className="text-right">Amount</th>
                  </tr>
                </thead>
                <tbody>
                  {transfers.map((transfer: TokenTransfer) => {
                    const fromReason = transfer.addressMetadata?.[transfer.from?.toLowerCase()];
                    const toReason = transfer.addressMetadata?.[transfer.to?.toLowerCase()];
                    const method = methodOf(transfer);
                    const isNft = transfer.tokenType === 'ERC721' || transfer.tokenType === 'ERC1155';
                    return (
                      <tr key={`${transfer.txHash}-${transfer.logIndex}`}>
                        <td>
                          <Link
                            to={`/tx/${transfer.txHash}`}
                            className="text-primary hover:text-primary-600 font-mono text-sm transition-colors"
                          >
                            {formatHash(transfer.txHash)}
                          </Link>
                          {transfer.timestamp && (
                            <div className="mt-0.5 text-xs text-neutral-400">
                              {formatTimeAgo(transfer.timestamp)}
                            </div>
                          )}
                        </td>
                        <td>
                          <MethodBadge method={method} />
                        </td>
                        <td>
                          <Link
                            to={`/block/${transfer.blockNumber}`}
                            className="text-primary hover:text-primary-600 transition-colors"
                          >
                            {transfer.blockNumber.toLocaleString()}
                          </Link>
                        </td>
                        <td>
                          <span className="inline-flex items-center gap-1">
                            <AddressLink address={transfer.from} chars={6} reason={fromReason} />
                            <AddressLabel reason={fromReason} />
                          </span>
                        </td>
                        <td className="text-center">
                          <ArrowRight className="w-4 h-4 text-neutral-400 inline-block" />
                        </td>
                        <td>
                          <span className="inline-flex items-center gap-1">
                            <AddressLink address={transfer.to} chars={6} reason={toReason} />
                            <AddressLabel reason={toReason} />
                          </span>
                        </td>
                        <td>
                          <div className="flex items-center gap-2">
                            <Link
                              to={`/token/${transfer.tokenAddress}`}
                              className="text-primary hover:text-primary-600 transition-colors font-mono text-sm"
                            >
                              {formatHash(transfer.tokenAddress, 6)}
                            </Link>
                            <span
                              className={`badge ${
                                isNft ? 'badge-primary' : 'badge-neutral'
                              }`}
                            >
                              {transfer.tokenType || 'ERC20'}
                            </span>
                          </div>
                        </td>
                        <td className="text-right font-mono text-sm text-neutral-700">
                          {isNft && transfer.tokenId ? (
                            <Link
                              to={`/nft/${transfer.tokenAddress}/${transfer.tokenId}`}
                              className="text-primary hover:text-primary-600 transition-colors"
                            >
                              #{transfer.tokenId}
                            </Link>
                          ) : (
                            <span className="text-neutral-300">—</span>
                          )}
                        </td>
                        <td className="text-right font-mono text-neutral-700">
                          {isNft
                            ? transfer.tokenId
                              ? '1'
                              : '—'
                            : formatTokenValue(
                                transfer.value,
                                tokenMap[transfer.tokenAddress.toLowerCase()]?.decimals ?? 18,
                              )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className="flex justify-between items-center px-4 py-3 border-t border-neutral-100">
              <span className="text-neutral-500 text-sm">
                Page {page} of {totalPages}
              </span>
              <div className="flex gap-2">
                <button
                  onClick={() => handlePageChange(page - 1)}
                  disabled={page <= 1}
                  className="btn-secondary text-sm"
                >
                  Previous
                </button>
                <button
                  onClick={() => handlePageChange(page + 1)}
                  disabled={page >= totalPages}
                  className="btn-secondary text-sm"
                >
                  Next
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function MethodBadge({ method }: { method: string }) {
  const tone =
    method === 'mint'
      ? 'bg-primary/10 text-primary'
      : method === 'burn'
      ? 'bg-error-50 text-error-700 dark:bg-error-500/10 dark:text-error-500'
      : 'bg-neutral-200/60 text-neutral-700';
  return (
    <span className={`inline-flex items-center rounded-md px-2 py-0.5 font-mono text-xs ${tone}`}>
      {method}
    </span>
  );
}
