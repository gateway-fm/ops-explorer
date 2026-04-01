import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { useState, useCallback, useEffect } from 'react';
import { api } from '../lib/api';
import type { Block } from '../lib/api';
import { formatHash, formatGas } from '../lib/utils';
import { LiveTimeAgo } from '../components/LiveTimeAgo';
import { PageHeader } from '../components/PageHeader';
import { NewItemsNotice } from '../components/NewItemsNotice';

export function Blocks() {
  const [searchParams, setSearchParams] = useSearchParams();
  const before = searchParams.get('before');
  const queryClient = useQueryClient();

  // Main blocks query — no auto-refetch on page 1
  const { data, isLoading } = useQuery({
    queryKey: ['blocks', 25, before],
    queryFn: () => api.getBlocks(25, before ? parseInt(before) : undefined),
  });

  // Track what the top block was when the user last loaded/refreshed
  const [snapshotTopBlock, setSnapshotTopBlock] = useState<number | null>(null);
  useEffect(() => {
    if (data?.data?.length && snapshotTopBlock === null) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time snapshot on initial data load
      setSnapshotTopBlock(data.data[0].number);
    }
  }, [data, snapshotTopBlock]);

  // Lightweight poll for the latest block number (only on first page)
  const { data: latestBlock } = useQuery({
    queryKey: ['latestBlock'],
    queryFn: api.getLatestBlock,
    refetchInterval: !before ? 2000 : false,
    enabled: !before,
  });

  // Calculate how many new blocks have come in since the snapshot
  const newBlockCount = (!before && snapshotTopBlock !== null && latestBlock)
    ? Math.max(0, latestBlock.number - snapshotTopBlock)
    : 0;

  const handleLoadNew = useCallback(() => {
    setSnapshotTopBlock(null);
    queryClient.invalidateQueries({ queryKey: ['blocks', 25, before] });
  }, [queryClient, before]);

  const loadMore = () => {
    if (data?.data?.length) {
      const lastBlock = data.data[data.data.length - 1];
      setSearchParams({ before: String(lastBlock.number) });
    }
  };

  return (
    <div>
      <PageHeader title="Blocks" />

      <div className="card overflow-hidden">
        {!before && (
          <NewItemsNotice
            count={newBlockCount}
            type="block"
            onClick={handleLoadNew}
          />
        )}

        <table className="table">
          <thead>
            <tr>
              <th>Block</th>
              <th>Age</th>
              <th>Txns</th>
              <th>Gas Used</th>
              <th>Hash</th>
            </tr>
          </thead>
          <tbody>
            {data?.data?.map((block) => (
              <BlockTableRow
                key={block.number}
                block={block}
              />
            ))}
          </tbody>
        </table>

        {isLoading && (
          <div className="px-4 py-8 text-center text-neutral-400">Loading...</div>
        )}

        {data?.hasMore && (
          <div className="px-4 py-3 border-t border-neutral-100">
            <button
              onClick={loadMore}
              className="w-full py-2 text-sm text-neutral-500 hover:text-neutral-700 transition-colors"
            >
              Load more
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function BlockTableRow({ block }: { block: Block }) {
  const gasPercent = ((block.gasUsed / block.gasLimit) * 100).toFixed(1);

  return (
    <tr>
      <td>
        <Link to={`/block/${block.number}`} className="font-mono text-primary hover:text-primary-600 transition-colors">
          {block.number}
        </Link>
      </td>
      <td className="text-neutral-500">
        <LiveTimeAgo timestamp={block.timestamp} />
      </td>
      <td className="text-neutral-700">{block.transactionCount}</td>
      <td>
        <span className="font-mono text-neutral-700">{formatGas(block.gasUsed)}</span>
        <span className="text-neutral-400 ml-1">({gasPercent}%)</span>
      </td>
      <td className="font-mono text-neutral-400">
        {formatHash(block.hash, 10)}
      </td>
    </tr>
  );
}
