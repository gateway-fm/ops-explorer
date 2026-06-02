import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, ArrowRight } from 'lucide-react';
import { api } from '../lib/api';
import type { Contract, NFTItem, Token, TokenHolder, TokenTransfer } from '../lib/api';
import { AddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';
import { CopyButton } from '../components/CopyButton';
import { TokenAvatar } from '../components/TokenAvatar';
import { NftCard } from '../components/NftCard';
import { formatTokenType, formatTokenValue } from '../lib/formatToken';
import { formatTimeAgo } from '../lib/utils';

const PAGE_SIZE = 25;

type Tab = 'transfers' | 'inventory' | 'holders' | 'contract';

function tabFromParams(s: string | null): Tab {
  if (s === 'holders' || s === 'contract' || s === 'inventory') return s;
  return 'transfers';
}

export default function TokenDetail() {
  const { address } = useParams<{ address: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab: Tab = tabFromParams(searchParams.get('tab'));
  const page = parseInt(searchParams.get('page') || '1', 10);

  const { data: token, isLoading: tokenLoading, error: tokenError } = useQuery({
    queryKey: ['token', address],
    queryFn: () => api.getToken(address!),
    enabled: !!address,
  });

  const { data: holders, isLoading: holdersLoading } = useQuery({
    queryKey: ['tokenHolders', address, page],
    queryFn: () => api.getTokenHolders(address!, page, PAGE_SIZE),
    enabled: !!address && activeTab === 'holders',
  });

  const { data: transfers, isLoading: transfersLoading } = useQuery({
    queryKey: ['tokenTransfers', address, page],
    queryFn: () => api.getTokenTransfers(address!, page, PAGE_SIZE),
    enabled: !!address && activeTab === 'transfers',
  });

  const { data: inventory, isLoading: inventoryLoading } = useQuery({
    queryKey: ['tokenInventory', address, page],
    queryFn: () => api.getTokenInventory(address!, page, PAGE_SIZE),
    enabled: !!address && activeTab === 'inventory',
  });

  // Inventory total for the tab badge — fetched whenever the token is an NFT
  // collection, independent of which tab is active (1 row, just for the count).
  const isNftCollection =
    token?.tokenType === 'ERC721' || token?.tokenType === 'ERC1155';
  const { data: inventoryCount } = useQuery({
    queryKey: ['tokenInventoryCount', address],
    queryFn: () => api.getTokenInventory(address!, 1, 1),
    enabled: !!address && isNftCollection,
    select: (r) => r.total,
  });

  const { data: contract, isLoading: contractLoading } = useQuery({
    queryKey: ['contract', address],
    queryFn: () => api.getContract(address!),
    enabled: !!address && activeTab === 'contract',
  });

  // Creation tx supplies the deploy bytecode (initcode + constructor args)
  // via its inputData. Only fetched once we have the contract row.
  const creationTxHash = contract?.creationTx;
  const { data: creationTx, isLoading: creationTxLoading } = useQuery({
    queryKey: ['creationTx', creationTxHash],
    queryFn: () => api.getTransaction(creationTxHash!),
    enabled: !!creationTxHash && activeTab === 'contract',
  });

  const handlePageChange = (newPage: number) => {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(newPage));
    setSearchParams(params);
  };

  const handleTabChange = (tab: Tab) => {
    const params = new URLSearchParams();
    if (tab !== 'transfers') params.set('tab', tab);
    params.set('page', '1');
    setSearchParams(params);
  };

  if (tokenLoading) {
    return (
      <div className="space-y-6">
        <BackLink />
        <HeroSkeleton />
      </div>
    );
  }

  if (
    tokenError instanceof Error &&
    (tokenError.message.includes('403') || tokenError.message.includes('500'))
  ) {
    return (
      <div className="space-y-6">
        <BackLink />
        <div className="rounded-2xl border border-neutral-200 bg-neutral-50 px-6 py-16 text-center">
          <h2 className="text-lg font-semibold text-neutral-900">Token Restricted</h2>
          <p className="mx-auto mt-2 max-w-md text-sm text-neutral-500">
            This token belongs to a private organization. Sign in to view it if you have access.
          </p>
        </div>
      </div>
    );
  }

  if (tokenError || !token) {
    return (
      <div className="space-y-6">
        <BackLink />
        <div className="rounded-2xl border border-dashed border-neutral-200 bg-neutral-100/30 px-6 py-12 text-center text-sm text-neutral-500">
          Token not found.
        </div>
      </div>
    );
  }

  const isNFT = token.tokenType === 'ERC721' || token.tokenType === 'ERC1155';
  const activePage =
    activeTab === 'holders'
      ? holders
      : activeTab === 'inventory'
      ? inventory
      : transfers;
  const totalPages = activePage?.totalPages ?? 1;
  const dataLength = activePage?.data?.length ?? 0;

  return (
    <div className="space-y-6">
      <BackLink />

      <Hero token={token} />

      <StatBar token={token} />

      <Tabs
        active={activeTab}
        onChange={handleTabChange}
        transfersCount={token.transferCount}
        holdersCount={token.holderCount}
        showInventory={isNFT}
        inventoryCount={inventoryCount}
      />

      <div>
        {activeTab === 'contract' ? (
          <ContractView
            contract={contract}
            creationInput={creationTx?.inputData}
            loading={contractLoading || creationTxLoading}
          />
        ) : activeTab === 'inventory' ? (
          inventoryLoading ? (
            <CardGridSkeleton />
          ) : (inventory?.data || []).length === 0 ? (
            <EmptyState>No NFTs found in this collection.</EmptyState>
          ) : (
            <InventoryGrid items={inventory!.data} collection={token.address} />
          )
        ) : activeTab === 'transfers' ? (
          transfersLoading ? (
            <TableSkeleton rows={6} />
          ) : (transfers?.data || []).length === 0 ? (
            <EmptyState>No transfers yet for this token.</EmptyState>
          ) : (
            <TransfersTable transfers={transfers!.data} token={token} />
          )
        ) : holdersLoading ? (
          <TableSkeleton rows={6} />
        ) : (holders?.data || []).length === 0 ? (
          <EmptyState>No holders yet for this token.</EmptyState>
        ) : (
          <HoldersTable holders={holders!.data} token={token} page={page} />
        )}

        {activeTab !== 'contract' && dataLength > 0 && (
          <Pagination page={page} totalPages={totalPages} onChange={handlePageChange} className="mt-5" />
        )}
      </div>
    </div>
  );
}

// ---------- Back link ----------

function BackLink() {
  const navigate = useNavigate();
  const goBack = () => {
    if (window.history.length > 1) navigate(-1);
    else navigate('/tokens');
  };
  return (
    <button
      onClick={goBack}
      className="inline-flex items-center gap-1.5 text-xs font-medium text-neutral-500 transition-colors hover:text-primary"
    >
      <ArrowLeft className="h-3.5 w-3.5" />
      Back
    </button>
  );
}

// ---------- Hero ----------

function Hero({ token }: { token: Token }) {
  const name = token.name || 'Unknown Token';
  const symbol = token.symbol || '?';
  return (
    <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between lg:gap-6">
      <div className="flex min-w-0 items-center gap-4">
        <TokenAvatar symbol={symbol} address={token.address} size="lg" />
        <div className="min-w-0">
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <h1 className="text-2xl font-bold text-neutral-900 sm:text-3xl">{name}</h1>
            <span className="text-lg text-neutral-500">{symbol}</span>
            <TypeBadge type={token.tokenType} />
            {token.usdPrice && (
              <span className="font-mono text-sm tabular-nums text-neutral-600">
                ${token.usdPrice.toFixed(4)}
              </span>
            )}
          </div>
        </div>
      </div>

      <div className="inline-flex min-w-0 max-w-full items-center gap-2 self-start rounded-full border border-neutral-200 bg-neutral-50 px-3 py-1.5 lg:self-auto">
        <Link
          to={`/address/${token.address}`}
          className="truncate font-mono text-sm text-neutral-600 transition-colors hover:text-primary"
          title={token.address}
        >
          {token.address}
        </Link>
        <CopyButton text={token.address} />
      </div>
    </div>
  );
}

function TypeBadge({ type }: { type: string }) {
  const tone =
    type === 'ERC721'
      ? 'bg-primary/10 text-primary'
      : type === 'ERC1155'
      ? 'bg-warning-500/10 text-warning-700 dark:text-warning-500'
      : 'bg-neutral-200/60 text-neutral-700';
  return (
    <span
      className={`inline-flex items-center rounded-md px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wider ${tone}`}
    >
      {formatTokenType(type)}
    </span>
  );
}

// ---------- Stat bar ----------

function StatBar({ token }: { token: Token }) {
  const isNFT = token.tokenType === 'ERC721' || token.tokenType === 'ERC1155';
  // Always 4 stats so the grid stays balanced. Price (when available) lives in
  // the hero as inline metadata, not in this bar. NFTs have no decimals, so the
  // last tile shows the token standard instead, and supply is an item count.
  const items: { label: string; value: string; unit?: string }[] = isNFT
    ? [
        {
          label: 'Total supply',
          value: formatTokenValue(token.totalSupply || '0', 0),
          unit: 'items',
        },
        { label: 'Holders', value: token.holderCount.toLocaleString() },
        { label: 'Transfers', value: token.transferCount.toLocaleString() },
        { label: 'Token standard', value: formatTokenType(token.tokenType) },
      ]
    : [
        {
          label: 'Max total supply',
          value: formatTokenValue(token.totalSupply || '0', token.decimals),
          unit: token.symbol,
        },
        { label: 'Holders', value: token.holderCount.toLocaleString() },
        { label: 'Transfers', value: token.transferCount.toLocaleString() },
        { label: 'Decimals', value: String(token.decimals) },
      ];

  return (
    <dl className="grid grid-cols-2 overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50 sm:grid-cols-4">
      {items.map((item, i) => (
        <div
          key={item.label}
          className={[
            'px-5 py-4',
            // Internal dividers only — outer border is on the wrapper.
            i % 2 === 1 ? 'border-l border-neutral-200/70' : '',
            i >= 2 ? 'border-t border-neutral-200/70 sm:border-t-0' : '',
            i > 0 ? 'sm:border-l sm:border-neutral-200/70' : '',
          ].join(' ')}
        >
          <dt className="text-[11px] font-medium uppercase tracking-wider text-neutral-500">
            {item.label}
          </dt>
          <dd className="mt-1 truncate text-lg leading-tight text-neutral-900">
            <span className="font-mono tabular-nums">{item.value}</span>
            {item.unit && (
              <span className="ml-1.5 text-sm text-neutral-500">{item.unit}</span>
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}

// ---------- Tabs ----------

function Tabs({
  active,
  onChange,
  transfersCount,
  holdersCount,
  showInventory,
  inventoryCount,
}: {
  active: Tab;
  onChange: (t: Tab) => void;
  transfersCount: number;
  holdersCount: number;
  showInventory: boolean;
  inventoryCount?: number;
}) {
  const tabs: { id: Tab; label: string; count?: number }[] = [
    { id: 'transfers', label: 'Transfers', count: transfersCount },
    ...(showInventory
      ? [{ id: 'inventory' as Tab, label: 'Inventory', count: inventoryCount }]
      : []),
    { id: 'holders', label: 'Holders', count: holdersCount },
    { id: 'contract', label: 'Contract' },
  ];
  return (
    <div className="flex items-center gap-1 border-b border-neutral-200">
      {tabs.map((t) => {
        const isActive = active === t.id;
        return (
          <button
            key={t.id}
            onClick={() => onChange(t.id)}
            className={[
              'relative -mb-px px-4 py-3 text-sm font-medium transition-colors',
              isActive
                ? 'text-primary'
                : 'text-neutral-500 hover:text-neutral-800',
            ].join(' ')}
          >
            <span>{t.label}</span>
            {typeof t.count === 'number' && (
              <span className="ml-2 rounded-full bg-neutral-200/60 px-1.5 py-0.5 text-[11px] tabular-nums text-neutral-600">
                {t.count.toLocaleString()}
              </span>
            )}
            {isActive && (
              <span className="absolute inset-x-3 -bottom-px h-0.5 rounded-full bg-primary" />
            )}
          </button>
        );
      })}
    </div>
  );
}

// ---------- Transfers ----------

function TransfersTable({ transfers, token }: { transfers: TokenTransfer[]; token: Token }) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-neutral-200 bg-neutral-50">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-200">
            <Th>Txn hash</Th>
            <Th className="hidden md:table-cell">Method</Th>
            <Th>From → To</Th>
            <Th align="right">Value {token.symbol}</Th>
          </tr>
        </thead>
        <tbody>
          {transfers.map((t) => (
            <TransferRow key={`${t.txHash}-${t.logIndex}`} transfer={t} token={token} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TransferRow({ transfer, token }: { transfer: TokenTransfer; token: Token }) {
  const isMint = transfer.from === '0x0000000000000000000000000000000000000000';
  const isBurn = transfer.to === '0x0000000000000000000000000000000000000000';
  const method = isMint ? 'mint' : isBurn ? 'burn' : 'transfer';
  const fromReason = transfer.addressMetadata?.[transfer.from?.toLowerCase()];
  const toReason = transfer.addressMetadata?.[transfer.to?.toLowerCase()];

  return (
    <tr className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10">
      <td className="px-4 py-4 align-middle">
        <Link
          to={`/tx/${transfer.txHash}`}
          className="font-mono text-sm text-primary transition-colors hover:text-primary-600"
        >
          {transfer.txHash.slice(0, 10)}...{transfer.txHash.slice(-4)}
        </Link>
        {transfer.timestamp && (
          <div className="mt-0.5 text-xs text-neutral-500">{formatTimeAgo(transfer.timestamp)}</div>
        )}
      </td>
      <td className="hidden px-4 py-4 align-middle md:table-cell">
        <MethodBadge method={method} />
      </td>
      <td className="px-4 py-4 align-middle">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="inline-flex items-center gap-1">
            <AddressLink address={transfer.from} reason={fromReason} chars={4} />
            <AddressLabel reason={fromReason} />
          </span>
          <ArrowRight className="h-3.5 w-3.5 text-neutral-400" />
          <span className="inline-flex items-center gap-1">
            <AddressLink address={transfer.to} reason={toReason} chars={4} />
            <AddressLabel reason={toReason} />
          </span>
        </div>
      </td>
      <td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">
        {formatTokenValue(transfer.value, token.decimals)}
      </td>
    </tr>
  );
}

function MethodBadge({ method }: { method: string }) {
  const tone =
    method === 'mint'
      ? 'bg-primary/10 text-primary'
      : method === 'burn'
      ? 'bg-error-50 text-error-700 dark:bg-error-500/10 dark:text-error-500'
      : 'bg-neutral-200/60 text-neutral-700';
  return (
    <span
      className={`inline-flex items-center rounded-md px-2 py-0.5 font-mono text-xs ${tone}`}
    >
      {method}
    </span>
  );
}

// ---------- Holders ----------

function HoldersTable({
  holders,
  token,
  page,
}: {
  holders: TokenHolder[];
  token: Token;
  page: number;
}) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-neutral-200 bg-neutral-50">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-200">
            <Th className="w-12">#</Th>
            <Th>Address</Th>
            <Th align="right">Balance</Th>
            <Th align="right" className="hidden sm:table-cell">Percentage</Th>
          </tr>
        </thead>
        <tbody>
          {holders.map((holder, idx) => {
            const reason = holder.addressMetadata?.[holder.address?.toLowerCase()];
            const pct = Math.min(100, Math.max(0, holder.percentage));
            return (
              <tr
                key={holder.address}
                className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10"
              >
                <td className="px-4 py-4 align-middle text-sm tabular-nums text-neutral-400">
                  {(page - 1) * PAGE_SIZE + idx + 1}
                </td>
                <td className="px-4 py-4 align-middle">
                  <div className="flex items-center gap-2">
                    <AddressLink address={holder.address} reason={reason} full />
                    <CopyButton text={holder.address} />
                    <AddressLabel reason={reason} />
                    {holder.isContract && (
                      <span className="inline-flex items-center rounded-md bg-neutral-200/60 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wider text-neutral-700">
                        Contract
                      </span>
                    )}
                  </div>
                </td>
                <td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">
                  {formatTokenValue(holder.balance, token.decimals)}
                </td>
                <td className="hidden px-4 py-4 align-middle sm:table-cell">
                  <div className="flex items-center justify-end gap-3">
                    <div className="hidden h-1.5 w-24 overflow-hidden rounded-full bg-neutral-200 md:block">
                      <div
                        className="h-full rounded-full bg-primary/70"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                    <span className="w-14 text-right text-sm tabular-nums text-neutral-700">
                      {pct.toFixed(2)}%
                    </span>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ---------- Inventory ----------

function InventoryGrid({ items, collection }: { items: NFTItem[]; collection: string }) {
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
      {items.map((it) => (
        <NftCard key={it.tokenId} item={it} collection={collection} />
      ))}
    </div>
  );
}

function CardGridSkeleton({ count = 8 }: { count?: number }) {
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50"
        >
          <div className="aspect-square w-full animate-pulse bg-neutral-200" />
          <div className="space-y-2 p-3">
            <div className="h-3 w-16 animate-pulse rounded bg-neutral-200" />
            <div className="h-3 w-24 animate-pulse rounded bg-neutral-200/70" />
          </div>
        </div>
      ))}
    </div>
  );
}

// ---------- Contract view ----------

function ContractView({
  contract,
  creationInput,
  loading,
}: {
  contract: Contract | null | undefined;
  creationInput: string | undefined;
  loading: boolean;
}) {
  if (loading && !contract) {
    return <TableSkeleton rows={3} />;
  }
  if (!contract) {
    return <EmptyState>No contract metadata available for this address.</EmptyState>;
  }

  const items: { label: string; value: React.ReactNode }[] = [
    {
      label: 'Creator',
      value: (
        <Link
          to={`/address/${contract.creator}`}
          className="font-mono text-sm text-primary transition-colors hover:text-primary-600"
        >
          {contract.creator}
        </Link>
      ),
    },
    {
      label: 'Creation tx',
      value: (
        <Link
          to={`/tx/${contract.creationTx}`}
          className="font-mono text-sm text-primary transition-colors hover:text-primary-600 truncate inline-block max-w-full"
          title={contract.creationTx}
        >
          {contract.creationTx}
        </Link>
      ),
    },
    {
      label: 'Deployed at block',
      value: (
        <Link
          to={`/block/${contract.blockNumber}`}
          className="font-mono text-sm text-primary transition-colors hover:text-primary-600 tabular-nums"
        >
          {contract.blockNumber.toLocaleString()}
        </Link>
      ),
    },
    {
      label: 'Verification',
      value: contract.isVerified ? (
        <span className="inline-flex items-center gap-1.5 text-sm text-success-600">
          <span className="h-1.5 w-1.5 rounded-full bg-success-500" />
          Verified{contract.contractName ? ` (${contract.contractName})` : ''}
        </span>
      ) : (
        <span className="text-sm text-neutral-500">Not verified</span>
      ),
    },
  ];

  return (
    <div className="space-y-5">
      <dl className="divide-y divide-neutral-200/70 overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
        {items.map((it) => (
          <div key={it.label} className="flex items-center gap-4 px-5 py-3">
            <dt className="w-40 shrink-0 text-sm text-neutral-500">{it.label}</dt>
            <dd className="min-w-0 flex-1 text-sm text-neutral-900">{it.value}</dd>
          </div>
        ))}
      </dl>

      <BytecodeBlock
        title="Contract creation code"
        hint="The data sent in the deployment transaction (initcode + constructor arguments)."
        hex={creationInput}
        loading={loading && !creationInput}
      />

      <BytecodeBlock
        title="Deployed bytecode"
        hint="The runtime bytecode currently stored at this address, returned by eth_getCode."
        hex={contract.bytecode}
      />
    </div>
  );
}

function BytecodeBlock({
  title,
  hint,
  hex,
  loading = false,
}: {
  title: string;
  hint?: string;
  hex: string | undefined;
  loading?: boolean;
}) {
  const value = hex && hex !== '0x' ? hex : '';
  // Bytes = (length - "0x") / 2; show as a quick scale cue.
  const bytes = value ? Math.max(0, Math.floor((value.length - 2) / 2)) : 0;

  return (
    <section className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-neutral-200 px-5 py-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-neutral-900">{title}</h3>
          {hint && <p className="mt-0.5 text-xs text-neutral-500">{hint}</p>}
        </div>
        <div className="flex items-center gap-3">
          {value && (
            <span className="font-mono text-xs tabular-nums text-neutral-500">
              {bytes.toLocaleString()} bytes
            </span>
          )}
          {value && <CopyButton text={value} />}
        </div>
      </header>
      <div className="bg-neutral-100/60 px-5 py-4">
        {loading ? (
          <div className="space-y-1.5">
            <div className="h-3 w-full animate-pulse rounded bg-neutral-200" />
            <div className="h-3 w-5/6 animate-pulse rounded bg-neutral-200/70" />
            <div className="h-3 w-2/3 animate-pulse rounded bg-neutral-200/70" />
          </div>
        ) : !value ? (
          <p className="text-sm text-neutral-500">Empty (no bytecode at this address).</p>
        ) : (
          <pre className="max-h-80 overflow-y-auto break-all font-mono text-xs leading-relaxed text-neutral-700">
            {value}
          </pre>
        )}
      </div>
    </section>
  );
}

// ---------- Small bits ----------

function Th({
  children,
  align = 'left',
  className = '',
}: {
  children: React.ReactNode;
  align?: 'left' | 'right';
  className?: string;
}) {
  return (
    <th
      className={`px-4 py-3 text-xs font-medium uppercase tracking-wider text-neutral-500 ${
        align === 'right' ? 'text-right' : 'text-left'
      } ${className}`}
    >
      {children}
    </th>
  );
}

function Pagination({
  page,
  totalPages,
  onChange,
  className = '',
}: {
  page: number;
  totalPages: number;
  onChange: (p: number) => void;
  className?: string;
}) {
  return (
    <div className={`flex items-center justify-between gap-3 ${className}`}>
      <span className="text-xs text-neutral-500 tabular-nums">
        Page {page} of {Math.max(totalPages, 1)}
      </span>
      <div className="inline-flex items-center overflow-hidden rounded-full border border-neutral-200 bg-neutral-50">
        <button
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Previous
        </button>
        <span className="h-5 w-px bg-neutral-200" aria-hidden />
        <button
          onClick={() => onChange(page + 1)}
          disabled={page >= totalPages}
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Next
        </button>
      </div>
    </div>
  );
}

function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-2xl border border-dashed border-neutral-200 bg-neutral-100/30 px-6 py-12 text-center text-sm text-neutral-500">
      {children}
    </div>
  );
}

function TableSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-4 border-b border-neutral-200/60 px-4 py-5 last:border-b-0"
        >
          <div className="h-3 w-32 animate-pulse rounded bg-neutral-200" />
          <div className="hidden h-3 w-16 animate-pulse rounded bg-neutral-200/70 md:block" />
          <div className="flex-1 h-3 animate-pulse rounded bg-neutral-200/70" />
          <div className="h-3 w-16 animate-pulse rounded bg-neutral-200" />
        </div>
      ))}
    </div>
  );
}

function HeroSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <div className="h-14 w-14 animate-pulse rounded-full bg-neutral-200" />
        <div className="h-7 w-64 animate-pulse rounded bg-neutral-200" />
      </div>
      <div className="h-9 w-96 animate-pulse rounded-full bg-neutral-200/60" />
    </div>
  );
}
