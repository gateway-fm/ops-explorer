import { nativeAPIReadGroups as groups } from '../lib/nativeAPIDocs';
import { useEffect, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { PageHeader } from '../components/PageHeader';
import { BlockInfoRow as InfoRow } from '../components/BlockInfoRow';
import { NativeContinuation, NativeReadState, useNativePage } from '../components/NativeReadPanels';
import { APIEndpointCard, APISidebar, JsonSyntax, APICopyButton, type APIEndpoint } from '../components/APIReadPresentation';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeSyncStatus, decodeIndexerProgress, decodeCatchup } from '../lib/nativeReadModels';
import { exactQuantity } from '../lib/nativePresentation';
import { readPage, searchSuggestionTarget } from '../lib/v2Reads';
import { appSession, type AppOperation } from '../lib/v2Session';
import { getConfig } from '../lib/runtimeConfig';

export function SyncNative() {
  const sync=useNativeRead('/v2/explorer/sync/status',decodeSyncStatus,10000);
  const index=useNativeRead('/v2/explorer/sync/indexer-progress',decodeIndexerProgress,10000);
  const catchup=useNativeRead('/v2/explorer/sync/catchup',decodeCatchup,10000);
  return <div className="space-y-6"><PageHeader title="Network sync" subtitle="Current indexer progress available to your session"><Link className="text-primary text-sm" to="/chain-info">Network setup</Link></PageHeader><section className="card divide-y divide-neutral-100"><h2 className="p-4 font-medium">Synchronization</h2><NativeReadState {...sync}/>{sync.data&&<><InfoRow label="Status" value={sync.data.isSyncing?'Syncing':'Not syncing'}/><InfoRow label="Last indexed block" value={exactQuantity(sync.data.lastIndexedBlock)}/><InfoRow label="Updated" value={sync.data.updatedAt}/></>}</section><section className="card divide-y divide-neutral-100"><h2 className="p-4 font-medium">Indexer progress</h2><NativeReadState {...index}/>{index.data&&<><InfoRow label="Earliest fetched block" value={exactQuantity(index.data.minFetchedBlock)}/><InfoRow label="Latest fetched block" value={exactQuantity(index.data.maxFetchedBlock)}/><InfoRow label="Backfill" value={index.data.backfillComplete?'Complete':'In progress'}/></>}</section><section className="card divide-y divide-neutral-100"><h2 className="p-4 font-medium">Catch-up</h2><NativeReadState {...catchup}/>{catchup.data&&<InfoRow label="Blocks behind" value={exactQuantity(catchup.data.behindBlocks)}/>}</section><p className="text-sm text-neutral-500">Progress describes the indexer, not network transaction volume or a guarantee that all historical records are available.</p></div>;
}
function decodeSearchPage(raw:unknown) {const page=readPage(raw);for(const item of page.items)if(!searchSuggestionTarget(item))throw Error('Invalid search result');return page;}
export function SearchNative() {
  const [params,setParams]=useSearchParams();const q=params.get('q')??'';
  const results=useNativePage(q?'/search/suggestions':null,decodeSearchPage,20,new URLSearchParams(q?{q}:{}).toString());
  return <div className="space-y-6"><PageHeader title="Search"/><form className="flex gap-3" key={q} onSubmit={e=>{e.preventDefault();const value=String(new FormData(e.currentTarget).get('q')??'').trim();setParams(value?{q:value}:{});}}><input required name="q" aria-label="Search the explorer" maxLength={128} defaultValue={q} placeholder="Block, transaction, token or address" className="input flex-1 min-w-0"/><button className="btn-primary">Search</button></form><NativeReadState {...results} reset={results.reset}/>{results.data&&<div className="card"><ul className="divide-y divide-neutral-100">{results.data.items.map(item=><li key={`${item.type}:${item.value}`}><Link to={searchSuggestionTarget(item)!} className="flex gap-3 p-4 hover:bg-neutral-100"><span className="text-neutral-500 capitalize">{String(item.type)}</span><span className="font-mono text-primary break-all">{String(item.value)}</span></Link></li>)}</ul>{!results.data.items.length&&<p className="p-6 text-neutral-500 text-sm">No visible results on this page.</p>}<NativeContinuation {...results} page={results.data.page}/></div>}</div>;
}

function TryNativeRead({endpoint:ep}:{endpoint:APIEndpoint}) {
  const [values,setValues]=useState<Record<string,string>>({}),[busy,setBusy]=useState(false),[error,setError]=useState<string>(),[result,setResult]=useState<unknown>();
  const scope=useRef<AppOperation|undefined>(undefined);const generation=useRef(0);
  useEffect(()=>()=>{++generation.current;scope.current?.dispose();},[]);
  const clear=()=>{++generation.current;scope.current?.dispose();setBusy(false);setResult(undefined);setError(undefined);};
  const run=async(e:React.FormEvent)=>{
    e.preventDefault();if(busy)return;setBusy(true);setResult(undefined);setError(undefined);const owned=++generation.current;let operation:AppOperation|undefined;
    try{operation=appSession.captureOperation();scope.current=operation;let path=ep.path;const query=new URLSearchParams();for(const param of ep.params??[]){const value=(values[param.name]??param.default??'').trim();if(path.includes(`{${param.name}}`)){if(!value||!(/^(0|[1-9][0-9]{0,18})$/.test(value)&&param.name==='number'||/^0x[0-9a-f]{40}$/.test(value)&&param.name==='address'||/^0x[0-9a-f]{64}$/.test(value)&&param.name==='hash'))throw Error(`Enter a valid ${param.name}.`);path=path.replace(`{${param.name}}`,value);}else if(value)query.set(param.name,value);}const raw=await appSession.read(path+(query.size?'?'+query.toString():''),operation.signal);operation.check();if(generation.current===owned)setResult(raw);}
    catch(e){if(generation.current===owned&&!operation?.signal.aborted)setError(e instanceof Error?e.message:'The request could not be completed.');}
    finally{operation?.dispose();if(generation.current===owned)setBusy(false);}
  };
  return <form onSubmit={run} className="border-t border-neutral-200 px-4 py-4 space-y-3"><h4 className="font-semibold text-sm">Try it with your current session</h4>{ep.params?.map(param=><label key={param.name} className="block text-xs text-neutral-500">{param.name}<input aria-label={`${ep.path} ${param.name}`} required={param.required} value={values[param.name]??param.default??''} onChange={e=>{clear();setValues(old=>({...old,[param.name]:e.target.value}));}} className="input block w-full mt-1"/></label>)}<button className="btn-primary text-sm" disabled={busy}>{busy?'Loading…':'Send request'}</button>{error&&<p role="alert" className="text-sm text-error-600">{error}</p>}{result!==undefined&&<div className="max-h-96 overflow-auto"><JsonSyntax data={result}/></div>}</form>;
}
export function ApiDocsNative() {
  const [active,setActive]=useState(groups[0].id);const base=getConfig('VITE_API_URL','/api');
  return <div className="space-y-6"><PageHeader title="Explorer API" subtitle="Native reads using your current application session"/><div className="card p-4"><p className="text-sm text-neutral-500">Base URL</p><div className="flex items-center gap-2"><code className="font-mono text-sm break-all">{base}</code><APICopyButton text={base}/></div><p className="text-sm text-neutral-500 mt-3">Private endpoints require the existing browser session and current application identity. Access tokens stay in the BFF. Continuations belong to the original filters, identity, dataset and history range.</p></div><div className="flex flex-col lg:flex-row gap-6"><aside className="lg:w-60 shrink-0"><APISidebar groups={groups} activeGroup={active} onSelect={setActive} showRateLimiting={false}/></aside><div className="min-w-0 flex-1 space-y-4"><h2 className="text-xl font-semibold">{groups.find(group=>group.id===active)?.label}</h2>{groups.find(group=>group.id===active)?.endpoints.map(ep=><APIEndpointCard key={ep.path} endpoint={ep} curl={`GET ${base}${ep.path}`} tryIt={<TryNativeRead endpoint={ep}/>}/>)}<div className="card p-4 text-sm text-neutral-500"><h3 className="font-medium text-neutral-800 mb-2">Pagination and availability</h3><p>Keep the returned opaque cursor even on an empty scan-limited page. CURRENT pages can change and skip repeated keys. HTTP 409 requires restarting the original query; HTTP 410 means older history was pruned. Unknown totals are omitted. The API can deny a request or ask you to narrow it; it never adds hidden records to complete a page.</p></div></div></div></div>;
}
