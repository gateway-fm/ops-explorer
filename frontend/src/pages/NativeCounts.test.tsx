import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { TooltipProvider } from '../components/ui/tooltip';
import { HomeNative } from './HomeNative';
import { TokensNative, TokenNative } from './TokensNative';
import { AccountsNative } from './CollectionsNative';
import { AddressNative } from './AddressNative';
const mock=vi.hoisted(()=>({state:{identity:{actor:'ops:p1:'+'A'.repeat(43),profile:'fixture',activation:'fixture',expires:2000000000}},read:vi.fn(),listeners:new Set<()=>void>()}));
vi.mock('../lib/v2Session',()=>({isV2:()=>true,appSession:{snapshot:()=>mock.state,subscribe:(fn:()=>void)=>{mock.listeners.add(fn);return()=>mock.listeners.delete(fn);},read:mock.read,captureOperation:()=>{const owner=mock.state.identity,controller=new AbortController();return{identity:owner,signal:controller.signal,check:()=>{if(owner!==mock.state.identity||controller.signal.aborted)throw Error('changed');},dispose:()=>controller.abort()};}},ReadHTTPError:class extends Error{code='unavailable';}}));
const address='0x'+'a'.repeat(40),headHash='0x'+'b'.repeat(64),observedAt='2026-11-20T11:12:13.123456789Z';
const current={status:'exact',value:'0',population:'ORDINARY_VISIBLE_ROWS',consistency:'CURRENT',coverage:'current',observedAt} as const;
const limited={status:'at_least',lowerBound:'9007199254740993',population:'ORDINARY_VISIBLE_ROWS',consistency:'SNAPSHOT',coverage:'scan_limited',observedAt,range:{fromBlock:'100',throughBlock:'123',headHash}} as const;
const unavailable={status:'unavailable',population:'ORDINARY_VISIBLE_ROWS',consistency:'CURRENT',coverage:'current'} as const;
const page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS'};
const token={address,name:'Exact token',symbol:'EX',decimals:6,tokenType:'ERC20',blockNumber:'123',createdAt:observedAt};
function mount(view:React.ReactNode,url='/'){return render(<TooltipProvider><MemoryRouter initialEntries={[url]}>{view}</MemoryRouter></TooltipProvider>);}
beforeEach(()=>{sessionStorage.clear();mock.read.mockReset();mock.state={identity:{...mock.state.identity,activation:Math.random().toString(),expires:Math.floor(Date.now()/1000)+300}};});
afterEach(()=>{cleanup();vi.useRealTimers();});
it('Home displays scoped accounts rather than observed participant addresses, with exact zero/lower-bound/unavailable',async()=>{
  let calls=0;mock.read.mockImplementation(async(path:string)=>path.includes('/stats?')?(calls++,{privacyEnabled:true,observedTransactions:'3',observedParticipantAddresses:'777',observedBlocks:'2',counts:{transactions:limited,accounts:current,tokens:unavailable},page:{...page,coverage:'OBSERVED_VISIBLE_PAGE'}}):path.endsWith('/blocks/latest/number')?{number:'123'}:{items:[],page});
  mount(<HomeNative/>);await waitFor(()=>expect(screen.getAllByTestId('stat-card')).toHaveLength(5));
  const card=(label:string)=>screen.getAllByTestId('stat-card').find(node=>within(node).queryByText(label))!;
  expect(card('Visible Accounts')).toHaveTextContent('0');expect(card('Visible Accounts')).not.toHaveTextContent('777');
  expect(card('Visible Transactions')).toHaveTextContent('≥9,007,199,254,740,993');expect(card('Visible Tokens')).toHaveTextContent('—');
  expect(card('Visible Transactions').querySelector('[title]')).toHaveAttribute('title',expect.stringContaining('Retained blocks 100–123'));
  const trigger=within(card('Visible Transactions')).getByRole('button');fireEvent.click(trigger);
  expect(await screen.findByRole('tooltip')).toHaveTextContent('Retained blocks 100–123');fireEvent.keyDown(trigger,{key:'Escape'});
  fireEvent.click(screen.getByRole('button',{name:'Refresh counts'}));await waitFor(()=>expect(calls).toBe(2));
  act(()=>{mock.state={identity:{...mock.state.identity,activation:'new'}};mock.listeners.forEach(fn=>fn());});expect(screen.queryByText('≥9,007,199,254,740,993')).toBeNull();
});
it('token list and detail retain styled supply/count columns with exact units, zero and honest lower bounds',async()=>{
  const value={...token,totalSupply:'900719925474099312345',counts:{holders:current,tokenTransfers:limited},holderCount:'0'};
  mock.read.mockResolvedValue({items:[value],page});mount(<TokensNative/>,'/tokens');
  await screen.findByText('Exact token');expect(screen.getByRole('columnheader',{name:'Indexed supply'})).toBeInTheDocument();
  expect(screen.getByText('900,719,925,474,099.312345')).toBeInTheDocument();expect(screen.getByText('≥9,007,199,254,740,993')).toBeInTheDocument();
  cleanup();mock.read.mockImplementation(async(path:string)=>path===`/v2/explorer/tokens/${address}`?value:{items:[],page});
  mount(<Routes><Route path="/token/:address" element={<TokenNative/>}/></Routes>,`/token/${address}`);
  expect(await screen.findByText('Indexed supply')).toBeInTheDocument();expect(screen.getByText('≥9,007,199,254,740,993')).toBeInTheDocument();expect(screen.getByText(/not a live contract read/)).toBeInTheDocument();
});
it('account list and address details/tabs use their own typed metrics and do not turn unavailable into zero',async()=>{
  const value={address,isContract:false,counts:{transactions:limited,tokenTransfers:unavailable,internalTransactions:{...current,value:'3'}}};
  mock.read.mockImplementation(async(path:string)=>path.startsWith('/v2/explorer/accounts?')?{items:[value],page}:null);mount(<AccountsNative/>,'/accounts');
  expect(await screen.findByTestId('account-row')).toHaveTextContent('≥9,007,199,254,740,993');expect(screen.getByTestId('account-row')).toHaveTextContent('—');cleanup();
  mock.read.mockImplementation(async(path:string)=>path.endsWith('/stats')?value:path.endsWith('/balance')?{address,balance:'0'}:null);
  mount(<Routes><Route path="/address/:address" element={<AddressNative/>}/></Routes>,`/address/${address}`);
  const tx=await screen.findByTestId('tab-transactions');expect(tx).toHaveTextContent('≥9,007,199,254,740,993');expect(screen.getByTestId('tab-internal')).toHaveTextContent('3');
  expect([...document.querySelectorAll('dt')].find(node=>node.textContent==='Token transfers')!.parentElement).toHaveTextContent('—');
});
it('a malformed count/price is a controlled read error rather than a visible partial result or render exception',async()=>{
  mock.read.mockResolvedValue({items:[{...token,usdPrice:'1'.repeat(129)}],page});mount(<TokensNative/>,'/tokens');
  await screen.findByText('The read could not be completed. Try again.');expect(screen.queryByText('Exact token')).toBeNull();expect(screen.getByRole('button',{name:'Try again'})).toBeInTheDocument();
});
