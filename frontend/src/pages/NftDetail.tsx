import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft } from 'lucide-react';
import { api } from '../lib/api';
import { resolveNftMetadata } from '../lib/nftMetadata';
import { AddressLink } from '../components/AddressLink';
import { CopyButton } from '../components/CopyButton';

export default function NftDetail() {
  const { address, tokenId } = useParams<{ address: string; tokenId: string }>();
  const navigate = useNavigate();

  const { data: token } = useQuery({
    queryKey: ['token', address],
    queryFn: () => api.getToken(address!),
    enabled: !!address,
  });

  const { data: item, isLoading, error } = useQuery({
    queryKey: ['nftItem', address, tokenId],
    queryFn: () => api.getNftItem(address!, tokenId!),
    enabled: !!address && !!tokenId,
  });

  const { data: meta } = useQuery({
    queryKey: ['nftMeta', address, tokenId, item?.tokenUri],
    queryFn: () => resolveNftMetadata(item?.tokenUri),
    enabled: !!item?.tokenUri,
    staleTime: 5 * 60 * 1000,
    retry: 1,
  });

  const goBack = () => {
    if (address) navigate(`/token/${address}?tab=inventory`);
    else navigate('/tokens');
  };

  const collectionName = token?.name || 'NFT';
  const title = meta?.name || `${collectionName} #${tokenId}`;

  return (
    <div className="space-y-6">
      <button
        onClick={goBack}
        className="inline-flex items-center gap-1.5 text-xs font-medium text-neutral-500 transition-colors hover:text-primary"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        Back to collection
      </button>

      {isLoading ? (
        <DetailSkeleton />
      ) : error || !item ? (
        <div className="rounded-2xl border border-dashed border-neutral-200 bg-neutral-100/30 px-6 py-12 text-center text-sm text-neutral-500">
          NFT not found.
        </div>
      ) : (
        <div className="grid gap-8 lg:grid-cols-[minmax(0,420px)_1fr]">
          {/* Artwork */}
          <div className="overflow-hidden rounded-3xl border border-neutral-200 bg-neutral-50">
            <div className="aspect-square w-full bg-neutral-100">
              {meta?.image ? (
                <img src={meta.image} alt={title} className="h-full w-full object-cover" />
              ) : (
                <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-neutral-100 to-neutral-200">
                  <span className="font-mono text-2xl font-semibold text-neutral-400">#{tokenId}</span>
                </div>
              )}
            </div>
          </div>

          {/* Details */}
          <div className="space-y-6">
            <div>
              <Link
                to={`/token/${address}`}
                className="text-sm font-medium text-primary transition-colors hover:text-primary-600"
              >
                {collectionName}
                {token?.symbol ? ` (${token.symbol})` : ''}
              </Link>
              <h1 className="mt-1 text-2xl font-bold text-neutral-900 sm:text-3xl">{title}</h1>
            </div>

            {meta?.description && (
              <p className="text-sm leading-relaxed text-neutral-600">{meta.description}</p>
            )}

            <dl className="divide-y divide-neutral-200/70 overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
              <Row label="Token ID">
                <span className="font-mono tabular-nums text-neutral-900">{item.tokenId}</span>
              </Row>
              {item.quantity !== undefined ? (
                // ERC-1155: an id has no single owner — show its live supply
                // and how many addresses hold it.
                <>
                  <Row label="Supply">
                    <span className="font-mono tabular-nums text-neutral-900">{item.quantity}</span>
                  </Row>
                  <Row label="Holders">
                    <span className="font-mono tabular-nums text-neutral-900">{item.holders}</span>
                  </Row>
                </>
              ) : (
                <Row label="Owner">
                  <div className="flex items-center gap-2">
                    <AddressLink address={item.owner} full />
                    <CopyButton text={item.owner} />
                  </div>
                </Row>
              )}
              <Row label="Contract">
                <div className="flex min-w-0 items-center gap-2">
                  <Link
                    to={`/token/${address}`}
                    className="truncate font-mono text-sm text-primary transition-colors hover:text-primary-600"
                    title={address}
                  >
                    {address}
                  </Link>
                  {address && <CopyButton text={address} />}
                </div>
              </Row>
              <Row label="Standard">
                <span className="text-sm text-neutral-700">{token?.tokenType || 'ERC721'}</span>
              </Row>
            </dl>

            {meta?.attributes && meta.attributes.length > 0 && (
              <div>
                <h2 className="mb-2 text-sm font-semibold text-neutral-900">Traits</h2>
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                  {meta.attributes.map((attr, i) => (
                    <div
                      key={i}
                      className="rounded-xl border border-primary/20 bg-primary/5 px-3 py-2"
                    >
                      <div className="truncate text-[11px] font-medium uppercase tracking-wider text-primary/70">
                        {attr.trait_type || 'Trait'}
                      </div>
                      <div
                        className="truncate text-sm font-semibold text-neutral-900"
                        title={String(attr.value)}
                      >
                        {String(attr.value)}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-4 px-5 py-3">
      <dt className="w-28 shrink-0 text-sm text-neutral-500">{label}</dt>
      <dd className="min-w-0 flex-1 text-sm text-neutral-900">{children}</dd>
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="grid gap-8 lg:grid-cols-[minmax(0,420px)_1fr]">
      <div className="aspect-square w-full animate-pulse rounded-3xl bg-neutral-200" />
      <div className="space-y-4">
        <div className="h-8 w-64 animate-pulse rounded bg-neutral-200" />
        <div className="h-24 w-full animate-pulse rounded-2xl bg-neutral-200/70" />
        <div className="h-40 w-full animate-pulse rounded-2xl bg-neutral-200/60" />
      </div>
    </div>
  );
}
