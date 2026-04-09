import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  FileText,
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  ArrowRight,
} from 'lucide-react';
import { api } from '../lib/api';
import type { SharedEventLog, SharedLogEntry } from '../lib/api';
import { decodeTransferLog } from '../lib/decodeTransferLog';
import type { DecodedTransfer } from '../lib/decodeTransferLog';
import { formatTokenValue } from '../lib/formatToken';
import { useTokenMap } from '../hooks/useTokenMap';
import { AddressLink } from './AddressLink';
import { formatAddress } from '../lib/utils';

const PAGE_SIZE = 25;

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

/** A single flattened row: either a decoded Transfer or a generic event. */
interface TransferRow {
  key: string;
  transfer: DecodedTransfer;
  sharedAt: string;
}

interface GenericRow {
  key: string;
  log: SharedEventLog;
  contractAddress: string;
  sharedAt: string;
}

type DisplayRow =
  | { kind: 'transfer'; data: TransferRow }
  | { kind: 'generic'; data: GenericRow };

function flattenEntries(entries: SharedLogEntry[]): DisplayRow[] {
  const rows: DisplayRow[] = [];
  for (const entry of entries) {
    for (const log of entry.logs) {
      const decoded = decodeTransferLog(log);
      if (decoded) {
        rows.push({
          kind: 'transfer',
          data: {
            key: `${entry.tx_hash}-${log.logIndex}`,
            transfer: decoded,
            sharedAt: entry.shared_at,
          },
        });
      } else {
        rows.push({
          kind: 'generic',
          data: {
            key: `${entry.tx_hash}-${log.logIndex}`,
            log,
            contractAddress: entry.contract_address,
            sharedAt: entry.shared_at,
          },
        });
      }
    }
  }
  return rows;
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
    placeholderData: (prev) => prev,
  });

  const entries = data?.shared_logs;
  const rows = useMemo(() => flattenEntries(entries ?? []), [entries]);

  // Collect token addresses from decoded transfers for useTokenMap
  const tokenAddresses = useMemo(
    () =>
      rows
        .filter((r): r is { kind: 'transfer'; data: TransferRow } => r.kind === 'transfer')
        .map((r) => r.data.transfer.tokenAddress),
    [rows],
  );
  const tokenMap = useTokenMap(tokenAddresses);

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

  if (rows.length === 0 && (!entries || entries.length === 0)) {
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
              <th>From</th>
              <th className="w-8"></th>
              <th>To</th>
              <th>Token</th>
              <th className="text-right">Amount</th>
              <th className="hidden md:table-cell">Block</th>
              <th className="text-right">Shared</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              if (row.kind === 'transfer') {
                const { transfer, sharedAt, key } = row.data;
                const token = tokenMap[transfer.tokenAddress];
                const decimals = token?.decimals ?? 18;
                const symbol = token?.symbol;

                return (
                  <tr key={key}>
                    <td>
                      <AddressLink address={transfer.from} chars={6} />
                    </td>
                    <td className="text-center">
                      <ArrowRight className="w-4 h-4 text-neutral-400 inline-block" />
                    </td>
                    <td>
                      <AddressLink address={transfer.to} chars={6} />
                    </td>
                    <td>
                      <Link
                        to={`/token/${transfer.tokenAddress}`}
                        className="text-primary hover:text-primary-600 transition-colors font-mono text-sm"
                      >
                        {symbol || formatAddress(transfer.tokenAddress, 4)}
                      </Link>
                    </td>
                    <td className="text-right font-mono text-neutral-700">
                      {formatTokenValue(transfer.amount, decimals)}
                    </td>
                    <td className="hidden md:table-cell">
                      <Link
                        to={`/block/${transfer.blockNumber}`}
                        className="text-primary hover:text-primary-600 transition-colors"
                      >
                        {transfer.blockNumber.toLocaleString()}
                      </Link>
                    </td>
                    <td className="text-right">
                      <span className="text-sm text-neutral-500">
                        {formatTime(sharedAt)}
                      </span>
                    </td>
                  </tr>
                );
              }

              // Generic (non-Transfer) event row
              const { log, contractAddress, sharedAt, key } = row.data;
              return (
                <tr key={key}>
                  <td colSpan={3}>
                    <span className="inline-flex items-center gap-2">
                      <AddressLink address={log.address || contractAddress} chars={6} />
                      <span className="badge badge-neutral text-xs">Event</span>
                    </span>
                  </td>
                  <td>
                    <span className="font-mono text-xs text-neutral-400">
                      {log.topic0 ? formatAddress(log.topic0, 6) : '-'}
                    </span>
                  </td>
                  <td className="text-right font-mono text-neutral-400 text-xs">-</td>
                  <td className="hidden md:table-cell">
                    <Link
                      to={`/block/${log.blockNumber}`}
                      className="text-primary hover:text-primary-600 transition-colors"
                    >
                      {log.blockNumber.toLocaleString()}
                    </Link>
                  </td>
                  <td className="text-right">
                    <span className="text-sm text-neutral-500">
                      {formatTime(sharedAt)}
                    </span>
                  </td>
                </tr>
              );
            })}
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
