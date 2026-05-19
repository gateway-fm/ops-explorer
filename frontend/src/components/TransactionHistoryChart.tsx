import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { api } from '../lib/api';
import type { TxHistoryPoint } from '../lib/api';
import { branding } from '../lib/branding';

const RANGE_STORAGE_KEY = 'tx-history-range';

interface ChartDataPoint {
  time: string;
  count: number;
  timestamp: number;
}

type TimeRange = '1h' | '24h' | '7d' | '30d';

// Time range configurations
const TIME_RANGES: Record<TimeRange, { interval: number; limit: number; label: string; refetchInterval: number }> = {
  '1h': {
    interval: 60,        // 1 minute intervals
    limit: 60,           // 60 points = 1 hour
    label: 'Last hour',
    refetchInterval: 15000,  // Refresh every 15 seconds
  },
  '24h': {
    interval: 3600,      // 1 hour intervals
    limit: 24,           // 24 points = 24 hours
    label: 'Last 24 hours',
    refetchInterval: 60000,  // Refresh every minute
  },
  '7d': {
    interval: 21600,     // 6 hour intervals
    limit: 28,           // 28 points = 7 days
    label: 'Last 7 days',
    refetchInterval: 300000, // Refresh every 5 minutes
  },
  '30d': {
    interval: 86400,     // 1 day intervals
    limit: 30,           // 30 points = 30 days
    label: 'Last 30 days',
    refetchInterval: 600000, // Refresh every 10 minutes
  },
};

function loadStoredRange(): TimeRange {
  try {
    const v = localStorage.getItem(RANGE_STORAGE_KEY);
    if (v === '1h' || v === '24h' || v === '7d' || v === '30d') return v;
  } catch {
    // localStorage unavailable in private mode
  }
  return '30d';
}

function formatTime(timestamp: number, timeRange: TimeRange): string {
  const date = new Date(timestamp * 1000);

  switch (timeRange) {
    case '1h':
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    case '24h':
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    case '7d':
      return date.toLocaleDateString([], { weekday: 'short', day: 'numeric' });
    case '30d':
      return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
    default:
      return date.toLocaleString();
  }
}

function CustomTooltip({ active, payload, timeRange }: { active?: boolean; payload?: Array<{ payload: ChartDataPoint }>; timeRange: TimeRange }) {
  if (active && payload && payload.length) {
    const data = payload[0].payload;
    const date = new Date(data.timestamp * 1000);

    let dateFormat: string;
    switch (timeRange) {
      case '1h':
        dateFormat = date.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
        break;
      case '24h':
        dateFormat = date.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
        break;
      case '7d':
        dateFormat = date.toLocaleString([], { weekday: 'short', month: 'short', day: 'numeric', hour: '2-digit' });
        break;
      case '30d':
        dateFormat = date.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' });
        break;
      default:
        dateFormat = date.toLocaleString();
    }

    return (
      <div className="card px-3 py-2 shadow-elevated">
        <p className="text-neutral-500 text-xs">{dateFormat}</p>
        <p className="text-neutral-900 font-medium">{data.count.toLocaleString()} transactions</p>
      </div>
    );
  }
  return null;
}

export function TransactionHistoryChart() {
  const [timeRange, setTimeRange] = useState<TimeRange>(loadStoredRange);
  const config = TIME_RANGES[timeRange];

  const { data: history, isLoading } = useQuery({
    queryKey: ['tx-history', timeRange],
    queryFn: () => api.getTransactionHistory(config.interval, config.limit),
    refetchInterval: config.refetchInterval,
  });

  const handleRangeChange = (range: TimeRange) => {
    setTimeRange(range);
    try {
      localStorage.setItem(RANGE_STORAGE_KEY, range);
    } catch {
      // localStorage unavailable in private mode
    }
  };

  const primaryColor = branding.colorPrimary || '#8950FA';

  if (isLoading || !history || history.length === 0) {
    return (
      <div className="card p-4 h-full flex flex-col">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-medium text-neutral-500">Transaction History</h3>
          <TimeRangeSelect value={timeRange} onChange={handleRangeChange} />
        </div>
        <div className="flex-1 flex items-center justify-center text-neutral-400 text-sm min-h-[120px]">
          {isLoading ? 'Loading...' : 'No data available'}
        </div>
      </div>
    );
  }

  const chartData: ChartDataPoint[] = history.map((point: TxHistoryPoint) => ({
    time: formatTime(point.timestamp, timeRange),
    count: point.count,
    timestamp: point.timestamp,
  }));

  // Calculate total for display
  const totalTxs = chartData.reduce((sum, point) => sum + point.count, 0);

  return (
    <div className="card p-4 h-full flex flex-col">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-medium text-neutral-500">Transaction History</h3>
        <div className="flex items-center gap-3">
          <TimeRangeSelect value={timeRange} onChange={handleRangeChange} />
          <span className="text-xs font-medium text-neutral-600">{totalTxs.toLocaleString()} txs</span>
        </div>
      </div>
      <div className="flex-1 min-h-[120px]">
        <ResponsiveContainer width="100%" height="100%" minWidth={100} minHeight={100}>
          <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id="txGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor={primaryColor} stopOpacity={0.3} />
                <stop offset="95%" stopColor={primaryColor} stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis
              dataKey="time"
              axisLine={false}
              tickLine={false}
              tick={{ fontSize: 10, fill: '#6B7280' }}
              interval="preserveStartEnd"
              dy={5}
            />
            <YAxis
              axisLine={false}
              tickLine={false}
              tick={{ fontSize: 10, fill: '#6B7280' }}
              width={35}
              allowDecimals={false}
              tickFormatter={(value) => value >= 1000 ? `${(value / 1000).toFixed(0)}k` : value}
            />
            <Tooltip content={<CustomTooltip timeRange={timeRange} />} />
            <Area
              type="monotone"
              dataKey="count"
              stroke={primaryColor}
              strokeWidth={2}
              fill="url(#txGradient)"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function TimeRangeSelect({ value, onChange }: { value: TimeRange; onChange: (value: TimeRange) => void }) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value as TimeRange)}
      className="text-xs bg-neutral-100 border border-neutral-200 rounded px-2 py-1 text-neutral-600 hover:bg-neutral-200 transition-colors cursor-pointer"
    >
      <option value="1h">Last hour</option>
      <option value="24h">Last 24 hours</option>
      <option value="7d">Last 7 days</option>
      <option value="30d">Last 30 days</option>
    </select>
  );
}
