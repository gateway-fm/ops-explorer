import type { ReactNode } from 'react';
import { Search, X } from 'lucide-react';
const FILTERS: { label: string; value: string }[] = [
  { label: 'All', value: '' },
  { label: 'ERC-20', value: 'ERC20' },
  { label: 'ERC-721', value: 'ERC721' },
];

export function TokensToolbar({
  activeFilter,
  onFilter,
  searchValue,
  onSearch,
  resultCount,
  searching,
  includeERC1155 = false,
}: {
  activeFilter: string;
  onFilter: (value: string) => void;
  searchValue: string;
  onSearch: (value: string) => void;
  resultCount: number | null;
  searching: boolean;
  includeERC1155?: boolean;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div
        role="tablist"
        aria-label="Token type"
        className="inline-flex items-center gap-1 self-start rounded-full border border-neutral-200 bg-neutral-100/60 p-1"
      >
        {[...FILTERS,...(includeERC1155?[{label:'ERC-1155',value:'ERC1155'}]:[])].map((f) => {
          const active = activeFilter === f.value;
          return (
            <button
              key={f.value || 'all'}
              role="tab"
              aria-selected={active}
              data-testid={`tab-${f.value || 'all'}`}
              onClick={() => onFilter(f.value)}
              className={[
                'px-3.5 py-1.5 rounded-full text-sm font-medium transition-colors',
                active
                  ? 'bg-neutral-50 text-neutral-900 shadow-card'
                  : 'text-neutral-500 hover:text-neutral-700',
              ].join(' ')}
            >
              {f.label}
            </button>
          );
        })}
      </div>

      <div className="flex items-center gap-3 sm:gap-4">
        {searching && resultCount !== null && (
          <span className="hidden text-xs text-neutral-500 tabular-nums sm:inline">
            {resultCount.toLocaleString()} {resultCount === 1 ? 'result' : 'results'}
          </span>
        )}
        <div className="relative w-full sm:w-80">
          <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-neutral-400" />
          <input
            type="search"
            value={searchValue}
            onChange={(e) => onSearch(e.target.value)}
            placeholder="Search name, symbol, address"
            aria-label="Search tokens"
            className="w-full rounded-full border border-neutral-200 bg-neutral-100/60 py-2 pl-10 pr-9 text-sm text-neutral-900 placeholder:text-neutral-400 transition-colors focus:border-primary focus:bg-neutral-50 focus:outline-none focus:ring-2 focus:ring-primary/20"
          />
          {searchValue && (
            <button
              type="button"
              onClick={() => onSearch('')}
              aria-label="Clear search"
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded-full p-1 text-neutral-400 transition-colors hover:bg-neutral-200/70 hover:text-neutral-700"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

export function TokensTableSkeleton() {
  return (
    <div role="status" aria-label="Loading tokens" className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-4 border-b border-neutral-200/60 px-4 py-5 last:border-b-0"
        >
          <div className="h-10 w-10 shrink-0 animate-pulse rounded-full bg-neutral-200" />
          <div className="flex-1 space-y-2">
            <div className="h-3 w-1/3 animate-pulse rounded bg-neutral-200" />
            <div className="h-3 w-2/3 animate-pulse rounded bg-neutral-200/70" />
          </div>
          <div className="hidden h-3 w-20 animate-pulse rounded bg-neutral-200 sm:block" />
        </div>
      ))}
    </div>
  );
}

// The V1 table layout accepts native exact-value cells without its API model.
export function TokensTablePresentation({children, ranked = false, supply = true, holders = true, transfers = true, supplyLabel = 'Total Supply'}: {children: ReactNode; ranked?: boolean; supply?: boolean; holders?: boolean; transfers?: boolean; supplyLabel?: string}) {
  return <div className="overflow-x-auto rounded-2xl border border-neutral-200 bg-neutral-50"><table className="w-full text-sm"><thead><tr className="border-b border-neutral-200">
    {ranked && <th className="w-12 px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-neutral-500">#</th>}
    <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-neutral-500">Token</th>
    {supply && <th className="hidden px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-neutral-500 md:table-cell">{supplyLabel}</th>}
    {holders && <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-neutral-500">Holders</th>}
    {transfers && <th className="hidden px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-neutral-500 sm:table-cell">Transfers</th>}
  </tr></thead><tbody>{children}</tbody></table></div>;
}
export function TokenTableRowPresentation({children}: {children: ReactNode}) {
  return <tr data-testid="token-row" className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10">{children}</tr>;
}

export function TokensEmptyState({
  children,
  tone = 'neutral',
}: {
  children: ReactNode;
  tone?: 'neutral' | 'error';
}) {
  return (
    <div
      data-testid={tone === 'error' ? 'app-error' : 'app-empty'}
      className={[
        'rounded-2xl border border-dashed px-6 py-12 text-center text-sm',
        tone === 'error'
          ? 'border-error-500/30 bg-error-50/40 text-error-700 dark:text-error-500'
          : 'border-neutral-200 bg-neutral-100/30 text-neutral-500',
      ].join(' ')}
    >
      {children}
    </div>
  );
}
