import type { ReactNode } from 'react';
import { BookOpen, ChevronDown, ChevronUp, FileCode2, Loader2, PenLine, Wallet, X } from 'lucide-react';
import { CopyButton } from './CopyButton';
export type ContractTab = 'overview' | 'read' | 'write';
export function ContractVerificationStatus({verified,name}: {verified?:boolean;name?:string}) {
  return verified === true ? <span className="inline-flex items-center gap-1.5 text-sm text-success-600"><span className="h-1.5 w-1.5 rounded-full bg-success-500"/>Verified{name ? ` (${name})` : ''}</span> : <span className="text-sm text-neutral-500">{verified === false ? 'Not verified' : 'Unavailable'}</span>;
}
export function ContractTabs({active,onChange,hasABI=true,unavailable='No approved ABI is available'}: {active:ContractTab;onChange:(tab:ContractTab)=>void;hasABI?:boolean;unavailable?:string}) {
  const tabs = [{id:'overview' as const,label:'Overview',icon:FileCode2},{id:'read' as const,label:'Read',icon:BookOpen},{id:'write' as const,label:'Write',icon:PenLine}];
  return <div className="inline-flex items-center gap-1 rounded-full border border-neutral-200 bg-neutral-100/60 p-1" role="tablist" aria-label="Contract sections">{tabs.map(({id,label,icon:Icon})=>{
    const disabled=id!=='overview'&&!hasABI;
    return <button key={id} role="tab" data-testid={`tab-${id}`} aria-selected={active===id} disabled={disabled} title={disabled?unavailable:undefined} onClick={()=>onChange(id)} className={`inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors ${disabled?'cursor-not-allowed text-neutral-300':active===id?'bg-neutral-50 text-neutral-900 shadow-card':'text-neutral-500 hover:text-neutral-800'}`}><Icon className="h-3.5 w-3.5"/><span>{label}</span></button>;
  })}</div>;
}
export function ContractFunctionHeader({open,onToggle,kind,children}: {open:boolean;onToggle:()=>void;kind:'read'|'write';children:ReactNode}) {
  return <button type="button" data-testid={`contract-${kind}-fn`} aria-expanded={open} onClick={onToggle} className="w-full px-4 py-3 flex items-center justify-between bg-neutral-50 hover:bg-neutral-100 transition-colors text-left"><span className="font-medium text-neutral-900 break-all">{children}</span>{open?<ChevronUp className="w-4 h-4 shrink-0 text-neutral-400"/>:<ChevronDown className="w-4 h-4 shrink-0 text-neutral-400"/>}</button>;
}
export function WalletConnectionStatus({account,connecting,connect,close,disabled,children}: {account?:string;connecting:boolean;connect:()=>void;close?:()=>void;disabled:boolean;children?:ReactNode}) {
  return account ? <div data-testid="wallet-status" className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-success-200 bg-success-50 px-3 py-2 text-sm dark:border-success-500/30 dark:bg-success-500/10"><div className="flex min-w-0 items-center gap-2"><Wallet className="h-4 w-4 shrink-0 text-success-600"/><span className="text-success-700 dark:text-success-500">Connected:</span><span className="truncate font-mono text-xs text-success-800 dark:text-success-500">{account}</span></div><div className="flex items-center gap-2"><button type="button" className="text-xs text-primary" disabled={disabled} onClick={connect}>Reconnect wallet</button>{close&&<button type="button" onClick={close} className="inline-flex shrink-0 items-center gap-1 rounded px-2 py-1 text-xs text-neutral-500 transition-colors hover:bg-error-50 hover:text-error-600"><X className="h-3.5 w-3.5"/>Close connection</button>}</div></div> : <div className="flex flex-col items-center gap-4 py-8 text-center"><div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10"><Wallet className="h-5 w-5 text-primary"/></div><h3 className="text-base font-semibold text-neutral-900">Connect a wallet to write</h3>{children}<button type="button" onClick={connect} disabled={disabled} data-testid="wallet-connect" className="inline-flex items-center gap-2 rounded-full bg-primary px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:opacity-50">{connecting?<Loader2 className="h-4 w-4 animate-spin"/>:<Wallet className="h-4 w-4"/>}{connecting?'Connecting…':'Connect wallet'}</button>{close&&<button type="button" onClick={close} className="text-xs text-neutral-400 hover:text-neutral-600">Close connection</button>}</div>;
}
export function ContractCodeBlock({
  title,
  rightMeta,
  copyText,
  children,
}: {
  title: string;
  rightMeta?: string;
  copyText?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-neutral-200 px-5 py-3">
        <h3 className="text-sm font-semibold text-neutral-900">{title}</h3>
        <div className="flex items-center gap-3">
          {rightMeta && <span className="font-mono text-xs tabular-nums text-neutral-500">{rightMeta}</span>}
          {copyText && <CopyButton text={copyText} />}
        </div>
      </header>
      <div className="bg-neutral-100/60 px-5 py-4">{children}</div>
    </section>
  );
}

