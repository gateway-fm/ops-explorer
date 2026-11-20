import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Box, FileCode, FilePlus, Coins, ArrowRightLeft, Shield } from 'lucide-react';
import { branding } from '../lib/branding';
import { formatHash } from '../lib/utils';
import { LiveTimeAgo } from './LiveTimeAgo';
import { Skeleton } from './ui/skeleton';

// Original Home presentation shared by the legacy and native read controllers.
// Its slots receive already-authorized data; this component does no fetching.
export function HomePresentation({ search, stats, chart, blocks, transactions, statsNote, statsColumns = 4 }: {
  search: ReactNode; stats: ReactNode; chart?: ReactNode; blocks: ReactNode; transactions: ReactNode; statsNote?: ReactNode; statsColumns?: 4|5;
}) {
  return <div className="space-y-4 sm:space-y-8">
    <div className={`relative overflow-visible rounded-xl p-5 sm:p-8 shadow-card ${branding.colorHeroBg ? '' : 'bg-gradient-to-br from-primary-900 via-primary-700 to-primary'}`} style={branding.colorHeroBg ? { background: branding.colorHeroBg } : undefined}>
      <div className="absolute inset-0 opacity-[0.07] overflow-hidden rounded-xl"><div className="absolute top-0 left-0 w-72 h-72 bg-white rounded-full -translate-x-1/2 -translate-y-1/2" /><div className="absolute bottom-0 right-0 w-96 h-96 bg-white rounded-full translate-x-1/3 translate-y-1/3" /></div>
      <div className="relative z-10"><h1 className="text-2xl sm:text-3xl font-bold text-white mb-4">{branding.name}</h1>{search}</div>
    </div>
    <div className={`grid gap-4 sm:gap-6 ${chart ? 'md:grid-cols-2' : ''}`}>
      <div data-testid="home-stats-grid" className={`grid grid-cols-2 gap-2 sm:gap-4 ${statsColumns===5?'md:grid-cols-3 xl:grid-cols-5':chart?'':'md:grid-cols-4'}`}>{stats}</div>
      {chart}
      {statsNote && <p className="text-xs text-neutral-500 col-span-full">{statsNote}</p>}
    </div>
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 sm:gap-6">
      <div className="card"><div className="px-3 sm:px-4 py-2 sm:py-3 border-b border-neutral-100 flex justify-between items-center"><h2 className="font-semibold text-sm sm:text-base text-neutral-900">Latest Blocks</h2><Link to="/blocks" className="text-sm text-neutral-500 hover:text-primary transition-colors">View all</Link></div><div className="divide-y divide-neutral-100 feed-container">{blocks}</div></div>
      <div className="card"><div className="px-3 sm:px-4 py-2 sm:py-3 border-b border-neutral-100 flex justify-between items-center"><h2 className="font-semibold text-sm sm:text-base text-neutral-900">Latest Transactions</h2><Link to="/transactions" className="text-sm text-neutral-500 hover:text-primary transition-colors">View all</Link></div><div className="divide-y divide-neutral-100 feed-container">{transactions}</div></div>
    </div>
  </div>;
}

const colorStyles = {
  blue: 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-800',
  green: 'bg-success-50 dark:bg-success-500/10 text-success-600 dark:text-success-500 border border-success-100 dark:border-success-500/20',
  purple: 'bg-primary-50 dark:bg-primary-900/20 text-primary dark:text-primary-400 border border-primary-200 dark:border-primary-800',
  amber: 'bg-warning-50 dark:bg-warning-500/10 text-warning-600 dark:text-warning-500 border border-warning-100 dark:border-warning-500/20',
};

export function HomeStatCard({ label, value, icon, color, note }: { label: string; value: React.ReactNode; icon: React.ReactNode; color: keyof typeof colorStyles; note?:ReactNode }) {
  return (
    <div className="card p-3 sm:p-4" data-testid="stat-card">
      <div className="flex items-center justify-between mb-2 sm:mb-3">
        <span className="text-xs sm:text-sm text-neutral-500">{label}</span>
        <div className={`p-1.5 sm:p-2 rounded-lg ${colorStyles[color]}`}>
          {icon}
        </div>
      </div>
      <div className="text-lg sm:text-2xl font-mono font-semibold text-neutral-900 truncate">{value}</div>
      {note&&<p className="mt-1 text-xs font-sans text-neutral-500 break-words">{note}</p>}
    </div>
  );
}

export function HomeStatCardSkeleton() {
  return (
    <div className="card p-3 sm:p-4">
      <div className="flex items-center justify-between mb-2 sm:mb-3">
        <Skeleton className="h-3 sm:h-4 w-20" />
        <Skeleton className="h-7 w-7 sm:h-8 sm:w-8 rounded-lg" />
      </div>
      <Skeleton className="h-6 sm:h-8 w-24" />
    </div>
  );
}


export function HomeFeedSkeleton() {
  return <div className="px-3 sm:px-4 h-[52px] sm:h-[60px] flex items-center gap-3"><Skeleton className="h-9 w-9 rounded-lg shrink-0" /><div className="flex-1 min-w-0 space-y-1.5"><Skeleton className="h-4 w-24" /><Skeleton className="h-3 w-32" /></div><div className="text-right shrink-0 space-y-1.5"><Skeleton className="h-3 w-16 ml-auto" /><Skeleton className="h-3 w-20 ml-auto" /></div></div>;
}
function HomeTime({ timestamp }: { timestamp?: number | string }) {
  if (timestamp === undefined) return null;
  const value = typeof timestamp === 'string' ? BigInt(timestamp) : BigInt(timestamp);
  return value <= 8640000000000n ? <LiveTimeAgo timestamp={Number(value)} /> : <>{String(timestamp)} Unix seconds</>;
}
export function HomeBlockRow({ block, isNew = false, transactionCountView }: { block: {
  number: number | string; hash: string; timestamp: number | string; transactionCount?: number | string; gasUsed?: number; gasLimit: number | string;
}; isNew?: boolean; transactionCountView?:ReactNode }) {
  const gasPercent = typeof block.gasLimit === 'number' && block.gasUsed !== undefined && block.gasLimit > 0 ? block.gasUsed / block.gasLimit * 100 : undefined;
  const gas = block.gasUsed === undefined ? undefined : block.gasUsed >= 1000000 ? `${(block.gasUsed / 1000000).toFixed(2)}M` : block.gasUsed >= 1000 ? `${(block.gasUsed / 1000).toFixed(1)}K` : String(block.gasUsed);
  return <div data-testid="block-row" className={`px-3 sm:px-4 h-[52px] sm:h-[60px] flex items-center gap-3 hover:bg-primary-50/50 dark:hover:bg-primary-900/10 transition-colors ${isNew ? 'feed-item-new' : ''}`}>
    <div className="p-2 rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 shrink-0"><Box className="w-5 h-5" /></div>
    <div className="flex-1 min-w-0"><div className="flex items-center gap-2"><Link to={`/block/${block.number}`} className="font-mono text-sm sm:text-base text-primary hover:text-primary-600 transition-colors">{block.number}</Link><span className="text-neutral-400 text-xs hidden sm:inline"><HomeTime timestamp={block.timestamp} /></span></div><div className="text-xs sm:text-sm text-neutral-500 font-mono">{formatHash(block.hash,10)}</div></div>
    <div className="text-right text-xs sm:text-sm shrink-0"><div className="text-neutral-700">{transactionCountView ?? (block.transactionCount === undefined ? 'Count unavailable' : `${block.transactionCount} txns`)}</div>{gas !== undefined && <div className="flex items-center gap-1.5 justify-end"><span className="text-neutral-500">{gas}</span><div className="w-12 h-1.5 bg-neutral-200 rounded-full overflow-hidden hidden sm:block"><div className="h-full bg-blue-500 rounded-full transition-all" style={{width:`${Math.min(gasPercent ?? 0,100)}%`}} /></div><span className="text-neutral-400 text-xs">{(gasPercent ?? 0).toFixed(0)}%</span></div>}</div>
  </div>;
}
const transactionKinds = {
  system_transaction: { label: 'System', icon: Shield, bgColor: 'bg-neutral-100', textColor: 'text-neutral-400' },
  contract_creation: { label: 'Contract Creation', icon: FilePlus, bgColor: 'bg-purple-50 dark:bg-purple-900/20', textColor: 'text-purple-600 dark:text-purple-400' },
  contract_call: { label: 'Contract Call', icon: FileCode, bgColor: 'bg-blue-50 dark:bg-blue-900/20', textColor: 'text-blue-600 dark:text-blue-400' },
  token_transfer: { label: 'Token Transfer', icon: Coins, bgColor: 'bg-orange-50 dark:bg-orange-900/20', textColor: 'text-orange-600 dark:text-orange-400' },
  coin_transfer: { label: 'Coin Transfer', icon: ArrowRightLeft, bgColor: 'bg-green-50 dark:bg-green-900/20', textColor: 'text-green-600 dark:text-green-400' },
};
export function HomeTransactionRow({ tx, from, to, isNew = false, wrapParties = false }: { tx: { hash: string; blockTimestamp?: number | string; status: number | null; txCategories?: string[] }; from: ReactNode; to?: ReactNode; isNew?: boolean; wrapParties?:boolean }) {
  const kind = Object.keys(transactionKinds).find(key => tx.txCategories?.includes(key)) as keyof typeof transactionKinds | undefined;
  const { icon: Icon, bgColor, textColor, label } = kind ? transactionKinds[kind] : { label: 'Transfer', icon: ArrowRightLeft, bgColor: 'bg-neutral-100', textColor: 'text-neutral-500' };
  return <div data-testid="tx-row" className={`px-3 sm:px-4 ${wrapParties?'min-h-[52px] sm:min-h-[60px] py-3':'h-[52px] sm:h-[60px]'} flex items-center gap-3 hover:bg-primary-50/50 dark:hover:bg-primary-900/10 transition-colors ${isNew ? 'feed-item-new' : ''}`}>
    <div className={`p-2 rounded-lg ${bgColor} ${textColor} shrink-0`}><Icon className="w-5 h-5" /></div>
    <div className="min-w-0 flex-1"><div className="flex items-center gap-2"><Link to={`/tx/${tx.hash}`} className={`font-mono text-primary hover:text-primary-600 transition-colors text-xs sm:text-sm ${wrapParties?'min-w-0 truncate':''}`}>{formatHash(tx.hash,10)}</Link>{tx.blockTimestamp !== undefined && <span className="text-neutral-400 text-xs hidden sm:inline"><HomeTime timestamp={tx.blockTimestamp} /></span>}</div><div className={`text-xs sm:text-sm text-neutral-500 ${wrapParties?'flex flex-wrap items-center gap-x-1 gap-y-1':'truncate'}`}>{from}{to && <> → {to}</>}</div></div>
    <div className="text-right text-xs sm:text-sm shrink-0"><div className={`${textColor} font-medium`}>{label}</div><div className={tx.status === 1 ? 'text-success-600' : tx.status === 0 ? 'text-error-600' : 'text-neutral-500'}>{tx.status === 1 ? 'Success' : tx.status === 0 ? 'Failed' : 'Pending / unknown'}</div></div>
  </div>;
}
