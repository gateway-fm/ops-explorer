import { useState } from 'react';
import { Clock } from 'lucide-react';
import { HomeStatCard } from './HomePresentation';
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip';
import { blockTimeSampleNote, type BlockTimeSample } from '../lib/nativeBlockTime';

export function NativeBlockTimeCard({sample}:{sample:BlockTimeSample}) {
  const [open,setOpen]=useState(false),text=sample.seconds===undefined?'Unavailable':sample.seconds+'s',note=blockTimeSampleNote(sample);
  return <HomeStatCard label="Avg Block Time" color="amber" icon={<Clock className="w-5 h-5"/>}
    note={<span data-testid="block-time-sample">{sample.range?`${sample.pairCount} usable interval${sample.pairCount===1?'':'s'} · page blocks ${sample.range.fromBlock}–${sample.range.throughBlock}`:'0 usable intervals · page range unavailable'}</span>}
    value={<Tooltip open={open} onOpenChange={setOpen} delayDuration={0}>
      <TooltipTrigger asChild><button type="button" className="break-all cursor-help underline decoration-dotted decoration-neutral-300 underline-offset-4 rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary" aria-label={text+'. '+note} title={text+'. '+note} onClick={event=>{event.preventDefault();setOpen(true);}}>{text}</button></TooltipTrigger>
      <TooltipContent className="max-w-[min(22rem,calc(100vw-2rem))] break-words font-sans font-normal" side="bottom">{note}</TooltipContent>
    </Tooltip>}/>
}
