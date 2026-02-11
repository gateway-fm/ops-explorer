import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { useState } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { api } from '../lib/api';
import type { Transaction } from '../lib/api';
import { formatHash, formatTimestamp, formatGas, formatAddress, formatWei } from '../lib/utils';
import { PageHeader } from '../components/PageHeader';
import { AddressLink } from '../components/AddressLink';

export function BlockDetail() {
  const { number } = useParams<{ number: string }>();
  const [showMore, setShowMore] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['block', number],
    queryFn: () => api.getBlock(parseInt(number!)),
    enabled: !!number,
  });

  if (isLoading) return <div className="text-neutral-400">Loading...</div>;
  if (error || !data) return <div className="text-error-600">Block not found</div>;

  const { block, transactions: txs } = data;
  const transactions = txs || [];  // Handle null transactions

  const gasPercent = block.gasLimit > 0 ? ((block.gasUsed / block.gasLimit) * 100).toFixed(2) : '0';

  return (
    <div className="space-y-6">
      <PageHeader title={`Block #${block.number}`} />

      <div className="card">
        <div className="divide-y divide-neutral-100">
          <InfoRow label="Block Height" value={block.number.toLocaleString()} />
          <InfoRow label="Timestamp" value={formatTimestamp(block.timestamp)} />
          <InfoRow label="Transactions" value={`${block.transactionCount} transactions`} />
          <InfoRow
            label="Miner"
            value={<AddressLink address={block.miner} className="font-mono text-sm" />}
          />
          <InfoRow label="Size" value={`${block.size.toLocaleString()} bytes`} />
          <InfoRow
            label="Gas Used"
            value={
              <div className="flex items-center gap-3">
                <span>{formatGas(block.gasUsed)} / {formatGas(block.gasLimit)}</span>
                <div className="flex items-center gap-2">
                  <div className="w-24 h-2 bg-neutral-200 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-primary rounded-full"
                      style={{ width: `${Math.min(parseFloat(gasPercent), 100)}%` }}
                    />
                  </div>
                  <span className="text-neutral-500 text-sm">{gasPercent}%</span>
                </div>
              </div>
            }
          />
          {block.baseFeePerGas && (
            <InfoRow label="Base Fee" value={`${block.baseFeePerGas} wei`} />
          )}
          <InfoRow
            label="Hash"
            value={<span className="font-mono text-sm break-all text-neutral-900">{block.hash}</span>}
          />
          <InfoRow
            label="Parent Hash"
            value={
              <Link to={`/block/${block.number - 1}`} className="font-mono text-sm text-primary hover:text-primary-600 break-all transition-colors">
                {block.parentHash}
              </Link>
            }
          />
        </div>

        {/* Show More Section */}
        <button
          onClick={() => setShowMore(!showMore)}
          className="w-full px-4 py-3 flex items-center justify-center gap-2 text-sm text-neutral-500 hover:text-neutral-700 hover:bg-neutral-50 transition-colors border-t border-neutral-100"
        >
          {showMore ? (
            <>
              <ChevronUp className="w-4 h-4" />
              Show Less
            </>
          ) : (
            <>
              <ChevronDown className="w-4 h-4" />
              Show More
            </>
          )}
        </button>

        {showMore && (
          <div className="divide-y divide-neutral-100 border-t border-neutral-100">
            <InfoRow label="Difficulty" value={block.difficulty} />
            <InfoRow label="Total Difficulty" value={block.totalDifficulty} />
            <InfoRow
              label="Nonce"
              value={<span className="font-mono text-sm">{block.nonce}</span>}
            />
            {block.extraData && (
              <InfoRow
                label="Extra Data"
                value={<span className="font-mono text-sm break-all">{block.extraData || '(empty)'}</span>}
              />
            )}
            <InfoRow
              label="State Root"
              value={<span className="font-mono text-sm break-all">{block.stateRoot}</span>}
            />
            <InfoRow
              label="Transactions Root"
              value={<span className="font-mono text-sm break-all">{block.transactionsRoot}</span>}
            />
            <InfoRow
              label="Receipts Root"
              value={<span className="font-mono text-sm break-all">{block.receiptsRoot}</span>}
            />
          </div>
        )}
      </div>

      {transactions.length > 0 && (
        <div className="card">
          <div className="px-4 py-3 border-b border-neutral-100">
            <h2 className="font-semibold text-neutral-900">Transactions ({transactions.length})</h2>
          </div>
          <div className="divide-y divide-neutral-100">
            {transactions.map((tx) => (
              <TxRow key={tx.hash} tx={tx} />
            ))}
          </div>
        </div>
      )}
    </div>
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

function TxRow({ tx }: { tx: Transaction }) {
  return (
    <div className="px-4 py-3 flex items-center justify-between hover:bg-primary-50/50 transition-colors">
      <div>
        <Link to={`/tx/${tx.hash}`} className="font-mono text-primary hover:text-primary-600 text-sm transition-colors">
          {formatHash(tx.hash, 12)}
        </Link>
        <div className="text-sm text-neutral-500">
          <Link to={`/address/${tx.from}`} className="hover:text-neutral-700 transition-colors">
            {formatAddress(tx.from, 6)}
          </Link>
          {tx.to && (
            <>
              {' → '}
              <Link to={`/address/${tx.to}`} className="hover:text-neutral-700 transition-colors">
                {formatAddress(tx.to, 6)}
              </Link>
            </>
          )}
        </div>
      </div>
      <div className="text-right text-sm">
        <div className="text-neutral-700">{formatWei(tx.value)} ETH</div>
        <div className={tx.status === 1 ? 'text-success-600' : 'text-error-600'}>
          {tx.status === 1 ? 'Success' : 'Failed'}
        </div>
      </div>
    </div>
  );
}
