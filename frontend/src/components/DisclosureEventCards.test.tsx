import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { Interface } from 'ethers';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DisclosureEventCards } from './DisclosureEventCards';
import actualDTO from '../lib/testdata/disclosure-events-go-dto.json';
import type { DataEvent } from '../lib/v2Disclosure';

const ordinaryMetadata = vi.hoisted(() => vi.fn(() => { throw Error('Case access must not fetch ordinary metadata'); }));
vi.mock('../hooks/useNativeMetadata', () => ({ useNativeMetadata: ordinaryMetadata }));
const network = vi.fn(() => { throw Error('Case event rendering must not fetch'); });
beforeEach(() => { vi.stubGlobal('fetch', network); });
afterEach(() => { expect(network).not.toHaveBeenCalled(); cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); });
const source = actualDTO.full.result.items[0] as DataEvent;
const from = '0x' + 'a'.repeat(40), to = '0x' + 'b'.repeat(40);
function encoded(abi: string, values: unknown[]): DataEvent {
  const iface = new Interface([abi]);
  const log = iface.encodeEventLog(iface.fragments[0], values);
  return {...source, topics: log.topics, data: log.data};
}
function mount(row = source) { return render(<DisclosureEventCards rows={[row]} level="full"/>); }

it('renders the actual Go Full DTO through the original log card shell and explicit case provenance', () => {
  const {container} = mount();
  const card = screen.getByTestId('disclosure-event-card');
  expect(card.querySelector('.card')).toBeInTheDocument();
  expect(within(card).getByText('Disclosed')).toHaveAttribute('data-reason', 'disclosure_grant');
  expect(within(card).getByText('Full')).toBeInTheDocument();
  expect(within(card).getByText('Log Index')).toBeInTheDocument();
  expect(within(card).getByText(source.emitter)).toBeInTheDocument();
  expect(within(card).getByText(source.data!)).toBeInTheDocument();
  expect(container.querySelector('a')).toBeNull();
  expect(ordinaryMetadata).not.toHaveBeenCalled();
});

it('shares local ERC20 decoding and topic controls while keeping every disclosed address inert', () => {
  const row = encoded('event Transfer(address indexed from,address indexed to,uint256 value)', [from, to, 900719925474099312345n]);
  const {container} = mount(row);
  expect(screen.getByTestId('decoded-event')).toHaveTextContent('Transfer');
  expect(screen.getByTestId('decoded-event')).toHaveTextContent('900,719,925,474,099,312,345');
  expect(screen.getByText('Known event format')).toBeInTheDocument();
  expect(screen.queryByText('Approved ABI')).toBeNull();
  fireEvent.change(screen.getByLabelText('Topic 1 format'), {target: {value: 'address'}});
  expect(screen.getAllByText(from)).toHaveLength(2);
  expect(screen.getByText(to)).toBeInTheDocument();
  expect(container.querySelector('a')).toBeNull();
  expect(ordinaryMetadata).not.toHaveBeenCalled();
});

it('shows ERC721 token IDs as indexed values and real empty data as 0x', () => {
  mount(encoded('event Transfer(address indexed from,address indexed to,uint256 indexed tokenId)', [from, to, 9007199254740993n]));
  expect(screen.getByTestId('decoded-event')).toHaveTextContent('tokenId:');
  expect(screen.getByTestId('decoded-event')).toHaveTextContent('9,007,199,254,740,993');
  expect(screen.getByText('0x')).toBeInTheDocument();
  expect(screen.queryByText('Payload withheld')).toBeNull();
});

it('keeps unknown and malformed event evidence raw without trying ordinary ABI enrichment', () => {
  const row = encoded('event PrivateBankRecord(address owner,uint256 amount)', [to, 7n]);
  mount(row);
  expect(screen.queryByTestId('decoded-event')).toBeNull();
  expect(screen.getByText(row.topics![0])).toBeInTheDocument();
  expect(screen.getByText(row.data!)).toBeInTheDocument();
  expect(ordinaryMetadata).not.toHaveBeenCalled();
  cleanup();
  mount({...encoded('event Transfer(address indexed from,address indexed to,uint256 value)', [from, to, 7n]), data: '0x01'});
  expect(screen.queryByTestId('decoded-event')).toBeNull();
  expect(screen.getByText('0x01')).toBeInTheDocument();
  expect(ordinaryMetadata).not.toHaveBeenCalled();
});

it('distinguishes an anonymous empty log from a withheld payload', () => {
  mount({...source, topics: [], data: '0x'});
  expect(screen.getByText('No topics')).toBeInTheDocument();
  expect(screen.getByText('0x')).toBeInTheDocument();
  expect(screen.queryByText('Payload withheld')).toBeNull();
});

it.each(['pseudonymous', 'redacted'] as const)('keeps actual Go %s metadata and provenance without raw payload, hashes or links', level => {
  const row = actualDTO[level].result.items[0] as DataEvent;
  const {container} = render(<DisclosureEventCards rows={[row]} level={level}/>);
  expect(screen.getByText('Disclosed')).toBeInTheDocument();
  expect(screen.getByText('Payload withheld')).toBeInTheDocument();
  expect(screen.getByText(row.emitter)).toBeInTheDocument();
  expect(screen.queryByTestId('decoded-event')).toBeNull();
  expect(screen.queryByText('Topics')).toBeNull();
  expect(screen.queryByText('Data')).toBeNull();
  expect(screen.queryByText('Transaction details')).toBeNull();
  expect(container.innerHTML).not.toContain(source.emitter);
  expect(container.innerHTML).not.toContain(source.transactionHash);
  expect(container.querySelector('a,select')).toBeNull();
  expect(ordinaryMetadata).not.toHaveBeenCalled();
});
