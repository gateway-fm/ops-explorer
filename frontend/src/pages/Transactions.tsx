import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { api } from '../lib/api';
import type { Transaction, TxCategory, AddressVisibility } from '../lib/api';
import { formatHash, formatWei } from '../lib/utils';
import { PageHeader } from '../components/PageHeader';
import { AddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';
import { useBatchAddressVisibility } from '../hooks/useAddressVisibility';

export function Transactions() {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = parseInt(searchParams.get('page') || '1', 10);
  const pageSize = 25;

  const { data, isLoading, error } = useQuery({
    queryKey: ['transactions', page, pageSize],
    queryFn: () => api.getTransactionsPaginated(page, pageSize),
    refetchInterval: page === 1 ? 5000 : false, // Only auto-refresh on first page
  });

  const uniqueAddresses = useMemo(() => {
    if (!data?.data) return [];
    const set = new Set<string>();
    for (const tx of data.data) {
      if (tx.from && tx.from !== '[PRIVATE]') set.add(tx.from.toLowerCase());
      if (tx.to && tx.to !== '[PRIVATE]') set.add(tx.to.toLowerCase());
    }
    return Array.from(set);
  }, [data]);

  const { visibilities } = useBatchAddressVisibility(uniqueAddresses);

  const goToPage = (newPage: number) => {
    setSearchParams({ page: String(newPage) });
  };

  if (error) return <div className="text-error-600">Error loading transactions</div>;

  return (
    <div className="space-y-6">
      <PageHeader title="Transactions">
        {data && (
          <span className="text-sm text-neutral-500">
            {data.total.toLocaleString()} transactions
          </span>
        )}
      </PageHeader>

      <div className="card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="table">
            <thead>
              <tr>
                <th>Tx Hash</th>
                <th>Type</th>
                <th>Block</th>
                <th>From</th>
                <th>To</th>
                <th>Value</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {data?.data?.map((tx) => (
                <TxTableRow key={tx.hash} tx={tx} visibilities={visibilities} />
              ))}
            </tbody>
          </table>
        </div>

        {isLoading && (
          <div className="px-4 py-8 text-center text-neutral-400">Loading...</div>
        )}

        {!isLoading && !data?.data?.length && (
          <div className="px-4 py-8 text-center text-neutral-400">No transactions yet</div>
        )}

        {/* Pagination */}
        {data && data.totalPages > 1 && (
          <div className="px-4 py-3 border-t border-neutral-100 flex flex-col sm:flex-row items-center justify-between gap-3">
            <div className="text-sm text-neutral-500">
              Page {data.page} of {data.totalPages}
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => goToPage(page - 1)}
                disabled={page <= 1}
                className="p-2 rounded-lg border border-neutral-200 bg-neutral-50 hover:bg-neutral-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft className="w-4 h-4 text-neutral-500" />
              </button>

              {/* Page numbers */}
              <div className="flex items-center gap-1">
                {Array.from({ length: Math.min(5, data.totalPages) }, (_, i) => {
                  let pageNum: number;
                  if (data.totalPages <= 5) {
                    pageNum = i + 1;
                  } else if (page <= 3) {
                    pageNum = i + 1;
                  } else if (page >= data.totalPages - 2) {
                    pageNum = data.totalPages - 4 + i;
                  } else {
                    pageNum = page - 2 + i;
                  }

                  return (
                    <button
                      key={pageNum}
                      onClick={() => goToPage(pageNum)}
                      className={`px-3 py-1 rounded-lg text-sm transition-colors ${
                        pageNum === page
                          ? 'bg-primary text-white'
                          : 'hover:bg-neutral-100 text-neutral-600'
                      }`}
                    >
                      {pageNum}
                    </button>
                  );
                })}
              </div>

              <button
                onClick={() => goToPage(page + 1)}
                disabled={page >= data.totalPages}
                className="p-2 rounded-lg border border-neutral-200 bg-neutral-50 hover:bg-neutral-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronRight className="w-4 h-4 text-neutral-500" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// Category display configuration
const CATEGORY_CONFIG: Record<TxCategory, { label: string; className: string }> = {
  contract_creation: { label: 'Contract Creation', className: 'bg-purple-100 text-purple-700' },
  contract_call: { label: 'Contract Call', className: 'bg-blue-100 text-blue-700' },
  coin_transfer: { label: 'Coin Transfer', className: 'bg-green-100 text-green-700' },
  token_transfer: { label: 'Token Transfer', className: 'bg-orange-100 text-orange-700' },
};

function TxCategoryBadge({ category }: { category: TxCategory }) {
  const config = CATEGORY_CONFIG[category];
  if (!config) return null;

  return (
    <span className={`inline-block text-xs px-2 py-0.5 rounded font-medium ${config.className}`}>
      {config.label}
    </span>
  );
}

function TxTypeCell({ categories, tokenTransferCount }: { categories?: TxCategory[]; tokenTransferCount?: number }) {
  if (!categories || categories.length === 0) {
    return <span className="text-neutral-400 text-xs">-</span>;
  }

  // Show the most important category (first one) and indicate if there are more
  const primaryCategory = categories[0];
  const showTokenCount = primaryCategory === 'token_transfer' && tokenTransferCount && tokenTransferCount > 1;

  return (
    <div className="flex flex-wrap gap-1">
      <TxCategoryBadge category={primaryCategory} />
      {showTokenCount && (
        <span className="text-xs text-neutral-500">×{tokenTransferCount}</span>
      )}
    </div>
  );
}

function TxTableRow({ tx, visibilities }: { tx: Transaction; visibilities: Record<string, AddressVisibility> }) {
  const fromVis = visibilities[tx.from?.toLowerCase()];
  const toVis = tx.to ? visibilities[tx.to.toLowerCase()] : undefined;

  return (
    <tr>
      <td>
        <Link to={`/tx/${tx.hash}`} className="font-mono text-sm text-primary hover:text-primary-600 transition-colors">
          {formatHash(tx.hash, 8)}
        </Link>
      </td>
      <td>
        <TxTypeCell categories={tx.txCategories} tokenTransferCount={tx.tokenTransferCount} />
      </td>
      <td>
        <Link to={`/block/${tx.blockNumber}`} className="text-primary hover:text-primary-600 transition-colors">
          {tx.blockNumber}
        </Link>
      </td>
      <td className="text-sm">
        <span className="inline-flex items-center gap-1">
          <AddressLink address={tx.from} chars={6} />
          <AddressLabel reason={fromVis?.reason} />
        </span>
      </td>
      <td className="text-sm">
        {tx.to ? (
          <span className="inline-flex items-center gap-1">
            <AddressLink address={tx.to} chars={6} />
            <AddressLabel reason={toVis?.reason} />
          </span>
        ) : (
          <span className="text-neutral-400">Contract</span>
        )}
      </td>
      <td className="text-sm text-neutral-700">
        {tx.value === '' || tx.value == null
          ? <span className="text-neutral-400 italic">hidden</span>
          : `${formatWei(tx.value)} ETH`}
      </td>
      <td>
        <span className={`badge ${tx.status === 1 ? 'badge-success' : 'badge-error'}`}>
          {tx.status === 1 ? 'Success' : 'Failed'}
        </span>
      </td>
    </tr>
  );
}
