const standards = [{label:'All',value:''},{label:'ERC-20',value:'ERC20'},{label:'ERC-721',value:'ERC721'}];

// Original V1 control, shared with native server-owned filtering.
export function TransferTypeTabs({active,onChange,includeERC1155=false}: {active:string;onChange:(value:string)=>void;includeERC1155?:boolean}) {
  return <div className="flex items-center gap-1 border-b border-neutral-200" role="tablist" aria-label="Transfer token standard">{[...standards,...(includeERC1155?[{label:'ERC-1155',value:'ERC1155'}]:[])].map(item=><button key={item.value||'all'} role="tab" aria-selected={active===item.value} onClick={()=>onChange(item.value)} className={`relative -mb-px px-4 py-2.5 text-sm font-medium transition-colors ${active===item.value?'text-primary':'text-neutral-500 hover:text-neutral-800'}`}>{item.label}{active===item.value&&<span className="absolute inset-x-3 -bottom-px h-0.5 rounded-full bg-primary"/>}</button>)}</div>;
}
