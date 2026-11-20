import { act,cleanup,render,screen,waitFor,within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter,Route,Routes } from 'react-router-dom';
import { afterEach,beforeEach,describe,expect,it,vi } from 'vitest';
import fixture from '../test/fixtures/native-block-counts-go.json';
import { TooltipProvider } from '../components/ui/tooltip';
import { HomeBlockRow } from '../components/HomePresentation';
import { BlocksNative,BlockNative } from './BlocksNative';
import { HomeNative } from './HomeNative';
const session=vi.hoisted(()=>({state:{identity:{actor:'ops:fixture',profile:'wire',activation:'first',expires:9999999999}},read:vi.fn(),listeners:new Set<()=>void>()}));
vi.mock('../lib/v2Session',()=>({isV2:()=>true,appSession:{snapshot:()=>session.state,subscribe:(fn:()=>void)=>{session.listeners.add(fn);return()=>session.listeners.delete(fn);},read:session.read,captureOperation:()=>{const original=session.state.identity,controller=new AbortController();return{signal:controller.signal,check:()=>{if(original!==session.state.identity||controller.signal.aborted)throw Error('changed');},dispose:()=>controller.abort()};}},ReadHTTPError:class extends Error{code='unavailable';}}));
const block=(name='exact-zero')=>structuredClone(fixture.Cases.find(entry=>entry.Name===name)!.Block);
const page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS'};
const stats={privacyEnabled:true,observedTransactions:'0',observedBlocks:'0',observedParticipantAddresses:'0',page:{...page,coverage:'OBSERVED_VISIBLE_PAGE'}};
function mount(node:React.ReactNode,path='/'){return render(<TooltipProvider><MemoryRouter initialEntries={[path]}>{node}</MemoryRouter></TooltipProvider>);}
function replies(value=block()){session.read.mockImplementation(async(path:string)=>path.includes('/stats?')?stats:path.endsWith('/latest/number')?{number:value.number}:path.includes('/blocks?')?{items:[value],page}:path===`/v2/explorer/blocks/${value.number}`?value:{items:[],page});}
beforeEach(()=>{sessionStorage.clear();session.read.mockReset();session.state={identity:{...session.state.identity,activation:Math.random().toString()}};});
afterEach(()=>cleanup());
describe('native per-block count presentation from its own response',()=>{
 it('Home uses exact typed large count in the V1 row without another request; default V1 scalar stays unchanged',async()=>{
  replies(block('exact-large'));mount(<HomeNative/>);const row=await screen.findByTestId('block-row');expect(row).toHaveTextContent('9,007,199,254,740,993 txns');
  const trigger=within(row).getByRole('button');await userEvent.click(trigger);expect(await screen.findByRole('tooltip')).toHaveTextContent('Ordinary transactions in block 9007199254740993');await userEvent.keyboard('{Escape}');
  expect(new Set(session.read.mock.calls.map(([path])=>path))).toEqual(new Set(['/v2/explorer/stats?limit=10','/v2/explorer/blocks/latest/number','/v2/explorer/blocks?limit=10','/v2/explorer/transactions?limit=10']));
  cleanup();mount(<HomeBlockRow block={{...block(),transactionCount:3}}/>);expect(await screen.findByTestId('block-row')).toHaveTextContent('3 txns');expect(screen.queryByRole('button')).toBeNull();
 });
 it('Blocks list retains exact zero with scoped details rather than an absent/raw/page count',async()=>{
  replies();mount(<BlocksNative/>,'/blocks');const row=await screen.findByTestId('block-row'),cell=row.querySelectorAll('td')[2];expect(cell).toHaveTextContent('0');await userEvent.click(within(cell).getByRole('button'));expect(await screen.findByRole('tooltip')).toHaveTextContent('Retained blocks 9007199254740993–9007199254740993');
 });
 it('partial detail and tab badge retain ≥ while first click/Enter/Space still selects the tab and exposes scope',async()=>{
  const value=block('partial');replies(value);mount(<Routes><Route path="/block/:number" element={<BlockNative/>}/></Routes>,'/block/'+value.number);
  const tab=await screen.findByTestId('tab-transactions');expect(tab).toHaveTextContent('≥82');expect(tab.querySelector('button')).toBeNull();
  for(const key of ['click','{Enter}',' ']) {
    await userEvent.click(screen.getByTestId('tab-details'));tab.focus();if(key==='click')await userEvent.click(tab);else await userEvent.keyboard(key);
    expect(tab).toHaveAttribute('aria-selected','true');expect(await screen.findByRole('tooltip')).toHaveTextContent('Ordinary transactions in block '+value.number);await userEvent.keyboard('{Escape}');
    await waitFor(()=>expect(screen.queryByRole('tooltip')).toBeNull());expect(tab).toHaveTextContent('≥82');
  }
  expect(screen.getByText('No visible transactions on this page')).toBeInTheDocument();expect(tab).not.toHaveTextContent('Transactions0');
 });
 it('known unavailable count stays honest on NODE header and explains pruning without a fake time',async()=>{
  const value={...block('pruned-unknown'),source:'NODE'};replies(value as ReturnType<typeof block>);mount(<Routes><Route path="/block/:number" element={<BlockNative/>}/></Routes>,'/block/'+value.number);
  const tab=await screen.findByTestId('tab-transactions');expect(tab).toHaveTextContent('—');await userEvent.click(tab);const note=await screen.findByRole('tooltip');expect(note).toHaveTextContent('has been pruned');expect(note).not.toHaveTextContent('Observed ');expect(tab).not.toHaveTextContent('0');
 });
 it('clears a point count on activation change and rejects delayed data from the old activation',async()=>{
  const value=block('exact-large');let calls=0,finish!:(value:unknown)=>void;session.read.mockImplementation(async(path:string)=>path===`/v2/explorer/blocks/${value.number}`?++calls===1?new Promise(resolve=>{finish=resolve;}):block():{items:[],page});
  mount(<Routes><Route path="/block/:number" element={<BlockNative/>}/></Routes>,'/block/'+value.number);await waitFor(()=>expect(calls).toBe(1));act(()=>{session.state={identity:{...session.state.identity,activation:'second'}};session.listeners.forEach(fn=>fn());});await screen.findByTestId('tab-transactions');await act(async()=>finish(value));expect(screen.getByTestId('tab-transactions')).toHaveTextContent('Transactions0');expect(screen.getByTestId('tab-transactions')).not.toHaveTextContent('9,007,199,254,740,993');
  act(()=>{session.state={identity:{...session.state.identity,activation:'third'}};session.read.mockImplementation(()=>new Promise(()=>{}));session.listeners.forEach(fn=>fn());});expect(screen.queryByTestId('tab-transactions')).toBeNull();
 });
});
