import type { ReactNode } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { PageHeader } from './PageHeader';
import { CopyButton } from './CopyButton';

export type TransactionTab = 'overview' | 'logs' | 'trace';
export function TransactionDetailPresentation({ activeTab, setActiveTab, logCount, traceCount, logsVisible, traceVisible, children }: {
  activeTab: TransactionTab; setActiveTab: (tab: TransactionTab) => void;
  logCount: ReactNode; traceCount: ReactNode; logsVisible: boolean; traceVisible: boolean; children: ReactNode;
}) {
  return <div className="space-y-6"><PageHeader title="Transaction Details" />
      {/* Tabs - only show when there are logs or internal calls to display */}
      {(logsVisible || traceVisible) && (
        <div className="tabs" role="tablist">
          <button
            onClick={() => setActiveTab('overview')}
            role="tab"
            data-testid="tab-overview"
            aria-selected={activeTab === 'overview'}
            className={activeTab === 'overview' ? 'tab-active' : 'tab'}
          >
            Overview
          </button>
          {logsVisible && (
            <button
              onClick={() => setActiveTab('logs')}
              role="tab"
              data-testid="tab-logs"
              aria-selected={activeTab === 'logs'}
              className={activeTab === 'logs' ? 'tab-active' : 'tab'}
            >
              Logs ({logCount})
            </button>
          )}
          {traceVisible && (
            <button
              onClick={() => setActiveTab('trace')}
              role="tab"
              data-testid="tab-trace"
              aria-selected={activeTab === 'trace'}
              className={activeTab === 'trace' ? 'tab-active' : 'tab'}
            >
              Internal Txns ({traceCount})
            </button>
          )}
        </div>
      )}

{children}</div>;
}

export function TransactionOverview({ children, extra, input, showMore, onToggle }: {children: ReactNode; extra?: ReactNode; input?: ReactNode; showMore: boolean; onToggle: () => void}) {
  return <div className="card"><div className="divide-y divide-neutral-100">{children}</div>
    <button onClick={onToggle} data-testid="show-more-toggle" className="w-full px-4 py-3 flex items-center justify-center gap-2 text-sm text-neutral-500 hover:text-neutral-700 hover:bg-neutral-50 transition-colors border-t border-neutral-100">
      {showMore ? <><ChevronUp className="w-4 h-4" />Show Less</> : <><ChevronDown className="w-4 h-4" />Show More</>}
    </button>
    {showMore && <div className="divide-y divide-neutral-100 border-t border-neutral-100">{extra}</div>}
    {input}
  </div>;
}

export function TransactionInput({ data }: {data: string}) {
  if (!data || data === '0x') return null;
  const hex = data.startsWith('0x') ? data : `0x${data}`;
  return <div className="border-t border-neutral-100 px-4 py-3"><div className="flex items-center justify-between mb-2"><div className="flex items-center gap-2"><span className="text-sm font-medium text-neutral-500">Input Data</span><span className="text-neutral-400 text-xs">({(hex.length - 2) / 2} bytes)</span></div><CopyButton text={hex} label="Copy input data" className="text-xs text-neutral-500 hover:text-neutral-700 bg-neutral-100 hover:bg-neutral-200 px-2 py-1 rounded transition-colors">Copy</CopyButton></div><div className="code-block p-2 text-xs max-h-64 overflow-y-auto"><div className="break-all whitespace-pre-wrap">{hex}</div></div></div>;
}

export function LogCardPresentation({ index, address, decoded, topics, data, fetching, provenance, context, showEmptyData = false }: {index: number; address: ReactNode; decoded?: ReactNode; topics: ReactNode; data: string; fetching?: boolean; provenance?: ReactNode; context?: ReactNode; showEmptyData?: boolean}) {
  const hex = data.startsWith('0x') ? data : `0x${data}`;
  return <div className="card p-4 space-y-3"><div className="flex flex-wrap items-center gap-2"><span className="badge badge-neutral font-mono">{index}</span><span className="text-neutral-500 text-sm">Log Index</span>{provenance}</div>{context}<div className="space-y-3"><div className="flex flex-col sm:flex-row gap-1 sm:gap-2"><span className="text-neutral-500 text-sm w-20 shrink-0">Address</span><span className="flex min-w-0 items-center gap-1 flex-wrap">{address}</span></div>{decoded}{fetching && <div className="bg-neutral-50 border border-neutral-200 rounded-lg p-3 flex items-center gap-2"><div className="w-4 h-4 border-2 border-neutral-300 border-t-primary-500 rounded-full animate-spin" /><span className="text-xs text-neutral-500">Looking up event signature...</span></div>}{topics}{data && (showEmptyData || hex !== '0x') && <div className="flex flex-col gap-1"><div className="flex items-center gap-2"><span className="text-neutral-500 text-sm">Data</span><span className="text-neutral-400 text-xs">({(hex.length - 2) / 2} bytes)</span></div><div className="code-block p-2 text-xs max-h-64 overflow-y-auto"><div className="break-all whitespace-pre-wrap">{hex}</div></div></div>}</div></div>;
}

export function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="info-row" data-testid="info-row" data-label={label}>
      <div className="info-label">{label}</div>
      <div className="info-value">{value}</div>
    </div>
  );
}

// Category display configuration
const CATEGORY_CONFIG: Record<string, { label: string; className: string }> = {
  contract_creation: { label: 'Contract Creation', className: 'bg-purple-100 text-purple-700' },
  contract_call: { label: 'Contract Call', className: 'bg-blue-100 text-blue-700' },
  coin_transfer: { label: 'Coin Transfer', className: 'bg-green-100 text-green-700' },
  token_transfer: { label: 'Token Transfer', className: 'bg-orange-100 text-orange-700' },
  system_transaction: { label: 'System Transaction', className: 'bg-neutral-100 text-neutral-500' },
};

export function TxCategoryBadges({ categories }: { categories?: string[] }) {
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
