import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import type { Token } from '../lib/api';
import { AddressLink } from '../components/AddressLink';
import { PageHeader } from '../components/PageHeader';
import { formatTokenValue } from '../lib/formatToken';

export default function Tokens() {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = parseInt(searchParams.get('page') || '1', 10);
  const tokenType = searchParams.get('type') || '';

  const { data, isLoading, error } = useQuery({
    queryKey: ['tokens', page, tokenType],
    queryFn: () => api.getTokens(page, 25, tokenType || undefined),
  });

  const handlePageChange = (newPage: number) => {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(newPage));
    setSearchParams(params);
  };

  const handleTypeFilter = (type: string) => {
    const params = new URLSearchParams();
    if (type) params.set('type', type);
    params.set('page', '1');
    setSearchParams(params);
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <PageHeader title="Tokens" />
        <div className="card p-8 text-center text-neutral-400">
          Loading tokens...
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Tokens" />
        <div className="card p-8 text-center text-error-600">
          Error loading tokens
        </div>
      </div>
    );
  }

  const tokens = data?.data || [];
  const totalPages = data?.totalPages || 1;

  return (
    <div className="space-y-6">
      <PageHeader title="Tokens">
        <div className="flex gap-2">
          <button
            onClick={() => handleTypeFilter('')}
            className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
              !tokenType
                ? 'bg-primary text-white'
                : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'
            }`}
          >
            All
          </button>
          <button
            onClick={() => handleTypeFilter('ERC20')}
            className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
              tokenType === 'ERC20'
                ? 'bg-primary text-white'
                : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'
            }`}
          >
            ERC-20
          </button>
          <button
            onClick={() => handleTypeFilter('ERC721')}
            className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
              tokenType === 'ERC721'
                ? 'bg-primary text-white'
                : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'
            }`}
          >
            ERC-721 (NFT)
          </button>
        </div>
      </PageHeader>

      <div className="card">
        {tokens.length === 0 ? (
          <div className="text-center text-neutral-400 py-8">
            No tokens found
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Token</th>
                    <th>Type</th>
                    <th>Holders</th>
                    <th>Transfers</th>
                    <th className="hidden md:table-cell">Total Supply</th>
                  </tr>
                </thead>
                <tbody>
                  {tokens.map((token: Token) => (
                    <tr key={token.address}>
                      <td>
                        <div className="flex flex-col">
                          <Link
                            to={`/token/${token.address}`}
                            className="text-primary hover:text-primary-600 font-medium transition-colors"
                          >
                            {token.symbol}
                          </Link>
                          <span className="text-xs text-neutral-400">
                            {token.name || 'Unknown Token'}
                          </span>
                          <AddressLink address={token.address} className="text-xs" chars={8} />
                        </div>
                      </td>
                      <td>
                        <span className={`badge ${
                          token.tokenType === 'ERC721'
                            ? 'badge-primary'
                            : 'badge-neutral'
                        }`}>
                          {token.tokenType}
                        </span>
                      </td>
                      <td className="text-neutral-700">
                        {token.holderCount.toLocaleString()}
                      </td>
                      <td className="text-neutral-700">
                        {token.transferCount.toLocaleString()}
                      </td>
                      <td className="text-neutral-700 hidden md:table-cell font-mono">
                        {formatTokenValue(token.totalSupply, token.decimals)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className="flex justify-between items-center px-4 py-3 border-t border-neutral-100">
              <span className="text-neutral-500 text-sm">
                Page {page} of {totalPages}
              </span>
              <div className="flex gap-2">
                <button
                  onClick={() => handlePageChange(page - 1)}
                  disabled={page <= 1}
                  className="btn-secondary text-sm"
                >
                  Previous
                </button>
                <button
                  onClick={() => handlePageChange(page + 1)}
                  disabled={page >= totalPages}
                  className="btn-secondary text-sm"
                >
                  Next
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
