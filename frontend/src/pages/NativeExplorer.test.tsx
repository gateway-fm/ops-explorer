import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { ReactNode, Context } from 'react';
import type { AuthContextType } from '../lib/authContext';
import { V2App } from '../V2App';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AddressNative } from './AddressNative';
import { TransactionNative } from './TransactionNative';
import { NativeContinuation, useNativePage } from '../components/NativeReadPanels';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeTransactions } from '../lib/nativeReadModels';
import { exactUnits } from '../lib/nativePresentation';
const session = vi.hoisted(() => {
  let state: {identity:{activation:string}|null} = {identity:{activation:'first'}};
  const listeners = new Set<() => void>();
  return {
    snapshot: () => state,
    subscribe: (listener: () => void) => { listeners.add(listener); return () => listeners.delete(listener); },
    change: (value:string|null) => {state={identity:value?{activation:value}:null};listeners.forEach(listener=>listener());},
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
    render(<MemoryRouter initialEntries={[`/tx/${hash}`]}><Routes><Route path="/tx/:hash" element={<TransactionNative/>}/></Routes></MemoryRouter>);
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
    const {result}=renderHook(()=>useNativePage('/transactions',decodeTransactions));
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
    session.read.mockResolvedValue(suffix === 'balances' ? {items:[],page} : {address,contractName:'Approved registry contract'});
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

  it.each([['failed','Type unavailable'],['empty','EOA']])('keeps account classification honest when code is %s',async(mode,label)=>{
    session.read.mockImplementation(async(path:string)=>{
      if(path.endsWith('/stats'))return {address};
      if(path.endsWith('/balance'))return {address,balance:'0'};
      if(path.endsWith('/code')){if(mode==='failed')throw Error('Unavailable');return {address,code:'0x'};}
      return null;
    });
    render(<MemoryRouter initialEntries={[`/address/${address}`]}><Routes><Route path="/address/:address" element={<AddressNative/>}/></Routes></MemoryRouter>);
    expect(await screen.findByText(label)).toBeInTheDocument();
    if(mode==='failed')expect(screen.queryByText('EOA')).toBeNull();
    expect(screen.getAllByText('Unavailable').length).toBeGreaterThan(0);
  });

});
