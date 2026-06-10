import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { ChevronLeft, ChevronRight, FileCode2, Copy, Check } from 'lucide-react';
import { api } from '../lib/api';
import { formatWei, getNetworkCurrency } from '../lib/utils';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';
import { PageHeader } from '../components/PageHeader';
import { StateMessage } from '../components/StateMessage';

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          onClick={handleCopy}
          className="p-1 rounded hover:bg-neutral-100 transition-colors"
        >
          {copied ? (
            <Check className="w-3.5 h-3.5 text-success-600" />
          ) : (
            <Copy className="w-3.5 h-3.5 text-neutral-400" />
          )}
        </button>
      </TooltipTrigger>
      <TooltipContent>{copied ? 'Copied!' : 'Copy address'}</TooltipContent>
    </Tooltip>
  );
}

export function Accounts() {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = parseInt(searchParams.get('page') || '1', 10);
  const pageSize = 25;

  const { data, isLoading, error } = useQuery({
    queryKey: ['accounts', page, pageSize],
    queryFn: () => api.getAccounts(page, pageSize),
  });

  const goToPage = (newPage: number) => {
    setSearchParams({ page: String(newPage) });
  };

  // Calculate total balance for percentage calculation
  const totalBalance = data?.data?.reduce((sum, account) => {
    const balance = typeof account.balance === 'string'
      ? BigInt(account.balance)
      : BigInt(account.balance);
    return sum + balance;
  }, BigInt(0)) || BigInt(0);

  const calculatePercentage = (balance: string | number): string => {
    if (totalBalance === BigInt(0)) return '0.00';
    const balanceBigInt = BigInt(balance);
    // Multiply by 10000 for 2 decimal precision, then divide
    const percentage = (balanceBigInt * BigInt(10000)) / totalBalance;
    return (Number(percentage) / 100).toFixed(2);
  };

  if (isLoading) return <StateMessage variant="loading" />;
  if (error) return <StateMessage variant="error" title="Error loading accounts" />;

  return (
    <div className="space-y-6">
      <PageHeader title="Top Accounts">
        {data && (
          <span className="text-sm text-neutral-500" data-testid="account-total-count">
            {data.total.toLocaleString()} accounts
          </span>
        )}
      </PageHeader>

      <div className="card">
        <div className="overflow-x-auto">
          <table className="table">
            <thead>
              <tr>
                <th className="w-16">Rank</th>
                <th>Address</th>
                <th className="text-right">Balance</th>
                <th className="text-right">Percentage</th>
                <th className="text-right">Txn Count</th>
              </tr>
            </thead>
            <tbody>
              {data?.data?.map((account, index) => (
                <tr key={account.address} data-testid="account-row">
                  <td className="text-neutral-400">
                    {(page - 1) * pageSize + index + 1}
                  </td>
                  <td>
                    <div className="flex items-center gap-2">
                      {account.isContract && (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <FileCode2 className="w-4 h-4 text-neutral-400 flex-shrink-0" />
                          </TooltipTrigger>
                          <TooltipContent>Contract</TooltipContent>
                        </Tooltip>
                      )}
                      <Link
                        to={`/address/${account.address}`}
                        className="font-mono text-primary hover:text-primary-600 transition-colors"
                      >
                        {account.address}
                      </Link>
                      <CopyButton text={account.address} />
                    </div>
                  </td>
                  <td className="text-right font-mono text-neutral-700">
                    {formatWei(account.balance)} {getNetworkCurrency()}
                  </td>
                  <td className="text-right text-neutral-500">
                    {calculatePercentage(account.balance)}%
                  </td>
                  <td className="text-right text-neutral-500">
                    {account.txCount.toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {data && data.totalPages > 1 && (
          <div className="px-4 py-3 border-t border-neutral-100 flex items-center justify-between">
            <div className="text-sm text-neutral-500" data-testid="pagination-status">
              Page {data.page} of {data.totalPages}
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => goToPage(page - 1)}
                disabled={page <= 1}
                data-testid="pagination-prev"
                className="p-2 rounded-lg border border-neutral-200 bg-neutral-50 hover:bg-neutral-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft className="w-4 h-4 text-neutral-500" />
              </button>

              {/* Page numbers */}
              <div className="flex items-center gap-1">
                {Array.from({ length: Math.min(5, data.totalPages) }, (_, i) => {
                  let pageNum: number;
                  if (data.totalPages <= 5) {
                    pageNum = i + 1;
                  } else if (page <= 3) {
                    pageNum = i + 1;
                  } else if (page >= data.totalPages - 2) {
                    pageNum = data.totalPages - 4 + i;
                  } else {
                    pageNum = page - 2 + i;
                  }

                  return (
                    <button
                      key={pageNum}
                      onClick={() => goToPage(pageNum)}
                      data-testid="pagination-page"
                      aria-current={pageNum === page}
                      className={`px-3 py-1 rounded-lg text-sm transition-colors ${
                        pageNum === page
                          ? 'bg-primary text-white'
                          : 'hover:bg-neutral-100 text-neutral-600'
                      }`}
                    >
                      {pageNum}
                    </button>
                  );
                })}
              </div>

              <button
                onClick={() => goToPage(page + 1)}
                disabled={page >= data.totalPages}
                data-testid="pagination-next"
                className="p-2 rounded-lg border border-neutral-200 bg-neutral-50 hover:bg-neutral-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronRight className="w-4 h-4 text-neutral-500" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
