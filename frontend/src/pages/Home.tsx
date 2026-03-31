import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { useState, useEffect, useRef, useMemo } from 'react';
import { Boxes, ArrowLeftRight, Users, Clock, Box, FileCode, FilePlus, Coins, ArrowRightLeft, Shield } from 'lucide-react';
import { api } from '../lib/api';
import type { Block, Transaction, TxCategory } from '../lib/api';
import { formatHash } from '../lib/utils';
import { LiveTimeAgo } from '../components/LiveTimeAgo';
import { AddressLink } from '../components/AddressLink';
import { TransactionHistoryChart } from '../components/TransactionHistoryChart';
import { SearchBar } from '../components/SearchBar';
import { branding } from '../lib/branding';

export function Home() {
  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: api.getStats,
    refetchInterval: 2000,
  });

  const { data: blocks } = useQuery({
    queryKey: ['blocks', 10],
    queryFn: () => api.getBlocks(10),
    refetchInterval: 2000,
  });

  const { data: txs } = useQuery({
    queryKey: ['transactions', 10],
    queryFn: () => api.getTransactions(10),
    refetchInterval: 2000,
  });

  // Removed useBatchAddressVisibility — visibility metadata is now provided natively in API response payloads

  // Track seen blocks and transactions for animations
  const seenBlocks = useRef<Set<number>>(new Set());
  const seenTxs = useRef<Set<string>>(new Set());
  const [newBlocks, setNewBlocks] = useState<Set<number>>(new Set());
  const [newTxs, setNewTxs] = useState<Set<string>>(new Set());
  const isFirstRender = useRef(true);

  useEffect(() => {
    if (!blocks?.data) return;

    if (isFirstRender.current) {
      // On first render, mark all as seen (don't animate)
      blocks.data.forEach(b => seenBlocks.current.add(b.number));
      isFirstRender.current = false;
    } else {
      // Find new blocks
      const newOnes = new Set<number>();
      blocks.data.forEach(b => {
        if (!seenBlocks.current.has(b.number)) {
          newOnes.add(b.number);
          seenBlocks.current.add(b.number);
        }
      });
      if (newOnes.size > 0) {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- animation tracking for new blocks
        setNewBlocks(newOnes);
        // Clear animation class after animation completes
        setTimeout(() => setNewBlocks(new Set()), 2000);
      }
    }
  }, [blocks?.data]);

  useEffect(() => {
    if (!txs?.data) return;

    if (isFirstRender.current) {
      txs.data.forEach(tx => seenTxs.current.add(tx.hash));
      isFirstRender.current = false;
    } else {
      const newOnes = new Set<string>();
      txs.data.forEach(tx => {
        if (!seenTxs.current.has(tx.hash)) {
          newOnes.add(tx.hash);
          seenTxs.current.add(tx.hash);
        }
      });
      if (newOnes.size > 0) {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- animation tracking for new transactions
        setNewTxs(newOnes);
        setTimeout(() => setNewTxs(new Set()), 2000);
      }
    }
  }, [txs?.data]);

  return (
    <div className="space-y-4 sm:space-y-8">
      {/* Hero Section */}
      <div
        className={`relative overflow-visible rounded-xl p-5 sm:p-8 shadow-card ${branding.colorHeroBg ? '' : 'bg-gradient-to-br from-primary-900 via-primary-700 to-primary'}`}
        style={branding.colorHeroBg ? { background: branding.colorHeroBg } : undefined}
      >
        {/* Background decoration */}
        <div className="absolute inset-0 opacity-[0.07] overflow-hidden rounded-xl">
          <div className="absolute top-0 left-0 w-72 h-72 bg-white rounded-full -translate-x-1/2 -translate-y-1/2" />
          <div className="absolute bottom-0 right-0 w-96 h-96 bg-white rounded-full translate-x-1/3 translate-y-1/3" />
        </div>

        <div className="relative z-10">
          <h1 className="text-2xl sm:text-3xl font-bold text-white mb-4">
            {branding.name}
          </h1>

          <SearchBar variant="hero" />
        </div>
      </div>

      {/* Stats + Chart */}
      <div className="grid md:grid-cols-2 gap-4 sm:gap-6">
        <div className="grid grid-cols-2 gap-2 sm:gap-4">
          <StatCard
            label="Total Blocks"
            value={stats?.totalBlocks?.toLocaleString() ?? '-'}
            icon={<Boxes className="w-5 h-5" />}
            color="blue"
          />
          <StatCard
            label="Total Transactions"
            value={stats?.totalTransactions?.toLocaleString() ?? '-'}
            icon={<ArrowLeftRight className="w-5 h-5" />}
            color="green"
          />
          <StatCard
            label="Total Addresses"
            value={stats?.totalAddresses?.toLocaleString() ?? '-'}
            icon={<Users className="w-5 h-5" />}
            color="purple"
          />
          <StatCard
            label="Avg Block Time"
            value={stats?.avgBlockTime ? `${stats.avgBlockTime.toFixed(2)}s` : '-'}
            icon={<Clock className="w-5 h-5" />}
            color="amber"
          />
        </div>
        <TransactionHistoryChart />
      </div>

      <div className="grid md:grid-cols-2 gap-4 sm:gap-6">
        {/* Latest Blocks */}
        <div className="card">
          <div className="px-3 sm:px-4 py-2 sm:py-3 border-b border-neutral-100 flex justify-between items-center">
            <h2 className="font-semibold text-sm sm:text-base text-neutral-900">Latest Blocks</h2>
            <Link to="/blocks" className="text-sm text-neutral-500 hover:text-primary transition-colors">View all</Link>
          </div>
          <div className="divide-y divide-neutral-100 feed-container">
            {blocks?.data?.map((block) => (
              <BlockRow
                key={block.number}
                block={block}
                isNew={newBlocks.has(block.number)}
              />
            ))}
            {!blocks?.data?.length && (
              <div className="px-4 py-8 text-center text-neutral-400">No blocks yet</div>
            )}
          </div>
        </div>

        {/* Latest Transactions */}
        <div className="card">
          <div className="px-3 sm:px-4 py-2 sm:py-3 border-b border-neutral-100 flex justify-between items-center">
            <h2 className="font-semibold text-sm sm:text-base text-neutral-900">Latest Transactions</h2>
            <Link to="/transactions" className="text-sm text-neutral-500 hover:text-primary transition-colors">View all</Link>
          </div>
          <div className="divide-y divide-neutral-100 feed-container">
            {txs?.data?.map((tx) => (
              <TxRow
                key={tx.hash}
                tx={tx}
                isNew={newTxs.has(tx.hash)}
              />
            ))}
            {!txs?.data?.length && (
              <div className="px-4 py-8 text-center text-neutral-400">No transactions yet</div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

const colorStyles = {
  blue: 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-800',
  green: 'bg-success-50 dark:bg-success-500/10 text-success-600 dark:text-success-500 border border-success-100 dark:border-success-500/20',
  purple: 'bg-primary-50 dark:bg-primary-900/20 text-primary dark:text-primary-400 border border-primary-200 dark:border-primary-800',
  amber: 'bg-warning-50 dark:bg-warning-500/10 text-warning-600 dark:text-warning-500 border border-warning-100 dark:border-warning-500/20',
};

function StatCard({ label, value, icon, color }: { label: string; value: string; icon: React.ReactNode; color: keyof typeof colorStyles }) {
  return (
    <div className="card p-3 sm:p-4">
      <div className="flex items-center justify-between mb-2 sm:mb-3">
        <span className="text-xs sm:text-sm text-neutral-500">{label}</span>
        <div className={`p-1.5 sm:p-2 rounded-lg ${colorStyles[color]}`}>
          {icon}
        </div>
      </div>
      <div className="text-lg sm:text-2xl font-mono font-semibold text-neutral-900 truncate">{value}</div>
    </div>
  );
}

// Format gas value for display
function formatGasCompact(gas: number): string {
  if (gas >= 1_000_000) {
    return `${(gas / 1_000_000).toFixed(2)}M`;
  }
  if (gas >= 1_000) {
    return `${(gas / 1_000).toFixed(1)}K`;
  }
  return gas.toString();
}

function BlockRow({ block, isNew }: { block: Block; isNew: boolean }) {
  const gasPercent = block.gasLimit > 0 ? (block.gasUsed / block.gasLimit) * 100 : 0;

  return (
    <div className={`px-3 sm:px-4 h-[52px] sm:h-[60px] flex items-center gap-3 hover:bg-primary-50/50 dark:hover:bg-primary-900/10 transition-colors ${isNew ? 'feed-item-new' : ''}`}>
      <div className="p-2 rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 shrink-0">
        <Box className="w-5 h-5" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <Link to={`/block/${block.number}`} className="font-mono text-sm sm:text-base text-primary hover:text-primary-600 transition-colors">
            {block.number}
          </Link>
          <span className="text-neutral-400 text-xs hidden sm:inline"><LiveTimeAgo timestamp={block.timestamp} /></span>
        </div>
        <div className="text-xs sm:text-sm text-neutral-500 font-mono">
          {formatHash(block.hash, 10)}
        </div>
      </div>
      <div className="text-right text-xs sm:text-sm shrink-0">
        <div className="text-neutral-700">{block.transactionCount} txns</div>
        <div className="flex items-center gap-1.5 justify-end">
          <span className="text-neutral-500">{formatGasCompact(block.gasUsed)}</span>
          <div className="w-12 h-1.5 bg-neutral-200 rounded-full overflow-hidden hidden sm:block">
            <div
              className="h-full bg-blue-500 rounded-full transition-all"
              style={{ width: `${Math.min(gasPercent, 100)}%` }}
            />
          </div>
          <span className="text-neutral-400 text-xs">{gasPercent.toFixed(0)}%</span>
        </div>
      </div>
    </div>
  );
}

// Transaction type display config
const TX_TYPE_CONFIG: Record<TxCategory, { label: string; icon: React.ReactNode; bgColor: string; textColor: string }> = {
  system_transaction: {
    label: 'System',
    icon: <Shield className="w-5 h-5" />,
    bgColor: 'bg-neutral-100',
    textColor: 'text-neutral-400',
  },
  contract_creation: {
    label: 'Contract Creation',
    icon: <FilePlus className="w-5 h-5" />,
    bgColor: 'bg-purple-50 dark:bg-purple-900/20',
    textColor: 'text-purple-600 dark:text-purple-400',
  },
  contract_call: {
    label: 'Contract Call',
    icon: <FileCode className="w-5 h-5" />,
    bgColor: 'bg-blue-50 dark:bg-blue-900/20',
    textColor: 'text-blue-600 dark:text-blue-400',
  },
  token_transfer: {
    label: 'Token Transfer',
    icon: <Coins className="w-5 h-5" />,
    bgColor: 'bg-orange-50 dark:bg-orange-900/20',
    textColor: 'text-orange-600 dark:text-orange-400',
  },
  coin_transfer: {
    label: 'Coin Transfer',
    icon: <ArrowRightLeft className="w-5 h-5" />,
    bgColor: 'bg-green-50 dark:bg-green-900/20',
    textColor: 'text-green-600 dark:text-green-400',
  },
};

const DEFAULT_TX_CONFIG = {
  label: 'Transfer',
  icon: <ArrowRightLeft className="w-5 h-5" />,
  bgColor: 'bg-neutral-100',
  textColor: 'text-neutral-500',
};

// Get icon and colors based on transaction type (priority order)
function getTxTypeConfig(categories?: TxCategory[]) {
  if (!categories || categories.length === 0) {
    return DEFAULT_TX_CONFIG;
  }

  // Priority: contract_creation > contract_call > token_transfer > coin_transfer
  const priority: TxCategory[] = ['system_transaction', 'contract_creation', 'contract_call', 'token_transfer', 'coin_transfer'];
  for (const cat of priority) {
    if (categories.includes(cat)) {
      return TX_TYPE_CONFIG[cat];
    }
  }

  return DEFAULT_TX_CONFIG;
}

function TxRow({ tx, isNew }: { tx: Transaction; isNew: boolean }) {
  const { icon, bgColor, textColor, label } = getTxTypeConfig(tx.txCategories);
  const fromReason = tx.addressMetadata?.[tx.from?.toLowerCase()];
  const toReason = tx.to ? tx.addressMetadata?.[tx.to.toLowerCase()] : undefined;

  return (
    <div className={`px-3 sm:px-4 h-[52px] sm:h-[60px] flex items-center gap-3 hover:bg-primary-50/50 dark:hover:bg-primary-900/10 transition-colors ${isNew ? 'feed-item-new' : ''}`}>
      <div className={`p-2 rounded-lg ${bgColor} ${textColor} shrink-0`}>
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <Link to={`/tx/${tx.hash}`} className="font-mono text-primary hover:text-primary-600 transition-colors text-xs sm:text-sm">
            {formatHash(tx.hash, 10)}
          </Link>
          {tx.blockTimestamp && (
            <span className="text-neutral-400 text-xs hidden sm:inline"><LiveTimeAgo timestamp={tx.blockTimestamp} /></span>
          )}
        </div>
        <div className="text-xs sm:text-sm text-neutral-500 truncate">
          <AddressLink address={tx.from} chars={8} className="text-neutral-500 hover:text-neutral-700" reason={fromReason} />
          {tx.to && (
            <>
              {' → '}
              <AddressLink address={tx.to} chars={8} className="text-neutral-500 hover:text-neutral-700" reason={toReason} />
            </>
          )}
        </div>
      </div>
      <div className="text-right text-xs sm:text-sm shrink-0">
        <div className={`${textColor} font-medium`}>{label}</div>
        <div className={tx.status === 1 ? 'text-success-600' : 'text-error-600'}>
          {tx.status === 1 ? 'Success' : 'Failed'}
        </div>
      </div>
    </div>
  );
}
