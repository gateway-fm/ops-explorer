import { scopedCountNote, scopedCountText, type ScopedCount } from '../lib/nativeCounts';
import { useState } from 'react';
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip';

export function ScopedCountValue({count,description}:{count?:ScopedCount;description?:string}) {
  const [open,setOpen]=useState(false);
  const text=scopedCountText(count),note=description??scopedCountNote(count);
  return <Tooltip open={open} onOpenChange={setOpen} delayDuration={0}>
    <TooltipTrigger asChild><button type="button" className="break-all tabular-nums cursor-help underline decoration-dotted decoration-neutral-300 underline-offset-4 rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary" title={text+'. '+note} aria-label={text+'. '+note} onClick={event=>{event.preventDefault();setOpen(true);}}>{text}</button></TooltipTrigger>
    <TooltipContent className="max-w-[min(22rem,calc(100vw-2rem))] break-words font-sans font-normal" side="bottom">{note}</TooltipContent>
  </Tooltip>;
}
