import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useState, useEffect, useCallback, useRef } from 'react';
import {
  ArrowLeft,
  ArrowRight,
  FileCode2,
  User as UserIcon,
  ShieldCheck,
  ShieldOff,
  Wallet,
  X,
  Loader2,
  BookOpen,
  PenLine,
  Network,
  Download,
} from 'lucide-react';
import { api } from '../lib/api';
import type {
  Balance,
  Contract,
  InternalTransaction,
  Log,
  Token,
  Transaction,
  TokenTransfer,
} from '../lib/api';
import { AddressLink } from '../components/AddressLink';
import { AddressLabel } from '../components/AddressLabel';
import { CopyButton } from '../components/CopyButton';
import { ContractInteraction } from '../components/ContractInteraction';
import { TokenAvatar } from '../components/TokenAvatar';
import { useTokenMap } from '../hooks/useTokenMap';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';
import { formatWei, formatHash, formatTimeAgo, getNetworkCurrency } from '../lib/utils';
import { formatTokenValue } from '../lib/formatToken';
import { useAuth } from '../lib/auth';
import { features } from '../lib/features';

const TABS = [
  'details',
  'transactions',
  'transfers',
  'tokens',
  'internal',
  'coin-history',
  'logs',
  'contract',
] as const;
type Tab = (typeof TABS)[number];

const CONTRACT_SUB_TABS = ['overview', 'read', 'write'] as const;
type ContractSubTab = (typeof CONTRACT_SUB_TABS)[number];

function tabFromParams(s: string | null): Tab {
  return TABS.includes(s as Tab) ? (s as Tab) : 'details';
}

function contractSubTabFromParams(s: string | null): ContractSubTab {
  return CONTRACT_SUB_TABS.includes(s as ContractSubTab) ? (s as ContractSubTab) : 'overview';
}

const PAGE_SIZE = 25;

export function Address() {
  const { address } = useParams<{ address: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab: Tab = tabFromParams(searchParams.get('tab'));
  const contractSubTab: ContractSubTab = contractSubTabFromParams(searchParams.get('sub'));
  const before = searchParams.get('before');
  const page = parseInt(searchParams.get('page') || '1', 10);

  const setTab = (tab: Tab) => {
    const params = new URLSearchParams();
    if (tab !== 'details') params.set('tab', tab);
    setSearchParams(params);
  };

  const setContractSubTab = (sub: ContractSubTab) => {
    const params = new URLSearchParams();
    params.set('tab', 'contract');
    if (sub !== 'overview') params.set('sub', sub);
    setSearchParams(params);
  };

  const { isAuthenticated } = useAuth();

  const { data: info, isLoading: infoLoading, error } = useQuery({
    queryKey: ['address', address, isAuthenticated],
    queryFn: () => api.getAddress(address!),
    enabled: !!address,
    retry: false,
  });

  const isContract = !!info?.isContract;

  // Always available regardless of which tab is active.
  const { data: contract } = useQuery({
    queryKey: ['contract', address],
    queryFn: () => api.getContract(address!),
    enabled: !!address && isContract,
    retry: false,
  });

  const { data: balances } = useQuery({
    queryKey: ['addressBalances', address],
    queryFn: () => api.getAddressTokenBalances(address!),
    enabled: !!address,
    retry: false,
  });

  // If the contract address happens to be a token, pick up its metadata so
  // the hero can flag it as a Token and Details can show name + symbol.
  // Falls back to null on a 404 instead of bubbling the error.
  const { data: tokenInfo } = useQuery<Token | null>({
    queryKey: ['tokenForAddress', address],
    queryFn: () => api.getToken(address!).catch(() => null),
    enabled: !!address && isContract,
    retry: false,
  });

  // Tab-scoped fetches.
  const { data: txs, isLoading: txsLoading } = useQuery({
    queryKey: ['addressTxs', address, before],
    queryFn: () => api.getAddressTransactions(address!, PAGE_SIZE, before ? parseInt(before) : undefined),
    enabled: !!address && activeTab === 'transactions',
    retry: false,
  });
  const { data: transfers, isLoading: transfersLoading } = useQuery({
    queryKey: ['addressTransfers', address, before],
    queryFn: () => api.getAddressTransfers(address!, PAGE_SIZE, before ? parseInt(before) : undefined),
    enabled: !!address && activeTab === 'transfers',
    retry: false,
  });
  const { data: internalTxs, isLoading: internalLoading } = useQuery({
    queryKey: ['addressInternal', address, page],
    queryFn: () => api.getAddressInternalTxs(address!, page, PAGE_SIZE),
    enabled: !!address && activeTab === 'internal',
    retry: false,
  });
  const { data: logs, isLoading: logsLoading } = useQuery({
    queryKey: ['addressLogs', address, page],
    queryFn: () => api.getAddressLogs(address!, page, PAGE_SIZE),
    enabled: !!address && activeTab === 'logs',
    retry: false,
  });

  const navigate = useNavigate();
  const goBack = () => (window.history.length > 1 ? navigate(-1) : navigate('/'));

  // Wallet state (only used by Write tab).
  const [walletAddress, setWalletAddress] = useState<string | null>(null);
  const manuallyDisconnected = useRef(false);
  useEffect(() => {
    if (!window.ethereum || manuallyDisconnected.current) return;
    window.ethereum.request({ method: 'eth_accounts' }).then((accounts: string[]) => {
      if (accounts && accounts.length > 0 && !manuallyDisconnected.current) {
        setWalletAddress(accounts[0]);
      }
    }).catch(() => {});
    const handleAccountsChanged = (accounts: string[]) => {
      if (manuallyDisconnected.current) return;
      setWalletAddress(accounts.length > 0 ? accounts[0] : null);
    };
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const eth = window.ethereum as any;
    eth.on('accountsChanged', handleAccountsChanged);
    return () => eth.removeListener('accountsChanged', handleAccountsChanged);
  }, []);

  const [walletPending, setWalletPending] = useState(false);
  const [walletConnecting, setWalletConnecting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const stopPolling = useCallback(() => {
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null; }
  }, []);
  const startPolling = useCallback(() => {
    if (pollRef.current) return;
    pollRef.current = setInterval(async () => {
      try {
        const accounts = await window.ethereum?.request({ method: 'eth_accounts' });
        if (accounts && accounts.length > 0) {
          setWalletAddress(accounts[0]);
          setWalletPending(false);
          setWalletConnecting(false);
          stopPolling();
        }
      } catch { /* ignore */ }
    }, 1000);
  }, [stopPolling]);
  useEffect(() => () => stopPolling(), [stopPolling]);
  const cancelConnect = useCallback(() => {
    stopPolling();
    setWalletPending(false);
    setWalletConnecting(false);
  }, [stopPolling]);
  const connectWallet = useCallback(async () => {
    if (!window.ethereum) {
      alert('MetaMask is not installed.');
      return;
    }
    manuallyDisconnected.current = false;
    setWalletConnecting(true);
    setWalletPending(false);
    try {
      const existing = await window.ethereum.request({ method: 'eth_accounts' });
      if (existing && existing.length > 0) {
        setWalletAddress(existing[0]);
        setWalletConnecting(false);
        return;
      }
      const accounts = await Promise.race([
        window.ethereum.request({ method: 'eth_requestAccounts' }),
        new Promise<never>((_, reject) => setTimeout(() => reject(new Error('timeout')), 500)),
      ]);
      if (accounts && accounts.length > 0) {
        setWalletAddress(accounts[0]);
        setWalletConnecting(false);
        return;
      }
    } catch { /* fall through to pending */ }
    setWalletPending(true);
    setWalletConnecting(false);
    startPolling();
  }, [startPolling]);
  const disconnectWallet = useCallback(() => {
    manuallyDisconnected.current = true;
    setWalletAddress(null);
    if (window.ethereum) {
      window.ethereum.request({
        method: 'wallet_revokePermissions',
        params: [{ eth_accounts: {} }],
      }).catch(() => {});
    }
  }, []);

  if (infoLoading) {
    return (
      <div className="space-y-6">
        <BackButton onClick={goBack} />
        <HeroSkeleton />
      </div>
    );
  }

  if (error instanceof Error && (error.message.includes('403') || error.message.includes('500'))) {
    return (
      <div className="space-y-6">
        <BackButton onClick={goBack} />
        <div className="rounded-2xl border border-neutral-200 bg-neutral-50 px-6 py-16 text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-neutral-100">
            <ShieldOff className="h-5 w-5 text-neutral-400" />
          </div>
          <h2 className="text-lg font-semibold text-neutral-900">Address restricted</h2>
          <p className="mx-auto mt-1 max-w-md text-sm text-neutral-500">
            This address is protected by privacy controls. You do not currently have permission to view its details.
          </p>
        </div>
      </div>
    );
  }

  if (error || !info) {
    return (
      <div className="space-y-6">
        <BackButton onClick={goBack} />
        <div className="rounded-2xl border border-dashed border-neutral-200 bg-neutral-100/30 px-6 py-12 text-center text-sm text-neutral-500">
          Address not found.
        </div>
      </div>
    );
  }

  const hasAbi = contract?.abi && contract.abi.length > 0;
  const tokenBalanceCount = balances?.length ?? 0;

  return (
    <div className="space-y-6">
      <BackButton onClick={goBack} />

      <Hero
        address={info.address}
        isContract={isContract}
        contract={contract ?? null}
        token={tokenInfo ?? null}
      />

      <Tabs
        active={activeTab}
        onChange={setTab}
        isContract={isContract}
        txCount={info.txCount}
        tokenTransferCount={info.tokenTransferCount}
        internalTxCount={info.internalTxCount}
        tokenCount={tokenBalanceCount}
      />

      <section>
        {activeTab === 'details' && (
          <DetailsPanel
            info={info}
            contract={contract ?? null}
            token={tokenInfo ?? null}
            tokenBalanceCount={tokenBalanceCount}
          />
        )}
        {activeTab === 'transactions' && (
          <TransactionsPanel
            data={txs?.data}
            hasMore={!!txs?.hasMore}
            isLoading={txsLoading}
            currentAddress={info.address}
            tokenTransferCount={info.tokenTransferCount}
            onShowTransfers={() => setTab('transfers')}
            onLoadMore={() => {
              const last = txs?.data?.[txs.data.length - 1];
              if (last) setSearchParams({ before: String(last.blockNumber) });
            }}
          />
        )}
        {activeTab === 'transfers' && (
          <TransfersPanel
            data={transfers?.data}
            hasMore={!!transfers?.hasMore}
            isLoading={transfersLoading}
            currentAddress={info.address}
            onLoadMore={() => {
              const last = transfers?.data?.[transfers.data.length - 1];
              if (last) {
                const p = new URLSearchParams({ tab: 'transfers', before: String(last.blockNumber) });
                setSearchParams(p);
              }
            }}
          />
        )}
        {activeTab === 'tokens' && <TokensPanel balances={balances} />}
        {activeTab === 'internal' && (
          <InternalPanel
            data={internalTxs?.data}
            isLoading={internalLoading}
            currentAddress={info.address}
            page={page}
            totalPages={internalTxs?.totalPages ?? 1}
            onPage={(p) => setSearchParams({ tab: 'internal', page: String(p) })}
          />
        )}
        {activeTab === 'coin-history' && <CoinHistoryPanel balance={info.balance} />}
        {activeTab === 'logs' && (
          <LogsPanel
            data={logs?.data}
            isLoading={logsLoading}
            page={page}
            totalPages={logs?.totalPages ?? 1}
            onPage={(p) => setSearchParams({ tab: 'logs', page: String(p) })}
          />
        )}
        {activeTab === 'contract' && (
          <ContractPanel
            contract={contract ?? null}
            addressForVerifyLink={info.address}
            address={address!}
            subTab={contractSubTab}
            onSubTab={setContractSubTab}
            hasAbi={!!hasAbi}
            walletAddress={walletAddress}
            walletConnecting={walletConnecting}
            walletPending={walletPending}
            connectWallet={connectWallet}
            disconnectWallet={disconnectWallet}
            cancelConnect={cancelConnect}
          />
        )}
      </section>
    </div>
  );
}

// ============================================================
// Header bits
// ============================================================

function BackButton({ onClick }: { onClick: () => void }) {
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

function Hero({
  address,
  isContract,
  contract,
  token,
}: {
  address: string;
  isContract: boolean;
  contract: Contract | null;
  token: Token | null;
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
              <span className="inline-flex items-center gap-1.5 rounded-md bg-success-50 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wider text-success-700 dark:bg-success-500/10 dark:text-success-500">
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

function TokenBadge({ token }: { token: Token }) {
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

function AddressAvatar({ isContract }: { isContract: boolean }) {
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

function TypeBadge({ isContract }: { isContract: boolean }) {
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

function Tabs({
  active,
  onChange,
  isContract,
  txCount,
  tokenTransferCount,
  internalTxCount,
  tokenCount,
}: {
  active: Tab;
  onChange: (t: Tab) => void;
  isContract: boolean;
  txCount: number;
  tokenTransferCount: number;
  internalTxCount: number;
  tokenCount: number;
}) {
  // Read / Write live as sub-tabs inside Contract (see ContractPanel).
  const items: { id: Tab; label: string; count?: number; icon?: React.ReactNode; visible: boolean }[] = [
    { id: 'details', label: 'Details', visible: true },
    { id: 'transactions', label: 'Transactions', count: txCount, visible: true },
    { id: 'transfers', label: 'Token transfers', count: tokenTransferCount, visible: true },
    { id: 'tokens', label: 'Tokens', count: tokenCount, visible: true },
    { id: 'internal', label: 'Internal txns', count: internalTxCount, visible: true },
    { id: 'coin-history', label: 'Coin balance', visible: true },
    { id: 'logs', label: 'Logs', visible: true },
    { id: 'contract', label: 'Contract', icon: <FileCode2 className="h-3.5 w-3.5" />, visible: isContract },
  ];
  return (
    <div className="flex flex-wrap items-center gap-x-1 gap-y-0 border-b border-neutral-200">
      {items.filter((i) => i.visible).map((t) => {
        const isActive = active === t.id;
        return (
          <button
            key={t.id}
            onClick={() => onChange(t.id)}
            className={[
              'relative -mb-px inline-flex items-center gap-1.5 whitespace-nowrap px-2.5 py-3 text-sm font-medium transition-colors sm:px-4',
              isActive ? 'text-primary' : 'text-neutral-500 hover:text-neutral-800',
            ].join(' ')}
          >
            {t.icon}
            <span>{t.label}</span>
            {typeof t.count === 'number' && (
              <span className="rounded-full bg-neutral-200/60 px-1.5 py-0.5 text-[11px] tabular-nums text-neutral-600">
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

// ============================================================
// Details
// ============================================================

function DetailsPanel({
  info,
  contract,
  token,
  tokenBalanceCount,
}: {
  info: import('../lib/api').AddressInfo;
  contract: Contract | null;
  token: Token | null;
  tokenBalanceCount: number;
}) {
  const rows: { label: string; value: React.ReactNode }[] = [];

  if (token) {
    rows.push({
      label: 'Token',
      value: (
        <Link
          to={`/token/${token.address}`}
          className="inline-flex flex-wrap items-baseline gap-x-2 text-sm text-primary transition-colors hover:text-primary-600"
        >
          <span className="font-semibold">{token.name || 'Unknown Token'}</span>
          {token.symbol && <span className="text-primary/70">({token.symbol})</span>}
        </Link>
      ),
    });
  }

  rows.push({
    label: 'Balance',
    value: (
      <span>
        <span className="font-mono tabular-nums text-neutral-900">{formatWei(info.balance)}</span>
        <span className="ml-1.5 text-sm text-neutral-500">{getNetworkCurrency()}</span>
      </span>
    ),
  });

  rows.push({
    label: 'Transactions',
    value: <span className="font-mono tabular-nums">{info.txCount.toLocaleString()}</span>,
  });
  rows.push({
    label: 'Token transfers',
    value: <span className="font-mono tabular-nums">{info.tokenTransferCount.toLocaleString()}</span>,
  });
  rows.push({
    label: 'Tokens held',
    value: <span className="font-mono tabular-nums">{tokenBalanceCount.toLocaleString()}</span>,
  });
  rows.push({
    label: 'Internal txns',
    value: <span className="font-mono tabular-nums">{info.internalTxCount.toLocaleString()}</span>,
  });

  if (contract) {
    rows.push({
      label: 'Creator',
      value: (
        <span className="inline-flex flex-wrap items-baseline gap-x-2 text-sm">
          <AddressLink address={contract.creator} chars={6} />
          <span className="text-neutral-500">at txn</span>
          <Link
            to={`/tx/${contract.creationTx}`}
            className="font-mono text-primary transition-colors hover:text-primary-600"
            title={contract.creationTx}
          >
            {formatHash(contract.creationTx, 6)}
          </Link>
        </span>
      ),
    });
    rows.push({
      label: 'Deployed at block',
      value: (
        <Link
          to={`/block/${contract.blockNumber}`}
          className="font-mono text-sm text-primary transition-colors hover:text-primary-600 tabular-nums"
        >
          {contract.blockNumber.toLocaleString()}
        </Link>
      ),
    });
  }

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

// ============================================================
// Tab panels
// ============================================================

function PanelShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-neutral-200 bg-neutral-50">
      {children}
    </div>
  );
}

function EmptyRow({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-6 py-12 text-center text-sm text-neutral-500">{children}</div>
  );
}

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

function LoadMore({ onClick }: { onClick: () => void }) {
  return (
    <div className="border-t border-neutral-200 px-4 py-3 text-center">
      <button
        onClick={onClick}
        className="text-sm font-medium text-primary transition-colors hover:text-primary-600"
      >
        Load more
      </button>
    </div>
  );
}

function OffsetPager({
  page,
  totalPages,
  onPage,
}: {
  page: number;
  totalPages: number;
  onPage: (p: number) => void;
}) {
  return (
    <div className="mt-5 flex items-center justify-between gap-3">
      <span className="text-xs text-neutral-500 tabular-nums">
        Page {page} of {Math.max(totalPages, 1)}
      </span>
      <div className="inline-flex items-center overflow-hidden rounded-full border border-neutral-200 bg-neutral-50">
        <button
          onClick={() => onPage(page - 1)}
          disabled={page <= 1}
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Previous
        </button>
        <span className="h-5 w-px bg-neutral-200" aria-hidden />
        <button
          onClick={() => onPage(page + 1)}
          disabled={page >= totalPages}
          className="px-4 py-1.5 text-sm font-medium text-neutral-700 transition-colors hover:bg-primary-50 hover:text-primary disabled:cursor-not-allowed disabled:text-neutral-300 disabled:hover:bg-transparent"
        >
          Next
        </button>
      </div>
    </div>
  );
}

// ---------- Transactions ----------

function TransactionsPanel({
  data,
  hasMore,
  isLoading,
  currentAddress,
  onLoadMore,
  tokenTransferCount,
  onShowTransfers,
}: {
  data: Transaction[] | undefined;
  hasMore: boolean;
  isLoading: boolean;
  currentAddress: string;
  onLoadMore: () => void;
  tokenTransferCount: number;
  onShowTransfers: () => void;
}) {
  if (isLoading && !data) return <RowsSkeleton rows={4} />;
  if (!data?.length) {
    return (
      <PanelShell>
        <EmptyRow>
          This address has no direct transactions.
          {tokenTransferCount > 0 && (
            <>
              {' '}
              <button
                onClick={onShowTransfers}
                className="text-primary transition-colors hover:text-primary-600"
              >
                View {tokenTransferCount.toLocaleString()} token transfer{tokenTransferCount === 1 ? '' : 's'} →
              </button>
            </>
          )}
        </EmptyRow>
      </PanelShell>
    );
  }

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
          {data.map((tx) => <TxRow key={tx.hash} tx={tx} currentAddress={currentAddress} />)}
        </tbody>
      </table>
      {hasMore && <LoadMore onClick={onLoadMore} />}
    </PanelShell>
  );
}

function TxRow({ tx, currentAddress }: { tx: Transaction; currentAddress: string }) {
  const isOutgoing = tx.from.toLowerCase() === currentAddress.toLowerCase();
  const txFee = BigInt(tx.gasUsed || 0) * BigInt(tx.gasPrice || 0);
  const fromReason = tx.addressMetadata?.[tx.from?.toLowerCase()];
  const toReason = tx.to ? tx.addressMetadata?.[tx.to.toLowerCase()] : undefined;
  return (
    <tr className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10">
      <td className="px-4 py-4 align-middle">
        <Link to={`/tx/${tx.hash}`} className="font-mono text-sm text-primary transition-colors hover:text-primary-600">
          {formatHash(tx.hash, 8)}
        </Link>
        {tx.blockTimestamp && (
          <div className="mt-0.5 text-xs text-neutral-500">{formatTimeAgo(tx.blockTimestamp)}</div>
        )}
      </td>
      <td className="hidden px-4 py-4 align-middle text-sm tabular-nums md:table-cell">
        <Link to={`/block/${tx.blockNumber}`} className="text-primary transition-colors hover:text-primary-600">
          {tx.blockNumber.toLocaleString()}
        </Link>
      </td>
      <td className="px-4 py-4 align-middle">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <SelfOrLink address={tx.from} currentAddress={currentAddress} reason={fromReason} />
          <DirectionPill outgoing={isOutgoing} />
          {tx.to ? (
            <SelfOrLink address={tx.to} currentAddress={currentAddress} reason={toReason} />
          ) : (
            <span className="text-xs italic text-primary">Contract creation</span>
          )}
        </div>
      </td>
      <td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">
        {tx.value === '' || tx.value == null
          ? <span className="text-neutral-400">hidden</span>
          : `${formatWei(tx.value)} ${getNetworkCurrency()}`}
      </td>
      <td className="hidden px-4 py-4 text-right align-middle font-mono text-xs tabular-nums text-neutral-500 lg:table-cell">
        {formatWei(txFee.toString())}
      </td>
    </tr>
  );
}

function SelfOrLink({
  address,
  currentAddress,
  reason,
}: {
  address: string;
  currentAddress: string;
  reason?: import('../lib/api').VisibilityReason;
}) {
  if (address.toLowerCase() === currentAddress.toLowerCase()) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-default font-mono text-sm text-neutral-400">{formatHash(address, 4)}</span>
        </TooltipTrigger>
        <TooltipContent><span className="font-mono">{address}</span></TooltipContent>
      </Tooltip>
    );
  }
  return (
    <span className="inline-flex items-center gap-1">
      <AddressLink address={address} chars={4} reason={reason} />
      <AddressLabel reason={reason} />
    </span>
  );
}

function DirectionPill({ outgoing }: { outgoing: boolean }) {
  return outgoing ? (
    <span className="inline-flex items-center gap-0.5">
      <ArrowRight className="h-3 w-3 text-warning-600" />
      <span className="rounded bg-warning-500/10 px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wider text-warning-700 dark:text-warning-500">OUT</span>
    </span>
  ) : (
    <span className="inline-flex items-center gap-0.5">
      <ArrowRight className="h-3 w-3 text-success-600" />
      <span className="rounded bg-success-500/10 px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wider text-success-700 dark:text-success-500">IN</span>
    </span>
  );
}

// ---------- Token transfers ----------

function TransfersPanel({
  data,
  hasMore,
  isLoading,
  currentAddress,
  onLoadMore,
}: {
  data: TokenTransfer[] | undefined;
  hasMore: boolean;
  isLoading: boolean;
  currentAddress: string;
  onLoadMore: () => void;
}) {
  if (isLoading && !data) return <RowsSkeleton rows={4} />;
  if (!data?.length) return <PanelShell><EmptyRow>No token transfers involving this address.</EmptyRow></PanelShell>;

  return (
    <PanelShell>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-200">
            <Th>Txn hash</Th>
            <Th className="hidden md:table-cell">Token</Th>
            <Th>From → To</Th>
            <Th align="right">Value</Th>
          </tr>
        </thead>
        <tbody>
          {data.map((t) => {
            const isOut = t.from.toLowerCase() === currentAddress.toLowerCase();
            const fromReason = t.addressMetadata?.[t.from?.toLowerCase()];
            const toReason = t.addressMetadata?.[t.to?.toLowerCase()];
            return (
              <tr key={`${t.txHash}-${t.logIndex}`} className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10">
                <td className="px-4 py-4 align-middle">
                  <Link to={`/tx/${t.txHash}`} className="font-mono text-sm text-primary transition-colors hover:text-primary-600">
                    {formatHash(t.txHash, 8)}
                  </Link>
                  {t.timestamp && <div className="mt-0.5 text-xs text-neutral-500">{formatTimeAgo(t.timestamp)}</div>}
                </td>
                <td className="hidden px-4 py-4 align-middle md:table-cell">
                  <Link to={`/token/${t.tokenAddress}`} className="font-mono text-xs text-primary transition-colors hover:text-primary-600">
                    {formatHash(t.tokenAddress, 6)}
                  </Link>
                </td>
                <td className="px-4 py-4 align-middle">
                  <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <SelfOrLink address={t.from} currentAddress={currentAddress} reason={fromReason} />
                    <DirectionPill outgoing={isOut} />
                    <SelfOrLink address={t.to} currentAddress={currentAddress} reason={toReason} />
                  </div>
                </td>
                <td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">
                  {formatTokenValue(t.value, 18)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {hasMore && <LoadMore onClick={onLoadMore} />}
    </PanelShell>
  );
}

// ---------- Tokens (balances) ----------

function TokensPanel({ balances }: { balances: Balance[] | undefined }) {
  const [filter, setFilter] = useState<'all' | 'ERC20' | 'ERC721'>('all');
  const addresses = balances?.map((b) => b.tokenAddress) ?? [];
  const tokenMap = useTokenMap(addresses);

  if (!balances) return <RowsSkeleton rows={3} />;
  if (balances.length === 0) {
    return <PanelShell><EmptyRow>This address doesn't hold any indexed tokens.</EmptyRow></PanelShell>;
  }

  // Enrich + group. Drop NFTs from the ERC-20 view (and vice versa); "all"
  // shows everything. Total value is the sum of (qty * usdPrice) when known.
  const enriched = balances.map((b) => {
    const meta = tokenMap[b.tokenAddress.toLowerCase()];
    const decimals = meta?.decimals ?? 18;
    const isNft = meta?.tokenType === 'ERC721' || meta?.tokenType === 'ERC1155';
    let valueUSD: number | null = null;
    if (!isNft && meta?.usdPrice) {
      const human = Number(b.balance) / Math.pow(10, decimals);
      valueUSD = human * meta.usdPrice;
    }
    return { balance: b, meta, decimals, isNft, valueUSD };
  });

  const filtered = enriched.filter((e) => {
    if (filter === 'all') return true;
    if (filter === 'ERC20') return !e.isNft;
    return e.isNft;
  });

  const totalUsd = enriched.reduce((acc, e) => acc + (e.valueUSD ?? 0), 0);
  const erc20Count = enriched.filter((e) => !e.isNft).length;
  const nftCount = enriched.filter((e) => e.isNft).length;
  const hasAnyValue = totalUsd > 0;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="inline-flex items-center gap-1 rounded-full border border-neutral-200 bg-neutral-100/60 p-1">
          <FilterChip active={filter === 'all'} onClick={() => setFilter('all')}>
            All <span className="text-neutral-400">· {balances.length}</span>
          </FilterChip>
          <FilterChip active={filter === 'ERC20'} onClick={() => setFilter('ERC20')}>
            ERC-20 <span className="text-neutral-400">· {erc20Count}</span>
          </FilterChip>
          <FilterChip active={filter === 'ERC721'} onClick={() => setFilter('ERC721')}>
            NFTs <span className="text-neutral-400">· {nftCount}</span>
          </FilterChip>
        </div>
        {hasAnyValue && (
          <span className="text-sm text-neutral-500">
            Total <span className="font-mono tabular-nums text-neutral-900">${totalUsd.toLocaleString(undefined, { maximumFractionDigits: 2 })}</span>
          </span>
        )}
      </div>

      <PanelShell>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-neutral-200">
              <Th>Asset</Th>
              <Th className="hidden md:table-cell">Contract</Th>
              {hasAnyValue && <Th align="right" className="hidden lg:table-cell">Price</Th>}
              <Th align="right">Quantity</Th>
              {hasAnyValue && <Th align="right">Value</Th>}
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-6 py-10 text-center text-sm text-neutral-500">
                  No {filter === 'ERC20' ? 'ERC-20 tokens' : 'NFTs'} held.
                </td>
              </tr>
            ) : (
              filtered.map((e) => <TokenHoldingRow key={e.balance.tokenAddress} entry={e} showValue={hasAnyValue} />)
            )}
          </tbody>
        </table>
      </PanelShell>
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={[
        'rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors',
        active ? 'bg-neutral-50 text-neutral-900 shadow-card' : 'text-neutral-500 hover:text-neutral-800',
      ].join(' ')}
    >
      {children}
    </button>
  );
}

function TokenHoldingRow({
  entry,
  showValue,
}: {
  entry: {
    balance: Balance;
    meta: Token | undefined;
    decimals: number;
    isNft: boolean;
    valueUSD: number | null;
  };
  showValue: boolean;
}) {
  const { balance, meta, decimals, isNft, valueUSD } = entry;
  const tokenAddress = balance.tokenAddress;
  const name = meta?.name || 'Unknown Token';
  const symbol = meta?.symbol || '?';
  const qty = isNft
    ? `${Number(balance.balance).toLocaleString()} ${meta?.tokenType === 'ERC721' ? 'NFT' : 'items'}`
    : formatTokenValue(balance.balance, decimals);

  return (
    <tr className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10">
      <td className="px-4 py-4 align-middle">
        <div className="flex items-center gap-3">
          <TokenAvatar symbol={symbol} address={tokenAddress} size="sm" />
          <div className="min-w-0">
            <Link
              to={`/token/${tokenAddress}`}
              className="block font-semibold text-neutral-900 transition-colors hover:text-primary"
            >
              {name}
            </Link>
            <div className="flex items-center gap-2 text-xs text-neutral-500">
              <span>{symbol}</span>
              {meta?.tokenType && (
                <span className="rounded bg-neutral-200/60 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider text-neutral-700">
                  {formatTokenTypeShort(meta.tokenType)}
                </span>
              )}
            </div>
          </div>
        </div>
      </td>
      <td className="hidden px-4 py-4 align-middle md:table-cell">
        <div className="flex items-center gap-1.5">
          <Link
            to={`/address/${tokenAddress}`}
            className="font-mono text-xs text-neutral-500 transition-colors hover:text-primary"
            title={tokenAddress}
          >
            {formatHash(tokenAddress, 6)}
          </Link>
          <CopyButton text={tokenAddress} />
        </div>
      </td>
      {showValue && (
        <td className="hidden px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-700 lg:table-cell">
          {meta?.usdPrice ? `$${meta.usdPrice.toLocaleString(undefined, { maximumFractionDigits: 4 })}` : '—'}
        </td>
      )}
      <td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">
        {qty}
      </td>
      {showValue && (
        <td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">
          {valueUSD !== null ? `$${valueUSD.toLocaleString(undefined, { maximumFractionDigits: 2 })}` : '—'}
        </td>
      )}
    </tr>
  );
}

function formatTokenTypeShort(type: string): string {
  if (type === 'ERC721') return 'NFT';
  if (type === 'ERC1155') return 'Multi';
  return type.replace(/^ERC(\d+)$/, 'ERC-$1');
}

// ---------- Internal txns ----------

function InternalPanel({
  data,
  isLoading,
  currentAddress,
  page,
  totalPages,
  onPage,
}: {
  data: InternalTransaction[] | undefined;
  isLoading: boolean;
  currentAddress: string;
  page: number;
  totalPages: number;
  onPage: (p: number) => void;
}) {
  if (isLoading && !data) return <RowsSkeleton rows={3} />;
  if (!data?.length) return <PanelShell><EmptyRow>No internal transactions for this address.</EmptyRow></PanelShell>;
  return (
    <>
      <PanelShell>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-neutral-200">
              <Th>Parent txn</Th>
              <Th>Type</Th>
              <Th>From → To</Th>
              <Th align="right">Value</Th>
            </tr>
          </thead>
          <tbody>
            {data.map((it) => {
              const isOut = it.from.toLowerCase() === currentAddress.toLowerCase();
              return (
                <tr key={it.id} className="border-b border-neutral-200/60 transition-colors last:border-b-0 hover:bg-primary-50/40 dark:hover:bg-primary-900/10">
                  <td className="px-4 py-4 align-middle">
                    <Link to={`/tx/${it.txHash}`} className="font-mono text-sm text-primary transition-colors hover:text-primary-600">
                      {formatHash(it.txHash, 8)}
                    </Link>
                    {it.timestamp && <div className="mt-0.5 text-xs text-neutral-500">{formatTimeAgo(it.timestamp)}</div>}
                  </td>
                  <td className="px-4 py-4 align-middle">
                    <span className="inline-flex items-center rounded-md bg-neutral-200/60 px-2 py-0.5 font-mono text-xs text-neutral-700">
                      {it.callType || 'call'}
                    </span>
                  </td>
                  <td className="px-4 py-4 align-middle">
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      <SelfOrLink address={it.from} currentAddress={currentAddress} />
                      <DirectionPill outgoing={isOut} />
                      {it.to ? <SelfOrLink address={it.to} currentAddress={currentAddress} /> : <span className="text-xs italic text-primary">Creation</span>}
                    </div>
                  </td>
                  <td className="px-4 py-4 text-right align-middle font-mono text-sm tabular-nums text-neutral-800">
                    {formatWei(it.value)} {getNetworkCurrency()}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </PanelShell>
      <OffsetPager page={page} totalPages={totalPages} onPage={onPage} />
    </>
  );
}

// ---------- Coin balance history ----------

function CoinHistoryPanel({ balance }: { balance: string | number }) {
  return (
    <div className="rounded-2xl border border-dashed border-neutral-200 bg-neutral-100/30 px-6 py-12 text-center">
      <p className="text-sm text-neutral-600">
        Current balance: <span className="font-mono tabular-nums text-neutral-900">{formatWei(balance)} {getNetworkCurrency()}</span>
      </p>
      <p className="mx-auto mt-3 max-w-md text-xs text-neutral-500">
        Per-block coin balance history isn't indexed yet on this network. The current balance above comes from a live RPC call.
      </p>
    </div>
  );
}

// ---------- Logs ----------

function LogsPanel({
  data,
  isLoading,
  page,
  totalPages,
  onPage,
}: {
  data: Log[] | undefined;
  isLoading: boolean;
  page: number;
  totalPages: number;
  onPage: (p: number) => void;
}) {
  if (isLoading && !data) return <RowsSkeleton rows={3} />;
  if (!data?.length) return <PanelShell><EmptyRow>No logs emitted from this address.</EmptyRow></PanelShell>;
  return (
    <>
      <PanelShell>
        <div className="divide-y divide-neutral-200/70">
          {data.map((l) => (
            <div key={`${l.txHash}-${l.logIndex}`} className="space-y-2 px-5 py-4">
              <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                <div className="flex items-center gap-3">
                  <Link to={`/tx/${l.txHash}`} className="font-mono text-primary transition-colors hover:text-primary-600">
                    {formatHash(l.txHash, 10)}
                  </Link>
                  <Link to={`/block/${l.blockNumber}`} className="text-neutral-500 transition-colors hover:text-primary">
                    Block {l.blockNumber.toLocaleString()}
                  </Link>
                </div>
                <span className="rounded-full bg-neutral-200/60 px-2 py-0.5 font-mono text-[10px] text-neutral-600">#{l.logIndex}</span>
              </div>
              <dl className="space-y-1 font-mono text-xs text-neutral-700">
                {l.topic0 && <TopicRow label="topic0" value={l.topic0} />}
                {l.topic1 && <TopicRow label="topic1" value={l.topic1} />}
                {l.topic2 && <TopicRow label="topic2" value={l.topic2} />}
                {l.topic3 && <TopicRow label="topic3" value={l.topic3} />}
                {l.data && l.data !== '0x' && (
                  <div className="space-y-1">
                    <dt className="text-[10px] uppercase tracking-wider text-neutral-500">data</dt>
                    <dd className="break-all rounded-md bg-neutral-100/80 px-2 py-1.5 leading-relaxed">{l.data}</dd>
                  </div>
                )}
              </dl>
            </div>
          ))}
        </div>
      </PanelShell>
      <OffsetPager page={page} totalPages={totalPages} onPage={onPage} />
    </>
  );
}

function TopicRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline gap-3">
      <dt className="w-14 shrink-0 text-[10px] uppercase tracking-wider text-neutral-500">{label}</dt>
      <dd className="break-all text-neutral-700">{value}</dd>
    </div>
  );
}

// ---------- Contract panel ----------

function ContractPanel({
  contract,
  addressForVerifyLink,
  address,
  subTab,
  onSubTab,
  hasAbi,
  walletAddress,
  walletConnecting,
  walletPending,
  connectWallet,
  disconnectWallet,
  cancelConnect,
}: {
  contract: Contract | null;
  addressForVerifyLink: string;
  address: string;
  subTab: ContractSubTab;
  onSubTab: (s: ContractSubTab) => void;
  hasAbi: boolean;
  walletAddress: string | null;
  walletConnecting: boolean;
  walletPending: boolean;
  connectWallet: () => void;
  disconnectWallet: () => void;
  cancelConnect: () => void;
}) {
  if (!contract) {
    return <PanelShell><EmptyRow>No contract metadata available for this address.</EmptyRow></PanelShell>;
  }

  const subItems: { id: ContractSubTab; label: string; icon: React.ReactNode; disabled?: boolean; title?: string }[] = [
    { id: 'overview', label: 'Overview', icon: <FileCode2 className="h-3.5 w-3.5" /> },
    { id: 'read', label: 'Read', icon: <BookOpen className="h-3.5 w-3.5" />, disabled: !hasAbi, title: !hasAbi ? 'Upload an ABI to enable' : undefined },
    { id: 'write', label: 'Write', icon: <PenLine className="h-3.5 w-3.5" />, disabled: !hasAbi, title: !hasAbi ? 'Upload an ABI to enable' : undefined },
  ];

  return (
    <div className="space-y-5">
      <div className="inline-flex items-center gap-1 rounded-full border border-neutral-200 bg-neutral-100/60 p-1">
        {subItems.map((s) => {
          const isActive = subTab === s.id;
          return (
            <button
              key={s.id}
              onClick={() => !s.disabled && onSubTab(s.id)}
              disabled={s.disabled}
              title={s.title}
              className={[
                'inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors',
                s.disabled
                  ? 'cursor-not-allowed text-neutral-300'
                  : isActive
                  ? 'bg-neutral-50 text-neutral-900 shadow-card'
                  : 'text-neutral-500 hover:text-neutral-800',
              ].join(' ')}
            >
              {s.icon}
              <span>{s.label}</span>
            </button>
          );
        })}
      </div>

      {subTab === 'read' && (
        <div className="rounded-2xl border border-neutral-200 bg-neutral-50 p-4">
          <ContractInteraction address={address} abi={contract.abi} type="read" />
        </div>
      )}
      {subTab === 'write' && (
        <WritePanel
          address={address}
          abi={contract.abi}
          walletAddress={walletAddress}
          walletConnecting={walletConnecting}
          walletPending={walletPending}
          connectWallet={connectWallet}
          disconnectWallet={disconnectWallet}
          cancelConnect={cancelConnect}
        />
      )}
      {subTab === 'overview' && (
        <ContractOverview contract={contract} addressForVerifyLink={addressForVerifyLink} />
      )}
    </div>
  );
}

function ContractOverview({
  contract,
  addressForVerifyLink,
}: {
  contract: Contract;
  addressForVerifyLink: string;
}) {
  const bytes = contract.bytecode ? Math.max(0, Math.floor(contract.bytecode.length / 2)) : 0;
  return (
    <div className="space-y-5">
      <dl className="divide-y divide-neutral-200/70 overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
        <KV label="Creator" value={<AddressLink address={contract.creator} chars={6} />} />
        <KV
          label="Creation tx"
          value={
            <Link to={`/tx/${contract.creationTx}`} className="font-mono text-sm text-primary transition-colors hover:text-primary-600">
              {contract.creationTx}
            </Link>
          }
        />
        <KV
          label="Deployed at block"
          value={
            <Link to={`/block/${contract.blockNumber}`} className="font-mono text-sm text-primary transition-colors hover:text-primary-600 tabular-nums">
              {contract.blockNumber.toLocaleString()}
            </Link>
          }
        />
        <KV
          label="Verification"
          value={
            contract.isVerified ? (
              <span className="inline-flex items-center gap-1.5 text-sm text-success-600">
                <span className="h-1.5 w-1.5 rounded-full bg-success-500" />
                Verified{contract.contractName ? ` (${contract.contractName})` : ''}
              </span>
            ) : features().contractVerification ? (
              <Link to={`/verify?address=${addressForVerifyLink}`} className="inline-flex items-center gap-1.5 text-sm text-primary transition-colors hover:text-primary-600">
                <ShieldCheck className="h-3.5 w-3.5" />
                Not verified, verify now
              </Link>
            ) : (
              <span className="text-sm text-neutral-500">Not verified</span>
            )
          }
        />
        {contract.compilerVersion && <KV label="Compiler" value={<span className="font-mono text-sm">{contract.compilerVersion}</span>} />}
        {contract.evmVersion && <KV label="EVM version" value={<span className="font-mono text-sm">{contract.evmVersion}</span>} />}
        {contract.licenseType && <KV label="License" value={<span className="text-sm">{contract.licenseType}</span>} />}
      </dl>

      {contract.isVerified && contract.sourceCode && <UmlDiagram address={contract.address} />}
      {contract.sourceCode && (
        <CodeBlock title="Source code">
          <pre className="max-h-96 overflow-y-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-neutral-700">
            {contract.sourceCode}
          </pre>
        </CodeBlock>
      )}
      {contract.abi && contract.abi.length > 0 && (
        <CodeBlock title="ABI" rightMeta={`${contract.abi.length} items`}>
          <pre className="max-h-80 overflow-y-auto whitespace-pre-wrap font-mono text-xs leading-relaxed text-neutral-700">
            {JSON.stringify(contract.abi, null, 2)}
          </pre>
        </CodeBlock>
      )}
      <CodeBlock title="Deployed bytecode" rightMeta={`${bytes.toLocaleString()} bytes`} copyText={contract.bytecode ? `0x${contract.bytecode.replace(/^0x/, '')}` : undefined}>
        {contract.bytecode ? (
          <pre className="max-h-80 overflow-y-auto break-all font-mono text-xs leading-relaxed text-neutral-700">
            0x{contract.bytecode.replace(/^0x/, '')}
          </pre>
        ) : (
          <p className="text-sm text-neutral-500">Empty (no bytecode at this address).</p>
        )}
      </CodeBlock>
    </div>
  );
}

function KV({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center gap-4 px-5 py-3">
      <dt className="w-40 shrink-0 text-sm text-neutral-500">{label}</dt>
      <dd className="min-w-0 flex-1 truncate text-sm text-neutral-900">{value}</dd>
    </div>
  );
}

// ---------- UML class diagram (sol2uml) ----------
//
// Mirrors Blockscout's "View UML diagram" button: lazily fetches an SVG class
// diagram generated by sol2uml from the verified contract source.
function UmlDiagram({ address }: { address: string }) {
  const [open, setOpen] = useState(false);
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['contract-uml', address],
    queryFn: () => api.getContractUML(address),
    enabled: open,
    staleTime: 5 * 60 * 1000,
    retry: false,
  });

  const downloadHref = data ? `data:image/svg+xml;charset=utf-8,${encodeURIComponent(data)}` : undefined;

  return (
    <section className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-neutral-200 px-5 py-3">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-neutral-900">
          <Network className="h-4 w-4 text-neutral-500" />
          UML class diagram
        </h3>
        <div className="flex items-center gap-3">
          {open && downloadHref && (
            <a
              href={downloadHref}
              download={`${address}-uml.svg`}
              className="inline-flex items-center gap-1.5 text-xs text-primary transition-colors hover:text-primary-600"
            >
              <Download className="h-3.5 w-3.5" />
              Download SVG
            </a>
          )}
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 py-1.5 text-xs font-medium text-neutral-700 transition-colors hover:bg-neutral-50"
          >
            {open ? 'Hide diagram' : 'View UML diagram'}
          </button>
        </div>
      </header>
      {open && (
        <div className="bg-neutral-100/60 px-5 py-4">
          {isLoading && (
            <div className="flex items-center justify-center gap-2 py-10 text-sm text-neutral-500">
              <Loader2 className="h-4 w-4 animate-spin" />
              Generating diagram…
            </div>
          )}
          {isError && (
            <p className="py-6 text-sm text-error-600">
              {(error as Error)?.message || 'Failed to generate UML diagram.'}
            </p>
          )}
          {data && (
            <div
              className="max-h-[32rem] overflow-auto rounded-xl border border-neutral-200 bg-white p-4 [&_svg]:h-auto [&_svg]:max-w-none"
              // sol2uml output is trusted server-generated SVG markup.
              dangerouslySetInnerHTML={{ __html: data }}
            />
          )}
        </div>
      )}
    </section>
  );
}

function CodeBlock({
  title,
  rightMeta,
  copyText,
  children,
}: {
  title: string;
  rightMeta?: string;
  copyText?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-neutral-200 px-5 py-3">
        <h3 className="text-sm font-semibold text-neutral-900">{title}</h3>
        <div className="flex items-center gap-3">
          {rightMeta && <span className="font-mono text-xs tabular-nums text-neutral-500">{rightMeta}</span>}
          {copyText && <CopyButton text={copyText} />}
        </div>
      </header>
      <div className="bg-neutral-100/60 px-5 py-4">{children}</div>
    </section>
  );
}

// ---------- Write panel ----------

function WritePanel({
  address,
  abi,
  walletAddress,
  walletConnecting,
  walletPending,
  connectWallet,
  disconnectWallet,
  cancelConnect,
}: {
  address: string;
  abi: Contract['abi'] | undefined;
  walletAddress: string | null;
  walletConnecting: boolean;
  walletPending: boolean;
  connectWallet: () => void;
  disconnectWallet: () => void;
  cancelConnect: () => void;
}) {
  return (
    <div className="rounded-2xl border border-neutral-200 bg-neutral-50 p-5">
      {!walletAddress ? (
        <div className="flex flex-col items-center gap-4 py-8 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
            <Wallet className="h-5 w-5 text-primary" />
          </div>
          <h3 className="text-base font-semibold text-neutral-900">Connect a wallet to write</h3>
          <p className="max-w-sm text-sm text-neutral-500">
            Transactions will be signed by your connected wallet and broadcast on this network.
          </p>
          {walletPending ? (
            <div className="w-full max-w-sm space-y-3">
              <div className="flex items-center gap-2 rounded-lg border border-warning-200 bg-warning-50 px-3 py-2 text-left text-sm text-warning-700 dark:border-warning-500/30 dark:bg-warning-500/10 dark:text-warning-500">
                <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
                <span>Approve the request in MetaMask.</span>
              </div>
              <button onClick={cancelConnect} className="text-xs text-neutral-400 transition-colors hover:text-neutral-600">
                Cancel
              </button>
            </div>
          ) : (
            <button
              onClick={connectWallet}
              disabled={walletConnecting}
              className="inline-flex items-center gap-2 rounded-full bg-primary px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:opacity-50"
            >
              {walletConnecting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Wallet className="h-4 w-4" />}
              Connect wallet
            </button>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between gap-2 rounded-lg border border-success-200 bg-success-50 px-3 py-2 text-sm dark:border-success-500/30 dark:bg-success-500/10">
            <div className="flex min-w-0 items-center gap-2">
              <Wallet className="h-4 w-4 shrink-0 text-success-600" />
              <span className="text-success-700 dark:text-success-500">Connected:</span>
              <span className="truncate font-mono text-xs text-success-800 dark:text-success-500">{walletAddress}</span>
            </div>
            <button
              onClick={disconnectWallet}
              className="inline-flex shrink-0 items-center gap-1 rounded px-2 py-1 text-xs text-neutral-500 transition-colors hover:bg-error-50 hover:text-error-600"
              title="Disconnect wallet"
            >
              <X className="h-3.5 w-3.5" />
              Disconnect
            </button>
          </div>
          <ContractInteraction address={address} abi={abi} type="write" />
        </div>
      )}
    </div>
  );
}

// ============================================================
// Skeletons
// ============================================================

function HeroSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <div className="h-14 w-14 animate-pulse rounded-full bg-neutral-200" />
        <div className="h-7 w-48 animate-pulse rounded bg-neutral-200" />
      </div>
      <div className="h-9 w-96 animate-pulse rounded-full bg-neutral-200/60" />
    </div>
  );
}

function RowsSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 border-b border-neutral-200/60 px-4 py-5 last:border-b-0">
          <div className="h-3 w-32 animate-pulse rounded bg-neutral-200" />
          <div className="hidden h-3 w-16 animate-pulse rounded bg-neutral-200/70 md:block" />
          <div className="flex-1 h-3 animate-pulse rounded bg-neutral-200/70" />
          <div className="h-3 w-16 animate-pulse rounded bg-neutral-200" />
        </div>
      ))}
    </div>
  );
}
