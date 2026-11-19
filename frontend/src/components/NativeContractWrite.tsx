import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { FunctionFragment, Interface } from 'ethers';
import { appSession, type AppOperation, type OwnSelf } from '../lib/v2Session';
import { connectNativeWallet, type NativeWalletContext } from '../lib/nativeWalletHelper';
import { networkSetup } from '../pages/NetworkSetupV2';
import { getNetworkCurrency } from '../lib/utils';
import { parseNativeABIInput } from '../lib/nativeABI';

type Stage='idle'|'connecting'|'ready'|'checking'|'submitting'|'submitted'|'uncertain';
interface Owner { operation:AppOperation; wallet?:NativeWalletContext; generation:number; dispatched?:boolean; expiry?:ReturnType<typeof setTimeout> }
export function NativeContractWrite({fragment,iface,address,self}:{fragment:FunctionFragment;iface:Interface;address:string;self:OwnSelf}) {
  const [open,setOpen]=useState(false),[inputs,setInputs]=useState<string[]>([]),[value,setValue]=useState('0');
  const [organization,setOrganization]=useState(self.organizations[0]?.id??''),[helperOrigin,setHelperOrigin]=useState('');
  const [stage,setStage]=useState<Stage>('idle'),[account,setAccount]=useState<string>(),[error,setError]=useState<string>(),[hash,setHash]=useState<string>(),[receipt,setReceipt]=useState<string>();
  const [configuration,setConfiguration]=useState<{rpcURL:string;expiresAt:number}>(),[copied,setCopied]=useState(false);
  const generation=useRef(0),owner=useRef<Owner|undefined>(undefined),receiptOwner=useRef<AppOperation|undefined>(undefined);
  const release=()=>{const current=owner.current;owner.current=undefined;if(current?.expiry)clearTimeout(current.expiry);current?.wallet?.dispose();current?.operation.dispose();};
  useEffect(()=>()=>{++generation.current;release();receiptOwner.current?.dispose();},[]);
  const cancel=()=>{++generation.current;release();setConfiguration(undefined);setAccount(undefined);setCopied(false);setStage('idle');setError('Connection closed. Dismiss any remaining wallet prompts.');};
  const connect=async()=>{
    if(stage==='connecting'||stage==='checking'||stage==='submitting'||stage==='submitted'||stage==='uncertain')return;
    release();const owned=++generation.current;setConfiguration(undefined);setAccount(undefined);setCopied(false);setError(undefined);setStage('connecting');
    let operation:AppOperation|undefined;
    try {
      operation=appSession.captureOperation();const current:Owner={operation,generation:owned};owner.current=current;
      const network=await networkSetup();operation.check();if(generation.current!==owned)throw Error('Wallet connection changed.');
      const wallet=await connectNativeWallet({operation,helperOrigin,organizationId:organization,network,chainName:'OPS network',currency:getNetworkCurrency(),onNetworkConfiguration:config=>{
        operation!.check();if(generation.current!==owned||owner.current!==current)throw Error('Wallet connection changed.');
        setConfiguration(config);current.expiry=setTimeout(()=>{if(owner.current!==current)return;++generation.current;release();setConfiguration(undefined);setAccount(undefined);setStage(current.dispatched?'uncertain':'idle');setError(current.dispatched?'The original context expired while waiting for the wallet. Check wallet activity before starting another submission.':'The original wallet context expired. Refresh the helper token and reconnect.');},Math.max(0,Math.min(config.expiresAt,operation!.identity.expires)*1000-Date.now()));
      }});
      if(generation.current!==owned||owner.current!==current){wallet.dispose();return;}
      try{operation.check();}catch(e){wallet.dispose();throw e;}
      current.wallet=wallet;setAccount(wallet.account);setStage('ready');
    }catch(e){if(generation.current===owned){setStage('idle');setError(e instanceof Error?e.message:'The wallet could not connect.');}}
  };
  const submit=async(e:React.FormEvent)=>{
    e.preventDefault();const current=owner.current;if(stage!=='ready'||!current?.wallet)return;
    setError(undefined);let dispatched=false;
    try{
      current.operation.check();const args=fragment.inputs.map((param,i)=>parseNativeABIInput(inputs[i]??'',param));
      const data=iface.encodeFunctionData(fragment,args);
      if(!/^(0|[1-9][0-9]{0,77})$/.test(value)||BigInt(value)>=(1n<<256n))throw Error('Enter an exact nonnegative wei amount.');
      if(generation.current!==current.generation||owner.current!==current)throw Error('Wallet connection changed.');
      // The helper rechecks account, linked ownership, chain and original
      // credential immediately before invoking the ordinary wallet provider.
      setStage('checking');
      const tx=await current.wallet.send({to:address,data,...(fragment.stateMutability==='payable'?{value:'0x'+BigInt(value).toString(16)}:{})},()=>{
        current.operation.check();
        if(generation.current!==current.generation||owner.current!==current)throw Error('Wallet connection changed.');
        dispatched=true;current.dispatched=true;setStage('submitting');
      });
      current.operation.check();if(generation.current!==current.generation||owner.current!==current)return;
      setHash(tx);setStage('submitted');setConfiguration(undefined);release();
    }catch(e){if(owner.current===current&&generation.current===current.generation){setError(dispatched?'The wallet request ended without a confirmed outcome. Check wallet activity before starting another submission.':e instanceof Error?e.message:'Check the function inputs.');setStage(dispatched?'uncertain':'ready');if(dispatched){setConfiguration(undefined);release();}}}
  };
  const checkReceipt=async()=>{
    if(!hash)return;receiptOwner.current?.dispose();let operation:AppOperation|undefined;
    try{operation=appSession.captureOperation();receiptOwner.current=operation;const response=await operation.rpc(organization,{jsonrpc:'2.0',id:crypto.randomUUID(),method:'eth_getTransactionReceipt',params:[hash]});operation.check();if(receiptOwner.current!==operation)return;if(response.error)throw Error('Receipt unavailable.');const item=response.result as {status?:unknown}|null;setReceipt(item===null?'Pending / unavailable':item&&item.status==='0x1'?'Confirmed success':item&&item.status==='0x0'?'Confirmed failure':'Status unavailable');}
    catch{if(!operation?.signal.aborted)setReceipt('Receipt unavailable. Open the transaction or check wallet activity.');}finally{operation?.dispose();}
  };
  const signature=fragment.format('sighash');const locked=stage==='connecting'||stage==='checking'||stage==='submitting'||stage==='submitted'||stage==='uncertain';
  return <div className="border border-neutral-200 rounded-lg overflow-hidden"><button data-testid="contract-write-fn" aria-expanded={open} className="w-full px-4 py-3 flex items-center justify-between bg-neutral-50 hover:bg-neutral-100 text-left" onClick={()=>setOpen(!open)}><span className="font-medium break-all">{signature}{fragment.stateMutability==='payable'&&<span className="ml-2 text-xs text-warning-700">Payable</span>}</span><span aria-hidden="true">⌄</span></button>{open&&<div className="p-4 space-y-4"><p className="text-sm text-neutral-500">Use your linked browser wallet through the local OPS wallet helper. Its token file must contain your own short-lived session for this organization and network.</p><div className="grid sm:grid-cols-2 gap-4"><label className="text-sm">Organization<select aria-label={`${signature} organization`} disabled={locked||stage==='ready'} className="input block w-full mt-1" value={organization} onChange={e=>{release();setConfiguration(undefined);setOrganization(e.target.value);}}>{self.organizations.map(org=><option key={org.id} value={org.id}>{org.name||org.slug}</option>)}</select></label><label className="text-sm">Local helper origin<input aria-label={`${signature} helper origin`} placeholder="http://localhost:PORT" autoComplete="off" disabled={locked||stage==='ready'} value={helperOrigin} onChange={e=>{release();setConfiguration(undefined);setHelperOrigin(e.target.value);}} className="input block w-full mt-1"/></label></div><div className="flex flex-wrap items-center gap-3">{stage!=='submitted'&&stage!=='uncertain'&&<button type="button" className="btn-secondary text-sm" disabled={stage==='connecting'||stage==='checking'||stage==='submitting'||!organization||!helperOrigin} onClick={()=>void connect()}>{stage==='connecting'?'Connecting…':stage==='ready'?'Reconnect wallet':'Connect wallet'}</button>}{(stage==='connecting'||stage==='ready'||stage==='checking'||stage==='idle'&&configuration)&&<button type="button" className="text-sm text-neutral-500 underline" onClick={cancel}>Close connection</button>}{account&&<span className="font-mono text-xs break-all">{account}</span>}</div>
    {configuration&&<details className="rounded-lg border border-neutral-200 p-3"><summary className="cursor-pointer text-sm text-primary">Manual wallet network setup</summary><p className="text-xs text-neutral-500 mt-2">If your wallet kept an older RPC, copy this private, temporary URL into that network’s RPC settings, then reconnect. Keep it private. It stops working at the original session expiry.</p><input readOnly type="password" aria-label="Current private wallet RPC URL" autoComplete="off" value={configuration.rpcURL} className="input w-full mt-2"/><button type="button" className="text-sm text-primary mt-2" onClick={()=>{try{const current=owner.current;current?.operation.check();if(!current||configuration.expiresAt<=Date.now()/1000)throw Error('Expired');void navigator.clipboard.writeText(configuration.rpcURL).then(()=>{current.operation.check();if(owner.current===current)setCopied(true);}).catch(()=>{if(owner.current===current)setError('Copy failed. Check browser clipboard permission.');});}catch{cancel();}}}>{copied?'Copied':'Copy private RPC URL'}</button></details>}
    <form onSubmit={submit} className="space-y-4">{fragment.inputs.map((param,i)=><label key={i} className="block text-sm text-neutral-500">{param.name||`Input ${i+1}`} ({param.format('sighash')})<input required aria-label={`${signature} ${param.name||`input ${i+1}`}`} disabled={locked} value={inputs[i]??''} onChange={e=>setInputs(old=>{const next=[...old];next[i]=e.target.value;return next;})} className="input w-full block mt-1" placeholder={param.baseType==='array'||param.baseType==='tuple'?'JSON array; quote integer values':param.type}/></label>)}{fragment.stateMutability==='payable'&&<label className="block text-sm">Value (wei)<input required aria-label={`${signature} value in wei`} disabled={locked} value={value} onChange={e=>setValue(e.target.value)} pattern="0|[1-9][0-9]*" className="input block w-full mt-1"/></label>}<button data-testid="contract-write-submit" className="btn-primary disabled:opacity-50" disabled={stage!=='ready'}>{stage==='checking'?'Checking wallet…':stage==='submitting'?'Waiting for wallet…':'Write'}</button></form>{stage==='checking'&&<p role="status" className="text-sm text-neutral-500">Verifying the wallet account, linked ownership, network and original context before submission.</p>}{stage==='submitting'&&<p role="status" className="text-sm text-neutral-500">Review the transaction in your wallet. Reject it there to stop. Closing or switching this page cannot undo a wallet submission.</p>}{error&&<p role="alert" className="text-sm text-error-600">{error}</p>}{hash&&<div className="rounded-lg bg-primary-50 p-3 space-y-2"><p className="text-sm">Submitted: <Link className="font-mono text-primary break-all" to={`/tx/${hash}`}>{hash}</Link></p><button className="text-sm text-primary underline" onClick={()=>void checkReceipt()}>Check receipt</button>{receipt&&<p className="text-sm">{receipt}</p>}</div>}</div>}</div>;
}
