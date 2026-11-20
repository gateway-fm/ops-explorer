import { advancePosition, firstPosition, navigationOwner, savePosition } from '../lib/nativeNavigation';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TransactionNative } from './TransactionNative';
import { TransactionsNative, TransfersNative } from './CollectionsNative';
import { NativeMetadataBoundary } from '../hooks/useNativeMetadata';
import type { InternalTransaction, TokenTransfer, Transaction, VisibilityReason } from '../lib/v2ReadTypes';

const session=vi.hoisted(()=>({state:{identity:{actor:'ops:p1:'+'A'.repeat(43),profile:'native-fixture',activation:'fixture',expires:2000000000}},listeners:new Set<()=>void>(),read:vi.fn(),captureOperation:vi.fn()}));
vi.mock('../lib/v2Session',()=>({appSession:{snapshot:()=>session.state,subscribe:(fn:()=>void)=>{session.listeners.add(fn);return()=>session.listeners.delete(fn);},read:session.read,captureOperation:session.captureOperation},ReadHTTPError:class extends Error{code='unavailable';}}));
const address='0x'+'a'.repeat(40),other='0x'+'b'.repeat(40),tokenAddress='0x'+'c'.repeat(40),zero='0x'+'0'.repeat(40),hash='0x'+'d'.repeat(64);
const page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS' as const};
const tx:Transaction={hash,blockNumber:'7',txIndex:0,blockTimestamp:'1700000000',from:address,to:other,value:'1',gasPrice:'1',gasUsed:'21000',inputData:'0x',status:1,txType:2};
const transfer:TokenTransfer={id:'transfer',txHash:hash,logIndex:0,tokenAddress,from:address,to:other,value:'1000001',blockNumber:'7',tokenType:'ERC20',transferType:'transfer',isInternal:false};
const trace:InternalTransaction={id:'trace',txHash:hash,blockNumber:'7',traceAddress:'',from:address,to:other,value:'1',callType:'call'};
let generation=0;
beforeEach(()=>{sessionStorage.clear();generation++;session.state={identity:{...session.state.identity,activation:`fixture-${generation}`,expires:Math.floor(Date.now()/1000)+300}};session.read.mockReset();session.captureOperation.mockReset();session.captureOperation.mockImplementation(()=>{const activation=session.state.identity.activation,abort=new AbortController();return{identity:session.state.identity,signal:abort.signal,check:()=>{if(abort.signal.aborted||activation!==session.state.identity.activation)throw Error('changed');},dispose:()=>abort.abort()};});});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
function mount(view:'detail'|'list'|'transfer'|'trace'='detail') {
  const url=view==='list'?'/transactions':view==='transfer'?'/token-transfers':`/tx/${hash}${view==='trace'?'?tab=trace':''}`;
  return render(<MemoryRouter initialEntries={[url]}><NativeMetadataBoundary><Routes><Route path="/tx/:hash" element={<TransactionNative/>}/><Route path="/transactions" element={<TransactionsNative/>}/><Route path="/token-transfers" element={<TransfersNative/>}/></Routes></NativeMetadataBoundary></MemoryRouter>);
}
const reasons:[VisibilityReason|string,string|undefined,boolean][]=[['own_address','Mine',true],['rbac_group_member','My Org',true],['visible_to_grant','Shared',true],['public_address','Public',true],['participant_override','Counterparty',false],['no_access',undefined,false],['masked','Private',false]];
describe.each(['detail','list','transfer','trace'] as const)('native %s composition uses current row reasons',view=>{
  it.each(reasons)('renders %s as %s with the matching navigation boundary',async(reason,label,navigable)=>{
    const from=reason==='masked'?zero:address,metadata={[address]:reason as VisibilityReason,[other]:'no_access' as const};
    session.read.mockImplementation(async(path:string)=>path.includes('/transactions/paginated')?{items:[{...tx,from,addressMetadata:metadata}],page}:path.includes('/internal?')?{items:[{...trace,from,addressMetadata:metadata}],page}:path.endsWith('/transfers?limit=25')||path==='/v2/explorer/transfers?limit=25'?{items:view==='transfer'?[{...transfer,from,addressMetadata:metadata,tokenAddress:zero}]:[],page}:path===`/v2/explorer/transactions/${hash}`?{...tx,from,addressMetadata:metadata}:null);
    mount(view);
    const row=view==='detail'?await waitFor(()=>{const value=document.querySelector('[data-testid="info-row"][data-label="From"]');expect(value).not.toBeNull();return value as HTMLElement;}):await screen.findByTestId(view==='list'?'tx-row':view==='transfer'?'transfer-row':'native-trace-node');
    const scope=within(row as HTMLElement);
    if(label==='Private')expect(scope.getAllByTestId('private-address').every(node=>node.textContent==='Private')).toBe(true);
    else if(label)expect(scope.getByTestId('address-label')).toHaveTextContent(label);
    else expect(scope.queryByTestId('address-label')).toBeNull();
    const link=row.querySelector(`a[href="/address/${from}"]`);expect(!!link).toBe(navigable);
    if(navigable)expect(row.querySelector('button[title="Copy to clipboard"]')).not.toBeNull();
    expect(scope.queryByText('Disclosed',{exact:true})).toBeNull();
    await act(async()=>{await Promise.resolve();});
    if(!navigable){expect(session.read.mock.calls.some(([path])=>path===`/v2/explorer/tokens/${from}`||path===`/v2/explorer/addresses/${from}/contract`)).toBe(false);expect(!!row.querySelector('button[title="Copy to clipboard"]')).toBe(view==='detail'&&reason==='participant_override');}
  });
});

it('copies exactly the full already-visible participant address on detail without granting navigation or enrichment',async()=>{
  const write=vi.spyOn(navigator.clipboard,'writeText').mockResolvedValue();
  session.read.mockImplementation(async(path:string)=>path===`/v2/explorer/transactions/${hash}`?{...tx,addressMetadata:{[address]:'participant_override',[other]:'no_access'}}:path.includes('/transfers?')?{items:[],page}:null);mount();
  const row=await waitFor(()=>{const node=document.querySelector('[data-label="From"]');expect(node).not.toBeNull();return node as HTMLElement;});
  expect(within(row).getByText(address,{exact:true})).toBeInTheDocument();expect(within(row).getByText('Counterparty',{exact:true})).toBeInTheDocument();fireEvent.click(within(row).getByTitle('Copy to clipboard'));
  expect(write).toHaveBeenCalledExactlyOnceWith(address);expect(row.querySelector('a')).toBeNull();expect(session.read.mock.calls.some(([path])=>path===`/v2/explorer/tokens/${address}`||path===`/v2/explorer/addresses/${address}/contract`)).toBe(false);
});

it('does not turn consent-era metadata into ordinary Disclosed provenance or a case read',async()=>{
  session.read.mockImplementation(async(path:string)=>path===`/v2/explorer/transactions/${hash}`?{...tx,addressMetadata:{[address]:'disclosure_grant'}}:path.includes('/transfers?')?{items:[],page}:null);mount();await screen.findByTestId('tx-status');expect(screen.queryByText('Disclosed',{exact:true})).toBeNull();expect(session.read.mock.calls.every(([path])=>path.startsWith('/v2/explorer/'))).toBe(true);
});

describe('compact native transaction transfers',()=>{
  function setup(first:unknown) {session.read.mockImplementation(async(path:string)=>path===`/v2/explorer/transactions/${hash}`?tx:path.includes('/transfers?')?first:path===`/v2/explorer/tokens/${tokenAddress}`?{address:tokenAddress,symbol:'USDX',name:'Exact token',decimals:6,tokenType:'ERC20',blockNumber:'7',createdAt:'now'}:null);}
  it('hides an exhausted empty first-page section after loading',async()=>{setup({items:[],page});mount();await screen.findByTestId('tx-status');await waitFor(()=>expect(document.querySelector('[data-label="ERC-20 Tokens Transferred"]')).toBeNull());expect(screen.queryByRole('table')).toBeNull();expect(screen.queryByRole('button',{name:'Continue'})).toBeNull();});
  it('keeps compact loading and error/retry states without inserting feed columns',async()=>{let reject!:(reason:Error)=>void;setup(new Promise((_resolve,fail)=>{reject=fail;}));mount();await screen.findByTestId('tx-status');expect(screen.getByText('Loading transfers…')).toBeInTheDocument();expect(screen.queryByRole('columnheader')).toBeNull();await act(async()=>reject(Error('unavailable')));const section=document.querySelector('[data-label="ERC-20 Tokens Transferred"]')!;expect(within(section as HTMLElement).getByRole('alert')).toBeInTheDocument();expect(within(section as HTMLElement).getByRole('button',{name:'Try again'})).toBeInTheDocument();expect(screen.queryByRole('table')).toBeNull();});
  it('uses the original inline From/To/For row with exact units, NFT ID and Counterparty',async()=>{setup({items:[{...transfer,addressMetadata:{[address]:'participant_override',[other]:'no_access'}},{...transfer,id:'nft',tokenAddress:zero,tokenType:'ERC721',tokenId:'9007199254740993',value:'1',from:zero,to:zero}],page});mount();const rows=await screen.findAllByTestId('transfer-row');await screen.findByText('1.000001 USDX');expect(rows[0]).toHaveTextContent('From');expect(rows[0]).toHaveTextContent('To');expect(rows[0]).toHaveTextContent('For');expect(rows[0]).toHaveTextContent('Counterparty');expect(rows[0].tagName).toBe('DIV');expect(rows[1]).toHaveTextContent('Token ID: 9,007,199,254,740,993');expect(screen.queryByRole('columnheader')).toBeNull();expect(screen.queryByRole('table')).toBeNull();expect(document.querySelector(`a[href="/address/${address}"]`)).not.toBeNull();expect(rows[0].querySelector(`a[href="/address/${address}"]`)).toBeNull();});
  it('retains a first-page restart when earlier cursor coordinates have been pruned',async()=>{
    const owner=navigationOwner(session.state.identity,`/transactions/${hash}/transfers`,25,'')!;let position=firstPosition(owner,session.state.identity.expires);for(let i=0;i<26;i++)position=advancePosition(position,`opaque-${i}`);savePosition({...position,index:0});setup({items:[],page});mount();await screen.findByText(new RegExp(`^Page ${position.offset+1}\\.`));expect(screen.getByRole('button',{name:'Previous'})).toBeDisabled();fireEvent.click(screen.getByRole('button',{name:'Restart transfers from the first page'}));await waitFor(()=>expect(document.querySelector('[data-label="ERC-20 Tokens Transferred"]')).toBeNull());
  });
  it('continues an empty scan-limited page and keeps Previous on a later exhausted empty page',async()=>{const advancing={items:[],page:{...page,hasMore:null,scanLimited:true,next:'opaque-next'}};session.read.mockImplementation(async(path:string)=>path===`/v2/explorer/transactions/${hash}`?tx:path.includes('/transfers?')?path.includes('cursor=opaque-next')?{items:[],page}:advancing:null);mount();const next=await screen.findByRole('button',{name:'Continue'});expect(next).toBeEnabled();expect(screen.queryByRole('table')).toBeNull();fireEvent.click(next);await screen.findByText(/^Page 2\./);expect(screen.getByRole('button',{name:'Previous'})).toBeEnabled();expect(screen.getByRole('button',{name:'Continue'})).toBeDisabled();fireEvent.click(screen.getByRole('button',{name:'Previous'}));await screen.findByText(/^Page 1\./);expect(screen.getByRole('button',{name:'Continue'})).toBeEnabled();});
});
