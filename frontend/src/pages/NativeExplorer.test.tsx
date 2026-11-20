import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { ReactNode, Context } from 'react';
import type { AuthContextType } from '../lib/authContext';
import { V2App } from '../V2App';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '../components/ui/tooltip';
import { AddressNative } from './AddressNative';
import { TransactionNative } from './TransactionNative';
import { NativeContinuation, useNativePage } from '../components/NativeReadPanels';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeTransactions } from '../lib/nativeReadModels';
import { exactUnits } from '../lib/nativePresentation';
const session = vi.hoisted(() => {
  let state: {identity:{activation:string;actor:string;profile:string;expires:number}|null} = {identity:{activation:'first',actor:'ops:test',profile:'azure',expires:9999999999}};
  const listeners = new Set<() => void>();
  return {
    snapshot: () => state,
    subscribe: (listener: () => void) => { listeners.add(listener); return () => listeners.delete(listener); },
    change: (value:string|null) => {state={identity:value?{activation:value,actor:'ops:test',profile:'azure',expires:9999999999}:null};listeners.forEach(listener=>listener());},
    captureOperation: vi.fn(() => {const original=state.identity;const controller=new AbortController();return {signal:controller.signal,check:()=>{if(!state.identity||state.identity!==original)throw Error('App context changed');},dispose:()=>controller.abort(),request:vi.fn(()=>{throw Error('Wallet operation cannot perform reads');})};}),
    read: vi.fn(),
  };
});
vi.mock('../lib/v2Session',()=>({appSession:session,isV2:()=>true,ReadHTTPError:class extends Error {code='unavailable';}}));
vi.mock('../lib/authV2',()=>({V2AuthProvider:({children,context:Context}:{children:ReactNode;context:Context<AuthContextType|undefined>})=><Context.Provider value={{isAuthenticated:true,isLoading:false,auth:{authenticated:true,did:'ops:test',expiresAt:9999999999},logout:async()=>{},checkStatus:async()=>{}}}>{children}</Context.Provider>}));
const hash=`0x${'a'.repeat(64)}`,address=`0x${'b'.repeat(40)}`,masked=`0x${'0'.repeat(40)}`;
const page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS'};
const tx={hash,from:address,to:masked,blockNumber:'9007199254740993123',blockTimestamp:'1700000000',value:null,gasPrice:'1',gasUsed:'21000',inputData:'0x',status:null,txIndex:null,txType:2};
beforeEach(()=>{vi.stubGlobal('localStorage',{getItem:()=>null,setItem:()=>{},removeItem:()=>{}});session.change('first');session.read.mockReset();session.captureOperation.mockClear();});
describe('native Explorer original presentation',()=>{
  it('does not invent success, zero values or links for masked addresses',async()=>{
    session.read.mockImplementation(async(path:string)=>path.includes('/transfers')?{items:[],page}:tx);
    render(<TooltipProvider><MemoryRouter initialEntries={[`/tx/${hash}`]}><Routes><Route path="/tx/:hash" element={<TransactionNative/>}/></Routes></MemoryRouter></TooltipProvider>);
    expect(await screen.findByText('Transaction Details')).toBeInTheDocument();
    expect(screen.getByTestId('tx-status')).toHaveTextContent('Pending / unknown');
    expect(screen.getByText('9,007,199,254,740,993,123')).toHaveAttribute('href','/block/9007199254740993123');
    expect(screen.getByTestId('private-address')).toHaveTextContent('Private');
    expect(document.querySelector(`a[href*="${masked}"]`)).toBeNull();
    expect(screen.getAllByText('Hidden').length).toBeGreaterThan(0);
    expect(session.read.mock.calls.every(([path])=>String(path).startsWith('/v2/explorer/'))).toBe(true);
  });
  it('keeps large quantities and decimal placement exact',()=>{
    expect(exactUnits('900719925474099312345678901234567890')).toBe('900,719,925,474,099,312.34567890123456789');
    expect(exactUnits(null)).toBe('Hidden');expect(exactUnits(undefined)).toBe('Unavailable');
  });
  it('continues an empty scan-limited page using its opaque cursor and resets on activation change',async()=>{
    const next='opaque+/=cursor';session.read.mockResolvedValue({items:[],page:{...page,hasMore:null,scanLimited:true,next}});
    const {result}=renderHook(()=>useNativePage('/transactions',decodeTransactions),{wrapper:({children})=><MemoryRouter>{children}</MemoryRouter>});
    await waitFor(()=>expect(result.current.data?.page.next).toBe(next));
    render(<NativeContinuation {...result.current} page={result.current.data?.page}/>);
    fireEvent.click(screen.getByRole('button',{name:'Continue'}));
    await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toContain('cursor=opaque%2B%2F%3Dcursor'));
    act(()=>session.change('second'));
    await waitFor(()=>expect(result.current.index).toBe(0));expect(session.read.mock.calls.at(-1)?.[0]).not.toContain('cursor=');
    const prior=session.read.mock.calls.length;act(()=>result.current.reset());await waitFor(()=>expect(session.read.mock.calls.length).toBeGreaterThan(prior));
  });
  it('rejects old-actor data when its response finishes after switching accounts',async()=>{
    let finish!:(data:unknown)=>void;
    session.read.mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;})).mockResolvedValue({name:'new'});
    const decode=(raw:unknown)=>raw as {name:string};
    const {result}=renderHook(()=>useNativeRead('/v2/explorer/example',decode));
    act(()=>session.change('second'));await waitFor(()=>expect(result.current.data?.name).toBe('new'));
    await act(async()=>{finish({name:'old'});await Promise.resolve();});expect(result.current.data?.name).toBe('new');
  });
  it.each(['contract','balances'])('opens the named %s address tab on a direct deep link',async(suffix)=>{
    session.read.mockImplementation(async (path:string) => path.includes('/balances') ? {items:[],page} : path.endsWith('/stats') ? {address,isContract:true} : path.endsWith('/code') ? {address,code:'0x6000'} : path.endsWith('/contract') ? {address,contractName:'Approved registry contract'} : null);
    window.history.pushState({},'',`/addresses/${address}/${suffix}`);
    render(<V2App/>);
    if(suffix==='balances')expect(await screen.findByText('No token balances on this visible page.')).toBeInTheDocument();
    else expect((await screen.findAllByText('Approved registry contract')).length).toBeGreaterThan(0);
    expect(document.querySelector(`[data-testid="tab-${suffix==='balances'?'tokens':'contract'}"]`)).toHaveAttribute('aria-selected','true');
    expect(session.read.mock.calls.some(([path])=>path===`/v2/explorer/addresses/${address}/${suffix}${suffix === 'balances' ? '?limit=25' : ''}`)).toBe(true);
  });

  it('clears private search and open account controls when a new activation is adopted in one render',async()=>{
    session.read.mockResolvedValue({items:[],page});
    window.history.pushState({},'','/');render(<V2App/>);
    const input=screen.getByPlaceholderText(/Search .* by address/);
    fireEvent.change(input,{target:{value:'private-bank-record'}});
    await waitFor(()=>expect(session.read.mock.calls.some(([path])=>String(path).includes('q=private-bank-record'))).toBe(true));
    fireEvent.click(screen.getByRole('button',{name:'Account menu'}));
    expect(screen.getByText('Refresh identity')).toBeInTheDocument();
    const before=session.read.mock.calls.length;
    act(()=>session.change('second'));
    expect(screen.getByPlaceholderText(/Search .* by address/)).toHaveValue('');
    expect(screen.queryByText('Refresh identity')).toBeNull();
    await act(async()=>{await new Promise(resolve=>setTimeout(resolve,180));});
    expect(session.read.mock.calls.slice(before).some(([path])=>String(path).includes('private-bank-record'))).toBe(false);
  });

  it('keeps mobile suggestions dismissed through late completion and shows current search errors',async()=>{
    let finish!:(value:unknown)=>void;
    session.read.mockImplementation((path:string)=>String(path).includes('/search/suggestions')?new Promise(resolve=>{finish=resolve;}):Promise.resolve({items:[],page}));
    window.history.pushState({},'','/blocks');render(<V2App/>);fireEvent.click(screen.getByRole('button',{name:'Toggle menu'}));
    const input=screen.getByRole('textbox',{name:'Search the explorer'});fireEvent.change(input,{target:{value:'bank'}});
    await waitFor(()=>expect(session.read.mock.calls.some(([path])=>String(path).includes('q=bank'))).toBe(true));fireEvent.keyDown(input,{key:'Escape'});
    await act(async()=>finish({items:[{type:'address',value:address}],page}));expect(screen.queryByRole('button',{name:new RegExp(address)})).toBeNull();
    fireEvent.focus(input);expect(screen.getByRole('button',{name:new RegExp(address)})).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByTestId('page-header-title'));expect(screen.queryByRole('button',{name:new RegExp(address)})).toBeNull();
    fireEvent.focus(input);fireEvent.keyDown(input,{key:'ArrowDown'});fireEvent.keyDown(input,{key:'Enter'});fireEvent.submit(input.closest('form')!);
    await waitFor(()=>expect(window.location.pathname).toBe('/address/'+address));
  });

  it('displays mobile failed and empty search feedback with working clear and Escape',async()=>{
    session.read.mockImplementation(async(path:string)=>{if(String(path).includes('q=failed'))throw Error('failed');return {items:[],page};});
    window.history.pushState({},'','/blocks');render(<V2App/>);fireEvent.click(screen.getByRole('button',{name:'Toggle menu'}));const input=screen.getByRole('textbox',{name:'Search the explorer'});
    fireEvent.change(input,{target:{value:'failed'}});expect(await screen.findByRole('alert')).toHaveTextContent('read could not be completed');
    fireEvent.keyDown(input,{key:'Escape'});expect(screen.queryByRole('alert')).toBeNull();
    fireEvent.change(input,{target:{value:'empty'}});expect(await screen.findByText('No visible results. Try an address, transaction hash or block number.')).toBeInTheDocument();
    fireEvent.change(input,{target:{value:''}});expect(screen.queryByText(/No visible results/)).toBeNull();
  });

  it.each([['/verify','Contract verification unavailable'],['/gas-tracker','Gas tracker unavailable'],['/stats','Global charts unavailable'],[`/nft/${address}/7`,'NFT item metadata unavailable'],['/privacy','Use Auditor for disclosure cases'],['/grant/legacy/alias','Legacy disclosure link unavailable'],['/view-as','View as user unavailable']])('explains the supported recovery for %s without legacy reads',async(path,title)=>{
    session.read.mockResolvedValue({items:[],page});window.history.pushState({},'',path);render(<V2App/>);
    expect(await screen.findByTestId('page-header-title')).toHaveTextContent(title);expect(screen.getByTestId('restricted-state')).toBeInTheDocument();expect(session.read).not.toHaveBeenCalled();
  });

  it.each([['failed','Type unavailable'],['empty','EOA']])('keeps account classification honest when code is %s',async(mode,label)=>{
    session.read.mockImplementation(async(path:string)=>{
      if(path.endsWith('/stats'))return {address};
      if(path.endsWith('/balance'))return {address,balance:'0'};
      if(path.endsWith('/code')){if(mode==='failed')throw Error('Unavailable');return {address,code:'0x'};}
      return null;
    });
    render(<TooltipProvider><MemoryRouter initialEntries={[`/address/${address}`]}><Routes><Route path="/address/:address" element={<AddressNative/>}/></Routes></MemoryRouter></TooltipProvider>);
    expect(await screen.findByText(label)).toBeInTheDocument();
    if(mode==='failed')expect(screen.queryByText('EOA')).toBeNull();
    const row=[...document.querySelectorAll('dt')].find(node=>node.textContent==='Transactions')!.parentElement!;
    expect(row).toHaveTextContent('—');
    expect(row.querySelector('[title]')).toHaveAttribute('title',expect.stringContaining('Count unavailable'));
  });
  it('retains an authorized current balance when stats are unindexed and explains unavailable coin history',async()=>{
    session.read.mockImplementation(async(path:string)=>path.endsWith('/balance')?{address,balance:'100000000000000001',blockNumber:'9007199254740993'}:path.endsWith('/code')?{address,code:'0x'}:null);
    render(<TooltipProvider><MemoryRouter initialEntries={[`/address/${address}?tab=coin-history`]}><Routes><Route path="/address/:address" element={<AddressNative/>}/></Routes></MemoryRouter></TooltipProvider>);
    expect(await screen.findByText(/0.100000000000000001 ETH/)).toBeInTheDocument();expect(screen.getByText(/Per-block coin balance history isn't indexed/)).toBeInTheDocument();
    expect(screen.getByRole('link',{name:'9,007,199,254,740,993'})).toHaveAttribute('href','/block/9007199254740993');expect(session.read.mock.calls.every(([path])=>!String(path).includes('history'))).toBe(true);
  });
  it('shows a failed stats read as an error with retry instead of a successful empty address frame',async()=>{
    session.read.mockImplementation(async(path:string)=>{if(path.endsWith('/stats'))throw Error('Denied');return null;});
    render(<TooltipProvider><MemoryRouter initialEntries={[`/address/${address}`]}><Routes><Route path="/address/:address" element={<AddressNative/>}/></Routes></MemoryRouter></TooltipProvider>);
    expect(await screen.findByTestId('app-error')).toBeInTheDocument();expect(screen.getByRole('button',{name:'Try again'})).toBeInTheDocument();expect(screen.queryByTestId('tab-details')).toBeNull();
  });

});
