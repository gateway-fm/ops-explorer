import { useState,type ReactNode } from 'react';
import { Tooltip,TooltipContent,TooltipTrigger } from './ui/tooltip';

// Shared table/card vocabulary from the original Explorer pages. Controllers
// supply cells so native quantities never pass through the legacy number model.
export function ExplorerTable({ headings, children }: {headings: (string|{label:string;className?:string})[]; children: ReactNode}) {
  return <div className="overflow-x-auto"><table className="table"><thead><tr>{headings.map(heading => {const {label,className}=typeof heading==='string'?{label:heading,className:undefined}:heading;return <th key={label} className={className}>{label}</th>;})}</tr></thead><tbody>{children}</tbody></table></div>;
}
export function ExplorerTabs({tabs, active, onChange}: {tabs: {id:string;label:string;count?:string;countNote?:string}[];active:string;onChange:(id:string)=>void}) {
  const [openNote,setOpenNote]=useState<string>();
  return <div className="flex flex-wrap items-center gap-1 border-b border-neutral-200" role="tablist">{tabs.map(tab=>{
    // Selection runs first. Prevent only Radix’s composed click dismissal
    // for a count note, so click/Enter/Space keep the explanation accessible.
    const control=<button key={tab.id} type="button" role="tab" aria-selected={active===tab.id} data-testid={`tab-${tab.id}`} onClick={event=>{onChange(tab.id);if(tab.countNote){event.preventDefault();setOpenNote(tab.id);}}} className={`relative -mb-px px-4 py-3 text-sm font-medium transition-colors ${active===tab.id?'text-primary border-b-2 border-primary':'text-neutral-500 hover:text-neutral-800'}`}>{tab.label}{tab.count!==undefined&&<span className="ml-2 rounded-full bg-neutral-200/60 px-1.5 py-0.5 text-xs">{tab.count}</span>}</button>;
    return tab.countNote?<Tooltip key={tab.id} open={openNote===tab.id} onOpenChange={open=>setOpenNote(open?tab.id:undefined)} delayDuration={0}><TooltipTrigger asChild>{control}</TooltipTrigger><TooltipContent className="max-w-[min(22rem,calc(100vw-2rem))] break-words font-sans font-normal" side="bottom">{tab.countNote}</TooltipContent></Tooltip>:control;
  })}</div>;
}

export function VisiblePageNote() { return <p className="text-xs text-neutral-500">Records reflect your current access. Unavailable counts are not zero; this page does not describe the whole network.</p>; }
