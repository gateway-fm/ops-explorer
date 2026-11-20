import { NativeNames } from '../components/NativeNames';
import { useCanonicalIdentifier } from '../lib/nativeRoute';
import { useState } from 'react';
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { TransactionDetailPresentation, TransactionOverview, TransactionInput, InfoRow, TxCategoryBadges, type TransactionTab } from '../components/TransactionDetailPresentation';
import { CopyButton } from '../components/CopyButton';
import { NativeAddress } from '../components/NativeAddress';
import { NativeContinuation, NativeLogCards, NativeReadState, NativeTraceRows, NativeTransferRows, nativeFee, useNativePage } from '../components/NativeReadPanels';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeTransaction, decodeTransfers, decodeLogs, decodeInternalTransactions } from '../lib/nativeReadModels';
import { exactQuantity, exactUnits, exactTimestamp } from '../lib/nativePresentation';
import { getNetworkCurrency } from '../lib/utils';
import { StateMessage } from '../components/StateMessage';

export function TransactionNative() {
  const {hash: rawIdentifier = ''} = useParams<{hash: string}>();
  const hash = useCanonicalIdentifier(rawIdentifier, 'hash') ?? '';
  const location = useLocation();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const suffix = location.pathname.split('/').at(-1);
  const wanted = params.get('tab') || (suffix === 'internal' ? 'trace' : suffix);
  const activeTab: TransactionTab = wanted === 'logs' || wanted === 'trace' ? wanted : 'overview';
  const [showMore,setShowMore] = useState(false);
  const valid = /^0x[0-9a-f]{64}$/.test(hash);
  const path = valid ? `/transactions/${hash}` : null;
  const read = useNativeRead(path && `/v2/explorer${path}`, decodeTransaction, 5000);
  const transfers = useNativePage(path && `${path}/transfers`, decodeTransfers);
  const logs = useNativePage(path && activeTab === 'logs' ? `${path}/logs` : null, decodeLogs);
  const internal = useNativePage(path && activeTab === 'trace' ? `${path}/internal` : null, decodeInternalTransactions);
  if (!valid) return <StateMessage variant="error" title="Invalid transaction hash"/>;
  if (read.loading || read.error) return <NativeReadState {...read}/>;
  const tx = read.data;
  if (!tx) return <StateMessage variant="empty" title="Transaction unavailable" detail="It may be outside your current access or the retained index."/>;
  const showTransfers = transfers.loading || !!transfers.error || !!transfers.data?.items.length || !!transfers.data?.page.next || transfers.index > 0;
  const setTab = (tab: TransactionTab) => { const query=new URLSearchParams(params);if(tab==='overview')query.delete('tab');else query.set('tab',tab);navigate({pathname:`/tx/${hash}`,search:query.toString(),hash:location.hash},{state:location.state}); };
  return <NativeNames items={[tx.from,tx.to,tx.contractAddress].filter((address):address is string=>!!address).map(address=>({address,metadata:tx.addressMetadata}))}><TransactionDetailPresentation activeTab={activeTab} setActiveTab={setTab} logsVisible traceVisible logCount={logs.data ? `${logs.data.items.length} on page` : 'view'} traceCount={internal.data ? `${internal.data.items.length} on page` : 'view'}>
    {activeTab === 'overview' && <TransactionOverview showMore={showMore} onToggle={() => setShowMore(!showMore)} input={<TransactionInput data={tx.inputData}/>} extra={<>
      <InfoRow label="Transaction Type" value={tx.txType}/><InfoRow label="Nonce" value={exactQuantity(tx.nonce)}/><InfoRow label="Position in Block" value={tx.txIndex === null ? 'Pending / unknown' : tx.txIndex}/><InfoRow label="Gas Limit" value={exactQuantity(tx.gasLimit)}/><InfoRow label="Gas Used" value={exactQuantity(tx.gasUsed)}/>{tx.maxFeePerGas !== undefined && <InfoRow label="Max Fee Per Gas" value={`${exactQuantity(tx.maxFeePerGas)} wei`}/>} {tx.maxPriorityFeePerGas !== undefined && <InfoRow label="Max Priority Fee Per Gas" value={`${exactQuantity(tx.maxPriorityFeePerGas)} wei`}/>} </>}>
      <InfoRow label="Transaction Hash" value={<span className="flex items-center gap-1"><span className="font-mono text-sm break-all text-neutral-900">{tx.hash}</span><CopyButton text={tx.hash}/></span>}/>
      <InfoRow label="Status" value={<div className="flex items-center gap-2 flex-wrap"><span data-testid="tx-status" className={`badge ${tx.status === 1 ? 'badge-success' : tx.status === 0 ? 'badge-error' : 'badge-neutral'}`}>{tx.status === 1 ? 'Success' : tx.status === 0 ? 'Failed' : 'Pending / unknown'}</span>{tx.error && <span className="text-sm text-error-600">{tx.error}</span>}{tx.revertReason && <span className="text-sm text-error-600 font-mono">Revert: {tx.revertReason}</span>}</div>}/>
      {!!tx.txCategories?.length && <InfoRow label="Transaction Action" value={<TxCategoryBadges categories={tx.txCategories}/>}/>}
      <InfoRow label="Block" value={tx.blockNumber === null ? 'Pending' : <Link className="text-primary hover:text-primary-600 transition-colors" to={`/block/${tx.blockNumber}`}>{exactQuantity(tx.blockNumber)}</Link>}/>
      <InfoRow label="Timestamp" value={exactTimestamp(tx.blockTimestamp)}/>
      <InfoRow label="From" value={<NativeAddress address={tx.from} metadata={tx.addressMetadata} full/>}/>
      <InfoRow label="To" value={tx.to ? <NativeAddress address={tx.to} metadata={tx.addressMetadata} full/> : tx.contractAddress ? <span className="flex items-center gap-1 text-sm"><NativeAddress address={tx.contractAddress} metadata={tx.addressMetadata} full/><span className="text-neutral-500">Contract created</span></span> : 'Contract creation'}/>
      {showTransfers && <InfoRow label="ERC-20 Tokens Transferred" value={<div className="space-y-3">
        {transfers.loading ? <p role="status" className="text-sm text-neutral-500">Loading transfers…</p> : <NativeReadState {...transfers} reset={transfers.reset}/>}
        {transfers.data && <><NativeTransferRows items={transfers.data.items} layout="compact"/>{!transfers.data.items.length && <p className="text-sm text-neutral-500">No token transfers on this visible page.</p>}{(transfers.data.page.next || transfers.index > 0) && <NativeContinuation {...transfers} page={transfers.data.page}/>} {transfers.trimmed && <button className="text-sm text-primary underline" onClick={transfers.reset}>Restart transfers from the first page</button>}</>}
      </div>}/>}
      <InfoRow label="Value" value={`${exactUnits(tx.value)}${tx.value !== null ? ` ${getNetworkCurrency()}` : ''}`}/><InfoRow label="Transaction Fee" value={nativeFee(tx)}/><InfoRow label="Gas Price" value={`${exactQuantity(tx.gasPrice)} wei`}/>
    </TransactionOverview>}
    {activeTab === 'logs' && <><NativeReadState {...logs} reset={logs.reset}/>{logs.data && <><NativeLogCards items={logs.data.items}/><NativeContinuation {...logs} page={logs.data.page}/></>}</>}
    {activeTab === 'trace' && <><NativeReadState {...internal} reset={internal.reset}/>{internal.data && <><NativeTraceRows items={internal.data.items}/><NativeContinuation {...internal} page={internal.data.page}/></>}</>}
  </TransactionDetailPresentation></NativeNames>;
}
