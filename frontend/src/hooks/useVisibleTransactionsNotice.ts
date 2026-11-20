import { useEffect, useState, useSyncExternalStore } from 'react';
import { appSession, type AppOperation } from '../lib/v2Session';
import { decodeTransactions } from '../lib/nativeReadModels';
import type { Page, Transaction } from '../lib/v2ReadTypes';
export type VisibleChange = {changed:true;count?:number};
export function compareVisibleTransactions(displayed:Page<Transaction>, latest:Page<Transaction>):VisibleChange|undefined {
  if (JSON.stringify(displayed.items)===JSON.stringify(latest.items)) return;
  const anchor=displayed.items[0]?.hash, index=anchor?latest.items.findIndex(tx=>tx.hash===anchor):-1;
  // Without a complete ordered overlap, describe only a change. Rights and
  // reorganization changes cannot be counted as new authorized population.
  if (index<=0||displayed.page.scanLimited||latest.page.scanLimited||displayed.page.consistency!==latest.page.consistency||displayed.page.population!==latest.page.population) return {changed:true};
  const overlap=latest.items.slice(index);
  if (!overlap.length||overlap.length>displayed.items.length||overlap.some((tx,i)=>JSON.stringify(tx)!==JSON.stringify(displayed.items[i]))) return {changed:true};
  return {changed:true,count:index};
}
export function useVisibleTransactionsNotice(endpoint:string|null, baseline:Page<Transaction>|undefined, firstPage:boolean) {
  const identity=useSyncExternalStore(appSession.subscribe,appSession.snapshot).identity;
  const key=`${identity?.actor}:${identity?.profile}:${identity?.activation}:${endpoint}:${JSON.stringify(baseline)}`;
  const [notice,setNotice]=useState<{key:string;value?:VisibleChange}>();
  useEffect(()=>{
    if(!identity||!endpoint||!baseline||!firstPage)return;
    let live=true,timer:ReturnType<typeof setTimeout>|undefined,operation:AppOperation|undefined;
    const schedule=()=>{if(live&&document.visibilityState==='visible')timer=setTimeout(()=>void poll(),3000);};
    const poll=async()=>{
      if(!live||document.visibilityState!=='visible')return;
      try{
        operation=appSession.captureOperation();const owned=operation;
        const raw=await appSession.read<unknown>(endpoint,owned.signal);owned.check();
        const latest=decodeTransactions(raw);owned.check();
        if(live&&document.visibilityState==='visible')setNotice({key,value:compareVisibleTransactions(baseline,latest)});
      }catch{/* Background failures leave the displayed authorized page intact. */}
      finally{operation?.dispose();operation=undefined;schedule();}
    };
    const visibility=()=>{if(timer)clearTimeout(timer);timer=undefined;if(document.visibilityState!=='visible'){operation?.dispose();}else if(!operation)schedule();};
    document.addEventListener('visibilitychange',visibility);schedule();
    return()=>{live=false;if(timer)clearTimeout(timer);operation?.dispose();document.removeEventListener('visibilitychange',visibility);};
  },[identity,key,endpoint,firstPage,baseline]);
  return {notice:firstPage&&notice?.key===key?notice.value:undefined,clear:()=>setNotice(undefined)};
}
