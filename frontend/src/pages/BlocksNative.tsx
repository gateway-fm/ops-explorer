import { NativeNames } from '../components/NativeNames';
import { useCanonicalIdentifier } from '../lib/nativeRoute';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { PageHeader } from '../components/PageHeader';
import { CopyButton } from '../components/CopyButton';
import { NativeAddress } from '../components/NativeAddress';
import { BlockInfoRow as InfoRow } from '../components/BlockInfoRow';
import { ExplorerTable, ExplorerTabs, VisiblePageNote } from '../components/ExplorerReadPresentation';
import { NativeContinuation, NativeReadState, NativeTransactionsTable, NativeTraceRows, useNativePage } from '../components/NativeReadPanels';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeBlock, decodeBlocks, decodeLatestBlock, decodeTransactions, decodeInternalTransactions } from '../lib/nativeReadModels';
import { exactQuantity, exactTimestamp } from '../lib/nativePresentation';
import { formatHash } from '../lib/utils';
import { StateMessage } from '../components/StateMessage';

export function BlocksNative() {
  const blocks = useNativePage('/blocks', decodeBlocks);
  const latest = useNativeRead(blocks.index === 0 ? '/v2/explorer/blocks/latest/number' : null, decodeLatestBlock, 5000);
  const top = blocks.data?.items[0]?.number;
  const fresh = latest.data && top && BigInt(latest.data.number) > BigInt(top) ? BigInt(latest.data.number)-BigInt(top) : 0n;
  return <div className="space-y-6"><PageHeader title="Blocks"/><VisiblePageNote/><div className="card overflow-hidden">
    {fresh > 0n && <button className="w-full px-4 py-3 text-primary bg-primary-50" onClick={blocks.reset}>{exactQuantity(fresh.toString())} new blocks — refresh</button>}
    <NativeReadState {...blocks} reset={blocks.reset}/>
    {blocks.data && <><ExplorerTable headings={['Block','Timestamp','Txns','Gas limit','Hash']}>{blocks.data.items.map(block => <tr key={block.hash} data-testid="block-row"><td><Link className="font-mono text-primary" to={`/block/${block.number}`}>{exactQuantity(block.number)}</Link></td><td className="text-neutral-500">{exactTimestamp(block.timestamp)}</td><td>{exactQuantity(block.transactionCount)}</td><td className="font-mono">{exactQuantity(block.gasLimit)}</td><td><Link className="font-mono text-primary" title={block.hash} to={`/blocks/hash/${block.hash}`}>{formatHash(block.hash,10)}</Link></td></tr>)}</ExplorerTable>{!blocks.data.items.length&&<StateMessage variant="empty" title="No blocks on this page"/>}<NativeContinuation {...blocks} page={blocks.data.page}/></>}
  </div></div>;
}
export function BlockNative() {
  const { number: rawNumber = '', hash: rawHash = '' } = useParams(); const number = useCanonicalIdentifier(rawNumber,'block')??'', hash = useCanonicalIdentifier(rawHash,'hash')??''; const [params, setParams] = useSearchParams();
  const validNumber = /^(0|[1-9][0-9]{0,18})$/.test(number) && BigInt(number) <= 9223372036854775807n;
  const selector = rawHash ? /^0x[0-9a-f]{64}$/.test(hash) ? `hash/${hash}` : null : validNumber ? number : null;
  const block = useNativeRead(selector ? `/v2/explorer/blocks/${selector}` : null, decodeBlock);
  const active = ['transactions','internal'].includes(params.get('tab') ?? '') ? params.get('tab')! : 'details';
  // Each child list keeps its own cursor while another tab is selected.
  const path = block.data ? `/blocks/${block.data.number}` : null;
  const txs = useNativePage(path && `${path}/transactions`, decodeTransactions, 25, '', active==='transactions');
  const internal = useNativePage(path && `${path}/internal`, decodeInternalTransactions, 25, '', active==='internal');
  if (!selector) return <StateMessage variant="error" title="Invalid block"/>;
  const b = block.data;
  return <NativeNames items={b?[{address:b.miner}]:[]}><div className="space-y-6"><PageHeader title={b ? `Block #${exactQuantity(b.number)}` : 'Block details'}><Link to="/blocks" className="text-primary">All blocks</Link>{b&&<div className="flex gap-3">{BigInt(b.number)>0n&&<Link aria-label="Previous block" to={`/block/${BigInt(b.number)-1n}`}>←</Link>}{BigInt(b.number)<9223372036854775807n&&<Link aria-label="Next block" to={`/block/${BigInt(b.number)+1n}`}>→</Link>}</div>}</PageHeader>
    <NativeReadState {...block}/>{b===null&&<StateMessage variant="empty" title="Block unavailable"/>}
    {b&&<><ExplorerTabs active={active} onChange={tab=>{const q=new URLSearchParams(params);q.set('tab',tab);setParams(q);}} tabs={[{id:'details',label:'Details'},{id:'transactions',label:'Transactions',count:b.transactionCount===undefined?undefined:exactQuantity(b.transactionCount)},{id:'internal',label:'Internal transactions'}]}/>
    {active==='details'&&<div className="card divide-y divide-neutral-100"><InfoRow label="Block height" value={exactQuantity(b.number)}/><InfoRow label="Timestamp" value={exactTimestamp(b.timestamp)}/><InfoRow label="Transactions" value={exactQuantity(b.transactionCount)}/><InfoRow label="Miner" value={<NativeAddress address={b.miner}/>}/><InfoRow label="Gas limit" value={exactQuantity(b.gasLimit)}/><InfoRow label="Base fee (wei)" value={exactQuantity(b.baseFeePerGas)}/><InfoRow label="Hash" value={<span className="font-mono break-all">{b.hash} <CopyButton text={b.hash}/></span>}/><InfoRow label="Parent hash" value={<span className="inline-flex flex-wrap items-center gap-1"><Link className="font-mono text-primary break-all" to={`/blocks/hash/${b.parentHash}`}>{b.parentHash}</Link>{b.parentHash && <CopyButton text={b.parentHash}/>}</span>}/><InfoRow label="Source" value={b.source==='NODE'?'Verified node point read':b.source==='INDEXER'?'Indexed block':'Unavailable'}/><details className="p-4"><summary className="cursor-pointer text-primary">Show more</summary>{[['Difficulty',b.difficulty],['Total difficulty',b.totalDifficulty],['Nonce',b.nonce],['Extra data',b.extraData],['State root',b.stateRoot],['Transactions root',b.transactionsRoot],['Receipts root',b.receiptsRoot]].map(([label,value])=><InfoRow key={label} label={label!} value={<span className="font-mono break-all">{value??'Unavailable'}{value !== undefined && ['State root','Transactions root','Receipts root'].includes(label!) && <CopyButton text={value}/>}</span>}/>)}</details></div>}
    {active==='transactions'&&<><NativeReadState {...txs} reset={txs.reset}/>{txs.data&&<><NativeTransactionsTable items={txs.data.items} footer={<NativeContinuation {...txs} page={txs.data.page}/>}/>{!txs.data.items.length&&<StateMessage variant="empty" title="No visible transactions on this page"/>}</>}</>}
    {active==='internal'&&<><NativeReadState {...internal} reset={internal.reset}/>{internal.data&&<><NativeTraceRows items={internal.data.items}/><NativeContinuation {...internal} page={internal.data.page}/></>}</>}<VisiblePageNote/></>}
  </div></NativeNames>;
}
