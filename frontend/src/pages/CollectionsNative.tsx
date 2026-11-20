import { useEffect, useState } from 'react';
import { useVisibleTransactionsNotice } from '../hooks/useVisibleTransactionsNotice';
import { NewItemsNotice } from '../components/NewItemsNotice';
import { NativeNames } from '../components/NativeNames';
import { useSearchParams } from 'react-router-dom';
import { PageHeader } from '../components/PageHeader';
import { NativeAddress } from '../components/NativeAddress';
import { ExplorerTable, ExplorerTabs, VisiblePageNote } from '../components/ExplorerReadPresentation';
import { NativeContinuation, NativeReadState, NativeTransactionsTable, NativeTransferRows, NativeLogCards, useNativePage } from '../components/NativeReadPanels';
import { decodeAccounts, decodeTransactions, decodeTransfers, decodeLogs } from '../lib/nativeReadModels';
import { exactQuantity, exactTimestamp } from '../lib/nativePresentation';
import { EmptyRow } from '../components/AddressPresentation';

const categories=[{id:'',label:'All'},{id:'coin_transfer',label:'Coin transfers'},{id:'contract_call',label:'Contract calls'},{id:'contract_creation',label:'Contract creation'},{id:'token_transfer',label:'Token transfers'}];
export function TransactionsNative() {
  const [params,setParams]=useSearchParams(); const category=params.get('category')??'';
  const filters=new URLSearchParams(); if(categories.some(c=>c.id===category)&&category)filters.set('category',category);
  const txs=useNativePage('/transactions/paginated',decodeTransactions,25,filters.toString());
  const [refreshing,setRefreshing]=useState(false);
  useEffect(()=>{setRefreshing(false);},[txs.data]);
  const live=useVisibleTransactionsNotice(txs.endpoint,txs.data,txs.index===0&&!refreshing);
  return <div className="space-y-6"><PageHeader title="Transactions"/><ExplorerTabs tabs={categories} active={category} onChange={id=>{const q=new URLSearchParams();if(id)q.set('category',id);setParams(q);}}/><VisiblePageNote/>{live.notice&&<NewItemsNotice changed observedWindow count={live.notice.count} type="transaction" onClick={()=>{live.clear();setRefreshing(true);txs.reset();}}/>}<NativeReadState {...txs} reset={txs.reset}/>{txs.data&&<><NativeTransactionsTable items={txs.data.items} footer={<NativeContinuation {...txs} page={txs.data.page}/>}/>{!txs.data.items.length&&<EmptyRow>No visible transactions on this page. Continue when more records are available.</EmptyRow>}</>}</div>;
}
export function AccountsNative() {
  const accounts=useNativePage('/accounts',decodeAccounts);
  return <div className="space-y-6"><PageHeader title="Accounts" subtitle="Addresses visible to your current access"/><VisiblePageNote/><NativeReadState {...accounts} reset={accounts.reset}/>{accounts.data&&<NativeNames items={accounts.data.items.map(account=>({address:account.address}))}><div className="card"><ExplorerTable headings={['Address','Type','Transactions','Token transfers','First seen','Last seen']}>{accounts.data.items.map(account=><tr key={account.address} data-testid="account-row"><td><NativeAddress address={account.address} full/></td><td>{account.isContract===undefined?'Unavailable':account.isContract?'Contract':'EOA'}</td><td>{exactQuantity(account.txCount)}</td><td>{exactQuantity(account.tokenTransferCount)}</td><td>{exactTimestamp(account.firstSeen)}</td><td>{exactTimestamp(account.lastSeen)}</td></tr>)}</ExplorerTable>{!accounts.data.items.length&&<EmptyRow>No visible accounts on this page.</EmptyRow>}<NativeContinuation {...accounts} page={accounts.data.page}/></div></NativeNames>}</div>;
}
export function TransfersNative() {
  const transfers=useNativePage('/transfers',decodeTransfers);
  return <div className="space-y-6"><PageHeader title="Token Transfers"/><VisiblePageNote/><p className="text-xs text-neutral-500">The authorized transfer feed includes all returned token standards. Filtering this feed by token type is unavailable from the current source.</p><NativeReadState {...transfers} reset={transfers.reset}/>{transfers.data&&<div className="card"><NativeTransferRows items={transfers.data.items}/><NativeContinuation {...transfers} page={transfers.data.page}/></div>}</div>;
}
export function LogsNative() {
  const [params,setParams]=useSearchParams();
  const filters=new URLSearchParams(); for(const key of ['address','topic0','from','to']){const v=params.get(key);if(v)filters.set(key,v);}
  const logs=useNativePage('/logs',decodeLogs,25,filters.toString());
  return <div className="space-y-6"><PageHeader title="Event Logs"/><form className="card p-4 grid sm:grid-cols-2 gap-4" onSubmit={e=>{e.preventDefault();const form=new FormData(e.currentTarget);const q=new URLSearchParams();for(const key of ['address','topic0','from','to']){const value=String(form.get(key)??'').trim().toLowerCase();if(value)q.set(key,value);}setParams(q);}} key={filters.toString()}>{[['address','Contract address'],['topic0','Event topic'],['from','From block'],['to','Through block']].map(([key,label])=><label key={key} className="text-sm text-neutral-600">{label}<input name={key} defaultValue={params.get(key)??''} className="input w-full block mt-1" pattern={key==='address'?'0x[0-9a-fA-F]{40}':key==='topic0'?'0x[0-9a-fA-F]{64}':'0|[1-9][0-9]*'}/></label>)}<button className="btn-primary justify-self-start">Apply filters</button></form><VisiblePageNote/><NativeReadState {...logs} reset={logs.reset}/>{logs.data&&<><NativeLogCards items={logs.data.items}/><NativeContinuation {...logs} page={logs.data.page}/></>}</div>;
}
