import { ArrowLeftRight, Boxes, Users } from 'lucide-react';
import { HomeBlockRow, HomeFeedSkeleton, HomePresentation, HomeStatCard, HomeStatCardSkeleton } from '../components/HomePresentation';
import { HomeTransactionRow } from '../components/HomePresentation';
import { NativeSearchBar } from '../components/NativeExplorerControls';
import { NativeAddress } from '../components/NativeAddress';
import { StateMessage } from '../components/StateMessage';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeBlocks, decodeLatestBlock, decodeObservedStats, decodeTransactions } from '../lib/nativeReadModels';
import { exactQuantity } from '../lib/nativePresentation';
import { readTarget } from '../lib/v2Reads';
import { ScopedCountValue } from '../components/ScopedCountValue';

const page = new URLSearchParams({ limit: '10' });
export function HomeNative() {
  const stats = useNativeRead(readTarget('/stats', page), decodeObservedStats, 30000);
  const latest = useNativeRead(readTarget('/blocks/latest/number', new URLSearchParams()), decodeLatestBlock, 5000);
  const blocks = useNativeRead(readTarget('/blocks', page), decodeBlocks, 5000);
  const transactions = useNativeRead(readTarget('/transactions', page), decodeTransactions, 5000);
  return <HomePresentation search={<NativeSearchBar variant="hero" />}
    stats={<>
      {latest.loading ? <HomeStatCardSkeleton/> : <HomeStatCard label="Current Block" value={exactQuantity(latest.data?.number)} icon={<Boxes className="w-5 h-5" />} color="blue" />}
      {stats.loading ? <HomeStatCardSkeleton/> : <HomeStatCard label="Visible Transactions" value={<ScopedCountValue count={stats.data?.counts?.transactions}/>} icon={<ArrowLeftRight className="w-5 h-5" />} color="green" />}
      {stats.loading ? <HomeStatCardSkeleton/> : <HomeStatCard label="Visible Accounts" value={<ScopedCountValue count={stats.data?.counts?.accounts}/>} icon={<Users className="w-5 h-5" />} color="purple" />}
      {stats.loading ? <HomeStatCardSkeleton/> : <HomeStatCard label="Visible Tokens" value={<ScopedCountValue count={stats.data?.counts?.tokens}/>} icon={<Boxes className="w-5 h-5" />} color="amber" />}
    </>}
    statsNote={<span className="space-x-2"><span>{stats.error ?? latest.error ?? 'Counts include only your permitted rows. ≥ means more may exist; tap or focus a count for its retained range and update time.'}</span><button className="text-primary hover:underline disabled:opacity-50" disabled={stats.loading} onClick={stats.refresh}>Refresh counts</button></span>}
    blocks={blocks.loading ? Array.from({ length: 5 }, (_, i) => <HomeFeedSkeleton key={i} />) : blocks.error ? <StateMessage variant="error" title={blocks.error} /> : blocks.data?.items.length ? blocks.data.items.map(block => <HomeBlockRow key={block.hash} block={block} />) : <StateMessage variant="empty" title="No blocks on this page" />}
    transactions={transactions.loading ? Array.from({ length: 5 }, (_, i) => <HomeFeedSkeleton key={i} />) : transactions.error ? <StateMessage variant="error" title={transactions.error} /> : transactions.data?.items.length ? transactions.data.items.map(tx => <HomeTransactionRow key={tx.hash} tx={tx} from={<NativeAddress address={tx.from} metadata={tx.addressMetadata} className="text-neutral-500" />} to={tx.to ? <NativeAddress address={tx.to} metadata={tx.addressMetadata} className="text-neutral-500" /> : undefined} />) : <StateMessage variant="empty" title="No transactions on this page" detail={transactions.data?.page.next ? 'More records may be available on another page.' : undefined} />}
  />;
}
