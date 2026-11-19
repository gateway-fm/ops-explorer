import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '../components/ui/tooltip';
import { BlocksNative, BlockNative } from './BlocksNative';
import { AccountsNative, TransactionsNative } from './CollectionsNative';
import { TokensNative, TokenNative, NativePortfolio } from './TokensNative';
import { ApiDocsNative, SyncNative } from './NetworkNative';
import { NativeContractPanel } from '../components/NativeContractPanel';
import { parseNativeABIInput } from '../lib/nativeABI';
import { Interface, ParamType } from 'ethers';
const session=vi.hoisted(()=>{
 let state={identity:{activation:'first',actor:'ops:actor',expires:9999999999}};
 const listeners=new Set<()=>void>();
 return {snapshot:()=>state,subscribe:(fn:()=>void)=>{listeners.add(fn);return()=>listeners.delete(fn);},change:()=>{state={identity:{...state.identity,activation:'second'}};listeners.forEach(fn=>fn());},read:vi.fn(),self:vi.fn(),rpc:vi.fn(),captureOperation:vi.fn()};
});
vi.mock('../lib/v2Session',()=>({appSession:session,ReadHTTPError:class extends Error{code='unavailable';}}));
const address='0x'+'a'.repeat(40),hash='0x'+'b'.repeat(64),number='9007199254740993123';
const page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS'};
const block={number,hash,parentHash:'0x'+'c'.repeat(64),timestamp:'1700000000',gasLimit:'30000000',difficulty:'0',nonce:'0x'+'0'.repeat(16),miner:address,extraData:'0x',stateRoot:hash,transactionsRoot:hash,receiptsRoot:hash,createdAt:'now',source:'NODE'};
const token={address,symbol:'EX',name:'Example token',decimals:18,tokenType:'ERC20',blockNumber:'1',createdAt:'now'};
function renderPage(node:React.ReactNode,path='/') {return render(<TooltipProvider><MemoryRouter initialEntries={[path]}>{node}</MemoryRouter></TooltipProvider>);}
beforeEach(()=>{
 session.read.mockReset();session.self.mockReset();session.rpc.mockReset();session.captureOperation.mockReset();
 session.captureOperation.mockImplementation(()=>{const original=session.snapshot().identity.activation;const abort=new AbortController();return {signal:abort.signal,check:()=>{if(original!==session.snapshot().identity.activation)throw Error('changed');},dispose:()=>abort.abort(),self:session.self,rpc:session.rpc};});
});
describe('named native Explorer screens',()=>{
 it('renders exact block positions and genuine omitted counts without legacy number conversion',async()=>{
  session.read.mockImplementation(async(path:string)=>path.includes('latest')?{number}:{items:[block],page});
  renderPage(<BlocksNative/>);
  expect(await screen.findByText('9,007,199,254,740,993,123')).toHaveAttribute('href','/block/'+number);
  expect(screen.getByText('Unavailable')).toBeInTheDocument();expect(screen.queryByText('Gas Used')).toBeNull();
  expect(screen.getByText('30,000,000')).toBeInTheDocument();
 });
 it('keeps block transactions and internal tab cursors independent and uses node provenance only as information',async()=>{
  session.read.mockImplementation(async(path:string)=>path.includes('/transactions')||path.includes('/internal')?{items:[],page:{...page,hasMore:null,scanLimited:true,next:path.includes('/transactions')?'tx-cursor':'internal-cursor'}}:block);
  renderPage(<Routes><Route path="/block/:number" element={<BlockNative/>}/></Routes>,`/block/${number}`);
  expect(await screen.findByText('Verified node point read')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('tab',{name:'Transactions'}));await screen.findByRole('button',{name:'Continue'});fireEvent.click(screen.getByRole('button',{name:'Continue'}));await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toContain('cursor=tx-cursor'));
  fireEvent.click(screen.getByRole('tab',{name:'Internal transactions'}));await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toContain('/internal?limit=25'));
  fireEvent.click(screen.getByRole('tab',{name:'Transactions'}));await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toContain('cursor=tx-cursor'));
 });
 it('uses source category filters and drops the prior category cursor',async()=>{
  session.read.mockResolvedValue({items:[],page:{...page,hasMore:null,scanLimited:true,next:'prior-category'}});
  renderPage(<TransactionsNative/>);
  fireEvent.click(await screen.findByRole('button',{name:'Continue'}));await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toContain('cursor='));
  fireEvent.click(screen.getByRole('tab',{name:'Contract calls'}));await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toBe('/v2/explorer/transactions/paginated?category=contract_call&limit=25'));
 });
 it('does not invent account balances, ranks or global percentages',async()=>{
  session.read.mockResolvedValue({items:[{address}],page});renderPage(<AccountsNative/>);
  expect(await screen.findByTestId('account-row')).toHaveTextContent('Unavailable');
  expect(screen.queryByText('Top Accounts')).toBeNull();expect(screen.queryByText('Percentage')).toBeNull();expect(screen.queryByText('Rank')).toBeNull();expect(screen.queryByText('0 ETH')).toBeNull();
 });
 it('keeps token search on the server and restores filters through browser history',async()=>{
  session.read.mockResolvedValue({items:[token],page});
  function Back(){const navigate=useNavigate();return <><button onClick={()=>navigate(-1)}>History back</button><TokensNative/></>;}
  renderPage(<Back/>,'/tokens?search=Example&type=ERC20');
  expect(await screen.findByTestId('token-row')).toHaveTextContent('Unavailable');
  expect(session.read.mock.calls[0]?.[0]).toContain('search=Example');
  fireEvent.click(screen.getByRole('tab',{name:'ERC-721'}));await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toContain('type=ERC721'));
  fireEvent.click(screen.getByRole('button',{name:'History back'}));await waitFor(()=>expect(screen.getByRole('tab',{name:'ERC-20'})).toHaveAttribute('aria-selected','true'));
  fireEvent.change(screen.getByRole('searchbox',{name:'Search tokens'}),{target:{value:'Other token'}});
  await waitFor(()=>expect(session.read.mock.calls.at(-1)?.[0]).toContain('search=Other+token'));expect(screen.getByTestId('token-row')).toBeInTheDocument();
 });
 it('opens a holder deep link, preserves uint256 balances, and labels mutable pagination',async()=>{
  session.read.mockImplementation(async(path:string)=>path.includes('/holders')?{items:[{address,balance:'900719925474099312345678901234567890'}],page:{...page,consistency:'CURRENT'}}:token);
  renderPage(<Routes><Route path="/tokens/:address/holders" element={<TokenNative/>}/></Routes>,`/tokens/${address}/holders`);
  expect(await screen.findByText('900,719,925,474,099,312.34567890123456789 EX')).toBeInTheDocument();expect(screen.getByText(/Current values may change/)).toBeInTheDocument();expect(screen.getByRole('tab',{name:'Holders'})).toHaveAttribute('aria-selected','true');
 });
 it('enriches portfolio units without converting balances through a number',async()=>{
  session.read.mockImplementation(async(path:string)=>path.includes('/balances')?{items:[{address,tokenAddress:address,balance:'900719925474099312345678901234567890',blockNumber:'1'}],page}:token);
  renderPage(<NativePortfolio address={address}/>);expect(await screen.findByText('900,719,925,474,099,312.34567890123456789 EX')).toBeInTheDocument();expect(screen.getByText('Example token')).toHaveAttribute('href','/token/'+address);
 });
 it('does not fetch metadata or create a token link for a masked portfolio key',async()=>{
  const masked='0x'+'0'.repeat(40);session.read.mockResolvedValue({items:[{address,tokenAddress:masked,balance:'5',blockNumber:'1'}],page});
  renderPage(<NativePortfolio address={address}/>);expect(await screen.findByTestId('private-address')).toHaveTextContent('Private');expect(document.querySelector(`a[href*="${masked}"]`)).toBeNull();expect(session.read.mock.calls.some(([path])=>String(path).includes(`/tokens/${masked}`))).toBe(false);
 });
 it('performs ABI reads through the original scoped RPC with the complete overload signature',async()=>{
  const abi=['function balanceOf(address owner) view returns (uint256)','function balanceOf(address owner,uint256 id) view returns (uint256)'];
  const iface=new Interface(abi),org='11111111-1111-1111-1111-111111111111';
  session.self.mockResolvedValue({organizations:[{id:org,name:'Example'}]});session.rpc.mockResolvedValue({result:iface.encodeFunctionResult('balanceOf(address)',['900719925474099312345678901234567890'])});
  renderPage(<NativeContractPanel address={address} contract={{address,abi}} code="0x"/>);
  fireEvent.click(await screen.findByRole('button',{name:'balanceOf(address)'}));fireEvent.change(screen.getByLabelText('balanceOf(address) owner'),{target:{value:address}});fireEvent.click(screen.getByRole('button',{name:'Query'}));
  expect(await screen.findByTestId('fn-result')).toHaveTextContent('900719925474099312345678901234567890');
  expect(session.rpc.mock.calls[0]?.[0]).toBe(org);expect(session.rpc.mock.calls[0]?.[1]).toMatchObject({method:'eth_call',params:[{to:address,data:iface.encodeFunctionData('balanceOf(address)',[address])},'latest']});
 });
 it('rejects lossy JSON numbers in nested ABI inputs and accepts quoted tuple/array integers',()=>{
  const param=ParamType.from('tuple(uint256 value,bool enabled)[]');expect(()=>parseNativeABIInput('[[9007199254740993,true]]',param)).toThrow('quoted');expect(parseNativeABIInput('[["9007199254740993",true]]',param)).toEqual([['9007199254740993',true]]);
 });
 it('offers real bounded native Try It and clears an in-flight result when its inputs change',async()=>{
  let resolve!:(value:unknown)=>void;session.read.mockImplementationOnce(()=>new Promise(r=>{resolve=r;}));
  renderPage(<ApiDocsNative/>);fireEvent.click(screen.getByRole('button',{name:'GET /v2/explorer/blocks'}));
  const send=screen.getByRole('button',{name:'Send request'});fireEvent.click(send);await waitFor(()=>expect(session.read).toHaveBeenCalledWith('/v2/explorer/blocks?limit=25',expect.any(AbortSignal)));
  fireEvent.change(screen.getByLabelText('/v2/explorer/blocks limit'),{target:{value:'10'}});await act(async()=>resolve({private:'old input'}));expect(screen.queryByText('old input')).toBeNull();expect(screen.queryByText('No authentication is required.')).toBeNull();
 });
 it('never turns an omitted catch-up distance into zero',async()=>{
  const sync={lastIndexedBlock:'1',isSyncing:false,updatedAt:'now'},indexer={minFetchedBlock:'0',maxFetchedBlock:'1',backfillComplete:true,updatedAt:'now'};
  session.read.mockImplementation(async(path:string)=>path.endsWith('/status')?sync:path.endsWith('/indexer-progress')?indexer:{sync,indexer});renderPage(<SyncNative/>);await screen.findByText('Not syncing');expect(await screen.findByText('Unavailable')).toBeInTheDocument();
 });
});
