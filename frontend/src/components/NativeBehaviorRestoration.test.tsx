import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Interface } from 'ethers';
import { NativeLogCards, NativeTraceRows, NativeTransferRows } from './NativeReadPanels';
import { TooltipProvider } from './ui/tooltip';
import type { InternalTransaction, Log, TokenTransfer } from '../lib/v2ReadTypes';
const session = vi.hoisted(() => {
  let state = {identity: {activation: 'first', actor: 'ops:actor', expires: 9999999999}};
  const listeners = new Set<() => void>();
  return {snapshot: () => state, subscribe: (f: () => void) => {listeners.add(f); return () => listeners.delete(f);}, reset: () => {state = {identity: {...state.identity, activation: 'first'}};}, change: () => {state = {identity: {...state.identity, activation: 'second'}}; listeners.forEach(f => f());}, read: vi.fn(), captureOperation: vi.fn()};
});
vi.mock('../lib/v2Session', () => ({appSession: session, ReadHTTPError: class extends Error {code = 'unavailable';}}));
const address = '0x' + 'a'.repeat(40), other = '0x' + 'b'.repeat(40), zero = '0x' + '0'.repeat(40), hash = '0x' + 'c'.repeat(64);
function mount(node: React.ReactNode) {return render(<TooltipProvider><MemoryRouter>{node}</MemoryRouter></TooltipProvider>);}
const token = {address, symbol: 'USD', decimals: 6, tokenType: 'ERC20', blockNumber: '1', createdAt: 'now'};
function transfer(id = 'row'): TokenTransfer {return {id, txHash: hash, logIndex: 1, tokenAddress: address, from: other, to: zero, value: '900719925474099312345', blockNumber: '1', transferType: 'transfer', tokenType: 'ERC20', isInternal: false};}
function trace(id: string, path: string): InternalTransaction {return {id, txHash: hash, blockNumber: '1', traceAddress: path, from: address, to: other, value: null, callType: 'call'};}
function eventLog(): Log {
  const iface = new Interface(['event Transfer(address indexed from,address indexed to,uint256 value)']);
  const encoded = iface.encodeEventLog(iface.getEvent('Transfer')!, [zero, other, 9007199254740993n]);
  return {id: 'log', txHash: hash, logIndex: 1, address, topic0: encoded.topics[0], topic1: encoded.topics[1], topic2: encoded.topics[2], topic3: null, data: encoded.data, blockNumber: '1', removed: false};
}
beforeEach(() => {
  session.reset(); session.read.mockReset(); session.captureOperation.mockReset(); session.read.mockResolvedValue(null);
  session.captureOperation.mockImplementation(() => {
    const original = session.snapshot().identity.activation, abort = new AbortController();
    return {signal: abort.signal, check: () => {if (abort.signal.aborted || session.snapshot().identity.activation !== original) throw Error('changed');}, dispose: () => abort.abort()};
  });
});
describe('restored native Explorer behavior', () => {
  it('shows decoded local values and topic choices without linking a masked participant', async () => {
    mount(<NativeLogCards items={[eventLog()]}/>);
    const decoded = screen.getByTestId('decoded-event');
    expect(decoded).toHaveTextContent('Transfer'); expect(decoded).toHaveTextContent('9,007,199,254,740,993');
    expect(within(decoded).getByTestId('private-address')).toHaveTextContent('Private');
    expect(document.querySelector(`a[href*="${zero}"]`)).toBeNull();
    fireEvent.change(screen.getByLabelText('Topic 1 format'), {target: {value: 'address'}});
    expect(screen.getAllByTestId('private-address').length).toBeGreaterThan(1);
    expect(session.read).not.toHaveBeenCalled();
  });
  it('decodes a custom event only from its authorized ABI and never requests an external signature', async () => {
    const abi = ['event BankRecord(uint256 amount)'], iface = new Interface(abi), encoded = iface.encodeEventLog(iface.getEvent('BankRecord')!, [12n]);
    session.read.mockResolvedValue({address, abi});
    mount(<NativeLogCards items={[{...eventLog(), topic0: encoded.topics[0], topic1: null, topic2: null, data: encoded.data}]}/>);
    expect(await screen.findByText('Approved ABI')).toBeInTheDocument(); expect(screen.getByTestId('decoded-event')).toHaveTextContent('BankRecord');
    expect(session.read.mock.calls.map(([path]) => path)).toEqual([`/v2/explorer/addresses/${address}/contract`]);
  });
  it('restores nested expansion and raw authorized rows without turning a hidden value into zero', () => {
    mount(<NativeTraceRows items={[trace('child', '0,0'), trace('parent', '0'), trace('orphan', '2,0')]}/>);
    const nodes = screen.getAllByTestId('native-trace-node');
    expect(nodes).toHaveLength(3); expect(nodes[0]).toHaveAttribute('data-trace-path', '0');
    expect(within(nodes[0]).getAllByTestId('native-trace-node')).toHaveLength(1);
    expect(screen.getAllByText(/Parent call is not/)).toHaveLength(2);
    expect(screen.getAllByText('Hidden')).toHaveLength(3); expect(screen.queryByText('0 ETH')).toBeNull();
    fireEvent.click(within(nodes[0]).getAllByRole('button', {name: 'Collapse'})[0]);
    expect(screen.getAllByTestId('native-trace-node')).toHaveLength(2);
    fireEvent.click(screen.getByRole('button', {name: 'Raw JSON'}));
    expect(screen.getByText(/"value": null/)).toBeInTheDocument(); expect(screen.queryByText(/"value": "0"/)).toBeNull();
  });
  it('shows ERC721 and ERC1155 IDs and quantities exactly, without inventing an NFT item route', () => {
    mount(<NativeTransferRows items={[{...transfer('721'), tokenType: 'ERC721', tokenId: '9007199254740993', value: '1'}, {...transfer('1155'), tokenType: 'ERC1155', tokenId: '9007199254740994', value: '9007199254740995'}]}/>);
    const [first, second] = screen.getAllByTestId('transfer-row');
    expect(first).toHaveTextContent('Token ID: 9,007,199,254,740,993'); expect(first).toHaveTextContent('Quantity1');
    expect(second).toHaveTextContent('Token ID: 9,007,199,254,740,994'); expect(second).toHaveTextContent('9,007,199,254,740,995');
    expect(screen.queryByText(/base units/)).toBeNull(); expect(session.read).not.toHaveBeenCalled();
    expect(document.querySelector('a[href*="/nft/"]')).toBeNull(); expect(document.querySelector(`a[href*="${zero}"]`)).toBeNull();
  });
  it('enriches repeated visible ERC20 rows with one metadata read and preserves decimal precision', async () => {
    session.read.mockResolvedValue(token); mount(<NativeTransferRows items={[transfer('one'), transfer('two')]}/>);
    expect(await screen.findAllByText('900,719,925,474,099.312345 USD')).toHaveLength(2); expect(session.read).toHaveBeenCalledTimes(1);
  });
  it('does not enrich a masked token and leaves denied metadata as exact base units', async () => {
    session.read.mockRejectedValue(Error('denied')); mount(<NativeTransferRows items={[transfer('full'), {...transfer('private'), tokenAddress: zero}]}/>);
    expect(screen.getAllByText('900,719,925,474,099,312,345 base units')).toHaveLength(2);
    await waitFor(() => expect(session.read).toHaveBeenCalledTimes(1)); expect(session.read.mock.calls[0][0]).not.toContain(zero);
  });
  it('retires late metadata on account replacement', async () => {
    let resolve!: (value: unknown) => void;
    session.read.mockImplementationOnce(() => new Promise(r => {resolve = r;})).mockResolvedValue(null);
    mount(<NativeTransferRows items={[transfer()]}/>); await waitFor(() => expect(session.read).toHaveBeenCalledTimes(1));
    act(() => session.change()); await act(async () => resolve({...token, symbol: 'OLD_PRIVATE'}));
    expect(screen.queryByText(/OLD_PRIVATE/)).toBeNull(); expect(screen.getByText(/base units/)).toBeInTheDocument();
  });
  it('bounds metadata work to25 distinct records and four concurrent reads', async () => {
    let release!: () => void, inflight = 0, maximum = 0;
    const gate = new Promise<void>(r => {release = r;});
    session.read.mockImplementation(async (path: string) => {
      maximum = Math.max(maximum, ++inflight); await gate; inflight--;
      return {...token, address: path.split('/').at(-1)};
    });
    const rows = Array.from({length: 26}, (_, i) => ({...transfer(String(i)), tokenAddress: '0x' + (i + 1).toString(16).padStart(40, '0')}));
    mount(<NativeTransferRows items={rows}/>); await waitFor(() => expect(session.read).toHaveBeenCalledTimes(4));
    await act(async () => release());
    await waitFor(() => expect(session.read).toHaveBeenCalledTimes(25)); expect(maximum).toBe(4);
    expect(screen.getByText('900,719,925,474,099,312,345 base units')).toBeInTheDocument();
  });
  it('does not apply a custom ABI returned for a different emitter', async () => {
    const abi = ['event BankRecord(uint256 amount)'], iface = new Interface(abi), encoded = iface.encodeEventLog(iface.getEvent('BankRecord')!, [12n]);
    session.read.mockResolvedValue({address: other, abi});
    mount(<NativeLogCards items={[{...eventLog(), topic0: encoded.topics[0], topic1: null, topic2: null, data: encoded.data}]}/>);
    await waitFor(() => expect(session.read).toHaveBeenCalledTimes(1));
    await act(async () => {await Promise.resolve();});
    expect(screen.queryByTestId('decoded-event')).toBeNull();
  });
});
