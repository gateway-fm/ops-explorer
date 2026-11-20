import { TokensTablePresentation, TokenTableRowPresentation } from '../components/TokensPresentation';
import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { TokensToolbar as Toolbar, TokensTableSkeleton as TableSkeleton, TokensEmptyState as EmptyState } from '../components/TokensPresentation';
import { api } from '../lib/api';
import type { Token } from '../lib/api';
import { CopyButton } from '../components/CopyButton';
import { PageHeader } from '../components/PageHeader';
import { TokenAvatar } from '../components/TokenAvatar';
import { formatTokenType, formatTokenValue } from '../lib/formatToken';

const PAGE_SIZE = 25;

export default function Tokens() {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = parseInt(searchParams.get('page') || '1', 10);
  const tokenType = searchParams.get('type') || '';
  const search = searchParams.get('search') || '';

  // Local input state so typing feels instant; debounced into the URL so the
  // backend query (and shareable URL) only update once the user pauses.
  const [searchInput, setSearchInput] = useState(search);
  useEffect(() => {
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

// ---------- Table ----------

function TokenTable({ tokens, firstIndex }: { tokens: Token[]; firstIndex: number }) {
  return <TokensTablePresentation ranked>{tokens.map((token, idx) => <TokenRow key={token.address} token={token} index={firstIndex + idx + 1}/>)}</TokensTablePresentation>;
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
    <TokenTableRowPresentation>
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
    </TokenTableRowPresentation>
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
      <span className="text-xs text-neutral-500 tabular-nums" data-testid="pagination-status">
        Page {page} of {Math.max(totalPages, 1)}
      </span>
      <div className="inline-flex items-center overflow-hidden rounded-full border border-neutral-200 bg-neutral-50">
        <button
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          data-testid="pagination-prev"
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Previous
        </button>
        <span className="h-5 w-px bg-neutral-200" aria-hidden />
        <button
          onClick={() => onChange(page + 1)}
          disabled={page >= totalPages}
          data-testid="pagination-next"
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Next
        </button>
      </div>
    </div>
  );
}
