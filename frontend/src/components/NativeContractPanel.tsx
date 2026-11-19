import { NativeContractWrite } from './NativeContractWrite';
import { parseNativeABIInput } from '../lib/nativeABI';
import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { Interface, FunctionFragment } from 'ethers';
import { ChevronDown } from 'lucide-react';
import { appSession, type AppOperation, type OwnSelf } from '../lib/v2Session';
import type { Contract } from '../lib/v2ReadTypes';
import { AddressDetailsPanel } from './AddressPresentation';
import { CopyButton } from './CopyButton';

function ReadFunction({fragment,iface,address,self}: {fragment:FunctionFragment;iface:Interface;address:string;self:OwnSelf}) {
  const [open,setOpen]=useState(false),[inputs,setInputs]=useState<string[]>([]),[org,setOrg]=useState(self.organizations[0]?.id??''),[result,setResult]=useState<string>(),[error,setError]=useState<string>(),[busy,setBusy]=useState(false);
  const scope=useRef<AppOperation|undefined>(undefined),ticket=useRef(0);
  useEffect(()=>()=>{++ticket.current;scope.current?.dispose();},[]);
  const submit=async(e:React.FormEvent)=>{
    e.preventDefault();if(busy)return;setBusy(true);setResult(undefined);setError(undefined);const owned=++ticket.current;
    let operation:AppOperation|undefined;
    try {
      operation=appSession.captureOperation();scope.current=operation;
      if(!self.organizations.some(value=>value.id===org))throw Error('Select one of your organizations.');
      const args=fragment.inputs.map((param,i)=>parseNativeABIInput(inputs[i]??'',param));
      const data=iface.encodeFunctionData(fragment,args);
      const response=await operation.rpc(org,{jsonrpc:'2.0',id:crypto.randomUUID(),method:'eth_call',params:[{to:address,data},'latest']});operation.check();
      if(response.error)throw Error('The contract read was denied or reverted.');
      if(typeof response.result!=='string'||!/^0x(?:[0-9a-fA-F]{2})*$/.test(response.result))throw Error('The contract returned invalid data.');
      const decoded=iface.decodeFunctionResult(fragment,response.result);
      const formatted=JSON.stringify(decoded,(_,value:unknown)=>typeof value==='bigint'?value.toString():value,2);
      operation.check();if(ticket.current===owned)setResult(formatted);
    } catch(e){if(ticket.current===owned&&!operation?.signal.aborted){try{operation?.check();setError(e instanceof Error?e.message:'Contract read failed.');}catch{/* the owner unmounts this view */}}}
    finally{operation?.dispose();if(ticket.current===owned)setBusy(false);}
  };
  const signature=fragment.format('sighash');
  return <div className="border border-neutral-200 rounded-lg overflow-hidden"><button data-testid="contract-read-fn" aria-expanded={open} onClick={()=>setOpen(!open)} className="w-full px-4 py-3 flex items-center justify-between bg-neutral-50 hover:bg-neutral-100 text-left"><span className="font-medium text-neutral-900 break-all">{signature}</span><ChevronDown className="w-4 h-4 shrink-0"/></button>{open&&<form onSubmit={submit} className="p-4 space-y-4"><label className="block text-sm">Organization<select required className="input block w-full mt-1" value={org} onChange={e=>{setOrg(e.target.value);setResult(undefined);}} disabled={busy}>{self.organizations.map(value=><option key={value.id} value={value.id}>{value.name||value.slug}</option>)}</select></label>{fragment.inputs.map((param,i)=><label key={i} className="block text-sm text-neutral-500">{param.name||`Input ${i+1}`} ({param.format('sighash')})<input required aria-label={`${signature} ${param.name||`input ${i+1}`}`} disabled={busy} value={inputs[i]??''} onChange={e=>setInputs(old=>{const next=[...old];next[i]=e.target.value;return next;})} className="input block w-full mt-1" placeholder={param.baseType==='array'||param.baseType==='tuple'?'JSON array; quote integer values':param.type}/></label>)}<p className="text-xs text-neutral-500">Returns: {fragment.outputs.map(output=>output.format('full')).join(', ')||'no outputs'}. Uses your current OPS permissions.</p><button data-testid="contract-read-submit" className="btn-primary" disabled={busy||!org}>{busy?'Reading…':'Query'}</button>{error&&<p role="alert" className="text-sm text-error-600">{error}</p>}{result!==undefined&&<pre data-testid="fn-result" className="p-3 bg-success-50 border border-success-200 rounded-lg font-mono text-sm break-all whitespace-pre-wrap">{result}</pre>}</form>}</div>;
}
export function NativeContractPanel({address,contract,code}: {address:string;contract:Contract|null|undefined;code?:string}) {
  const identity=useSyncExternalStore(appSession.subscribe,appSession.snapshot).identity;
  const [own,setOwn]=useState<{key:string;self?:OwnSelf;error?:string}>();
  const key=`${identity?.activation??''}:${address}`;
  useEffect(()=>{
    if(!identity||!contract?.abi)return;
    let live=true;let operation:AppOperation;
    try{operation=appSession.captureOperation();}catch{return;}
    void operation.self().then(self=>{operation.check();if(live)setOwn({key,self});}).catch(()=>{if(live&&!operation.signal.aborted)setOwn({key,error:'Cannot verify your organizations. Reload the contract to try again.'});});
    return()=>{live=false;operation.dispose();};
  },[identity,key,contract?.abi]);
  let iface:Interface|undefined;
  try{if(contract?.abi)iface=new Interface(contract.abi as ConstructorParameters<typeof Interface>[0]);}catch{/* show explicit invalid metadata below */}
  const functions=iface?.fragments.filter((fragment):fragment is FunctionFragment=>fragment.type==='function' && ['view','pure'].includes((fragment as FunctionFragment).stateMutability))??[];
  const writes=iface?.fragments.filter((fragment):fragment is FunctionFragment=>fragment.type==='function' && !['view','pure'].includes((fragment as FunctionFragment).stateMutability))??[];
  const current=own?.key===key?own:undefined;
  return <div className="space-y-4"><AddressDetailsPanel rows={[{label:'Name',value:contract?.contractName??'Unavailable'},{label:'Compiler',value:contract?.compilerVersion??'Unavailable'},{label:'ABI source',value:contract?.abi?'Approved configuration':'Unavailable'}]}/>{contract?.sourceCode&&<div className="card p-4"><h3 className="font-medium mb-3">Source code</h3><pre className="code-block p-3 text-xs overflow-auto max-h-96 whitespace-pre-wrap">{contract.sourceCode}</pre></div>}<div className="card p-4"><h3 className="font-medium mb-3">Bytecode</h3>{code!==undefined?<><CopyButton text={code}/><pre className="code-block p-3 font-mono text-xs break-all whitespace-pre-wrap max-h-64 overflow-y-auto">{code}</pre></>:<p className="text-sm text-neutral-500">Unavailable</p>}</div><section className="space-y-3"><h3 className="text-lg font-medium">Read contract</h3>{!contract?.abi?<p className="text-sm text-neutral-500">No approved ABI is available for this contract.</p>:!iface?<p role="alert">The approved ABI could not be read.</p>:!functions.length?<p className="text-sm text-neutral-500">This ABI has no read functions.</p>:current?.error?<p role="alert">{current.error}</p>:!current?.self?<p role="status">Verifying your organizations…</p>:!current.self.organizations.length?<p className="text-sm text-neutral-500">Your current identity has no organization available for contract reads.</p>:functions.map(fragment=><ReadFunction key={`${key}:${fragment.format('sighash')}`} address={address} iface={iface!} fragment={fragment} self={current.self!}/>)}</section>{writes.length>0&&<section className="space-y-3"><h3 className="text-lg font-medium">Write contract</h3>{current?.self?(!current.self.organizations.length?<p className="text-sm text-neutral-500">Your current identity has no organization available for contract writes.</p>:writes.map(fragment=><NativeContractWrite key={`${key}:${fragment.format('sighash')}`} fragment={fragment} iface={iface!} address={address} self={current.self!}/>)):<p className="text-sm text-neutral-500">Verify your organizations before connecting a wallet.</p>}</section>}</div>;
}
