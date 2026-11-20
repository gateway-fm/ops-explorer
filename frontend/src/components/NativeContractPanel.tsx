import { NativeNames } from './NativeNames';
import { NativeContractWrite } from './NativeContractWrite';
import { parseNativeABIInput } from '../lib/nativeABI';
import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Interface, FunctionFragment, type ParamType } from 'ethers';
import { Loader2 } from 'lucide-react';
import { appSession, type AppOperation, type OwnSelf } from '../lib/v2Session';
import type { Contract } from '../lib/v2ReadTypes';
import { AddressDetailsPanel } from './AddressPresentation';
import { ContractTabs, ContractCodeBlock, ContractFunctionHeader, ContractVerificationStatus } from './ContractPresentation';
import { NativeAddress } from './NativeAddress';
import { exactQuantity } from '../lib/nativePresentation';

function OutputValue({param,value,label}: {param:ParamType;value:unknown;label:string}) {
  const sequence = Array.isArray(value) ? value : undefined;
  return <div className="border-l-2 border-success-200 pl-3 py-1"><dt className="text-sm font-medium">{label} <span className="font-mono text-xs text-neutral-500">({param.format('sighash')})</span></dt><dd className="font-mono text-sm break-all">{param.baseType==='array' && sequence && param.arrayChildren ? sequence.length ? <dl>{sequence.map((item,i)=><OutputValue key={i} label={`[${i}]`} param={param.arrayChildren!} value={item}/>)}</dl> : '[]' : param.baseType==='tuple' && sequence ? <dl>{param.components?.map((part,i)=><OutputValue key={i} label={part.name||`Field ${i+1}`} param={part} value={sequence[i]}/>)}</dl> : typeof value==='bigint' ? value.toString() : typeof value==='boolean' ? String(value) : value===null || value===undefined ? 'Unavailable' : String(value)}</dd></div>;
}
function ReadFunction({fragment,iface,address,self}: {fragment:FunctionFragment;iface:Interface;address:string;self:OwnSelf}) {
  const [open,setOpen]=useState(false),[inputs,setInputs]=useState<string[]>([]),[org,setOrg]=useState(self.organizations[0]?.id??''),[result,setResult]=useState<readonly unknown[]>(),[error,setError]=useState<string>(),[busy,setBusy]=useState(false);
  const scope=useRef<AppOperation|undefined>(undefined),ticket=useRef(0);
  const clear=()=>{++ticket.current;scope.current?.dispose();setResult(undefined);setError(undefined);setBusy(false);};
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
      operation.check();if(ticket.current===owned)setResult(decoded);
    } catch(e){if(ticket.current===owned&&!operation?.signal.aborted){try{operation?.check();setError(e instanceof Error?e.message:'Contract read failed.');}catch{/* current owner already retired */}}}
    finally{operation?.dispose();if(ticket.current===owned)setBusy(false);}
  };
  const signature=fragment.format('sighash');
  return <div className="border border-neutral-200 rounded-lg overflow-hidden"><ContractFunctionHeader kind="read" open={open} onToggle={()=>setOpen(!open)}>{signature}</ContractFunctionHeader>{open&&<form onSubmit={submit} className="p-4 space-y-4"><label className="block text-sm">Organization<select required className="input block w-full mt-1" value={org} onChange={e=>{clear();setOrg(e.target.value);}} disabled={busy}>{self.organizations.map(value=><option key={value.id} value={value.id}>{value.name||value.slug}</option>)}</select></label>{fragment.inputs.map((param,i)=><label key={i} className="block text-sm text-neutral-500">{param.name||`Input ${i+1}`} ({param.format('sighash')})<input required aria-label={`${signature} ${param.name||`input ${i+1}`}`} disabled={busy} value={inputs[i]??''} onChange={e=>{clear();setInputs(old=>{const next=[...old];next[i]=e.target.value;return next;});}} className="input block w-full mt-1" placeholder={param.baseType==='array'||param.baseType==='tuple'?'JSON array; quote integer values':param.type}/></label>)}<p className="text-xs text-neutral-500">Returns: {fragment.outputs.map(output=>output.format('full')).join(', ')||'no outputs'}. Uses your current OPS permissions.</p><button data-testid="contract-read-submit" className="btn-primary flex items-center gap-2 text-sm disabled:opacity-50" disabled={busy||!org}>{busy&&<Loader2 className="h-4 w-4 animate-spin"/>}{busy?'Reading…':'Query'}</button>{error&&<p role="alert" className="text-sm text-error-600">{error}</p>}{result!==undefined&&<dl data-testid="fn-result" className="p-3 bg-success-50 border border-success-200 rounded-lg space-y-2">{fragment.outputs.length?fragment.outputs.map((param,i)=><OutputValue key={i} label={param.name||`Output ${i+1}`} param={param} value={result[i]}/>):<p>Read completed with no outputs.</p>}</dl>}</form>}</div>;
}
export function NativeContractPanel({address,contract,code}: {address:string;contract:Contract|null|undefined;code?:string}) {
  const identity=useSyncExternalStore(appSession.subscribe,appSession.snapshot).identity;
  const [params,setParams]=useSearchParams(), selected=params.get('contractTab'), tab=selected==='read'||selected==='write'?selected:'overview';
  const [own,setOwn]=useState<{key:string;self?:OwnSelf;error?:string}>();
  const key=`${identity?.activation??''}:${address}:${JSON.stringify(contract?.abi)}`;
  useEffect(()=>{
    if(!identity||!contract?.abi)return;
    let live=true;let operation:AppOperation;
    try{operation=appSession.captureOperation();}catch{return;}
    void operation.self().then(self=>{operation.check();if(live)setOwn({key,self});}).catch(()=>{if(live&&!operation.signal.aborted){try{operation.check();setOwn({key,error:'Cannot verify your organizations. Reload the contract to try again.'});}catch{/* old owner */}}});
    return()=>{live=false;operation.dispose();};
  },[identity,key,contract?.abi]);
  let iface:Interface|undefined;
  try{if(contract?.abi)iface=new Interface(contract.abi as ConstructorParameters<typeof Interface>[0]);}catch{/* show invalid metadata */}
  const functions=iface?.fragments.filter((fragment):fragment is FunctionFragment=>fragment.type==='function' && ['view','pure'].includes((fragment as FunctionFragment).stateMutability))??[];
  const writes=iface?.fragments.filter((fragment):fragment is FunctionFragment=>fragment.type==='function' && !['view','pure'].includes((fragment as FunctionFragment).stateMutability))??[];
  const current=own?.key===key?own:undefined, abi=contract?.abi ? JSON.stringify(contract.abi,null,2) : undefined;
  const empty=!contract?.abi?'No approved ABI is available for this contract.':!iface?'The approved ABI could not be read.':current?.error;
  return <NativeNames items={contract?.creator?[{address:contract.creator}]:[]}><div className="space-y-4" data-testid="native-contract-panel"><ContractTabs hasABI={!!iface} active={tab} onChange={value=>{const query=new URLSearchParams(params);query.set('contractTab',value);setParams(query,{replace:true});}}/>
  {tab==='overview'&&<><AddressDetailsPanel rows={[
    {label:'Name',value:contract?.contractName??'Unavailable'}, {label:'Verification',value:<ContractVerificationStatus verified={contract?.isVerified} name={contract?.contractName}/>}, {label:'Compiler',value:contract?.compilerVersion??'Unavailable'}, {label:'EVM version',value:contract?.evmVersion??'Unavailable'}, {label:'License',value:contract?.licenseType??'Unavailable'}, {label:'Optimization',value:contract?.optimizationUsed===undefined?'Unavailable':contract.optimizationUsed?'Enabled':'Disabled'}, {label:'Optimization runs',value:contract?.optimizationRuns===undefined?'Unavailable':String(contract.optimizationRuns)}, {label:'Constructor arguments',value:contract?.constructorArgs??'Unavailable'}, {label:'ABI source',value:abi?'Approved configuration':'Unavailable'},
    ...(contract?.creator?[{label:'Creator',value:<NativeAddress address={contract.creator}/>}]:[]), ...(contract?.creationTx?[{label:'Creation transaction',value:<Link className="text-primary font-mono break-all" to={`/tx/${contract.creationTx}`}>{contract.creationTx}</Link>}]:[]), ...(contract?.blockNumber!==undefined?[{label:'Deployment block',value:<Link className="text-primary" to={`/block/${contract.blockNumber}`}>{exactQuantity(contract.blockNumber)}</Link>}]:[]), ...(contract?.createdAt?[{label:'Created',value:contract.createdAt}]:[]), ...(contract?.bytecodeHash?[{label:'Bytecode hash',value:<span className="font-mono break-all">{contract.bytecodeHash}</span>}]:[]),
  ]}/><ContractCodeBlock title="ABI" copyText={abi} rightMeta={contract?.abi?`${contract.abi.length} items`:undefined}>{abi?<pre className="max-h-80 overflow-y-auto whitespace-pre-wrap font-mono text-xs leading-relaxed text-neutral-700">{abi}</pre>:<p className="text-sm text-neutral-500">No approved ABI is available for this contract.</p>}</ContractCodeBlock><ContractCodeBlock title="Source code" copyText={contract?.sourceCode}>{contract?.sourceCode?<pre className="max-h-96 overflow-y-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-neutral-700">{contract.sourceCode}</pre>:<p className="text-sm text-neutral-500">Unavailable</p>}</ContractCodeBlock><ContractCodeBlock title="Deployed bytecode" copyText={code??contract?.bytecode}>{(code??contract?.bytecode)!==undefined?<pre className="max-h-80 overflow-y-auto break-all whitespace-pre-wrap font-mono text-xs leading-relaxed text-neutral-700">{code??contract?.bytecode}</pre>:<p className="text-sm text-neutral-500">Unavailable</p>}</ContractCodeBlock></>}

  {tab!=='overview'&&<section className="space-y-3 rounded-2xl border border-neutral-200 bg-neutral-50 p-4"><h3 className="text-lg font-medium">{tab==='read'?'Read contract':'Write contract'}</h3>{empty?<p className="text-sm text-neutral-500">{empty}</p>:!(tab==='read'?functions:writes).length?<p className="text-sm text-neutral-500">This ABI has no {tab} functions.</p>:!current?.self?<p role="status">Verifying your organizations…</p>:!current.self.organizations.length?<p className="text-sm text-neutral-500">Your current identity has no organization available for contract {tab}s.</p>:tab==='read'?functions.map(fragment=><ReadFunction key={`${key}:${fragment.format('sighash')}`} address={address} iface={iface!} fragment={fragment} self={current.self!}/>):writes.map(fragment=><NativeContractWrite key={`${key}:${fragment.format('sighash')}`} fragment={fragment} iface={iface!} address={address} self={current.self!}/>)}</section>}</div></NativeNames>;
}
