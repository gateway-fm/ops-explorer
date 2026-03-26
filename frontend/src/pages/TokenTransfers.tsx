import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import type { TokenTransfer } from '../lib/api';
import { AddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';
import { PageHeader } from '../components/PageHeader';
import { useBatchAddressVisibility } from '../hooks/useAddressVisibility';
import { ArrowRight } from 'lucide-react';

function formatTokenValue(value: string | number, decimals = 18): string {
  const strValue = String(value);
  if (!strValue || strValue === '0') return '0';
  try {
    const num = BigInt(strValue);
    const divisor = BigInt(10 ** decimals);
    const wholePart = num / divisor;
    const fracPart = num % divisor;
    if (fracPart === BigInt(0)) {
      return wholePart.toLocaleString();
    }
    const fracStr = fracPart.toString().padStart(decimals, '0').slice(0, 4);
    return `${wholePart.toLocaleString()}.${fracStr}`;
  } catch {
    return strValue;
  }
}

function truncateHash(hash: string, chars = 8): string {
  if (!hash) return '';
  return `${hash.slice(0, chars + 2)}...${hash.slice(-chars)}`;
}

export default function TokenTransfers() {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = parseInt(searchParams.get('page') || '1', 10);

  const { data, isLoading, error } = useQuery({
    queryKey: ['allTokenTransfers', page],
    queryFn: () => api.getAllTokenTransfers(page, 25),
  });

  const uniqueAddresses = useMemo(() => {
    if (!data?.data) return [];
    const set = new Set<string>();
    for (const t of data.data) {
      if (t.from && t.from !== '[PRIVATE]') set.add(t.from.toLowerCase());
      if (t.to && t.to !== '[PRIVATE]') set.add(t.to.toLowerCase());
    }
    return Array.from(set);
  }, [data]);

  const { visibilities } = useBatchAddressVisibility(uniqueAddresses);

  const handlePageChange = (newPage: number) => {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(newPage));
    setSearchParams(params);
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <PageHeader title="Token Transfers" />
        <div className="card p-8 text-center text-neutral-400">
          Loading token transfers...
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Token Transfers" />
        <div className="card p-8 text-center text-error-600">
          Error loading token transfers
        </div>
      </div>
    );
  }

  const transfers = data?.data || [];
  const totalPages = data?.totalPages || 1;

  return (
    <div className="space-y-6">
      <PageHeader title="Token Transfers" />

      <div className="card">
        {transfers.length === 0 ? (
          <div className="text-center text-neutral-400 py-8">
            No token transfers found
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Tx Hash</th>
                    <th>Block</th>
                    <th>From</th>
                    <th></th>
                    <th>To</th>
                    <th>Token</th>
                    <th className="text-right">Value</th>
                  </tr>
                </thead>
                <tbody>
                  {transfers.map((transfer: TokenTransfer) => {
                    const fromVis = visibilities[transfer.from?.toLowerCase()];
                    const toVis = visibilities[transfer.to?.toLowerCase()];
                    return (
                    <tr key={`${transfer.txHash}-${transfer.logIndex}`}>
                      <td>
                        <Link
                          to={`/tx/${transfer.txHash}`}
                          className="text-primary hover:text-primary-600 font-mono text-sm transition-colors"
                        >
                          {truncateHash(transfer.txHash)}
                        </Link>
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
                          <AddressLink address={transfer.from} chars={6} visibility={fromVis} />
                          <AddressLabel reason={fromVis?.reason} visibility={fromVis} />
                        </span>
                      </td>
                      <td className="text-center">
                        <ArrowRight className="w-4 h-4 text-neutral-400 inline-block" />
                      </td>
                      <td>
                        <span className="inline-flex items-center gap-1">
                          <AddressLink address={transfer.to} chars={6} visibility={toVis} />
                          <AddressLabel reason={toVis?.reason} visibility={toVis} />
                        </span>
                      </td>
                      <td>
                        <div className="flex items-center gap-2">
                          <Link
                            to={`/token/${transfer.tokenAddress}`}
                            className="text-primary hover:text-primary-600 transition-colors font-mono text-sm"
                          >
                            {truncateHash(transfer.tokenAddress, 6)}
                          </Link>
                          <span className={`badge ${
                            transfer.tokenType === 'ERC721'
                              ? 'badge-primary'
                              : 'badge-neutral'
                          }`}>
                            {transfer.tokenType || 'ERC20'}
                          </span>
                        </div>
                      </td>
                      <td className="text-right font-mono text-neutral-700">
                        {transfer.tokenType === 'ERC721' && transfer.tokenId
                          ? `#${transfer.tokenId}`
                          : formatTokenValue(transfer.value)}
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
