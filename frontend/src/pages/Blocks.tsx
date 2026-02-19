import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { useState, useEffect, useRef } from 'react';
import { api } from '../lib/api';
import type { Block } from '../lib/api';
import { formatHash, formatGas } from '../lib/utils';
import { LiveTimeAgo } from '../components/LiveTimeAgo';
import { PageHeader } from '../components/PageHeader';

export function Blocks() {
  const [searchParams, setSearchParams] = useSearchParams();
  const before = searchParams.get('before');

  const { data, isLoading } = useQuery({
    queryKey: ['blocks', 25, before],
    queryFn: () => api.getBlocks(25, before ? parseInt(before) : undefined),
    refetchInterval: before ? false : 2000,
  });

  // Track seen blocks for animations
  const seenBlocks = useRef<Set<number>>(new Set());
  const [newBlocks, setNewBlocks] = useState<Set<number>>(new Set());
  const isFirstRender = useRef(true);

  useEffect(() => {
    if (!data?.data) return;

    if (isFirstRender.current) {
      data.data.forEach(b => seenBlocks.current.add(b.number));
      isFirstRender.current = false;
    } else {
      const newOnes = new Set<number>();
      data.data.forEach(b => {
        if (!seenBlocks.current.has(b.number)) {
          newOnes.add(b.number);
          seenBlocks.current.add(b.number);
        }
      });
      if (newOnes.size > 0) {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- animation tracking for new blocks
        setNewBlocks(newOnes);
        setTimeout(() => setNewBlocks(new Set()), 2000);
      }
    }
  }, [data?.data]);

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
                isNew={newBlocks.has(block.number)}
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

function BlockTableRow({ block, isNew }: { block: Block; isNew: boolean }) {
  const gasPercent = ((block.gasUsed / block.gasLimit) * 100).toFixed(1);

  return (
    <tr className={isNew ? 'feed-item-new' : ''}>
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
