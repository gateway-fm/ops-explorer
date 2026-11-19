import type React from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, FileCode2, User as UserIcon } from 'lucide-react';
import { CopyButton } from './CopyButton';
export type AddressTab = 'details' | 'transactions' | 'transfers' | 'tokens' | 'internal' | 'coin-history' | 'logs' | 'contract';
interface AddressContractSummary { isVerified?: boolean; contractName?: string }
interface AddressTokenSummary {address: string; tokenType: string; symbol?: string}
export function AddressBackButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="inline-flex items-center gap-1.5 text-xs font-medium text-neutral-500 transition-colors hover:text-primary"
    >
      <ArrowLeft className="h-3.5 w-3.5" />
      Back
    </button>
  );
}

export function AddressHero({
  address,
  isContract,
  contract,
  token,
}: {
  address: string;
  isContract: boolean | undefined;
  contract: AddressContractSummary | null;
  token: AddressTokenSummary | null;
}) {
  return (
    <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between lg:gap-6">
      <div className="flex min-w-0 items-center gap-4">
        <AddressAvatar isContract={isContract} />
        <div className="min-w-0">
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1.5">
            <h1 className="text-2xl font-bold text-neutral-900 sm:text-3xl">
              {isContract ? 'Contract' : 'Address'}
            </h1>
            <TypeBadge isContract={isContract} />
            {token && <TokenBadge token={token} />}
            {contract?.isVerified && (
              <span data-testid="verified-badge" className="inline-flex items-center gap-1.5 rounded-md bg-success-50 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wider text-success-700 dark:bg-success-500/10 dark:text-success-500">
                <span className="h-1.5 w-1.5 rounded-full bg-success-500" />
                Verified
              </span>
            )}
            {contract?.contractName && !token && (
              <span className="inline-flex items-center rounded-md bg-primary/10 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wider text-primary">
                {contract.contractName}
              </span>
            )}
          </div>
        </div>
      </div>

      <div className="flex min-w-0 max-w-full items-center gap-2 self-start rounded-full border border-neutral-200 bg-neutral-50 px-3 py-1.5 lg:w-auto lg:self-auto">
        <span
          className="min-w-0 flex-1 truncate font-mono text-sm text-neutral-700 lg:flex-initial"
          title={address}
        >
          {address}
        </span>
        <CopyButton text={address} />
      </div>
    </div>
  );
}

function TokenBadge({ token }: { token: AddressTokenSummary }) {
  const kind = token.tokenType === 'ERC721'
    ? 'NFT'
    : token.tokenType === 'ERC1155'
    ? 'Multi-token'
    : 'Token';
  return (
    <Link
      to={`/token/${token.address}`}
      className="group inline-flex items-center gap-1.5 rounded-md bg-primary/10 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wider text-primary transition-colors hover:bg-primary/15"
    >
      <span>{kind}</span>
      {token.symbol && (
        <span className="text-primary/70 group-hover:text-primary">· {token.symbol}</span>
      )}
    </Link>
  );
}

function AddressAvatar({ isContract }: { isContract: boolean | undefined }) {
  // Neutral avatar (not a token — no hash needed); icon distinguishes EOA / Contract.
  const Icon = isContract ? FileCode2 : UserIcon;
  return (
    <div
      className="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"
    >
      <Icon className="h-6 w-6" />
    </div>
  );
}

function TypeBadge({ isContract }: { isContract: boolean | undefined }) {
  if (isContract === undefined) return <span className="text-xs text-neutral-500">Type unavailable</span>;
  return isContract ? (
    <span className="inline-flex items-center rounded-md bg-primary/10 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wider text-primary">
      Contract
    </span>
  ) : (
    <span className="inline-flex items-center rounded-md bg-neutral-200/60 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wider text-neutral-700">
      EOA
    </span>
  );
}

// ============================================================
// Tabs
// ============================================================

export function AddressTabs({
  active,
  onChange,
  isContract,
  txCount,
  tokenTransferCount,
  internalTxCount,
  tokenCount,
}: {
  active: AddressTab;
  onChange: (t: AddressTab) => void;
  isContract: boolean | undefined;
  txCount?: number | string;
  tokenTransferCount?: number | string;
  internalTxCount?: number | string;
  tokenCount?: number | string;
}) {
  // Read / Write live as sub-tabs inside Contract (see ContractPanel).
  const items: { id: AddressTab; label: string; count?: number | string; icon?: React.ReactNode; visible: boolean }[] = [
    { id: 'details', label: 'Details', visible: true },
    { id: 'transactions', label: 'Transactions', count: txCount, visible: true },
    { id: 'transfers', label: 'Token transfers', count: tokenTransferCount, visible: true },
    { id: 'tokens', label: 'Tokens', count: tokenCount, visible: true },
    { id: 'internal', label: 'Internal txns', count: internalTxCount, visible: true },
    { id: 'coin-history', label: 'Coin balance', visible: true },
    { id: 'logs', label: 'Logs', visible: true },
    { id: 'contract', label: 'Contract', icon: <FileCode2 className="h-3.5 w-3.5" />, visible: isContract === true },
  ];
  return (
    <div className="flex flex-wrap items-center gap-x-1 gap-y-0 border-b border-neutral-200" role="tablist">
      {items.filter((i) => i.visible).map((t) => {
        const isActive = active === t.id;
        return (
          <button
            key={t.id}
            onClick={() => onChange(t.id)}
            role="tab"
            data-testid={`tab-${t.id}`}
            aria-selected={isActive}
            className={[
              'relative -mb-px inline-flex items-center gap-1.5 whitespace-nowrap px-2.5 py-3 text-sm font-medium transition-colors sm:px-4',
              isActive ? 'text-primary' : 'text-neutral-500 hover:text-neutral-800',
            ].join(' ')}
          >
            {t.icon}
            <span>{t.label}</span>
            {t.count !== undefined && (
              <span data-testid={`tab-count-${t.id}`} className="rounded-full bg-neutral-200/60 px-1.5 py-0.5 text-[11px] tabular-nums text-neutral-600">
                {typeof t.count === 'number' ? t.count.toLocaleString() : t.count}
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

export function AddressDetailsPanel({ rows }: { rows: {label: string; value: React.ReactNode}[]}) {
  return (
    <dl className="divide-y divide-neutral-200/70 overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      {rows.map((r) => (
        <div key={r.label} className="flex flex-col gap-1 px-5 py-3.5 sm:flex-row sm:items-center sm:gap-6">
          <dt className="text-xs uppercase tracking-wider text-neutral-500 sm:w-44 sm:shrink-0 sm:text-sm sm:normal-case sm:tracking-normal">
            {r.label}
          </dt>
          <dd className="min-w-0 flex-1 text-sm text-neutral-900">{r.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function PanelShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-neutral-200 bg-neutral-50">
      {children}
    </div>
  );
}

export function EmptyRow({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-6 py-12 text-center text-sm text-neutral-500">{children}</div>
  );
}

export function Th({
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

export function LoadMore({ onClick }: { onClick: () => void }) {
  return (
    <div className="border-t border-neutral-200 px-4 py-3 text-center">
      <button
        onClick={onClick}
        data-testid="load-more"
        className="text-sm font-medium text-primary transition-colors hover:text-primary-600"
      >
        Load more
      </button>
    </div>
  );
}

export function AddressTransactionsTable({ children, footer }: {children: React.ReactNode; footer?: React.ReactNode}) {
  return (
    <PanelShell>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-200">
            <Th>Txn hash</Th>
            <Th className="hidden md:table-cell">Block</Th>
            <Th>From → To</Th>
            <Th align="right">Value</Th>
            <Th align="right" className="hidden lg:table-cell">Fee</Th>
          </tr>
        </thead>
        <tbody>
          {children}
        </tbody>
      </table>
      {footer}
    </PanelShell>
  );
}
