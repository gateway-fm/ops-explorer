import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { Boxes, ArrowLeftRight, Users, Clock } from 'lucide-react';
import { api } from '../lib/api';
import { PageHeader } from '../components/PageHeader';
import { features } from '../lib/features';
import { FeatureUnavailable } from '../components/FeatureUnavailable';

// ---------------------------------------------------------------------------
// Chart definitions — hardcoded, mapping to backend chart line IDs
// ---------------------------------------------------------------------------

interface ChartDef {
  id: string;
  title: string;
  description: string;
  section: string;
  color: string;
  units: string;
}

const CHART_DEFINITIONS: ChartDef[] = [
  // Transactions
  { id: 'new_txns', title: 'Daily Transactions', description: 'Number of transactions per day', section: 'transactions', color: '#EA580C', units: '' },
  { id: 'txns_growth', title: 'Cumulative Transactions', description: 'Total transactions over time', section: 'transactions', color: '#EA580C', units: '' },
  { id: 'avg_txn_fee', title: 'Average Transaction Fee', description: 'Average gas price per transaction', section: 'transactions', color: '#F59E0B', units: 'Gwei' },
  { id: 'txns_success_rate', title: 'Transaction Success Rate', description: 'Percentage of successful transactions', section: 'transactions', color: '#22C55E', units: '%' },

  // Blocks
  { id: 'new_blocks', title: 'Daily Blocks', description: 'Number of blocks produced per day', section: 'blocks', color: '#3B82F6', units: '' },
  { id: 'avg_block_size', title: 'Average Block Size', description: 'Average block size in bytes', section: 'blocks', color: '#3B82F6', units: 'bytes' },
  { id: 'avg_block_time', title: 'Average Block Time', description: 'Average time between blocks', section: 'blocks', color: '#3B82F6', units: 's' },

  // Accounts
  { id: 'active_accounts', title: 'Active Accounts', description: 'Unique addresses active per day', section: 'accounts', color: '#8B5CF6', units: '' },
  { id: 'new_accounts', title: 'New Accounts', description: 'New addresses seen per day', section: 'accounts', color: '#8B5CF6', units: '' },
  { id: 'accounts_growth', title: 'Total Accounts', description: 'Cumulative unique addresses over time', section: 'accounts', color: '#8B5CF6', units: '' },

  // Gas
  { id: 'avg_gas_price', title: 'Average Gas Price', description: 'Average gas price in Gwei', section: 'gas', color: '#EF4444', units: 'Gwei' },
  { id: 'avg_gas_used', title: 'Average Gas Used', description: 'Average gas used per block', section: 'gas', color: '#EF4444', units: '' },
  { id: 'gas_used_growth', title: 'Cumulative Gas Used', description: 'Total gas used over time', section: 'gas', color: '#EF4444', units: '' },

  // Contracts
  { id: 'new_contracts', title: 'New Contracts', description: 'Contract deployments per day', section: 'contracts', color: '#06B6D4', units: '' },
  { id: 'contracts_growth', title: 'Total Contracts', description: 'Cumulative contracts over time', section: 'contracts', color: '#06B6D4', units: '' },

  // Tokens
  { id: 'new_token_transfers', title: 'Token Transfers', description: 'Number of token transfers per day', section: 'tokens', color: '#F97316', units: '' },
];

// ---------------------------------------------------------------------------
// Sections & time ranges
// ---------------------------------------------------------------------------

const SECTIONS = ['all', 'transactions', 'blocks', 'accounts', 'gas', 'contracts', 'tokens'] as const;
type Section = (typeof SECTIONS)[number];

const SECTION_LABELS: Record<Section, string> = {
  all: 'All',
  transactions: 'Transactions',
  blocks: 'Blocks',
  accounts: 'Accounts',
  gas: 'Gas',
  contracts: 'Contracts',
  tokens: 'Tokens',
};

interface TimeRange {
  label: string;
  days: number | null; // null = all
}

const TIME_RANGES: TimeRange[] = [
  { label: '7D', days: 7 },
  { label: '1M', days: 30 },
  { label: '3M', days: 90 },
  { label: '6M', days: 180 },
  { label: '1Y', days: 365 },
  { label: 'All', days: null },
];

// ---------------------------------------------------------------------------
// Value formatting helpers
// ---------------------------------------------------------------------------

function formatChartValue(value: number, units: string): string {
  if (units === '%') {
    return `${value.toFixed(1)}%`;
  }
  if (units === 'Gwei') {
    return value < 1 ? value.toFixed(4) : value < 100 ? value.toFixed(2) : formatCompact(value);
  }
  if (units === 'bytes') {
    if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)} MB`;
    if (value >= 1_000) return `${(value / 1_000).toFixed(1)} KB`;
    return `${value} B`;
  }
  if (units === 's') {
    return `${value.toFixed(2)}s`;
  }
  return formatCompact(value);
}

function formatCompact(value: number): string {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`;
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  if (Number.isInteger(value)) return value.toLocaleString();
  return value.toFixed(2);
}

function formatAxisValue(value: number, units: string): string {
  if (units === '%') return `${value.toFixed(0)}%`;
  if (units === 's') return `${value.toFixed(1)}s`;
  if (units === 'Gwei') {
    if (value >= 1_000) return `${(value / 1_000).toFixed(0)}K`;
    return value.toFixed(0);
  }
  if (units === 'bytes') {
    if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(0)}M`;
    if (value >= 1_000) return `${(value / 1_000).toFixed(0)}K`;
    return String(value);
  }
  return formatCompact(value);
}

// ---------------------------------------------------------------------------
// Date helpers
// ---------------------------------------------------------------------------

function formatDateStr(isoDate: string): string {
  const d = new Date(isoDate);
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function getDateRange(days: number | null): { from?: string; to?: string } {
  if (days === null) return {};
  const to = new Date();
  const from = new Date();
  from.setDate(from.getDate() - days);
  return {
    from: from.toISOString().split('T')[0],
    to: to.toISOString().split('T')[0],
  };
}

// ---------------------------------------------------------------------------
// ChartWidget
// ---------------------------------------------------------------------------

function ChartWidget({ def, from, to }: { def: ChartDef; from?: string; to?: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['chart-line', def.id, from, to],
    queryFn: () => api.getChartLine(def.id, from, to),
    staleTime: 60_000,
  });

  const gradientId = `grad-${def.id}`;
  const chartData = data?.chart ?? [];

  return (
    <div className="card p-4">
      <div className="mb-3">
        <h3 className="text-sm font-semibold text-neutral-900">{def.title}</h3>
        <p className="text-xs text-neutral-500">{def.description}</p>
      </div>

      {isLoading ? (
        <div className="space-y-2" style={{ height: 200 }}>
          <div className="skeleton h-4 w-1/3 rounded" />
          <div className="skeleton h-full w-full rounded" />
        </div>
      ) : chartData.length === 0 ? (
        <div className="flex items-center justify-center text-neutral-400 text-sm" style={{ height: 200 }}>
          No data available
        </div>
      ) : (
        <div style={{ height: 200 }}>
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={chartData} margin={{ top: 5, right: 5, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor={def.color} stopOpacity={0.3} />
                  <stop offset="95%" stopColor={def.color} stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="date"
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 10, fill: '#9CA3AF' }}
                interval="preserveStartEnd"
                tickFormatter={formatDateStr}
                dy={5}
              />
              <YAxis
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 10, fill: '#9CA3AF' }}
                width={45}
                tickFormatter={(v: number) => formatAxisValue(v, def.units)}
              />
              <Tooltip content={<ChartTooltip units={def.units} />} />
              <Area
                type="monotone"
                dataKey="value"
                stroke={def.color}
                strokeWidth={2}
                fill={`url(#${gradientId})`}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Chart tooltip
// ---------------------------------------------------------------------------

interface TooltipPayloadItem {
  payload: { date: string; value: number };
}

function ChartTooltip({
  active,
  payload,
  units,
}: {
  active?: boolean;
  payload?: TooltipPayloadItem[];
  units: string;
}) {
  if (!active || !payload || payload.length === 0) return null;
  const point = payload[0].payload;
  const d = new Date(point.date);
  const dateStr = d.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });

  return (
    <div className="rounded-lg bg-neutral-900 px-3 py-2 shadow-lg text-white text-xs">
      <p className="text-neutral-400 mb-0.5">{dateStr}</p>
      <p className="font-medium">
        {formatChartValue(point.value, units)}
        {units && units !== '%' && units !== 's' ? ` ${units}` : ''}
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Counter / stat cards
// ---------------------------------------------------------------------------

const STAT_COLOR_STYLES = {
  blue: 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-800',
  green: 'bg-success-50 dark:bg-success-500/10 text-success-600 dark:text-success-500 border border-success-100 dark:border-success-500/20',
  purple: 'bg-primary-50 dark:bg-primary-900/20 text-primary dark:text-primary-400 border border-primary-200 dark:border-primary-800',
  amber: 'bg-warning-50 dark:bg-warning-500/10 text-warning-600 dark:text-warning-500 border border-warning-100 dark:border-warning-500/20',
} as const;

function CounterCard({
  label,
  value,
  icon,
  color,
}: {
  label: string;
  value: string;
  icon: React.ReactNode;
  color: keyof typeof STAT_COLOR_STYLES;
}) {
  return (
    <div className="card p-3 sm:p-4">
      <div className="flex items-center justify-between mb-2 sm:mb-3">
        <span className="text-xs sm:text-sm text-neutral-500">{label}</span>
        <div className={`p-1.5 sm:p-2 rounded-lg ${STAT_COLOR_STYLES[color]}`}>{icon}</div>
      </div>
      <div className="text-lg sm:text-2xl font-mono font-semibold text-neutral-900 truncate">
        {value}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Stats page
// ---------------------------------------------------------------------------

export default function Stats() {
  // RD-1063: feature gate must be the very first statement — before any hook —
  // so the disabled branch never subscribes the stats/chart-line queries and
  // fires no /charts requests in privacy mode. features() is not a hook, so an
  // early return here is safe (no conditional-hook violation).
  if (!features().charts) return <FeatureUnavailable feature="Charts" />;

  const [section, setSection] = useState<Section>('all');
  const [rangeIdx, setRangeIdx] = useState(1); // default 1M

  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: api.getStats,
    refetchInterval: 30_000,
  });

  const range = TIME_RANGES[rangeIdx];
  const { from, to } = useMemo(() => getDateRange(range.days), [range.days]);

  const visibleCharts = useMemo(
    () =>
      section === 'all'
        ? CHART_DEFINITIONS
        : CHART_DEFINITIONS.filter((c) => c.section === section),
    [section],
  );

  return (
    <div className="space-y-6">
      <PageHeader title="Charts & Stats" />

      {/* Counter cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2 sm:gap-4">
        <CounterCard
          label="Total Blocks"
          value={stats?.totalBlocks?.toLocaleString() ?? '-'}
          icon={<Boxes className="w-5 h-5" />}
          color="blue"
        />
        <CounterCard
          label="Total Transactions"
          value={stats?.totalTransactions?.toLocaleString() ?? '-'}
          icon={<ArrowLeftRight className="w-5 h-5" />}
          color="green"
        />
        <CounterCard
          label="Total Addresses"
          value={stats?.totalAddresses?.toLocaleString() ?? '-'}
          icon={<Users className="w-5 h-5" />}
          color="purple"
        />
        <CounterCard
          label="Avg Block Time"
          value={stats?.avgBlockTime ? `${stats.avgBlockTime.toFixed(2)}s` : '-'}
          icon={<Clock className="w-5 h-5" />}
          color="amber"
        />
      </div>

      {/* Section filter + time range */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        {/* Section pills */}
        <div className="flex flex-wrap gap-2">
          {SECTIONS.map((s) => (
            <button
              key={s}
              onClick={() => setSection(s)}
              className={`px-3 py-1.5 rounded-full text-xs font-medium transition-colors ${
                section === s
                  ? 'bg-primary text-white'
                  : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'
              }`}
            >
              {SECTION_LABELS[s]}
            </button>
          ))}
        </div>

        {/* Time range pills */}
        <div className="flex gap-1.5">
          {TIME_RANGES.map((tr, idx) => (
            <button
              key={tr.label}
              onClick={() => setRangeIdx(idx)}
              className={`px-3 py-1.5 rounded-full text-xs font-medium transition-colors ${
                rangeIdx === idx
                  ? 'bg-primary text-white'
                  : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200'
              }`}
            >
              {tr.label}
            </button>
          ))}
        </div>
      </div>

      {/* Charts grid */}
      <div className="grid md:grid-cols-2 gap-4 sm:gap-6">
        {visibleCharts.map((def) => (
          <ChartWidget key={def.id} def={def} from={from} to={to} />
        ))}
      </div>
    </div>
  );
}
