import { useNativeMetadata } from '../hooks/useNativeMetadata';
import { mayEnrich, nativeTokenPath, NativeNames } from '../components/NativeNames';
import { useCanonicalIdentifier } from '../lib/nativeRoute';
import { useEffect, useState } from 'react';
import { Link, useLocation, useParams, useSearchParams } from 'react-router-dom';
import { PageHeader } from '../components/PageHeader';
import { TokenAvatar } from '../components/TokenAvatar';
import { TokenHero, TokenTypeBadge } from '../components/TokenPresentation';
import { TokensToolbar, TokensEmptyState } from '../components/TokensPresentation';
import { ExplorerTable, ExplorerTabs, VisiblePageNote } from '../components/ExplorerReadPresentation';
import { NativeAddress } from '../components/NativeAddress';
import { NativeContinuation, NativeReadState, NativeTransferRows, useNativePage } from '../components/NativeReadPanels';
import { NativeContractPanel } from '../components/NativeContractPanel';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeToken, decodeTokens, decodeHolders, decodeTransfers, decodeBalances, decodeContract, decodeCode } from '../lib/nativeReadModels';
import { exactQuantity, exactUnits, exactFiatValue } from '../lib/nativePresentation';
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
  return <div className="space-y-6"><PageHeader title="Tokens" subtitle="Token contracts visible to your current access"/><TokensToolbar includeERC1155 activeFilter={type} onFilter={value=>{const q=new URLSearchParams();if(value)q.set('type',value);if(search)q.set('search',search);setParams(q);}} searchValue={input} onSearch={value=>setInput(value.slice(0,128))} resultCount={null} searching={!!search}/><VisiblePageNote/><NativeReadState {...tokens} reset={tokens.reset}/>{tokens.data&&<div className="card"><ExplorerTable headings={['Token','Total supply','Holders','Transfers']}>{tokens.data.items.map(token=><tr key={token.address} data-testid="token-row"><td><div className="flex items-center gap-3"><TokenAvatar symbol={token.symbol||'?'} address={token.address}/><div><div className="flex flex-wrap gap-2 items-center"><Link className="font-semibold hover:text-primary" to={`/token/${token.address}`}>{token.name||'Unknown token'}</Link><span className="text-neutral-500">{token.symbol}</span><TokenTypeBadge type={token.tokenType}/></div><div className="flex gap-1 items-center"><NativeAddress address={token.address}/></div></div></div></td><td className="font-mono"><TokenSupply token={token}/></td><td>{exactQuantity(token.holderCount)}</td><td>{exactQuantity(token.transferCount)}</td></tr>)}</ExplorerTable>{!tokens.data.items.length&&<TokensEmptyState>{search?'No visible tokens match this search on this page.':'No visible tokens on this page.'}</TokensEmptyState>}<NativeContinuation {...tokens} page={tokens.data.page}/></div>}</div>;
}
export function TokenNative() {
  const {address:rawAddress=''}=useParams();const address=useCanonicalIdentifier(rawAddress,'address')??'';const [params,setParams]=useSearchParams();const location=useLocation();
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
  return <div className="space-y-6"><Link to="/tokens" className="text-sm text-primary">← All tokens</Link><NativeReadState {...token}/>{t===null&&<StateMessage variant="empty" title="Token unavailable"/>}{t&&<><TokenHero token={t}/><dl className="grid grid-cols-2 overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50 sm:grid-cols-4">{[['Total supply',exactUnits(t.totalSupply,t.decimals)],['Holders',exactQuantity(t.holderCount)],['Transfers',exactQuantity(t.transferCount)],['Decimals',String(t.decimals)]].map(([label,value],i)=><div key={label} data-testid="stat-tile" data-label={label} className={`px-5 py-4 ${i?'border-l border-neutral-200/70':''}`}><dt className="text-xs uppercase tracking-wider text-neutral-500">{label}</dt><dd className="mt-1 font-mono text-lg break-all">{value}</dd></div>)}</dl><ExplorerTabs active={active} tabs={[{id:'transfers',label:'Transfers'},{id:'holders',label:'Holders'},{id:'contract',label:'Contract'}]} onChange={tab=>{const q=new URLSearchParams(params);q.set('tab',tab);setParams(q);}}/>
  {active==='transfers'&&<><NativeReadState {...transfers} reset={transfers.reset}/>{transfers.data&&<div className="card"><NativeTransferRows items={transfers.data.items} token={t}/><NativeContinuation {...transfers} page={transfers.data.page}/></div>}</>}
  {active==='holders'&&<><NativeReadState {...holders} reset={holders.reset}/>{holders.data&&<NativeNames items={holders.data.items.map(holder=>({address:holder.address}))}><div className="card"><ExplorerTable headings={['Address','Balance']}>{holders.data.items.map(holder=><tr key={holder.address}><td><NativeAddress address={holder.address} full/></td><td className="font-mono">{exactUnits(holder.balance,t.decimals)} {t.symbol}</td></tr>)}</ExplorerTable>{!holders.data.items.length&&<TokensEmptyState>No holders on this visible page.</TokensEmptyState>}<NativeContinuation {...holders} page={holders.data.page}/></div></NativeNames>}</>}
  {active==='contract'&&<><NativeReadState {...contract}/><NativeReadState {...code}/><NativeContractPanel key={address} address={address} contract={contract.data} code={code.data?.code}/></>}<VisiblePageNote/></>}</div>;
}
function PortfolioRow({balance,token}:{balance:Balance;token?:Token}) {
  const visible=mayEnrich(balance.tokenAddress), known=token?.address===balance.tokenAddress?token:undefined;
  return <tr data-testid="portfolio-row"><td><div className="flex gap-3 items-center"><TokenAvatar address={balance.tokenAddress} symbol={known?.symbol||'?'}/><div><div className="flex flex-wrap gap-2 items-center">{visible?<Link className="text-primary font-semibold" to={`/token/${balance.tokenAddress}`}>{known?.name||known?.symbol||'Unknown asset'}</Link>:<span>Private</span>}{known&&<><span className="text-neutral-500">{known.symbol}</span><TokenTypeBadge type={known.tokenType}/></>}</div><div className="flex gap-1 items-center"><NativeAddress address={balance.tokenAddress} token/></div></div></div></td><td className="font-mono break-all">{known?`${exactUnits(balance.balance,known.decimals)} ${known.symbol}`:`${exactQuantity(balance.balance)} base units`}</td><td>{known?.usdPrice!==undefined?<><span className="font-mono">{known.usdPrice} USD</span><div className="text-xs text-neutral-500">Returned token price{known.offChainUpdatedAt?` · ${known.offChainUpdatedAt}`:''}</div></>:'Unavailable'}</td><td className="font-mono break-all">{known?.usdPrice!==undefined?`${exactFiatValue(balance.balance,known.decimals,known.usdPrice)} USD`:'Unavailable'}</td><td><Link className="text-primary font-mono" to={`/block/${balance.blockNumber}`}>{exactQuantity(balance.blockNumber)}</Link></td></tr>;
}
export function NativePortfolio({address}:{address:string}) {
  const balances=useNativePage(`/addresses/${address}/balances`,decodeBalances);
  const items=balances.data?.items??[];
  const metadata=useNativeMetadata(items.filter(balance=>mayEnrich(balance.tokenAddress)).map(balance=>nativeTokenPath(balance.tokenAddress)),decodeToken,1);
  const [search,setSearch]=useState(''),[type,setType]=useState('');
  const matching=items.filter(balance=>{const token=metadata[nativeTokenPath(balance.tokenAddress)];return (!type||token?.tokenType.toUpperCase().replaceAll('-','')===type)&&(!search.trim()||[balance.tokenAddress,token?.name,token?.symbol].some(value=>value?.toLowerCase().includes(search.trim().toLowerCase())));});
  return <div className="space-y-4"><div className="flex flex-wrap gap-3"><label className="text-sm">Filter loaded assets<input aria-label="Filter loaded assets" className="input block mt-1" value={search} onChange={event=>setSearch(event.target.value.slice(0,128))} placeholder="Name, symbol or address"/></label><label className="text-sm">Token type<select aria-label="Loaded asset token type" className="input block mt-1" value={type} onChange={event=>setType(event.target.value)}><option value="">All types</option><option value="ERC20">ERC20</option><option value="ERC721">ERC721</option><option value="ERC1155">ERC1155</option></select></label></div><p className="text-xs text-neutral-500">Filters apply only to this loaded authorized page. USD values use returned token prices; no complete portfolio total is inferred.</p><NativeReadState {...balances} reset={balances.reset}/>{balances.data&&<div className="card"><ExplorerTable headings={['Asset','Balance','Price','Value at returned price','Observed block']}>{matching.map(balance=><PortfolioRow key={balance.tokenAddress} balance={balance} token={metadata[nativeTokenPath(balance.tokenAddress)]??undefined}/>)}</ExplorerTable>{!matching.length&&<TokensEmptyState>{items.length?'No loaded assets match these filters.':'No token balances on this visible page.'}</TokensEmptyState>}<NativeContinuation {...balances} page={balances.data.page}/></div>}</div>;
}
