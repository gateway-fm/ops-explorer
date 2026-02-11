import { useQuery } from '@tanstack/react-query';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { useState } from 'react';
import { api } from '../lib/api';
import type { Transaction } from '../lib/api';
import { formatWei, formatHash, formatAddress, formatTimeAgo } from '../lib/utils';
import { AddressLink } from '../components/AddressLink';
import { Tooltip, TooltipContent, TooltipTrigger } from '../components/ui/tooltip';
import { ContractInteraction, AbiUpload } from '../components/ContractInteraction';
import { FileCode2, BookOpen, PenLine } from 'lucide-react';
import { PageHeader } from '../components/PageHeader';

type TabType = 'transactions' | 'code' | 'read' | 'write';

export function Address() {
  const { address } = useParams<{ address: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeTab, setActiveTab] = useState<TabType>('transactions');
  const before = searchParams.get('before');

  const { data: info, isLoading: infoLoading, error } = useQuery({
    queryKey: ['address', address],
    queryFn: () => api.getAddress(address!),
    enabled: !!address,
    retry: false,
  });

  const { data: txs, isLoading: txsLoading } = useQuery({
    queryKey: ['addressTxs', address, before],
    queryFn: () => api.getAddressTransactions(address!, 25, before ? parseInt(before) : undefined),
    enabled: !!address && activeTab === 'transactions',
  });

  const { data: contract, isLoading: contractLoading, refetch: refetchContract } = useQuery({
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

  const handleAbiUpdate = () => {
    refetchContract();
  };

  if (infoLoading) return <div className="text-neutral-400">Loading...</div>;
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
      </PageHeader>

      {/* Address Info Card */}
      <div className="card">
        <div className="divide-y divide-neutral-100">
          <InfoRow
            label="Address"
            value={<span className="font-mono text-sm break-all text-neutral-900">{info.address}</span>}
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
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                  <div>
                    <span className="text-neutral-500">Creator:</span>{' '}
                    <AddressLink address={contract.creator} chars={8} />
                  </div>
                  <div>
                    <span className="text-neutral-500">Creation Tx:</span>{' '}
                    <Link to={`/tx/${contract.creationTx}`} className="font-mono text-primary hover:text-primary-600 transition-colors">
                      {formatHash(contract.creationTx, 8)}
                    </Link>
                  </div>
                  <div>
                    <span className="text-neutral-500">Block:</span>{' '}
                    <Link to={`/block/${contract.blockNumber}`} className="text-primary hover:text-primary-600 transition-colors">
                      {contract.blockNumber}
                    </Link>
                  </div>
                  {contract.compilerVersion && (
                    <div>
                      <span className="text-neutral-500">Compiler:</span>{' '}
                      <span className="text-neutral-700">{contract.compilerVersion}</span>
                    </div>
                  )}
                </div>

                {/* ABI Upload Section */}
                <div className="border-t border-neutral-100 pt-6">
                  <AbiUpload address={address!} onAbiUpdate={handleAbiUpdate} />
                </div>

                {/* ABI Display */}
                {hasAbi && (
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <h3 className="text-sm font-medium text-neutral-900">Contract ABI</h3>
                      <span className="text-xs text-neutral-400">
                        {contract.abi!.length} items
                      </span>
                    </div>
                    <div className="code-block overflow-x-auto max-h-64">
                      <pre className="text-xs whitespace-pre-wrap">
                        {JSON.stringify(contract.abi, null, 2)}
                      </pre>
                    </div>
                  </div>
                )}

                {/* Bytecode */}
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <h3 className="text-sm font-medium text-neutral-900">Contract Bytecode</h3>
                    <span className="text-xs text-neutral-400">{contract.bytecode.length / 2} bytes</span>
                  </div>
                  <div className="code-block overflow-x-auto max-h-48">
                    <pre className="text-xs whitespace-pre-wrap break-all">
                      0x{contract.bytecode}
                    </pre>
                  </div>
                </div>

                {/* Source Code (if verified) */}
                {contract.sourceCode && (
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <h3 className="text-sm font-medium text-neutral-900">Source Code</h3>
                    </div>
                    <div className="code-block overflow-x-auto max-h-96">
                      <pre className="text-xs whitespace-pre-wrap">
                        {contract.sourceCode}
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
            ) : (
              <div className="space-y-4">
                <div className="alert alert-warning">
                  Connect your wallet to write to this contract. Transactions will be sent to the connected network.
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
  const txFee = (BigInt(tx.gasUsed) * BigInt(tx.gasPrice));

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
