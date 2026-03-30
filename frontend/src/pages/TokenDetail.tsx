import { useState, useMemo } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { TokenHolder, TokenTransfer } from '../lib/api';
import { AddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';
import { PageHeader } from '../components/PageHeader';
import { useBatchAddressVisibility } from '../hooks/useAddressVisibility';

function formatTokenValue(value: string | number, decimals: number): string {
  const strValue = String(value);
  if (!strValue || strValue === '0') return '0';
  try {
    const num = BigInt(strValue);
    const divisor = BigInt(10 ** decimals);
    const wholePart = num / divisor;
    const fracPart = num % divisor;
    if (fracPart === BigInt(0)) {
      return wholePart.toLocaleString();
    }
    const fracStr = fracPart.toString().padStart(decimals, '0').slice(0, 6);
    return `${wholePart.toLocaleString()}.${fracStr}`;
  } catch {
    return strValue;
  }
}

export default function TokenDetail() {
  const { address } = useParams<{ address: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeTab, setActiveTab] = useState<'holders' | 'transfers'>('holders');
  const page = parseInt(searchParams.get('page') || '1', 10);

  const { data: token, isLoading: tokenLoading, error: tokenError } = useQuery({
    queryKey: ['token', address],
    queryFn: () => api.getToken(address!),
    enabled: !!address,
  });

  const { data: holders, isLoading: holdersLoading } = useQuery({
    queryKey: ['tokenHolders', address, page],
    queryFn: () => api.getTokenHolders(address!, page, 25),
    enabled: !!address && activeTab === 'holders',
  });

  const { data: transfers, isLoading: transfersLoading } = useQuery({
    queryKey: ['tokenTransfers', address, page],
    queryFn: () => api.getTokenTransfers(address!, page, 25),
    enabled: !!address && activeTab === 'transfers',
  });

  const handlePageChange = (newPage: number) => {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(newPage));
    setSearchParams(params);
  };

  const handleTabChange = (tab: 'holders' | 'transfers') => {
    setActiveTab(tab);
    setSearchParams({ page: '1' });
  };

  // Collect unique addresses from holders and transfers for batch visibility check
  const uniqueAddresses = useMemo(() => {
    const set = new Set<string>();
    if (holders?.data) {
      for (const h of holders.data) {
        if (h.address && h.address !== '[PRIVATE]') set.add(h.address.toLowerCase());
      }
    }
    if (transfers?.data) {
      for (const t of transfers.data) {
        if (t.from && t.from !== '[PRIVATE]') set.add(t.from.toLowerCase());
        if (t.to && t.to !== '[PRIVATE]') set.add(t.to.toLowerCase());
      }
    }
    return Array.from(set);
  }, [holders, transfers]);

  const { visibilities } = useBatchAddressVisibility(uniqueAddresses);

  if (tokenLoading) {
    return (
      <div className="space-y-6">
        <PageHeader title="Token" />
        <div className="card p-8 text-center text-neutral-400">
          Loading token...
        </div>
      </div>
    );
  }

  if (tokenError && tokenError instanceof Error && (tokenError.message.includes('403') || tokenError.message.includes('500'))) {
    return (
      <div className="space-y-6">
        <PageHeader title="Token" />
        <div className="card">
          <div className="flex flex-col items-center justify-center py-16 space-y-4">
            <div className="w-16 h-16 rounded-full bg-neutral-100 flex items-center justify-center border border-neutral-200">
              <svg className="w-8 h-8 text-neutral-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" /></svg>
            </div>
            <h2 className="text-xl font-semibold text-neutral-900">Token Restricted</h2>
            <p className="text-neutral-500 text-center max-w-md text-sm">
              This token belongs to a private organization. Sign in to view it if you have access.
            </p>
          </div>
        </div>
      </div>
    );
  }
  if (tokenError || !token) {
    return (
      <div className="space-y-6">
        <PageHeader title="Token" />
        <div className="card p-8 text-center text-error-600">
          Token not found
        </div>
      </div>
    );
  }

  const currentData = activeTab === 'holders' ? holders : transfers;
  const isTabLoading = activeTab === 'holders' ? holdersLoading : transfersLoading;
  const totalPages = currentData?.totalPages || 1;

  return (
    <div className="space-y-6">
      <PageHeader title={`${token.symbol}${token.name ? ` (${token.name})` : ''}`}>
        <span className={`badge ${
          token.tokenType === 'ERC721'
            ? 'badge-primary'
            : 'badge-neutral'
        }`}>
          {token.tokenType}
        </span>
      </PageHeader>

      {/* Token Info Card */}
      <div className="card">
        <div className="px-6 py-4 border-b border-neutral-100">
          <div className="flex items-center gap-3">
            <h2 className="text-lg font-semibold text-neutral-900">Token Info</h2>
          </div>
        </div>
        <div className="p-6">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <div className="bg-neutral-50 rounded-lg p-4 border border-neutral-100">
              <div className="text-sm text-neutral-500">Contract Address</div>
              <AddressLink address={token.address} className="text-neutral-900 break-all" />
            </div>
            <div className="bg-neutral-50 rounded-lg p-4 border border-neutral-100">
              <div className="text-sm text-neutral-500">Decimals</div>
              <div className="text-neutral-900 font-mono">{token.decimals}</div>
            </div>
            <div className="bg-neutral-50 rounded-lg p-4 border border-neutral-100">
              <div className="text-sm text-neutral-500">Total Supply</div>
              <div className="text-neutral-900 font-mono">
                {formatTokenValue(token.totalSupply || '0', token.decimals)}
              </div>
            </div>
            <div className="bg-neutral-50 rounded-lg p-4 border border-neutral-100">
              <div className="text-sm text-neutral-500">Holders</div>
              <div className="text-neutral-900 font-mono">{token.holderCount.toLocaleString()}</div>
            </div>
            <div className="bg-neutral-50 rounded-lg p-4 border border-neutral-100">
              <div className="text-sm text-neutral-500">Transfers</div>
              <div className="text-neutral-900 font-mono">{token.transferCount.toLocaleString()}</div>
            </div>
            {token.usdPrice && (
              <div className="bg-neutral-50 rounded-lg p-4 border border-neutral-100">
                <div className="text-sm text-neutral-500">Price</div>
                <div className="text-neutral-900 font-mono">${token.usdPrice.toFixed(4)}</div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Tabs Card */}
      <div className="card">
        <div className="tabs px-4">
          <button
            onClick={() => handleTabChange('holders')}
            className={activeTab === 'holders' ? 'tab-active' : 'tab'}
          >
            Holders ({token.holderCount})
          </button>
          <button
            onClick={() => handleTabChange('transfers')}
            className={activeTab === 'transfers' ? 'tab-active' : 'tab'}
          >
            Transfers ({token.transferCount})
          </button>
        </div>

        <div className="p-4">
          {isTabLoading ? (
            <div className="text-center text-neutral-400 py-8">Loading...</div>
          ) : activeTab === 'holders' ? (
            /* Holders Table */
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Rank</th>
                    <th>Address</th>
                    <th>Balance</th>
                    <th>Percentage</th>
                  </tr>
                </thead>
                <tbody>
                  {(holders?.data || []).map((holder: TokenHolder, index: number) => {
                    const holderVis = visibilities[holder.address?.toLowerCase()];
                    return (
                    <tr key={holder.address}>
                      <td className="text-neutral-400">
                        {(page - 1) * 25 + index + 1}
                      </td>
                      <td>
                        <div className="flex items-center gap-2">
                          <AddressLink address={holder.address} visibility={holderVis} />
                          <AddressLabel reason={holderVis?.reason} />
                          {holder.isContract && (
                            <span className="badge badge-neutral text-xs">
                              Contract
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="text-neutral-700 font-mono">
                        {formatTokenValue(holder.balance, token.decimals)}
                      </td>
                      <td className="text-neutral-700">
                        {holder.percentage.toFixed(2)}%
                      </td>
                    </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            /* Transfers Table */
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Tx Hash</th>
                    <th>From</th>
                    <th>To</th>
                    <th>Value</th>
                    <th>Block</th>
                  </tr>
                </thead>
                <tbody>
                  {(transfers?.data || []).map((transfer: TokenTransfer) => {
                    const fromVis = visibilities[transfer.from?.toLowerCase()];
                    const toVis = visibilities[transfer.to?.toLowerCase()];
                    return (
                    <tr key={`${transfer.txHash}-${transfer.logIndex}`}>
                      <td>
                        <a
                          href={`/tx/${transfer.txHash}`}
                          className="text-primary hover:text-primary-600 font-mono text-sm transition-colors"
                        >
                          {transfer.txHash.slice(0, 10)}...
                        </a>
                      </td>
                      <td>
                        <span className="inline-flex items-center gap-1">
                          <AddressLink address={transfer.from} visibility={fromVis} />
                          <AddressLabel reason={fromVis?.reason} />
                        </span>
                      </td>
                      <td>
                        <span className="inline-flex items-center gap-1">
                          <AddressLink address={transfer.to} visibility={toVis} />
                          <AddressLabel reason={toVis?.reason} />
                        </span>
                      </td>
                      <td className="text-neutral-700 font-mono">
                        {formatTokenValue(transfer.value, token.decimals)}
                      </td>
                      <td>
                        <a
                          href={`/block/${transfer.blockNumber}`}
                          className="text-primary hover:text-primary-600 transition-colors"
                        >
                          {transfer.blockNumber}
                        </a>
                      </td>
                    </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}

          {/* Pagination */}
          {(currentData?.data || []).length > 0 && (
            <div className="flex justify-between items-center mt-4 pt-4 border-t border-neutral-100">
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
          )}
        </div>
      </div>
    </div>
  );
}
