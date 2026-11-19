import { ArrowLeftRight, Boxes, Users } from 'lucide-react';
import { HomeBlockRow, HomeFeedSkeleton, HomePresentation, HomeStatCard } from '../components/HomePresentation';
import { HomeTransactionRow } from '../components/HomePresentation';
import { NativeSearchBar } from '../components/NativeExplorerControls';
import { NativeAddress } from '../components/NativeAddress';
import { StateMessage } from '../components/StateMessage';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeBlocks, decodeLatestBlock, decodeObservedStats, decodeTransactions } from '../lib/nativeReadModels';
import { exactQuantity } from '../lib/nativePresentation';
import { readTarget } from '../lib/v2Reads';

const page = new URLSearchParams({ limit: '10' });
export function HomeNative() {
  const stats = useNativeRead(readTarget('/stats', page), decodeObservedStats, 5000);
  const latest = useNativeRead(readTarget('/blocks/latest/number', new URLSearchParams()), decodeLatestBlock, 5000);
  const blocks = useNativeRead(readTarget('/blocks', page), decodeBlocks, 5000);
  const transactions = useNativeRead(readTarget('/transactions', page), decodeTransactions, 5000);
  return <HomePresentation search={<NativeSearchBar variant="hero" />}
    stats={<>
      <HomeStatCard label="Current Block" value={exactQuantity(latest.data?.number)} icon={<Boxes className="w-5 h-5" />} color="blue" />
      <HomeStatCard label="Visible Transactions" value={exactQuantity(stats.data?.observedTransactions as string | undefined)} icon={<ArrowLeftRight className="w-5 h-5" />} color="green" />
      <HomeStatCard label="Visible Addresses" value={exactQuantity(stats.data?.observedParticipantAddresses as string | undefined)} icon={<Users className="w-5 h-5" />} color="purple" />
      <HomeStatCard label="Blocks Represented" value={exactQuantity(stats.data?.observedBlocks as string | undefined)} icon={<Boxes className="w-5 h-5" />} color="amber" />
    </>}
    statsNote={stats.error ?? latest.error ?? 'Visible counts cover one permitted page, not totals for the chain or all retained history.'}
    blocks={blocks.loading ? Array.from({ length: 5 }, (_, i) => <HomeFeedSkeleton key={i} />) : blocks.error ? <StateMessage variant="error" title={blocks.error} /> : blocks.data?.items.length ? blocks.data.items.map(block => <HomeBlockRow key={block.hash} block={block} />) : <StateMessage variant="empty" title="No blocks on this page" />}
    transactions={transactions.loading ? Array.from({ length: 5 }, (_, i) => <HomeFeedSkeleton key={i} />) : transactions.error ? <StateMessage variant="error" title={transactions.error} /> : transactions.data?.items.length ? transactions.data.items.map(tx => <HomeTransactionRow key={tx.hash} tx={tx} from={<NativeAddress address={tx.from} metadata={tx.addressMetadata} className="text-neutral-500" />} to={tx.to ? <NativeAddress address={tx.to} metadata={tx.addressMetadata} className="text-neutral-500" /> : undefined} />) : <StateMessage variant="empty" title="No transactions on this page" detail={transactions.data?.page.next ? 'More records may be available on another page.' : undefined} />}
  />;
}
