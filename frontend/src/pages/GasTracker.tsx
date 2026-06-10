import { useQuery } from '@tanstack/react-query';
import { Fuel, Zap, Clock, Gauge, RefreshCw } from 'lucide-react';
import { api } from '../lib/api';
import type { GasPrice } from '../lib/api';
import { PageHeader } from '../components/PageHeader';
import { features } from '../lib/features';
import { FeatureUnavailable } from '../components/FeatureUnavailable';

export function GasTracker() {
  // RD-1063: hooks must run unconditionally (react-hooks/rules-of-hooks), but
  // the disabled branch must still fire NO /gas request in privacy mode. So the
  // query is gated via `enabled` and the FeatureUnavailable early-return moves
  // below the hook. features() is a plain config read, stable per session.
  const gasEnabled = features().gasTracker;

  const { data, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: ['gasPrices'],
    queryFn: api.getGasPrices,
    refetchInterval: 15000, // Refresh every 15s
    enabled: gasEnabled,
  });

  if (!gasEnabled) return <FeatureUnavailable feature="Gas Tracker" />;

  if (isLoading) {
    return (
      <div className="space-y-6">
        <PageHeader title="Gas Tracker" />
        <div className="text-neutral-400">Loading gas prices...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Gas Tracker" />
        <div className="text-error-600">Failed to load gas prices</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader title="Gas Tracker">
        <div className="flex items-center gap-3">
          <span className="text-sm text-neutral-500">
            Updated {data?.updatedAt ? new Date(data.updatedAt).toLocaleTimeString() : '-'}
          </span>
          <button
            onClick={() => refetch()}
            disabled={isFetching}
            className="p-2 rounded-lg bg-neutral-100 hover:bg-neutral-200 border border-neutral-200 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 text-neutral-600 ${isFetching ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </PageHeader>

      <div className="grid md:grid-cols-3 gap-4">
        <GasPriceCard
          label="Slow"
          icon={<Clock className="w-5 h-5" />}
          price={data?.slow}
          color="amber"
          description="Lower priority"
        />
        <GasPriceCard
          label="Normal"
          icon={<Gauge className="w-5 h-5" />}
          price={data?.normal}
          color="blue"
          description="Standard speed"
        />
        <GasPriceCard
          label="Fast"
          icon={<Zap className="w-5 h-5" />}
          price={data?.fast}
          color="green"
          description="High priority"
        />
      </div>

      {/* Info section */}
      <div className="card p-6">
        <div className="flex items-start gap-3">
          <div className="p-2 rounded-lg bg-primary-50 border border-primary-200">
            <Fuel className="w-5 h-5 text-primary-600" />
          </div>
          <div>
            <h3 className="font-semibold text-neutral-900 mb-1">How Gas Prices Are Calculated</h3>
            <p className="text-sm text-neutral-600 leading-relaxed">
              Gas prices are calculated using percentile analysis of recent transactions.
              <strong> Slow</strong> uses the 35th percentile, <strong>Normal</strong> the 60th percentile,
              and <strong>Fast</strong> the 90th percentile. This approach provides more accurate estimates
              than simple averages because it's resilient to outliers.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

function GasPriceCard({
  label,
  icon,
  price,
  color,
  description
}: {
  label: string;
  icon: React.ReactNode;
  price: GasPrice | null | undefined;
  color: 'amber' | 'blue' | 'green';
  description: string;
}) {
  const colorStyles = {
    amber: {
      bg: 'bg-amber-50',
      text: 'text-amber-600',
      border: 'border-amber-200',
    },
    blue: {
      bg: 'bg-blue-50',
      text: 'text-blue-600',
      border: 'border-blue-200',
    },
    green: {
      bg: 'bg-green-50',
      text: 'text-green-600',
      border: 'border-green-200',
    },
  };

  const styles = colorStyles[color];

  return (
    <div className="card p-6" data-testid={`gas-card-${label.toLowerCase()}`}>
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm font-medium text-neutral-500">{label}</span>
        <div className={`p-2 rounded-lg border ${styles.bg} ${styles.text} ${styles.border}`}>
          {icon}
        </div>
      </div>
      <div className="text-3xl font-bold text-neutral-900 mb-1">
        {price?.price !== undefined ? price.price.toFixed(2) : '-'}{' '}
        <span className="text-lg font-normal text-neutral-500">Gwei</span>
      </div>
      <div className="text-sm text-neutral-500 mb-3">{description}</div>
      {price?.baseFee !== undefined && (
        <div className="pt-3 border-t border-neutral-100 space-y-1">
          <div className="flex justify-between text-xs text-neutral-500">
            <span>Base Fee</span>
            <span className="font-medium text-neutral-700">{price.baseFee.toFixed(2)} Gwei</span>
          </div>
          {price.priorityFee !== undefined && (
            <div className="flex justify-between text-xs text-neutral-500">
              <span>Priority Fee</span>
              <span className="font-medium text-neutral-700">{price.priorityFee.toFixed(2)} Gwei</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default GasTracker;
