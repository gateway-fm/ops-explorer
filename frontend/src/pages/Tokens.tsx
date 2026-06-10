import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { Search, X } from 'lucide-react';
import { api } from '../lib/api';
import type { Token } from '../lib/api';
import { CopyButton } from '../components/CopyButton';
import { PageHeader } from '../components/PageHeader';
import { TokenAvatar } from '../components/TokenAvatar';
import { formatTokenType, formatTokenValue } from '../lib/formatToken';

const PAGE_SIZE = 25;

const FILTERS: { label: string; value: string }[] = [
  { label: 'All', value: '' },
  { label: 'ERC-20', value: 'ERC20' },
  { label: 'ERC-721', value: 'ERC721' },
];

export default function Tokens() {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = parseInt(searchParams.get('page') || '1', 10);
  const tokenType = searchParams.get('type') || '';
  const search = searchParams.get('search') || '';

  // Local input state so typing feels instant; debounced into the URL so the
  // backend query (and shareable URL) only update once the user pauses.
  const [searchInput, setSearchInput] = useState(search);
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- sync URL search param into local input when it changes externally (nav)
    setSearchInput(search);
  }, [search]);
  useEffect(() => {
    if (searchInput === search) return;
    const handle = window.setTimeout(() => {
      const params = new URLSearchParams(searchParams);
      if (searchInput) params.set('search', searchInput);
      else params.delete('search');
      params.set('page', '1');
      setSearchParams(params, { replace: true });
    }, 300);
    return () => window.clearTimeout(handle);
  }, [searchInput, search, searchParams, setSearchParams]);

  const { data, isLoading, error } = useQuery({
    queryKey: ['tokens', page, tokenType, search],
    queryFn: () => api.getTokens(page, PAGE_SIZE, tokenType || undefined, search || undefined),
  });

  const handlePageChange = (newPage: number) => {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(newPage));
    setSearchParams(params);
  };

  const handleTypeFilter = (type: string) => {
    const params = new URLSearchParams();
    if (type) params.set('type', type);
    if (search) params.set('search', search);
    params.set('page', '1');
    setSearchParams(params);
  };

  const tokens = data?.data ?? [];
  const totalPages = data?.totalPages ?? 1;
  const total = data?.total ?? 0;
  const firstIndex = (page - 1) * PAGE_SIZE;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Tokens"
        subtitle="ERC-20, ERC-721, and ERC-1155 contracts indexed on this network"
      />

      <Toolbar
        activeFilter={tokenType}
        onFilter={handleTypeFilter}
        searchValue={searchInput}
        onSearch={setSearchInput}
        resultCount={data ? total : null}
        searching={!!search}
      />

      {isLoading ? (
        <TableSkeleton />
      ) : error ? (
        <EmptyState tone="error">Couldn't load tokens. Try again in a moment.</EmptyState>
      ) : tokens.length === 0 ? (
        <EmptyState>
          {search
            ? <>No tokens match <span className="text-neutral-700 font-medium">"{search}"</span>.</>
            : <>No tokens have been indexed on this network yet.</>}
        </EmptyState>
      ) : (
        <TokenTable tokens={tokens} firstIndex={firstIndex} />
      )}

      {tokens.length > 0 && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onChange={handlePageChange}
        />
      )}
    </div>
  );
}

// ---------- Toolbar ----------

function Toolbar({
  activeFilter,
  onFilter,
  searchValue,
  onSearch,
  resultCount,
  searching,
}: {
  activeFilter: string;
  onFilter: (value: string) => void;
  searchValue: string;
  onSearch: (value: string) => void;
  resultCount: number | null;
  searching: boolean;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div
        role="tablist"
        aria-label="Token type"
        className="inline-flex items-center gap-1 self-start rounded-full border border-neutral-200 bg-neutral-100/60 p-1"
      >
        {FILTERS.map((f) => {
          const active = activeFilter === f.value;
          return (
            <button
              key={f.value || 'all'}
              role="tab"
              aria-selected={active}
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

// ---------- Table ----------

function TokenTable({ tokens, firstIndex }: { tokens: Token[]; firstIndex: number }) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-neutral-200 bg-neutral-50">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-200">
            <th className="w-12 px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-neutral-500">
              #
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-neutral-500">
              Token
            </th>
            <th className="hidden px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-neutral-500 md:table-cell">
              Total Supply
            </th>
            <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-neutral-500">
              Holders
            </th>
            <th className="hidden px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-neutral-500 sm:table-cell">
              Transfers
            </th>
          </tr>
        </thead>
        <tbody>
          {tokens.map((token, idx) => (
            <TokenRow key={token.address} token={token} index={firstIndex + idx + 1} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TokenRow({ token, index }: { token: Token; index: number }) {
  const symbol = token.symbol || '?';
  const name = token.name || 'Unknown Token';
  const badgeClass = [
    'inline-flex shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider',
    token.tokenType === 'ERC721'
      ? 'bg-primary/10 text-primary'
      : token.tokenType === 'ERC1155'
      ? 'bg-warning-500/10 text-warning-700 dark:text-warning-500'
      : 'bg-neutral-200/60 text-neutral-600',
  ].join(' ');

  return (
    <tr className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10">
      <td className="px-4 py-4 align-middle text-sm tabular-nums text-neutral-400">{index}</td>
      <td className="px-4 py-4 align-middle">
        <div className="flex items-center gap-3">
          <TokenAvatar symbol={symbol} address={token.address} />
          <div className="min-w-0 space-y-0.5">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <Link
                to={`/token/${token.address}`}
                className="font-semibold text-neutral-900 transition-colors hover:text-primary"
              >
                {name}
              </Link>
              <span className="text-sm text-neutral-500">{symbol}</span>
              <span className={badgeClass}>{formatTokenType(token.tokenType)}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <Link
                to={`/address/${token.address}`}
                className="truncate font-mono text-xs text-neutral-500 transition-colors hover:text-primary"
                title={token.address}
              >
                {token.address}
              </Link>
              <CopyButton text={token.address} />
            </div>
          </div>
        </div>
      </td>
      <td className="hidden px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-700 md:table-cell">
        {formatTokenValue(token.totalSupply, token.decimals)}
      </td>
      <td className="px-4 py-4 text-right align-middle text-sm tabular-nums text-neutral-700">
        {token.holderCount.toLocaleString()}
      </td>
      <td className="hidden px-4 py-4 text-right align-middle text-sm tabular-nums text-neutral-700 sm:table-cell">
        {token.transferCount.toLocaleString()}
      </td>
    </tr>
  );
}

// ---------- Pagination ----------

function Pagination({
  page,
  totalPages,
  onChange,
}: {
  page: number;
  totalPages: number;
  onChange: (p: number) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-xs text-neutral-500 tabular-nums">
        Page {page} of {Math.max(totalPages, 1)}
      </span>
      <div className="inline-flex items-center overflow-hidden rounded-full border border-neutral-200 bg-neutral-50">
        <button
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Previous
        </button>
        <span className="h-5 w-px bg-neutral-200" aria-hidden />
        <button
          onClick={() => onChange(page + 1)}
          disabled={page >= totalPages}
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Next
        </button>
      </div>
    </div>
  );
}

// ---------- Loading / empty ----------

function TableSkeleton() {
  return (
    <div className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
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

function EmptyState({
  children,
  tone = 'neutral',
}: {
  children: React.ReactNode;
  tone?: 'neutral' | 'error';
}) {
  return (
    <div
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
