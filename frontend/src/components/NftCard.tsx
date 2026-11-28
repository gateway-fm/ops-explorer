import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import type { NFTItem } from '../lib/api';
import { resolveNftMetadata } from '../lib/nftMetadata';

function shortAddr(a: string): string {
  return a.length > 12 ? `${a.slice(0, 6)}…${a.slice(-4)}` : a;
}

export function NftCard({ item, collection }: { item: NFTItem; collection: string }) {
  const { data: meta, isLoading } = useQuery({
    queryKey: ['nftMeta', collection, item.tokenId, item.tokenUri],
    queryFn: () => resolveNftMetadata(item.tokenUri),
    enabled: !!item.tokenUri,
    staleTime: 5 * 60 * 1000,
    retry: 1,
  });

  const title = meta?.name || `#${item.tokenId}`;

  return (
    <Link
      to={`/nft/${collection}/${item.tokenId}`}
      className="group block overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50 transition-shadow hover:shadow-md"
    >
      <div className="aspect-square w-full overflow-hidden bg-neutral-100">
        {meta?.image ? (
          <img
            src={meta.image}
            alt={title}
            loading="lazy"
            className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
          />
        ) : (
          <Placeholder tokenId={item.tokenId} loading={isLoading && !!item.tokenUri} />
        )}
      </div>
      <div className="space-y-1 p-3">
        <div className="truncate text-sm font-semibold text-neutral-900" title={title}>
          {title}
        </div>
        {item.quantity !== undefined ? (
          // ERC-1155: an id is shared across many owners, so show its live
          // supply and holder count rather than a single owner.
          <div className="flex items-center justify-between text-xs text-neutral-500">
            <span title="Total live supply of this id">×{item.quantity}</span>
            <span className="text-neutral-400">
              {item.holders} holder{item.holders === 1 ? '' : 's'}
            </span>
          </div>
        ) : (
          <div className="flex items-center gap-1.5 text-xs text-neutral-500">
            <span className="shrink-0">Owner</span>
            <span className="truncate font-mono text-neutral-600" title={item.owner}>
              {shortAddr(item.owner)}
            </span>
          </div>
        )}
      </div>
    </Link>
  );
}

function Placeholder({ tokenId, loading }: { tokenId: string; loading: boolean }) {
  return (
    <div
      className={`flex h-full w-full items-center justify-center bg-gradient-to-br from-neutral-100 to-neutral-200 ${
        loading ? 'animate-pulse' : ''
      }`}
    >
      <span className="font-mono text-lg font-semibold text-neutral-400">#{tokenId}</span>
    </div>
  );
}
