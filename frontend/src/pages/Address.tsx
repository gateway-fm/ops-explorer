import { useQuery } from '@tanstack/react-query';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '../lib/api';
import type { Transaction } from '../lib/api';
import { formatWei, formatHash, formatAddress, formatTimeAgo } from '../lib/utils';
import { AddressLink } from '../components/AddressLink';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';
import { ContractInteraction } from '../components/ContractInteraction';
import { FileCode2, BookOpen, Loader2, PenLine, Fingerprint, Unlock, ShieldCheck, Wallet, X, ShieldOff } from 'lucide-react';
import { PageHeader } from '../components/PageHeader';
import { useAuth } from '../lib/auth';
import { useAddressVisibility } from '../hooks/useAddressVisibility';
import { redirectToLogin } from '../lib/login';
import { usePrivacyEnabled } from '../hooks/usePrivacyEnabled';

type TabType = 'transactions' | 'code' | 'read' | 'write';
type CodeSubTab = 'source' | 'abi' | 'bytecode' | 'compiler';

function formatExpirationDate(expiresAt?: string): string {
  if (!expiresAt) return 'Never';
  const date = new Date(expiresAt);
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
}

export function Address() {
  const { address } = useParams<{ address: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeTab, setActiveTab] = useState<TabType>('transactions');
  const [codeSubTab, setCodeSubTab] = useState<CodeSubTab | null>(null);
  const [walletAddress, setWalletAddress] = useState<string | null>(null);
  const manuallyDisconnected = useRef(false);

  // Check for already-connected wallet on mount & listen for account changes
  useEffect(() => {
    if (!window.ethereum || manuallyDisconnected.current) return;

    // Passive check — doesn't prompt, just reads current state
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
    return () => {
      eth.removeListener('accountsChanged', handleAccountsChanged);
    };
  }, []);

  const [walletPending, setWalletPending] = useState(false);
  const [walletConnecting, setWalletConnecting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  // Poll eth_accounts to detect when user approves the pending MetaMask request.
  // eth_accounts is passive (no popup) and returns accounts once permission is granted.
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
      } catch {
        // ignore
      }
    }, 1000);
  }, [stopPolling]);

  useEffect(() => {
    return () => stopPolling();
  }, [stopPolling]);

  const cancelConnect = useCallback(() => {
    stopPolling();
    setWalletPending(false);
    setWalletConnecting(false);
  }, [stopPolling]);

  const connectWallet = useCallback(async () => {
    if (!window.ethereum) {
      alert('MetaMask is not installed. Please install MetaMask to interact with contracts.');
      return;
    }
    manuallyDisconnected.current = false;
    setWalletConnecting(true);
    setWalletPending(false);

    try {
      // First, check if we already have permission (no popup)
      const existing = await window.ethereum.request({ method: 'eth_accounts' });
      if (existing && existing.length > 0) {
        setWalletAddress(existing[0]);
        setWalletConnecting(false);
        return;
      }

      // Request permission — this opens the MetaMask popup
      const accounts = await Promise.race([
        window.ethereum.request({ method: 'eth_requestAccounts' }),
        new Promise<never>((_, reject) =>
          setTimeout(() => reject(new Error('timeout')), 500)
        ),
      ]);
      if (accounts && accounts.length > 0) {
        setWalletAddress(accounts[0]);
        setWalletConnecting(false);
        return;
      }
    } catch {
      // Either -32002 (already pending) or timeout (popup opened but not yet approved).
      // In both cases: show the pending UI and poll until the user approves.
    }

    setWalletPending(true);
    setWalletConnecting(false);
    startPolling();
  }, [startPolling]);

  const disconnectWallet = useCallback(() => {
    manuallyDisconnected.current = true;
    setWalletAddress(null);
    // Revoke MetaMask permissions so eth_accounts returns empty
    if (window.ethereum) {
      window.ethereum.request({
        method: 'wallet_revokePermissions',
        params: [{ eth_accounts: {} }],
      }).catch(() => {});
    }
  }, []);

  const before = searchParams.get('before');
  const privacyEnabled = usePrivacyEnabled();
  const { isAuthenticated } = useAuth();
  const { visibility } = useAddressVisibility(address);

  const { data: info, isLoading: infoLoading, error } = useQuery({
    queryKey: ['address', address, privacyEnabled ? isAuthenticated : true],
    queryFn: () => api.getAddress(address!),
    enabled: !!address && (!privacyEnabled || isAuthenticated),
    retry: false,
  });

  const { data: txs, isLoading: txsLoading } = useQuery({
    queryKey: ['addressTxs', address, before],
    queryFn: () => api.getAddressTransactions(address!, 25, before ? parseInt(before) : undefined),
    enabled: !!address && activeTab === 'transactions',
  });

  const { data: contract, isLoading: contractLoading } = useQuery({
    queryKey: ['contract', address],
    queryFn: () => api.getContract(address!),
    enabled: !!address && info?.isContract,
    retry: false,
  });

  const loadMore = () => {
    if (txs?.data?.length) {
      const lastTx = txs.data[txs.data.length - 1];
      setSearchParams({ before: String(lastTx.blockNumber) });
    }
  };

  // Default code sub-tab based on verification status
  const activeCodeSubTab = codeSubTab ?? (contract?.isVerified ? 'source' : 'bytecode');

  // Show authentication prompt if not logged in (only when privacy is enabled)
  if (privacyEnabled && !isAuthenticated) {
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-primary-50 flex items-center justify-center border border-primary-200">
          <Fingerprint className="w-8 h-8 text-primary" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Authentication Required</h2>
        <p className="text-neutral-500 text-center max-w-md">
          Sign in with Privado ID to view address details and transaction history.
        </p>
        <div className="mt-4">
          <button
            onClick={() => redirectToLogin(`/address/${address}`)}
            className="btn-primary flex items-center gap-2"
          >
            <Fingerprint className="w-4 h-4" />
            Sign in with Privado
          </button>
        </div>
      </div>
    );
  }

  if (infoLoading) return <div className="text-neutral-400">Loading...</div>;

  // Privacy-restricted address (API returns 403)
  if (error && error instanceof Error && error.message.includes('403')) {
    return (
      <div className="space-y-6">
        <PageHeader title="Address" />
        <div className="card">
          <div className="flex flex-col items-center justify-center py-16 space-y-4">
            <div className="w-16 h-16 rounded-full bg-neutral-100 flex items-center justify-center border border-neutral-200">
              <ShieldOff className="w-8 h-8 text-neutral-400" />
            </div>
            <h2 className="text-xl font-semibold text-neutral-900">Address Restricted</h2>
            <p className="text-neutral-500 text-center max-w-md text-sm">
              This address is protected by privacy controls. You do not currently have permission to view its details.
            </p>
            <div className="font-mono text-xs text-neutral-400 break-all max-w-md text-center px-4">
              {address}
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (error || !info) return <div className="text-error-600">Address not found</div>;

  const hasAbi = contract?.abi && contract.abi.length > 0;

  return (
    <div className="space-y-6">
      <PageHeader title={info.isContract ? 'Contract' : 'Address'}>
        {contract?.isVerified && contract?.contractName && (
          <span className="badge badge-primary">
            {contract.contractName}
          </span>
        )}
        {info.isContract && (
          <span className="badge bg-primary-50 text-primary-600 border border-primary-200">
            Contract
          </span>
        )}
        {contract?.isVerified && (
          <span className="badge badge-success">
            Verified
          </span>
        )}
        {info.isContract && !contract?.isVerified && (
          <Link
            to={`/verify?address=${address}`}
            className="btn-secondary text-xs gap-1.5"
          >
            <ShieldCheck className="w-3.5 h-3.5" />
            Verify Contract
          </Link>
        )}
      </PageHeader>

      {/* Disclosed Address Banner */}
      {privacyEnabled && visibility?.reason === 'disclosure_grant' && (
        <div className="card p-4 border-primary-200 bg-primary-50">
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-xl bg-primary-100 flex items-center justify-center shrink-0">
              <Unlock className="w-5 h-5 text-primary" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h3 className="font-medium text-primary-900">Disclosed Address</h3>
                {visibility.level && visibility.level !== 'full' && (
                  <span className={`badge text-xs ${
                    visibility.level === 'pseudonymous'
                      ? 'bg-amber-100 text-amber-700 border-amber-200'
                      : 'bg-red-100 text-red-700 border-red-200'
                  }`}>
                    {visibility.level === 'pseudonymous' ? 'Pseudonymous' : 'Redacted'}
                  </span>
                )}
              </div>
              <p className="text-sm text-primary-700 mt-0.5">
                This address was shared with you
                {visibility.level === 'pseudonymous' && visibility.pseudonym && (
                  <span className="font-mono text-xs ml-1">
                    as <span className="font-semibold">{visibility.pseudonym}</span>
                  </span>
                )}
                {visibility.grant_id && (
                  <span className="font-mono text-xs ml-1">
                    (Grant: {visibility.grant_id.slice(0, 8)}...)
                  </span>
                )}
              </p>
              {visibility.expires_at && (
                <p className="text-xs text-primary-600 mt-1">
                  Expires: {formatExpirationDate(visibility.expires_at)}
                </p>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Address Info Card */}
      <div className="card">
        <div className="divide-y divide-neutral-100">
          <InfoRow
            label="Address"
            value={
              privacyEnabled && visibility?.level === 'pseudonymous' && visibility.pseudonym ? (
                <div>
                  <span className="font-mono text-sm break-all text-neutral-900">{visibility.pseudonym}</span>
                  <span className="text-xs text-neutral-400 ml-2">(pseudonymous view)</span>
                </div>
              ) : privacyEnabled && visibility?.level === 'redacted' ? (
                <span className="text-neutral-400 italic">Address hidden (redacted disclosure)</span>
              ) : (
                <span className="font-mono text-sm break-all text-neutral-900">{info.address}</span>
              )
            }
          />
          <InfoRow label="Type" value={info.isContract ? 'Contract' : 'EOA (Externally Owned Account)'} />
          <InfoRow label="Balance" value={`${formatWei(info.balance)} ETH`} />
          <InfoRow label="Transactions" value={info.txCount.toLocaleString()} />
          {contract?.contractName && (
            <InfoRow label="Contract Name" value={contract.contractName} />
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="card">
        <div className="flex border-b border-neutral-200 overflow-x-auto">
          <TabButton
            active={activeTab === 'transactions'}
            onClick={() => setActiveTab('transactions')}
          >
            Transactions
          </TabButton>
          {info.isContract && (
            <>
              <TabButton
                active={activeTab === 'code'}
                onClick={() => setActiveTab('code')}
                icon={<FileCode2 className="w-4 h-4" />}
              >
                Code
              </TabButton>
              <TabButton
                active={activeTab === 'read'}
                onClick={() => setActiveTab('read')}
                icon={<BookOpen className="w-4 h-4" />}
                disabled={!hasAbi}
                title={!hasAbi ? 'Upload ABI to enable' : undefined}
              >
                Read Contract
              </TabButton>
              <TabButton
                active={activeTab === 'write'}
                onClick={() => setActiveTab('write')}
                icon={<PenLine className="w-4 h-4" />}
                disabled={!hasAbi}
                title={!hasAbi ? 'Upload ABI to enable' : undefined}
              >
                Write Contract
              </TabButton>
            </>
          )}
        </div>

        {/* Transactions Tab */}
        {activeTab === 'transactions' && (
          <>
            <div className="overflow-x-auto">
              <table className="table">
                <thead>
                  <tr>
                    <th>Txn Hash</th>
                    <th>Block</th>
                    <th>Age</th>
                    <th>From</th>
                    <th></th>
                    <th>To</th>
                    <th className="text-right">Value</th>
                    <th className="text-right">Txn Fee</th>
                  </tr>
                </thead>
                <tbody>
                  {txs?.data?.map((tx) => (
                    <TxTableRow key={tx.hash} tx={tx} currentAddress={info.address} />
                  ))}
                </tbody>
              </table>
            </div>

            {txsLoading && (
              <div className="px-4 py-8 text-center text-neutral-400">Loading...</div>
            )}

            {!txsLoading && !txs?.data?.length && (
              <div className="px-4 py-8 text-center text-neutral-400">No transactions</div>
            )}

            {txs?.hasMore && (
              <div className="px-4 py-3 border-t border-neutral-100">
                <button
                  onClick={loadMore}
                  className="w-full py-2 text-sm text-neutral-500 hover:text-neutral-700 transition-colors"
                >
                  Load more
                </button>
              </div>
            )}
          </>
        )}

        {/* Code Tab */}
        {activeTab === 'code' && info.isContract && (
          <div className="p-4">
            {contractLoading && (
              <div className="text-center text-neutral-400 py-8">Loading contract code...</div>
            )}

            {!contractLoading && contract && (
              <div className="space-y-6">
                {/* Contract Info */}
                <div className="card">
                  <div className="divide-y divide-neutral-100">
                    <InfoRow
                      label="Creator"
                      value={<AddressLink address={contract.creator} chars={8} />}
                    />
                    <InfoRow
                      label="Creation Tx"
                      value={
                        <Link to={`/tx/${contract.creationTx}`} className="font-mono text-primary hover:text-primary-600 transition-colors">
                          {formatHash(contract.creationTx, 8)}
                        </Link>
                      }
                    />
                    <InfoRow
                      label="Block"
                      value={
                        <Link to={`/block/${contract.blockNumber}`} className="text-primary hover:text-primary-600 transition-colors">
                          {contract.blockNumber}
                        </Link>
                      }
                    />
                    {contract.compilerVersion && (
                      <InfoRow label="Compiler" value={contract.compilerVersion} />
                    )}
                    {contract.optimizationUsed !== undefined && contract.optimizationUsed !== null && (
                      <InfoRow
                        label="Optimization"
                        value={
                          contract.optimizationUsed
                            ? `Enabled${contract.optimizationRuns ? ` (${contract.optimizationRuns} runs)` : ''}`
                            : 'Disabled'
                        }
                      />
                    )}
                    <InfoRow label="EVM Version" value={contract.evmVersion || 'default'} />
                    {contract.licenseType && (
                      <InfoRow label="License" value={contract.licenseType} />
                    )}
                    {contract.constructorArgs && (
                      <InfoRow
                        label="Constructor Args"
                        value={
                          <span className="font-mono text-xs break-all">{contract.constructorArgs}</span>
                        }
                      />
                    )}
                  </div>
                </div>

                {/* Sub-tabs */}
                <div className="flex border-b border-neutral-200 overflow-x-auto">
                  <button
                    onClick={() => setCodeSubTab('source')}
                    className={`px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap ${
                      activeCodeSubTab === 'source'
                        ? 'text-neutral-900 border-b-2 border-primary'
                        : 'text-neutral-500 hover:text-neutral-700'
                    }`}
                  >
                    Source Code
                  </button>
                  <button
                    onClick={() => setCodeSubTab('abi')}
                    className={`px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap ${
                      activeCodeSubTab === 'abi'
                        ? 'text-neutral-900 border-b-2 border-primary'
                        : 'text-neutral-500 hover:text-neutral-700'
                    }`}
                  >
                    ABI
                  </button>
                  <button
                    onClick={() => setCodeSubTab('bytecode')}
                    className={`px-4 py-2 text-sm font-medium transition-colors whitespace-nowrap ${
                      activeCodeSubTab === 'bytecode'
                        ? 'text-neutral-900 border-b-2 border-primary'
                        : 'text-neutral-500 hover:text-neutral-700'
                    }`}
                  >
                    Bytecode
                  </button>
                </div>

                {/* Sub-tab Content */}
                {activeCodeSubTab === 'source' && (
                  <div>
                    {contract.sourceCode ? (
                      <>
                        <div className="flex items-center justify-between mb-2">
                          <h3 className="text-sm font-medium text-neutral-900">Source Code</h3>
                        </div>
                        <div className="code-block overflow-x-auto max-h-96">
                          <pre className="text-xs whitespace-pre-wrap">
                            {contract.sourceCode}
                          </pre>
                        </div>
                      </>
                    ) : (
                      <div className="text-center text-neutral-400 py-8">
                        Contract source code is not verified.{' '}
                        <Link to={`/verify?address=${address}`} className="text-primary hover:text-primary-600">
                          Verify it now
                        </Link>
                      </div>
                    )}
                  </div>
                )}

                {activeCodeSubTab === 'abi' && (
                  <div>
                    {hasAbi ? (
                      <>
                        <div className="flex items-center justify-between mb-2">
                          <h3 className="text-sm font-medium text-neutral-900">Contract ABI</h3>
                          <span className="text-xs text-neutral-400">
                            {contract.abi!.length} items
                          </span>
                        </div>
                        <div className="code-block overflow-x-auto max-h-96">
                          <pre className="text-xs whitespace-pre-wrap">
                            {JSON.stringify(contract.abi, null, 2)}
                          </pre>
                        </div>
                      </>
                    ) : (
                      <div className="text-center text-neutral-400 py-8">
                        No ABI available. Verify the contract to generate its ABI.
                      </div>
                    )}
                  </div>
                )}

                {activeCodeSubTab === 'bytecode' && (
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <h3 className="text-sm font-medium text-neutral-900">Contract Bytecode</h3>
                      <span className="text-xs text-neutral-400">{contract.bytecode.length / 2} bytes</span>
                    </div>
                    <div className="code-block overflow-x-auto max-h-96">
                      <pre className="text-xs whitespace-pre-wrap break-all">
                        0x{contract.bytecode}
                      </pre>
                    </div>
                  </div>
                )}
              </div>
            )}

            {!contractLoading && !contract && (
              <div className="text-center text-neutral-400 py-8">
                Contract code not available
              </div>
            )}
          </div>
        )}

        {/* Read Contract Tab */}
        {activeTab === 'read' && info.isContract && (
          <div className="p-4">
            {contractLoading ? (
              <div className="text-center text-neutral-400 py-8">Loading...</div>
            ) : (
              <ContractInteraction
                address={address!}
                abi={contract?.abi}
                type="read"
              />
            )}
          </div>
        )}

        {/* Write Contract Tab */}
        {activeTab === 'write' && info.isContract && (
          <div className="p-4">
            {contractLoading ? (
              <div className="text-center text-neutral-400 py-8">Loading...</div>
            ) : !walletAddress ? (
              <div className="flex flex-col items-center justify-center py-12 space-y-4">
                <div className="w-14 h-14 rounded-full bg-primary-50 flex items-center justify-center border border-primary-200">
                  <Wallet className="w-7 h-7 text-primary" />
                </div>
                <h3 className="text-lg font-semibold text-neutral-900">Connect Your Wallet</h3>
                <p className="text-neutral-500 text-center max-w-sm text-sm">
                  Connect your wallet to write to this contract. Transactions will be sent to the connected network.
                </p>
                {walletPending ? (
                  <div className="max-w-sm space-y-3">
                    <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-amber-50 border border-amber-200 text-sm text-amber-800">
                      <Loader2 className="w-4 h-4 shrink-0 animate-spin" />
                      <span>Approve the request in MetaMask. Click the MetaMask icon in your browser toolbar if you can't see it.</span>
                    </div>
                    <button
                      onClick={cancelConnect}
                      className="text-xs text-neutral-400 hover:text-neutral-600 transition-colors"
                    >
                      Cancel
                    </button>
                  </div>
                ) : (
                  <button
                    onClick={connectWallet}
                    disabled={walletConnecting}
                    className="btn-primary flex items-center gap-2"
                  >
                    {walletConnecting ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <>
                        <Wallet className="w-4 h-4" />
                        Connect Wallet
                      </>
                    )}
                  </button>
                )}
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex items-center justify-between px-3 py-2 rounded-lg bg-success-50 border border-success-200 text-sm">
                  <div className="flex items-center gap-2">
                    <Wallet className="w-4 h-4 text-success-600" />
                    <span className="text-success-700">Connected:</span>
                    <span className="font-mono text-success-800 text-xs">{walletAddress}</span>
                  </div>
                  <button
                    onClick={disconnectWallet}
                    className="flex items-center gap-1 px-2 py-1 text-xs text-neutral-500 hover:text-error-600 hover:bg-error-50 rounded transition-colors"
                    title="Disconnect wallet"
                  >
                    <X className="w-3.5 h-3.5" />
                    Disconnect
                  </button>
                </div>
                <ContractInteraction
                  address={address!}
                  abi={contract?.abi}
                  type="write"
                />
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

interface TabButtonProps {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
  icon?: React.ReactNode;
  disabled?: boolean;
  title?: string;
}

function TabButton({ active, onClick, children, icon, disabled, title }: TabButtonProps) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={title}
      className={`px-4 py-3 text-sm font-medium transition-colors flex items-center gap-2 whitespace-nowrap ${
        active
          ? 'text-neutral-900 border-b-2 border-primary'
          : disabled
          ? 'text-neutral-300 cursor-not-allowed'
          : 'text-neutral-500 hover:text-neutral-700'
      }`}
    >
      {icon}
      {children}
    </button>
  );
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="info-row">
      <div className="info-label">{label}</div>
      <div className="info-value">{value}</div>
    </div>
  );
}

function TxTableRow({ tx, currentAddress }: { tx: Transaction; currentAddress: string }) {
  const isOutgoing = tx.from.toLowerCase() === currentAddress.toLowerCase();
  const txFee = (BigInt(tx.gasUsed || 0) * BigInt(tx.gasPrice || 0));

  return (
    <tr>
      {/* Txn Hash */}
      <td>
        <Link to={`/tx/${tx.hash}`} className="font-mono text-primary hover:text-primary-600 transition-colors">
          {formatHash(tx.hash, 8)}
        </Link>
      </td>

      {/* Block */}
      <td>
        <Link to={`/block/${tx.blockNumber}`} className="text-primary hover:text-primary-600 transition-colors">
          {tx.blockNumber}
        </Link>
      </td>

      {/* Age */}
      <td className="text-neutral-500">
        {tx.blockTimestamp ? formatTimeAgo(tx.blockTimestamp) : '-'}
      </td>

      {/* From */}
      <td>
        {tx.from.toLowerCase() === currentAddress.toLowerCase() ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="font-mono text-neutral-400 cursor-default">{formatAddress(tx.from, 8)}</span>
            </TooltipTrigger>
            <TooltipContent>
              <span className="font-mono">{tx.from}</span>
            </TooltipContent>
          </Tooltip>
        ) : (
          <AddressLink address={tx.from} chars={8} />
        )}
      </td>

      {/* Direction indicator */}
      <td>
        <span className={`badge ${
          isOutgoing
            ? 'badge-warning'
            : 'badge-success'
        }`}>
          {isOutgoing ? 'OUT' : 'IN'}
        </span>
      </td>

      {/* To */}
      <td>
        {tx.to ? (
          tx.to.toLowerCase() === currentAddress.toLowerCase() ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="font-mono text-neutral-400 cursor-default">{formatAddress(tx.to, 8)}</span>
              </TooltipTrigger>
              <TooltipContent>
                <span className="font-mono">{tx.to}</span>
              </TooltipContent>
            </Tooltip>
          ) : (
            <AddressLink address={tx.to} chars={8} />
          )
        ) : (
          <span className="text-primary italic">Contract Creation</span>
        )}
      </td>

      {/* Value */}
      <td className="text-right font-mono text-neutral-700">
        {formatWei(tx.value)} ETH
      </td>

      {/* Txn Fee */}
      <td className="text-right text-neutral-500 font-mono">
        {formatWei(txFee.toString())}
      </td>
    </tr>
  );
}
