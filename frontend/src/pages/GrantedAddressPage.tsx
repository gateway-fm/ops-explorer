import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { api } from '../lib/api';
import type { PseudonymizedTransaction, GrantedAddressResponse } from '../lib/api';
import { formatWei, formatTimestamp, getNetworkCurrency } from '../lib/utils';
import { useAuth } from '../lib/auth';
import { redirectToLogin } from '../lib/login';
import { PageHeader } from '../components/PageHeader';
import { Fingerprint, Unlock, ShieldAlert, EyeOff, ArrowDownLeft, ArrowUpRight, RotateCcw, FileText, Activity, ChevronLeft, ChevronRight } from 'lucide-react';

/**
 * GrantedAddressPage displays address information for disclosed addresses.
 * Uses opaque address_id for routing - the real address is never exposed to
 * the frontend for pseudonymous/redacted disclosures.
 *
 * SECURITY: This page receives already-redacted data from the backend.
 * The display_address field contains the pseudonym or "[REDACTED]" for
 * non-full disclosures - the real address is never sent.
 *
 * Scope-aware tabs:
 * - "Transactions" tab shown when scope includes "transaction_history" or "full_disclosure",
 *   or when no scope_methods are specified (backwards-compatible default).
 * - "Activity Logs" tab shown when scope includes "activity_logs" or "full_disclosure".
 * - For activity_logs-only scope, the Transactions tab is NOT shown to avoid 500 errors.
 */

type TabId = 'transactions' | 'activity_logs';

/** Determines which tabs are available based on scope methods. */
function getAvailableTabs(data: GrantedAddressResponse): TabId[] {
  const methods = data.scope_methods ?? [];
  const isRedacted = data.disclosure_level === 'redacted';

  // If no scope methods specified, default to showing transactions
  // (backwards-compatible with grants created before scope methods were added).
  if (methods.length === 0) {
    const tabs: TabId[] = [];
    if (!isRedacted) tabs.push('transactions');
    return tabs;
  }

  const hasFullDisclosure = methods.includes('full_disclosure');
  const hasTransactionHistory = methods.includes('transaction_history');
  const hasActivityLogs = methods.includes('activity_logs');

  const tabs: TabId[] = [];

  if ((hasFullDisclosure || hasTransactionHistory) && !isRedacted) {
    tabs.push('transactions');
  }

  if (hasFullDisclosure || hasActivityLogs) {
    tabs.push('activity_logs');
  }

  return tabs;
}

export function GrantedAddressPage() {
  const { grantId, addressId } = useParams<{ grantId: string; addressId: string }>();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [activeTab, setActiveTab] = useState<TabId | null>(null);
  const [activityOffset, setActivityOffset] = useState(0);
  const activityLimit = 25;

  const { data, isLoading, error } = useQuery({
    queryKey: ['grantedAddress', grantId, addressId],
    queryFn: () => api.getGrantedAddress(grantId!, addressId!),
    enabled: !!grantId && !!addressId && isAuthenticated,
    retry: false,
  });

  // Compute available tabs from the data
  const availableTabs = data ? getAvailableTabs(data) : [];
  const currentTab = activeTab ?? availableTabs[0] ?? null;
  const showTransactionsTab = availableTabs.includes('transactions');
  const showActivityLogsTab = availableTabs.includes('activity_logs');

  // Fetch transactions only when the transactions tab is available and selected
  const shouldFetchTransactions = showTransactionsTab && currentTab === 'transactions';
  const { data: txData, isLoading: txLoading } = useQuery({
    queryKey: ['grantedAddressTransactions', grantId, addressId],
    queryFn: () => api.getGrantedAddressTransactions(grantId!, addressId!),
    enabled: !!grantId && !!addressId && isAuthenticated && shouldFetchTransactions,
    retry: false,
  });

  // Fetch activity logs when the activity_logs tab is available and selected
  const shouldFetchActivityLogs = showActivityLogsTab && currentTab === 'activity_logs';
  const { data: activityData, isLoading: activityLoading } = useQuery({
    queryKey: ['grantActivityLogs', grantId, activityLimit, activityOffset],
    queryFn: () => api.getGrantActivityLogs(grantId!, activityLimit, activityOffset),
    enabled: !!grantId && isAuthenticated && shouldFetchActivityLogs,
    retry: false,
  });

  // Show authentication prompt if not logged in
  if (!isAuthenticated && !authLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-primary-50 flex items-center justify-center border border-primary-200">
          <Fingerprint className="w-8 h-8 text-primary" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Authentication Required</h2>
        <p className="text-neutral-500 text-center max-w-md">
          Sign in to view disclosed address details.
        </p>
        <div className="mt-4">
          <button
            onClick={() => redirectToLogin(`/grant/${grantId}/${addressId}`)}
            className="btn-primary flex items-center gap-2"
          >
            <Fingerprint className="w-4 h-4" />
            Sign In
          </button>
        </div>
      </div>
    );
  }

  if (authLoading || isLoading) {
    return <div className="text-neutral-400">Loading...</div>;
  }

  if (error) {
    const errorMessage = error instanceof Error ? error.message : 'Unknown error';
    const isAccessDenied = errorMessage.includes('403') ||
                          errorMessage.includes('forbidden') ||
                          errorMessage.includes('expired') ||
                          errorMessage.includes('revoked');
    const isNotFound = errorMessage.includes('404') || errorMessage.includes('not found');

    if (isAccessDenied) {
      return (
        <div className="flex flex-col items-center justify-center py-16 space-y-4">
          <div className="w-16 h-16 rounded-full bg-error-50 flex items-center justify-center border border-error-200">
            <ShieldAlert className="w-8 h-8 text-error-600" />
          </div>
          <h2 className="text-xl font-semibold text-neutral-900">Access Denied</h2>
          <p className="text-neutral-500 text-center max-w-md">
            This disclosure grant has expired or been revoked. Contact the address owner to request a new disclosure.
          </p>
          <Link to="/privacy" className="btn btn-primary mt-4">
            Back to Privacy Dashboard
          </Link>
        </div>
      );
    }

    if (isNotFound) {
      return (
        <div className="flex flex-col items-center justify-center py-16 space-y-4">
          <div className="w-16 h-16 rounded-full bg-warning-50 flex items-center justify-center border border-warning-200">
            <EyeOff className="w-8 h-8 text-warning-600" />
          </div>
          <h2 className="text-xl font-semibold text-neutral-900">Address Not Found</h2>
          <p className="text-neutral-500 text-center max-w-md">
            The requested address could not be found. The grant or address may not exist.
          </p>
          <Link to="/privacy" className="btn btn-primary mt-4">
            Back to Privacy Dashboard
          </Link>
        </div>
      );
    }

    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-error-50 flex items-center justify-center border border-error-200">
          <ShieldAlert className="w-8 h-8 text-error-600" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Error Loading Address</h2>
        <p className="text-neutral-500 text-center max-w-md">
          {errorMessage}
        </p>
        <Link to="/privacy" className="btn btn-primary mt-4">
          Back to Privacy Dashboard
        </Link>
      </div>
    );
  }

  if (!data) {
    return <div className="text-error-600">Address not found</div>;
  }

  const isPseudonymous = data.disclosure_level === 'pseudonymous';
  const isRedacted = data.disclosure_level === 'redacted';
  const isFull = data.disclosure_level === 'full';

  return (
    <div className="space-y-6">
      <PageHeader title={data.is_contract ? 'Contract' : 'Address'}>
        {data.is_contract && (
          <span className="badge bg-primary-50 text-primary-600 border border-primary-200">
            Contract
          </span>
        )}
        <span data-testid="disclosure-level-badge" className={`badge capitalize ${
          isPseudonymous
            ? 'bg-amber-100 text-amber-700 border-amber-200'
            : isRedacted
              ? 'bg-red-100 text-red-700 border-red-200'
              : 'badge-primary'
        }`}>
          {data.disclosure_level || 'Full'} Disclosure
        </span>
      </PageHeader>

      {/* Disclosure Banner */}
      <div data-testid="disclosure-banner" className={`card p-4 ${
        isPseudonymous
          ? 'border-amber-200 bg-amber-50'
          : isRedacted
            ? 'border-red-200 bg-red-50'
            : 'border-primary-200 bg-primary-50'
      }`}>
        <div className="flex items-start gap-3">
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${
            isPseudonymous
              ? 'bg-amber-100'
              : isRedacted
                ? 'bg-red-100'
                : 'bg-primary-100'
          }`}>
            <Unlock className={`w-5 h-5 ${
              isPseudonymous
                ? 'text-amber-600'
                : isRedacted
                  ? 'text-red-600'
                  : 'text-primary'
            }`} />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className={`font-medium ${
                isPseudonymous
                  ? 'text-amber-900'
                  : isRedacted
                    ? 'text-red-900'
                    : 'text-primary-900'
              }`}>
                {isPseudonymous && 'Pseudonymous Disclosure'}
                {isRedacted && 'Redacted Disclosure'}
                {isFull && 'Full Disclosure'}
              </h3>
            </div>
            <p className={`text-sm mt-0.5 ${
              isPseudonymous
                ? 'text-amber-700'
                : isRedacted
                  ? 'text-red-700'
                  : 'text-primary-700'
            }`}>
              {isPseudonymous && 'You can view this address with a consistent pseudonym. The real address is hidden.'}
              {isRedacted && 'Address details are redacted. You can only view limited information.'}
              {isFull && 'You have full access to view this address.'}
            </p>
            <p className={`text-xs mt-1 font-mono ${
              isPseudonymous
                ? 'text-amber-600'
                : isRedacted
                  ? 'text-red-600'
                  : 'text-primary-600'
            }`}>
              Grant: {data.grant_id.slice(0, 8)}...
            </p>
          </div>
        </div>
      </div>

      {/* Address Info Card */}
      <div className="card">
        <div className="divide-y divide-neutral-100">
          <InfoRow
            label="Address"
            value={
              isRedacted ? (
                <span className="text-neutral-400 italic">[REDACTED]</span>
              ) : isPseudonymous ? (
                <div>
                  <span className="font-mono text-sm break-all text-neutral-900">{data.display_address}</span>
                  <span className="text-xs text-neutral-400 ml-2">(pseudonym)</span>
                </div>
              ) : (
                <span className="font-mono text-sm break-all text-neutral-900">{data.display_address}</span>
              )
            }
          />
          <InfoRow label="Type" value={data.is_contract ? 'Contract' : 'EOA (Externally Owned Account)'} />
          <InfoRow label="Balance" value={`${formatWei(data.balance)} ${getNetworkCurrency()}`} />
          <InfoRow label="Transactions" value={data.tx_count.toLocaleString()} />
          <InfoRow label="Disclosure Level" value={
            <span className={`badge capitalize ${
              isPseudonymous
                ? 'bg-amber-100 text-amber-700 border-amber-200'
                : isRedacted
                  ? 'bg-red-100 text-red-700 border-red-200'
                  : 'badge-primary'
            }`}>
              {data.disclosure_level}
            </span>
          } />
        </div>
      </div>

      {/* Scope-aware tabs */}
      {availableTabs.length > 1 && (
        <div className="border-b border-neutral-200">
          <nav className="flex -mb-px">
            {showTransactionsTab && (
              <button
                onClick={() => setActiveTab('transactions')}
                role="tab"
                data-testid="tab-transactions"
                aria-selected={currentTab === 'transactions'}
                className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                  currentTab === 'transactions'
                    ? 'border-primary text-primary'
                    : 'border-transparent text-neutral-500 hover:text-neutral-700 hover:border-neutral-300'
                }`}
              >
                <FileText className="w-4 h-4" />
                Transactions
              </button>
            )}
            {showActivityLogsTab && (
              <button
                onClick={() => setActiveTab('activity_logs')}
                role="tab"
                data-testid="tab-activity_logs"
                aria-selected={currentTab === 'activity_logs'}
                className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                  currentTab === 'activity_logs'
                    ? 'border-primary text-primary'
                    : 'border-transparent text-neutral-500 hover:text-neutral-700 hover:border-neutral-300'
                }`}
              >
                <Activity className="w-4 h-4" />
                Activity Logs
              </button>
            )}
          </nav>
        </div>
      )}

      {/* Transactions Tab Content */}
      {currentTab === 'transactions' && !isRedacted && (
        <div className="card">
          <div className="px-4 py-3 border-b border-neutral-100">
            <h3 className="font-medium text-neutral-900">Transactions</h3>
            {isPseudonymous && (
              <p className="text-xs text-neutral-500 mt-1">
                Addresses are shown as pseudonyms. Transaction hashes are hidden to prevent lookup.
              </p>
            )}
          </div>
          {txLoading ? (
            <div className="p-8 text-center text-neutral-400">Loading transactions...</div>
          ) : txData?.transactions.length === 0 ? (
            <div className="p-8 text-center text-neutral-400">No transactions found</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    {isFull && <th>Tx Hash</th>}
                    <th>Block</th>
                    <th>Time</th>
                    <th>Direction</th>
                    <th>From</th>
                    <th>To</th>
                    <th className="text-right">Value</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {txData?.transactions.map((tx, index) => (
                    <TransactionRow
                      key={tx.tx_hash || `${tx.block_number}-${index}`}
                      tx={tx}
                      isFull={isFull}
                      addressLabels={txData.address_labels}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Activity Logs Tab Content */}
      {currentTab === 'activity_logs' && (
        <div className="card">
          <div className="px-4 py-3 border-b border-neutral-100">
            <h3 className="font-medium text-neutral-900">Activity Logs</h3>
            <p className="text-xs text-neutral-500 mt-1">
              RPC method calls made by the disclosed address during the grant period.
            </p>
          </div>
          {activityLoading ? (
            <div className="p-8 text-center text-neutral-400">Loading activity logs...</div>
          ) : activityData?.logs.length === 0 ? (
            <div className="p-8 text-center text-neutral-400">No activity logs found</div>
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Method</th>
                      <th>Status</th>
                      <th>Timestamp</th>
                    </tr>
                  </thead>
                  <tbody>
                    {activityData?.logs.map((log, index) => (
                      <tr key={`${log.timestamp}-${index}`}>
                        <td>
                          <span className="font-mono text-sm text-neutral-900">{log.method}</span>
                        </td>
                        <td>
                          {log.status_code >= 200 && log.status_code < 300 ? (
                            <span className="badge badge-success">{log.status_code}</span>
                          ) : log.status_code >= 400 ? (
                            <span className="badge bg-error-50 text-error-600 border-error-200">{log.status_code}</span>
                          ) : (
                            <span className="badge">{log.status_code}</span>
                          )}
                        </td>
                        <td className="text-neutral-500 text-sm whitespace-nowrap">
                          {new Date(log.timestamp).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {/* Pagination */}
              {activityData && activityData.total > activityLimit && (
                <div className="flex items-center justify-between px-4 py-3 border-t border-neutral-100">
                  <span className="text-sm text-neutral-500">
                    Showing {activityOffset + 1}–{Math.min(activityOffset + activityLimit, activityData.total)} of {activityData.total}
                  </span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setActivityOffset(Math.max(0, activityOffset - activityLimit))}
                      disabled={activityOffset === 0}
                      className="btn btn-sm flex items-center gap-1 disabled:opacity-50"
                    >
                      <ChevronLeft className="w-4 h-4" />
                      Previous
                    </button>
                    <button
                      onClick={() => setActivityOffset(activityOffset + activityLimit)}
                      disabled={activityOffset + activityLimit >= activityData.total}
                      className="btn btn-sm flex items-center gap-1 disabled:opacity-50"
                    >
                      Next
                      <ChevronRight className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* Redacted notice for transactions (when no tabs available) */}
      {isRedacted && availableTabs.length === 0 && (
        <div className="card p-6 text-center">
          <EyeOff className="w-8 h-8 mx-auto mb-3 text-neutral-300" />
          <p className="text-neutral-500">
            Transaction details are not available for redacted disclosures.
          </p>
          <p className="text-xs text-neutral-400 mt-1">
            Only aggregate statistics (transaction count, balance) are shown.
          </p>
        </div>
      )}

      {/* Back to Dashboard */}
      <div className="flex justify-center">
        <Link
          to="/privacy"
          className="text-sm text-neutral-500 hover:text-neutral-700 transition-colors"
        >
          Back to Privacy Dashboard
        </Link>
      </div>
    </div>
  );
}

function TransactionRow({
  tx,
  isFull,
  addressLabels,
}: {
  tx: PseudonymizedTransaction;
  isFull: boolean;
  addressLabels: Record<string, string>;
}) {
  const DirectionIcon = tx.direction === 'in' ? ArrowDownLeft :
                        tx.direction === 'out' ? ArrowUpRight : RotateCcw;
  const directionColor = tx.direction === 'in' ? 'text-success-600' :
                         tx.direction === 'out' ? 'text-error-600' : 'text-neutral-500';
  const directionBg = tx.direction === 'in' ? 'bg-success-50' :
                      tx.direction === 'out' ? 'bg-error-50' : 'bg-neutral-100';

  return (
    <tr>
      {isFull && (
        <td>
          <Link
            to={`/tx/${tx.tx_hash}`}
            className="font-mono text-xs text-primary hover:text-primary-600 transition-colors"
          >
            {tx.tx_hash?.slice(0, 10)}...
          </Link>
        </td>
      )}
      <td>
        <Link
          to={`/block/${tx.block_number}`}
          className="text-primary hover:text-primary-600 transition-colors"
        >
          {tx.block_number.toLocaleString()}
        </Link>
      </td>
      <td className="text-neutral-500 text-sm whitespace-nowrap">
        {formatTimestamp(tx.block_timestamp)}
      </td>
      <td>
        <div className={`inline-flex items-center gap-1 px-2 py-1 rounded-lg ${directionBg}`}>
          <DirectionIcon className={`w-3.5 h-3.5 ${directionColor}`} />
          <span className={`text-xs font-medium capitalize ${directionColor}`}>
            {tx.direction}
          </span>
        </div>
      </td>
      <td>
        <AddressCell
          address={tx.from}
          isThisAddress={addressLabels[tx.from] === 'This Address'}
          isFull={isFull}
        />
      </td>
      <td>
        {tx.to ? (
          <AddressCell
            address={tx.to}
            isThisAddress={addressLabels[tx.to] === 'This Address'}
            isFull={isFull}
          />
        ) : (
          <span className="text-neutral-400 italic text-sm">Contract Creation</span>
        )}
      </td>
      <td className="text-right font-mono text-sm">
        {tx.value === '' || tx.value == null
          ? <span className="text-neutral-400 italic">hidden</span>
          : `${formatWei(String(tx.value))} ${getNetworkCurrency()}`}
      </td>
      <td>
        {tx.status === 1 ? (
          <span className="badge badge-success">Success</span>
        ) : (
          <span className="badge bg-error-50 text-error-600 border-error-200">Failed</span>
        )}
      </td>
    </tr>
  );
}

function AddressCell({
  address,
  isThisAddress,
  isFull,
}: {
  address: string;
  isThisAddress: boolean;
  isFull: boolean;
}) {
  if (isThisAddress) {
    return (
      <span className="inline-flex items-center gap-1">
        <span className="font-mono text-xs text-primary-600 font-medium">
          {isFull ? `${address.slice(0, 8)}...` : address}
        </span>
        <span className="text-[10px] px-1.5 py-0.5 bg-primary-100 text-primary-700 rounded">
          This
        </span>
      </span>
    );
  }

  if (isFull) {
    return (
      <Link
        to={`/address/${address}`}
        className="font-mono text-xs text-neutral-600 hover:text-primary transition-colors"
      >
        {address.slice(0, 8)}...{address.slice(-4)}
      </Link>
    );
  }

  // Pseudonymous external address - not linkable
  return (
    <span className="font-mono text-xs text-neutral-500">
      {address}
    </span>
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
