import { useEffect, useState } from 'react';
import { Link, useLocation, useParams, useSearchParams } from 'react-router-dom';
import { PageHeader } from '../components/PageHeader';
import { CopyButton } from '../components/CopyButton';
import { TokenAvatar } from '../components/TokenAvatar';
import { TokenHero, TokenTypeBadge } from '../components/TokenPresentation';
import { TokensToolbar, TokensEmptyState } from '../components/TokensPresentation';
import { ExplorerTable, ExplorerTabs, VisiblePageNote } from '../components/ExplorerReadPresentation';
import { NativeAddress } from '../components/NativeAddress';
import { NativeContinuation, NativeReadState, NativeTransferRows, useNativePage } from '../components/NativeReadPanels';
import { NativeContractPanel } from '../components/NativeContractPanel';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeToken, decodeTokens, decodeHolders, decodeTransfers, decodeBalances, decodeContract, decodeCode } from '../lib/nativeReadModels';
import { exactQuantity, exactUnits } from '../lib/nativePresentation';
import type { Balance, Token } from '../lib/v2ReadTypes';
import { StateMessage } from '../components/StateMessage';

function TokenSupply({token}:{token:Token}) {return <>{exactUnits(token.totalSupply,token.decimals)}</>;}
export function TokensNative() {
  const [params,setParams]=useSearchParams();const type=params.get('type')??'', search=params.get('search')??'';
  const [input,setInput]=useState(search);
  useEffect(()=>{setInput(search);},[search]);
  useEffect(()=>{if(input===search)return;const timer=setTimeout(()=>{const q=new URLSearchParams(params);q.delete('cursor');q.delete('page');if(input.trim())q.set('search',input.trim());else q.delete('search');setParams(q,{replace:true});},300);return()=>clearTimeout(timer);},[input,search,params,setParams]);
  const filters=new URLSearchParams();if(type)filters.set('type',type);if(search)filters.set('search',search);
  const tokens=useNativePage('/tokens',decodeTokens,25,filters.toString());
  return <div className="space-y-6"><PageHeader title="Tokens" subtitle="Token contracts visible to your current access"/><TokensToolbar includeERC1155 activeFilter={type} onFilter={value=>{const q=new URLSearchParams();if(value)q.set('type',value);if(search)q.set('search',search);setParams(q);}} searchValue={input} onSearch={value=>setInput(value.slice(0,128))} resultCount={null} searching={!!search}/><VisiblePageNote/><NativeReadState {...tokens} reset={tokens.reset}/>{tokens.data&&<div className="card"><ExplorerTable headings={['Token','Total supply','Holders','Transfers']}>{tokens.data.items.map(token=><tr key={token.address} data-testid="token-row"><td><div className="flex items-center gap-3"><TokenAvatar symbol={token.symbol||'?'} address={token.address}/><div><div className="flex flex-wrap gap-2 items-center"><Link className="font-semibold hover:text-primary" to={`/token/${token.address}`}>{token.name||'Unknown token'}</Link><span className="text-neutral-500">{token.symbol}</span><TokenTypeBadge type={token.tokenType}/></div><div className="flex gap-1 items-center"><NativeAddress address={token.address}/><CopyButton text={token.address}/></div></div></div></td><td className="font-mono"><TokenSupply token={token}/></td><td>{exactQuantity(token.holderCount)}</td><td>{exactQuantity(token.transferCount)}</td></tr>)}</ExplorerTable>{!tokens.data.items.length&&<TokensEmptyState>{search?'No visible tokens match this search on this page.':'No visible tokens on this page.'}</TokensEmptyState>}<NativeContinuation {...tokens} page={tokens.data.page}/></div>}</div>;
}
export function TokenNative() {
  const {address=''}=useParams();const [params,setParams]=useSearchParams();const location=useLocation();
  const suffix=location.pathname.split('/').at(-1);const selected=params.get('tab')??(suffix==='holders'||suffix==='contract'?suffix:'transfers');
  const active=['holders','contract','transfers'].includes(selected)?selected:'transfers';
  const valid=/^0x[0-9a-f]{40}$/.test(address)&&address!==`0x${'0'.repeat(40)}`;const path=valid?`/tokens/${address}`:null;
  const token=useNativeRead(path&&`/v2/explorer${path}`,decodeToken);
  const holders=useNativePage(path&&`${path}/holders`,decodeHolders,25,'',active==='holders');
  const transfers=useNativePage(path&&`${path}/transfers`,decodeTransfers,25,'',active==='transfers');
  const contract=useNativeRead(valid&&active==='contract'?`/v2/explorer/addresses/${address}/contract`:null,decodeContract);
  const code=useNativeRead(valid&&active==='contract'?`/v2/explorer/addresses/${address}/code`:null,decodeCode);
  if(!valid)return <StateMessage variant="error" title="Invalid token address"/>;
  const t=token.data;
  return <div className="space-y-6"><Link to="/tokens" className="text-sm text-primary">← All tokens</Link><NativeReadState {...token}/>{t===null&&<StateMessage variant="empty" title="Token unavailable"/>}{t&&<><TokenHero token={t}/><dl className="grid grid-cols-2 overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50 sm:grid-cols-4">{[['Total supply',exactUnits(t.totalSupply,t.decimals)],['Holders',exactQuantity(t.holderCount)],['Transfers',exactQuantity(t.transferCount)],['Decimals',String(t.decimals)]].map(([label,value],i)=><div key={label} data-testid="stat-tile" data-label={label} className={`px-5 py-4 ${i?'border-l border-neutral-200/70':''}`}><dt className="text-xs uppercase tracking-wider text-neutral-500">{label}</dt><dd className="mt-1 font-mono text-lg break-all">{value}</dd></div>)}</dl><ExplorerTabs active={active} tabs={[{id:'transfers',label:'Transfers'},{id:'holders',label:'Holders'},{id:'contract',label:'Contract'}]} onChange={tab=>{const q=new URLSearchParams();q.set('tab',tab);setParams(q);}}/>
  {active==='transfers'&&<><NativeReadState {...transfers} reset={transfers.reset}/>{transfers.data&&<div className="card"><div className="p-4"><NativeTransferRows items={transfers.data.items} token={t}/></div><NativeContinuation {...transfers} page={transfers.data.page}/></div>}</>}
  {active==='holders'&&<><NativeReadState {...holders} reset={holders.reset}/>{holders.data&&<div className="card"><ExplorerTable headings={['Address','Balance']}>{holders.data.items.map(holder=><tr key={holder.address}><td><NativeAddress address={holder.address} full/></td><td className="font-mono">{exactUnits(holder.balance,t.decimals)} {t.symbol}</td></tr>)}</ExplorerTable>{!holders.data.items.length&&<TokensEmptyState>No holders on this visible page.</TokensEmptyState>}<NativeContinuation {...holders} page={holders.data.page}/></div>}</>}
  {active==='contract'&&<><NativeReadState {...contract}/><NativeReadState {...code}/><NativeContractPanel key={address} address={address} contract={contract.data} code={code.data?.code}/></>}<VisiblePageNote/></>}</div>;
}
function PortfolioRow({balance}:{balance:Balance}) {
  // At most 25 metadata reads, one per visible portfolio row. Failed metadata
  // enrichment leaves the exact base-unit amount and its original token link.
  const visible=balance.tokenAddress!==`0x${'0'.repeat(40)}`;
  const token=useNativeRead(visible?`/v2/explorer/tokens/${balance.tokenAddress}`:null,decodeToken);
  return <tr><td>{visible?<Link className="text-primary" to={`/token/${balance.tokenAddress}`}>{token.data?.name||token.data?.symbol||balance.tokenAddress}</Link>:<NativeAddress address={balance.tokenAddress} token/>}</td><td className="font-mono">{token.data?`${exactUnits(balance.balance,token.data.decimals)} ${token.data.symbol}`:`${exactQuantity(balance.balance)} base units`}</td><td><Link className="text-primary" to={`/block/${balance.blockNumber}`}>{exactQuantity(balance.blockNumber)}</Link></td></tr>;
}
export function NativePortfolio({address}:{address:string}) {
  const balances=useNativePage(`/addresses/${address}/balances`,decodeBalances);
  return <><NativeReadState {...balances} reset={balances.reset}/>{balances.data&&<div className="card"><ExplorerTable headings={['Token','Balance','At block']}>{balances.data.items.map(balance=><PortfolioRow key={balance.tokenAddress} balance={balance}/>)}</ExplorerTable>{!balances.data.items.length&&<TokensEmptyState>No token balances on this visible page.</TokensEmptyState>}<NativeContinuation {...balances} page={balances.data.page}/></div>}</>;
}
