import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  FileText,
  AlertTriangle,
  Fingerprint,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { useAuth } from '../lib/auth';
import { api } from '../lib/api';
import { PageHeader } from '../components/PageHeader';
import { redirectToLogin } from '../lib/login';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';
import { formatDID } from '../lib/utils';

const PAGE_SIZE = 25;

function truncateHash(hash: string): string {
  if (!hash) return '';
  if (hash.length <= 14) return hash;
  return `${hash.slice(0, 8)}...${hash.slice(-6)}`;
}

function truncateAddress(address: string): string {
  if (!address) return '';
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

function formatTime(timestamp: string): string {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffSecs < 60) return `${diffSecs}s ago`;
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 30) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

export function SharedLogs() {
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [page, setPage] = useState(1);

  const offset = (page - 1) * PAGE_SIZE;

  const { data, isLoading, error } = useQuery({
    queryKey: ['sharedLogs', page],
    queryFn: () => api.getSharedLogs(PAGE_SIZE, offset),
    enabled: isAuthenticated,
    retry: false,
    staleTime: 30000,
  });

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1;

  // Show authentication prompt if not authenticated
  if (!isAuthenticated && !authLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-primary-50 flex items-center justify-center border border-primary-200">
          <Fingerprint className="w-8 h-8 text-primary" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Shared Logs</h2>
        <p className="text-neutral-500 text-center max-w-md">
          Sign in to view event logs shared with you via logVisibleTo.
        </p>
        <div className="mt-4">
          <button
            onClick={() => redirectToLogin('/shared-logs')}
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
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-error-50 flex items-center justify-center border border-error-200">
          <AlertTriangle className="w-8 h-8 text-error-600" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Error Loading Shared Logs</h2>
        <p className="text-neutral-500 text-center max-w-md">
          {error instanceof Error ? error.message : 'Failed to load shared logs. Please try again.'}
        </p>
      </div>
    );
  }

  const logs = data?.logs ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Shared Logs"
        subtitle="Event logs shared with you via logVisibleTo"
      />

      <div className="card">
        {logs.length === 0 ? (
          <div className="empty-state">
            <FileText className="w-12 h-12 mx-auto mb-4 text-neutral-300" />
            <p className="text-neutral-500">No logs have been shared with you yet</p>
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Tx Hash</th>
                    <th>Block</th>
                    <th>Contract</th>
                    <th className="hidden md:table-cell">Topics</th>
                    <th className="hidden lg:table-cell">Shared By</th>
                    <th className="text-right">Time</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map((log) => (
                    <tr key={`${log.tx_hash}-${log.log_index}`}>
                      <td>
                        <Link
                          to={`/tx/${log.tx_hash}`}
                          className="font-mono text-sm text-primary hover:text-primary-600 transition-colors"
                        >
                          <span className="hidden sm:inline">{truncateHash(log.tx_hash)}</span>
                          <span className="sm:hidden">{truncateHash(log.tx_hash)}</span>
                        </Link>
                      </td>
                      <td>
                        <Link
                          to={`/block/${log.block_number}`}
                          className="text-primary hover:text-primary-600 transition-colors"
                        >
                          {log.block_number}
                        </Link>
                      </td>
                      <td>
                        <Link
                          to={`/address/${log.address}`}
                          className="font-mono text-sm text-primary hover:text-primary-600 transition-colors"
                        >
                          <span className="hidden sm:inline">{truncateAddress(log.address)}</span>
                          <span className="sm:hidden">{truncateAddress(log.address)}</span>
                        </Link>
                      </td>
                      <td className="hidden md:table-cell">
                        <span className="badge badge-primary">
                          {log.topics.length} topic{log.topics.length !== 1 ? 's' : ''}
                        </span>
                      </td>
                      <td className="hidden lg:table-cell">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="font-mono text-xs text-neutral-500 cursor-help">
                              {formatDID(log.sender_did)}
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <span className="font-mono text-xs">{log.sender_did}</span>
                          </TooltipContent>
                        </Tooltip>
                      </td>
                      <td className="text-right">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="text-sm text-neutral-500 cursor-help">
                              {formatTime(log.created_at)}
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <span className="text-xs">{new Date(log.created_at).toLocaleString()}</span>
                          </TooltipContent>
                        </Tooltip>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-neutral-200">
                <div className="text-sm text-neutral-500">
                  Showing {offset + 1}-{Math.min(offset + PAGE_SIZE, data?.total ?? 0)} of {data?.total ?? 0}
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setPage(p => Math.max(1, p - 1))}
                    disabled={page === 1}
                    className="p-2 rounded-lg hover:bg-neutral-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <span className="text-sm text-neutral-700">
                    Page {page} of {totalPages}
                  </span>
                  <button
                    onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                    disabled={page >= totalPages}
                    className="p-2 rounded-lg hover:bg-neutral-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
