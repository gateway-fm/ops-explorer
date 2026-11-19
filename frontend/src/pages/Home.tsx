import { useQuery } from '@tanstack/react-query';
import { useState, useEffect, useRef } from 'react';
import { Boxes, ArrowLeftRight, Users, Clock } from 'lucide-react';
import { api } from '../lib/api';
import type { Transaction } from '../lib/api';
import { AddressLink } from '../components/AddressLink';
import { TransactionHistoryChart } from '../components/TransactionHistoryChart';
import { SearchBar } from '../components/SearchBar';
import { features } from '../lib/features';
import { HomePresentation, HomeStatCard, HomeStatCardSkeleton, HomeFeedSkeleton, HomeBlockRow, HomeTransactionRow } from '../components/HomePresentation';

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

  return <HomePresentation
    search={<SearchBar variant="hero" />}
    stats={stats ? <>
      <HomeStatCard label="Total Blocks" value={stats.totalBlocks?.toLocaleString() ?? '-'} icon={<Boxes className="w-5 h-5" />} color="blue" />
      <HomeStatCard label="Total Transactions" value={stats.totalTransactions?.toLocaleString() ?? '-'} icon={<ArrowLeftRight className="w-5 h-5" />} color="green" />
      <HomeStatCard label="Total Addresses" value={stats.totalAddresses?.toLocaleString() ?? '-'} icon={<Users className="w-5 h-5" />} color="purple" />
      <HomeStatCard label="Avg Block Time" value={stats.avgBlockTime ? `${stats.avgBlockTime.toFixed(2)}s` : '-'} icon={<Clock className="w-5 h-5" />} color="amber" />
    </> : <>{Array.from({length:4},(_,i)=><HomeStatCardSkeleton key={i} />)}</>}
    chart={features().charts ? <TransactionHistoryChart /> : undefined}
    blocks={<>{blocks?.data?.map(block=><HomeBlockRow key={block.number} block={block} isNew={newBlocks.has(block.number)} />)}{!blocks && Array.from({length:10},(_,i)=><HomeFeedSkeleton key={i} />)}{blocks && !blocks.data?.length && <div className="px-4 py-8 text-center text-neutral-400">No blocks yet</div>}</>}
    transactions={<>{txs?.data?.map(tx=><TxRow key={tx.hash} tx={tx} isNew={newTxs.has(tx.hash)} addressInfo={txs.addressInfo} />)}{!txs && Array.from({length:10},(_,i)=><HomeFeedSkeleton key={i} />)}{txs && !txs.data?.length && <div className="px-4 py-8 text-center text-neutral-400">No transactions yet</div>}</>}
  />;
}

function TxRow({ tx, isNew, addressInfo }: { tx: Transaction; isNew: boolean; addressInfo?: Record<string, import('../lib/api').RowAddressInfo> }) {
  const fromReason = tx.addressMetadata?.[tx.from?.toLowerCase()];
  const toReason = tx.to ? tx.addressMetadata?.[tx.to.toLowerCase()] : undefined;
  return <HomeTransactionRow tx={tx} isNew={isNew}
    from={<AddressLink address={tx.from} chars={8} className="text-neutral-500 hover:text-neutral-700" reason={fromReason} info={addressInfo?.[tx.from?.toLowerCase()]} />}
    to={tx.to ? <AddressLink address={tx.to} chars={8} className="text-neutral-500 hover:text-neutral-700" reason={toReason} info={addressInfo?.[tx.to.toLowerCase()]} /> : undefined}
  />;
}
