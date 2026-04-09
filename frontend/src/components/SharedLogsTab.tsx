import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  FileText,
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { api } from '../lib/api';

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

/** Embeddable shared logs table with pagination. Expects parent to handle auth gating. */
export function SharedLogsTab() {
  const [page, setPage] = useState(1);

  const offset = (page - 1) * PAGE_SIZE;

  const { data, error } = useQuery({
    queryKey: ['sharedLogs', page],
    queryFn: () => api.getSharedLogs(PAGE_SIZE, offset),
    retry: false,
    staleTime: 30000,
    placeholderData: (prev) => prev, // keep previous data while fetching
  });

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1;

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-12 space-y-3">
        <AlertTriangle className="w-8 h-8 text-error-600" />
        <p className="text-neutral-500 text-center max-w-md text-sm">
          {error instanceof Error ? error.message : 'Failed to load shared logs.'}
        </p>
      </div>
    );
  }

  const logs = data?.shared_logs ?? [];

  if (logs.length === 0) {
    return (
      <div className="empty-state">
        <FileText className="w-12 h-12 mx-auto mb-4 text-neutral-300" />
        <p className="text-neutral-500">No logs have been shared with you yet</p>
      </div>
    );
  }

  return (
    <>
      <div className="overflow-x-auto">
        <table className="table">
          <thead>
            <tr>
              <th>Transaction</th>
              <th className="hidden md:table-cell">Contract</th>
              <th className="hidden md:table-cell">Event Logs</th>
              <th className="text-right">Shared</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((log) => (
              <tr key={log.tx_hash}>
                <td>
                  <Link
                    to={`/tx/${log.tx_hash}`}
                    className="font-mono text-sm text-primary hover:text-primary-600 transition-colors"
                  >
                    {truncateHash(log.tx_hash)}
                  </Link>
                </td>
                <td className="hidden md:table-cell">
                  {log.contract_address ? (
                    <Link
                      to={`/address/${log.contract_address}`}
                      className="font-mono text-sm text-primary hover:text-primary-600 transition-colors"
                    >
                      {truncateAddress(log.contract_address)}
                    </Link>
                  ) : (
                    <span className="text-neutral-400 text-sm">ETH transfer</span>
                  )}
                </td>
                <td className="hidden md:table-cell">
                  <span className="badge badge-neutral">
                    {(log.logs ?? []).length} log{(log.logs ?? []).length !== 1 ? 's' : ''}
                  </span>
                </td>
                <td className="text-right">
                  <span className="text-sm text-neutral-500">
                    {formatTime(log.shared_at)}
                  </span>
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
  );
}
