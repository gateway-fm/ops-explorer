import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { FunctionFragment, Interface } from 'ethers';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NativeContractWrite } from './NativeContractWrite';
const mocks=vi.hoisted(()=>({connect:vi.fn(),send:vi.fn(),dispose:vi.fn(),capture:vi.fn(),rpc:vi.fn()}));
vi.mock('../lib/nativeWalletHelper',()=>({connectNativeWallet:mocks.connect}));
vi.mock('../pages/NetworkSetupV2',()=>({networkSetup:async()=>({chainId:'0x1',chainIdDecimal:'1',genesisHash:'0x'+'a'.repeat(64)})}));
vi.mock('../lib/v2Session',()=>({appSession:{captureOperation:mocks.capture}}));
const address='0x'+'b'.repeat(40),hash='0x'+'c'.repeat(64),org='11111111-1111-1111-1111-111111111111';
const self={actorId:'actor',sourceProfile:'profile',sourceProfileDigest:'sha256:'+'a'.repeat(64),principalKind:'human',expiresAt:9999999999,organizations:[{id:org,slug:'test',name:'Test org',isOrgAdmin:false,isOrgReadonlyAdmin:false}]};
const abi=new Interface(['function setValue(uint256 amount) payable']);const fragment=abi.getFunction('setValue') as FunctionFragment;
function mount(){return render(<MemoryRouter><NativeContractWrite fragment={fragment} iface={abi} address={address} self={self}/></MemoryRouter>);}
function enter(){fireEvent.click(screen.getByRole('button',{name:/setValue/}));fireEvent.change(screen.getByLabelText('setValue(uint256) helper origin'),{target:{value:'http://localhost:18545'}});}
beforeEach(()=>{
 mocks.connect.mockReset();mocks.send.mockReset();mocks.dispose.mockReset();mocks.capture.mockReset();mocks.rpc.mockReset();
 mocks.capture.mockImplementation(()=>{const abort=new AbortController();return {identity:{expires:Math.floor(Date.now()/1000)+300},signal:abort.signal,check:()=>{if(abort.signal.aborted)throw Error('changed');},dispose:()=>abort.abort(),rpc:mocks.rpc};});
 mocks.connect.mockImplementation(async(options)=>{options.onNetworkConfiguration({rpcURL:'http://localhost:18545/native-v2/rpc/private-context/123',expiresAt:Math.floor(Date.now()/1000)+200});return {account:address,expiresAt:Math.floor(Date.now()/1000)+200,send:mocks.send,dispose:mocks.dispose};});
});
afterEach(()=>vi.useRealTimers());
describe('ordinary-wallet ABI Write presentation',()=>{
 it('prepares through the reviewed helper, keeps exact wei, and submits only after an explicit Write click',async()=>{
  mocks.send.mockImplementation(async(_transaction,onDispatch)=>{onDispatch();return hash;});mount();enter();fireEvent.click(screen.getByRole('button',{name:'Connect wallet'}));await waitFor(()=>expect(screen.getByRole('button',{name:'Write'})).toBeEnabled());expect(mocks.send).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText('setValue(uint256) amount'),{target:{value:'900719925474099312345678901234567890'}});fireEvent.change(screen.getByLabelText('setValue(uint256) value in wei'),{target:{value:'123'}});fireEvent.click(screen.getByRole('button',{name:'Write'}));
  expect(await screen.findByText(hash)).toHaveAttribute('href','/tx/'+hash);expect(mocks.send).toHaveBeenCalledWith({to:address,data:abi.encodeFunctionData(fragment,['900719925474099312345678901234567890']),value:'0x7b'},expect.any(Function));expect(screen.queryByLabelText('Current private wallet RPC URL')).toBeNull();
  expect(screen.getByRole('button',{name:'Write'})).toBeDisabled();expect(screen.queryByRole('button',{name:'Close connection'})).toBeNull();
 });
 it('does not retry or offer cancellation after handing a request to the wallet',async()=>{
  let finish!:(hash:string)=>void;mocks.send.mockImplementation((_transaction,onDispatch)=>{onDispatch();return new Promise<string>(resolve=>{finish=resolve;});});mount();enter();fireEvent.click(screen.getByRole('button',{name:'Connect wallet'}));await waitFor(()=>expect(screen.getByRole('button',{name:'Write'})).toBeEnabled());fireEvent.change(screen.getByLabelText('setValue(uint256) amount'),{target:{value:'1'}});fireEvent.click(screen.getByRole('button',{name:'Write'}));
  expect(screen.getByRole('button',{name:'Waiting for wallet…'})).toBeDisabled();expect(screen.queryByRole('button',{name:'Close connection'})).toBeNull();expect(screen.getByRole('status')).toHaveTextContent('cannot undo');expect(mocks.send).toHaveBeenCalledTimes(1);
  await act(async()=>finish(hash));expect(await screen.findByText(hash)).toBeInTheDocument();
 });
 it('cancels only a pre-submission connection and disposes a late helper context',async()=>{
  let finish!:(value:unknown)=>void;mocks.connect.mockImplementation(options=>{options.onNetworkConfiguration({rpcURL:'http://localhost:18545/private',expiresAt:Math.floor(Date.now()/1000)+200});return new Promise(resolve=>{finish=resolve;});});
  mount();enter();fireEvent.click(screen.getByRole('button',{name:'Connect wallet'}));fireEvent.click(await screen.findByRole('button',{name:'Close connection'}));expect(screen.queryByLabelText('Current private wallet RPC URL')).toBeNull();
  await act(async()=>finish({account:address,send:mocks.send,dispose:mocks.dispose}));expect(mocks.dispose).toHaveBeenCalledTimes(1);expect(mocks.send).not.toHaveBeenCalled();expect(screen.getByRole('button',{name:'Write'})).toBeDisabled();
 });
 it('keeps an uncertain effect closed to another submission and never shows a false rollback',async()=>{
  mocks.send.mockImplementation(async(_transaction,onDispatch)=>{onDispatch();throw Error('provider disconnected');});mount();enter();fireEvent.click(screen.getByRole('button',{name:'Connect wallet'}));await waitFor(()=>expect(screen.getByRole('button',{name:'Write'})).toBeEnabled());fireEvent.change(screen.getByLabelText('setValue(uint256) amount'),{target:{value:'1'}});fireEvent.click(screen.getByRole('button',{name:'Write'}));
  expect(await screen.findByRole('alert')).toHaveTextContent('Check wallet activity');expect(screen.queryByRole('button',{name:'Connect wallet'})).toBeNull();expect(screen.getByRole('button',{name:'Write'})).toBeDisabled();expect(mocks.send).toHaveBeenCalledTimes(1);
 });
 it('can close a held attestation before the provider receives any transaction',async()=>{
  let finish!:()=>void;const held=new Promise<void>(resolve=>{finish=resolve;}),providerSend=vi.fn();let check!:()=>void;
  mocks.connect.mockImplementation(async(options)=>{check=options.operation.check;return {account:address,send:mocks.send,dispose:mocks.dispose};});
  mocks.send.mockImplementation(async(_transaction,onDispatch)=>{await held;check();onDispatch();return providerSend();});
  mount();enter();fireEvent.click(screen.getByRole('button',{name:'Connect wallet'}));await waitFor(()=>expect(screen.getByRole('button',{name:'Write'})).toBeEnabled());fireEvent.change(screen.getByLabelText('setValue(uint256) amount'),{target:{value:'1'}});fireEvent.click(screen.getByRole('button',{name:'Write'}));
  expect(screen.getByRole('button',{name:'Checking wallet…'})).toBeDisabled();fireEvent.click(screen.getByRole('button',{name:'Close connection'}));await act(async()=>finish());
  expect(providerSend).not.toHaveBeenCalled();expect(screen.getByRole('alert')).toHaveTextContent('Connection closed');expect(screen.queryByText(/without a confirmed outcome/)).toBeNull();
 });
 it('keeps attestation denial recoverable before dispatch without automatically retrying',async()=>{
  mocks.send.mockRejectedValue(Error('The wallet kept an older RPC context. Update it and reconnect.'));mount();enter();fireEvent.click(screen.getByRole('button',{name:'Connect wallet'}));await waitFor(()=>expect(screen.getByRole('button',{name:'Write'})).toBeEnabled());fireEvent.change(screen.getByLabelText('setValue(uint256) amount'),{target:{value:'1'}});fireEvent.click(screen.getByRole('button',{name:'Write'}));
  expect(await screen.findByRole('alert')).toHaveTextContent('older RPC context');expect(screen.getByRole('button',{name:'Close connection'})).toBeInTheDocument();expect(screen.getByRole('button',{name:'Write'})).toBeEnabled();fireEvent.click(screen.getByRole('button',{name:'Reconnect wallet'}));await waitFor(()=>expect(mocks.connect).toHaveBeenCalledTimes(2));expect(mocks.send).toHaveBeenCalledTimes(1);
 });
 it('clears the sensitive setup URL and leaves a held submission uncertain at original expiry',async()=>{
  vi.useFakeTimers();mocks.send.mockImplementation((_transaction,onDispatch)=>{onDispatch();return new Promise(()=>{});});mount();enter();fireEvent.click(screen.getByRole('button',{name:'Connect wallet'}));await act(async()=>{await Promise.resolve();await Promise.resolve();});fireEvent.change(screen.getByLabelText('setValue(uint256) amount'),{target:{value:'1'}});fireEvent.click(screen.getByRole('button',{name:'Write'}));
  act(()=>vi.advanceTimersByTime(201000));expect(screen.queryByLabelText('Current private wallet RPC URL')).toBeNull();expect(screen.getByRole('alert')).toHaveTextContent('Check wallet activity');expect(screen.getByRole('button',{name:'Write'})).toBeDisabled();expect(mocks.dispose).toHaveBeenCalled();
 });
});
