import { act,cleanup,fireEvent,render,screen,waitFor,within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach,beforeEach,describe,expect,it,vi } from 'vitest';
import { TooltipProvider } from '../components/ui/tooltip';
import { HomeNative } from './HomeNative';
import { HomePresentation,HomeTransactionRow } from '../components/HomePresentation';
import { NativeAddress } from '../components/NativeAddress';
const session=vi.hoisted(()=>({state:{identity:{actor:'ops:own',profile:'fixture',activation:'first',expires:9999999999}},read:vi.fn(),listeners:new Set<()=>void>()}));
vi.mock('../lib/v2Session',()=>({isV2:()=>true,appSession:{snapshot:()=>session.state,subscribe:(fn:()=>void)=>{session.listeners.add(fn);return()=>session.listeners.delete(fn);},read:session.read,captureOperation:()=>{const identity=session.state.identity,controller=new AbortController();return{signal:controller.signal,check:()=>{if(identity!==session.state.identity||controller.signal.aborted)throw Error('changed');},dispose:()=>controller.abort()};}},ReadHTTPError:class extends Error{code='unavailable';}}));
const address='0x'+'a'.repeat(40),hash='0x'+'b'.repeat(64),page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS'};
function block(number:number,timestamp:number){const own='0x'+number.toString(16).padStart(64,'0');return{number:String(number),timestamp:String(timestamp),hash:own,parentHash:hash,gasLimit:'30000000',difficulty:'0',nonce:'0x'+'0'.repeat(16),miner:address,extraData:'0x',stateRoot:hash,transactionsRoot:hash,receiptsRoot:hash,createdAt:'now'};}
const stats={privacyEnabled:true,observedTransactions:'0',observedBlocks:'0',observedParticipantAddresses:'0',page:{...page,coverage:'OBSERVED_VISIBLE_PAGE'}};
const mounted=(view:React.ReactNode)=><TooltipProvider><MemoryRouter>{view}</MemoryRouter></TooltipProvider>;
function source(blocks:()=>unknown){session.read.mockImplementation(async(path:string)=>path.includes('/stats?')?stats:path.endsWith('/latest/number')?{number:'9'}:path.includes('/blocks?')?await blocks():{items:[],page});}
const avg=()=>screen.getAllByTestId('stat-card').find(card=>within(card).queryByText('Avg Block Time'))!;
beforeEach(()=>{sessionStorage.clear();session.read.mockReset();session.state={identity:{...session.state.identity,activation:Math.random().toString()}};});
afterEach(()=>{cleanup();vi.useRealTimers();});
describe('native Home page sample and preserved V1 presentation',()=>{
 it('adds the fifth card with visible range/pair count and click/focus/Escape sample details, without another read',async()=>{
  source(()=>({items:[block(3,15),block(2,12),block(1,10),block(0,0)],page}));render(mounted(<HomeNative/>));
  await waitFor(()=>expect(screen.getAllByTestId('stat-card')).toHaveLength(5));expect(avg()).toHaveTextContent('2.50s');expect(avg()).toHaveTextContent('2 usable intervals · page blocks 1–3');
  expect(screen.getByTestId('home-stats-grid')).toHaveClass('xl:grid-cols-5');expect(screen.getByText('Visible Tokens')).toBeInTheDocument();
  const button=within(avg()).getByRole('button');fireEvent.focus(button);fireEvent.click(button);expect(await screen.findByRole('tooltip')).toHaveTextContent('not a whole-chain or last-100-block average');fireEvent.keyDown(button,{key:'Escape'});await waitFor(()=>expect(screen.queryByRole('tooltip')).toBeNull());
  expect(new Set(session.read.mock.calls.map(([path])=>path))).toEqual(new Set(['/v2/explorer/stats?limit=10','/v2/explorer/blocks/latest/number','/v2/explorer/blocks?limit=10','/v2/explorer/transactions?limit=10']));
 });
 it('does not claim zero for a sparse page with no usable positive interval',async()=>{
  source(()=>({items:[block(9,50),block(7,45)],page}));render(mounted(<HomeNative/>));await screen.findByText('Avg Block Time');expect(avg()).toHaveTextContent('Unavailable');expect(avg()).toHaveTextContent('0 usable intervals · page blocks 7–9');expect(avg()).not.toHaveTextContent('0.00s');
 });
 it('clears the previous sample immediately when the original activation changes',async()=>{
  let calls=0;source(()=>++calls===1?{items:[block(2,16),block(1,10)],page}:new Promise(()=>{}));render(mounted(<HomeNative/>));await screen.findByText('6.00s');
  act(()=>{session.state={identity:{...session.state.identity,activation:'second'}};session.listeners.forEach(fn=>fn());});expect(screen.queryByText('6.00s')).toBeNull();expect(screen.queryByTestId('block-time-sample')).toBeNull();
 });
 it('replaces one polled page with another without joining their timestamps or ranges',async()=>{
  vi.useFakeTimers();let count=0;source(()=>({items:++count===1?[block(2,16),block(1,10)]:[block(4,104),block(3,100)],page}));render(mounted(<HomeNative/>));await act(async()=>{await Promise.resolve();await Promise.resolve();});expect(screen.getByText('6.00s')).toBeInTheDocument();
  await act(async()=>{vi.advanceTimersByTime(5000);await Promise.resolve();await Promise.resolve();});expect(screen.getByText('4.00s')).toBeInTheDocument();expect(screen.getByTestId('block-time-sample')).toHaveTextContent('page blocks 3–4');expect(screen.queryByText('6.00s')).toBeNull();
 });
 it('retains default V1 four-column/grid and fixed-height rows, opting native badge rows into growth',()=>{
  const tx={hash,status:1};render(mounted(<><HomePresentation search={null} stats={null} blocks={null} transactions={null}/><HomeTransactionRow tx={tx} from="V1"/><HomeTransactionRow wrapParties tx={{...tx,hash:'0x'+'c'.repeat(64)}} from={<NativeAddress address={address} metadata={{[address]:'participant_override'}}/>} to={<NativeAddress address={'0x'+'d'.repeat(40)} metadata={{['0x'+'d'.repeat(40)]:'rbac_group_member'}}/>}/></>));
  expect(screen.getByTestId('home-stats-grid')).toHaveClass('md:grid-cols-4');const rows=screen.getAllByTestId('tx-row');expect(rows[0]).toHaveClass('h-[52px]');expect(rows[1]).toHaveClass('min-h-[52px]','py-3');expect(rows[1]).not.toHaveClass('h-[52px]');expect(within(rows[1]).getByText('Counterparty')).toBeInTheDocument();expect(within(rows[1]).getByText('My Org')).toBeInTheDocument();expect(rows[1].querySelector(`a[href="/address/${address}"]`)).toBeNull();
 });
});
