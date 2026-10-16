import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, useCallback, useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { api } from '../lib/api';
import type { OffsetPaginatedResponse, Transaction, TxCategory } from '../lib/api';
import { formatHash, formatWei, getNetworkCurrency } from '../lib/utils';
import { PageHeader } from '../components/PageHeader';
import { AddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';
import { NewItemsNotice } from '../components/NewItemsNotice';
import { StateMessage } from '../components/StateMessage';
import { useAuth } from '../lib/auth';
import { useImpersonation } from '../hooks/useImpersonation';
import { useDisclosedTransactions } from '../hooks/useDisclosedTransactions';
import { grantLink, grantWindow, loadGrantBoundary, mergeTransactions } from './Transactions.disclosure';
import type { DisclosedRow } from './Transactions.disclosure';

export function Transactions() {
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedPage = Number(searchParams.get('page') || '1');
  const page = Number.isSafeInteger(requestedPage) && requestedPage > 0 ? requestedPage : 1;
  const pageSize = 25;
  const queryClient = useQueryClient();
  const { isAuthenticated, isLoading: authLoading, auth } = useAuth();
  const { isActive: viewingAs, token } = useImpersonation();
  const viewer = isAuthenticated ? auth.did : null;
  const compose = !authLoading && isAuthenticated && !viewingAs && !searchParams.has('as');

  // Main transactions query — no auto-refetch
  const { data, isLoading, error } = useQuery({
    queryKey: ['transactions', page, pageSize, viewer, token],
    queryFn: ({ signal }) => api.getTransactionsPaginated(page, pageSize, signal),
  });

  const lastPage = data ? Math.max(1, data.totalPages) : undefined;
  useEffect(() => {
    if (lastPage !== undefined && page > lastPage) {
      setSearchParams((current) => { const next = new URLSearchParams(current); next.set('page', String(lastPage)); return next; }, { replace: true });
    }
  }, [lastPage, page, setSearchParams]);

  const window = data && page <= lastPage!
    ? grantWindow(data.data ?? [], undefined, page === lastPage)
    : undefined;
  const disclosed = useDisclosedTransactions(compose, viewer, page, window, page > 1 ? {
    page: page - 1,
    load: (beforePage, signal) => loadGrantBoundary(beforePage, async (previousPage, signal) => {
      const key = ['transactions', previousPage, pageSize, viewer, token];
      // Reuse the ordinary pages already displayed in this viewer's scope.
      // The lookup only runs when grants actually need a page boundary.
      const cached = queryClient.getQueryData<OffsetPaginatedResponse<Transaction>>(key);
      if (cached) return cached.data ?? [];
      const response = await api.getTransactionsPaginated(previousPage, pageSize, signal);
      signal.throwIfAborted();
      queryClient.setQueryData(key, response);
      return response.data ?? [];
    }, signal),
  } : undefined);
  const rows = mergeTransactions(data?.data ?? [], disclosed.rows);
  const disclosedCount = rows.filter((row) => row.kind === 'disclosed').length;

  // Separate lightweight poll of page 1 to detect new transactions.
  // We can't share the main query key (we want the table to stay static
  // until the user clicks "load new"), so we use a distinct key and the
  // same endpoint. Snapshotting the top hash and counting entries above
  // it in the live window works under both the privacy proxy (where
  // /stats.totalTransactions is 0 because it isn't user-scoped) and the
  // indexer provider (where /transactions.total is page-local), since
  // it only relies on the ordered list itself.
  const { data: livePage } = useQuery({
    queryKey: ['transactions-live', pageSize, viewer, token],
    queryFn: ({ signal }) => api.getTransactionsPaginated(1, pageSize, signal),
    refetchInterval: page === 1 ? 3000 : false,
    enabled: page === 1,
  });

  // Snapshot the top tx hash at the moment the table first shows results.
  const scope = JSON.stringify([viewer, token]);
  const [snapshot, setSnapshot] = useState<{ scope: string; hash: string } | null>(null);
  // A new account or View-as session needs its own baseline immediately,
  // before the effect runs, so its existing rows are not counted as new.
  const snapshotTopHash = snapshot?.scope === scope ? snapshot.hash : null;
  useEffect(() => {
    if (page === 1 && data?.data?.length && snapshotTopHash === null) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time snapshot on initial data load
      setSnapshot({ scope, hash: data.data[0].hash });
    }
  }, [page, data, snapshotTopHash, scope]);

  // Count entries in the live window above the snapshot. If the snapshot
  // has fallen off the visible page, we know there are at least pageSize
  // new entries — we surface that as "N+" in the banner.
  const { newTxCount, newTxApproximate } = (() => {
    if (page !== 1 || !snapshotTopHash || !livePage?.data?.length) {
      return { newTxCount: 0, newTxApproximate: false };
    }
    const idx = livePage.data.findIndex((tx) => tx.hash === snapshotTopHash);
    if (idx === -1) return { newTxCount: livePage.data.length, newTxApproximate: true };
    return { newTxCount: idx, newTxApproximate: false };
  })();

  const handleLoadNew = useCallback(() => {
    setSnapshot(null);
    queryClient.invalidateQueries({ queryKey: ['transactions', page, pageSize] });
    queryClient.invalidateQueries({ queryKey: ['disclosed-transactions', viewer, page] });
  }, [queryClient, page, pageSize, viewer]);

  const goToPage = (newPage: number) => {
    setSnapshot(null);
    setSearchParams((previous) => { const next = new URLSearchParams(previous); next.set('page', String(newPage)); return next; });
  };

  if (error) return <StateMessage variant="error" title="Error loading transactions" />;

  return (
    <div className="space-y-6">
      <PageHeader title="Transactions">
        {data && (
          <span className="text-sm text-neutral-500" data-testid="tx-total-count">
            {data.total.toLocaleString()} {compose ? 'regular transactions' : 'transactions'}
          </span>
        )}
        {compose && <span className="text-sm text-neutral-500" data-testid="disclosed-count">{disclosedCount} disclosed shown{disclosed.hasMore ? ' · more available' : ''}</span>}
      </PageHeader>

      <div className="card overflow-hidden">
        {page === 1 && (
          <NewItemsNotice
            count={newTxCount}
            type="transaction"
            onClick={handleLoadNew}
            approximate={newTxApproximate}
          />
        )}

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
              {rows.map((row) => (
                row.kind === 'regular'
                  ? <TxTableRow key={row.key} tx={row.tx} />
                  : <DisclosedTableRow key={row.key} source={row.source} tx={row.tx} />
              ))}
            </tbody>
          </table>
        </div>

        {isLoading && <StateMessage variant="loading" />}
        {disclosed.loading && <p className="px-4 py-3 text-sm text-neutral-500">Loading disclosures…</p>}
        {disclosed.error && (
          <div className="px-4 py-3 text-sm text-amber-700" role="alert">
            Some disclosed transactions could not be loaded.{' '}
            <button className="underline" onClick={disclosed.retry}>Retry</button>
          </div>
        )}
        {disclosed.hasMore && <div className="p-4"><button className="btn-primary" disabled={disclosed.loadingMore} onClick={() => void disclosed.loadMore()}>{disclosed.loadingMore ? 'Loading disclosures…' : 'Load more disclosed transactions'}</button></div>}

        {!isLoading && !disclosed.loading && !disclosed.hasMore && !disclosed.error && !rows.length && (
          <StateMessage variant="empty" title={data?.total ? 'No transactions on this page' : 'No transactions yet'} />
        )}

        {/* Pagination */}
        {data && data.totalPages > 1 && (
          <div className="px-4 py-3 border-t border-neutral-100 flex flex-col sm:flex-row items-center justify-between gap-3">
            <div className="text-sm text-neutral-500" data-testid="pagination-status">
              Page {data.page} of {data.totalPages}
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => goToPage(page - 1)}
                disabled={page <= 1}
                data-testid="pagination-prev"
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
                      data-testid="pagination-page"
                      aria-current={pageNum === page}
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
                data-testid="pagination-next"
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
  system_transaction: { label: 'System Transaction', className: 'bg-neutral-100 text-neutral-500' },
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

function DisclosedTableRow({ source, tx }: DisclosedRow) {
  return (
    <tr data-testid="tx-row">
      <td>
        <Link to={grantLink(source, tx)} className="font-mono text-sm text-primary hover:text-primary-600">
          {source.level === 'full' && tx.tx_hash ? formatHash(tx.tx_hash, 8) : 'Hidden transaction'}
        </Link>
        <span className="ml-2 inline-block text-xs px-2 py-0.5 rounded bg-amber-100 text-amber-800">Disclosed</span>
      </td>
      <td className="text-neutral-400">-</td>
      <td>{tx.block_number}</td>
      <td className="font-mono text-sm">{tx.from}</td>
      <td className="font-mono text-sm">{tx.to || 'Contract'}</td>
      <td className="text-sm text-neutral-700">{tx.value === 'hidden' || tx.value === '' || tx.value == null
        ? <span className="text-neutral-400 italic">hidden</span>
        : `${formatWei(tx.value)} ${getNetworkCurrency()}`}</td>
      <td><span className={`badge ${tx.status === 1 ? 'badge-success' : 'badge-error'}`}>{tx.status === 1 ? 'Success' : 'Failed'}</span></td>
    </tr>
  );
}

function TxTableRow({ tx }: { tx: Transaction }) {
  const fromReason = tx.addressMetadata?.[tx.from?.toLowerCase()];
  const toReason = tx.to ? tx.addressMetadata?.[tx.to.toLowerCase()] : undefined;

  return (
    <tr data-testid="tx-row">
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
          <AddressLink address={tx.from} chars={6} reason={fromReason} />
          <AddressLabel reason={fromReason} />
        </span>
      </td>
      <td className="text-sm">
        {tx.to ? (
          <span className="inline-flex items-center gap-1">
            <AddressLink address={tx.to} chars={6} reason={toReason} />
            <AddressLabel reason={toReason} />
          </span>
        ) : (
          <span className="text-neutral-400">Contract</span>
        )}
      </td>
      <td className="text-sm text-neutral-700">
        {tx.value === '' || tx.value == null
          ? <span className="text-neutral-400 italic">hidden</span>
          : `${formatWei(tx.value)} ${getNetworkCurrency()}`}
      </td>
      <td>
        <span className={`badge ${tx.status === 1 ? 'badge-success' : 'badge-error'}`}>
          {tx.status === 1 ? 'Success' : 'Failed'}
        </span>
      </td>
    </tr>
  );
}
