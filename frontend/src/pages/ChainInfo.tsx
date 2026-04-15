import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { RefreshCw, CheckCircle2, AlertCircle, ExternalLink } from 'lucide-react';
import { api } from '../lib/api';
import { PageHeader } from '../components/PageHeader';

function formatGwei(wei: string): string {
  if (!wei || wei === '0') return '0';
  const n = BigInt(wei);
  const gwei = Number(n) / 1e9;
  if (gwei < 0.001) return gwei.toExponential(2);
  if (gwei < 1) return gwei.toFixed(4);
  return gwei.toFixed(2);
}

function InfoRow({ label, value, mono, link }: { label: string; value: React.ReactNode; mono?: boolean; link?: string }) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-start gap-1 sm:gap-4 py-3 border-b border-neutral-100 last:border-b-0">
      <dt className="text-sm text-neutral-500 sm:w-48 shrink-0">{label}</dt>
      <dd className={`text-sm text-neutral-900 break-all ${mono ? 'font-mono' : ''}`}>
        {link ? (
          <Link to={link} className="text-primary hover:text-primary-600 transition-colors">
            {value}
          </Link>
        ) : (
          value
        )}
      </dd>
    </div>
  );
}

export default function ChainInfo() {
  const { data, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: ['chainInfo'],
    queryFn: api.getChainInfo,
    staleTime: 60000,
    refetchInterval: 60000,
  });

  const { data: syncStatus } = useQuery({
    queryKey: ['syncStatus'],
    queryFn: api.getSyncStatus,
    staleTime: 10000,
    refetchInterval: 10000,
  });

  if (isLoading) {
    return (
      <div className="space-y-6">
        <PageHeader title="Chain Info" />
        <div className="text-neutral-400">Loading chain info...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Chain Info" />
        <div className="text-error-600">Failed to load chain info</div>
      </div>
    );
  }

  const latestChainBlock = syncStatus?.latestChainBlock ?? data?.latestBlock;
  const lastIndexed = syncStatus?.syncStatus?.lastIndexedBlock;
  const blocksRemaining = syncStatus?.blocksRemaining;

  return (
    <div className="space-y-6">
      <PageHeader title="Chain Info">
        <div className="flex items-center gap-3">
          <span className="text-sm text-neutral-500">
            Updated {data?.updatedAt ? new Date(data.updatedAt).toLocaleTimeString() : '-'}
          </span>
          <button
            onClick={() => refetch()}
            disabled={isFetching}
            className="p-2 rounded-lg bg-neutral-100 hover:bg-neutral-200 border border-neutral-200 transition-colors disabled:opacity-50"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 text-neutral-600 ${isFetching ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </PageHeader>

      {/* Status Banner */}
      <div className={`flex items-center gap-3 px-4 py-3 rounded-xl border ${
        data?.isSyncing
          ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-300'
          : 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800 text-green-800 dark:text-green-300'
      }`}>
        {data?.isSyncing ? (
          <>
            <AlertCircle className="w-5 h-5 shrink-0" />
            <span className="text-sm font-medium">Node is syncing</span>
          </>
        ) : (
          <>
            <CheckCircle2 className="w-5 h-5 shrink-0" />
            <span className="text-sm font-medium">Node is fully synced</span>
          </>
        )}
      </div>

      {/* Network Identity */}
      <div className="card">
        <div className="px-5 py-4 border-b border-neutral-100">
          <h2 className="text-base font-semibold text-neutral-900">Network Identity</h2>
        </div>
        <div className="px-5">
          <dl>
            <InfoRow label="Chain ID" value={`${data?.chainIdDecimal ?? '-'} (${data?.chainId ?? '-'})`} mono />
            <InfoRow label="Network ID" value={data?.networkId || '-'} mono />
            <InfoRow label="Client Version" value={data?.clientVersion || '-'} />
            <InfoRow label="Protocol Version" value={data?.protocolVersion || '-'} mono />
            <InfoRow label="Genesis Hash" value={data?.genesisHash || '-'} mono />
          </dl>
        </div>
      </div>

      {/* Node Status */}
      <div className="card">
        <div className="px-5 py-4 border-b border-neutral-100">
          <h2 className="text-base font-semibold text-neutral-900">Node Status</h2>
        </div>
        <div className="px-5">
          <dl>
            <InfoRow
              label="Latest Block"
              value={latestChainBlock?.toLocaleString() ?? '-'}
              link={latestChainBlock ? `/block/${latestChainBlock}` : undefined}
            />
            {lastIndexed !== undefined && (
              <InfoRow
                label="Last Indexed Block"
                value={
                  <span>
                    {lastIndexed.toLocaleString()}
                    {blocksRemaining !== undefined && blocksRemaining > 0 && (
                      <span className="ml-2 text-amber-600 text-xs">
                        ({blocksRemaining.toLocaleString()} behind)
                      </span>
                    )}
                  </span>
                }
                link={`/block/${lastIndexed}`}
              />
            )}
            <InfoRow label="Gas Price" value={data?.gasPrice ? `${formatGwei(data.gasPrice)} Gwei` : '-'} />
            <InfoRow label="Peer Count" value={data?.peerCount?.toString() ?? '-'} />
            <InfoRow
              label="Sync Status"
              value={
                <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  data?.isSyncing
                    ? 'bg-amber-100 text-amber-800'
                    : 'bg-green-100 text-green-800'
                }`}>
                  <span className={`w-1.5 h-1.5 rounded-full ${data?.isSyncing ? 'bg-amber-500' : 'bg-green-500'}`} />
                  {data?.isSyncing ? 'Syncing' : 'Synced'}
                </span>
              }
            />
          </dl>
        </div>
      </div>

      {/* Quick Links */}
      <div className="card">
        <div className="px-5 py-4 border-b border-neutral-100">
          <h2 className="text-base font-semibold text-neutral-900">Explore</h2>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 p-5">
          {[
            { to: '/blocks', label: 'Blocks' },
            { to: '/transactions', label: 'Transactions' },
            { to: '/accounts', label: 'Top Accounts' },
            { to: '/tokens', label: 'Tokens' },
            { to: '/gas-tracker', label: 'Gas Tracker' },
            { to: '/verify', label: 'Verify Contract' },
          ].map(({ to, label }) => (
            <Link
              key={to}
              to={to}
              className="flex items-center gap-2 px-4 py-3 rounded-lg bg-neutral-50 hover:bg-primary-50 dark:hover:bg-primary-900/20 border border-neutral-200 hover:border-primary-200 transition-colors text-sm text-neutral-700 hover:text-primary-700 dark:hover:text-primary-400"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              {label}
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
