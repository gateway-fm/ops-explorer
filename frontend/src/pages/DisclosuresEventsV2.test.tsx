import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DisclosureCaseV2 } from './DisclosuresV2';
import financeDTO from '../lib/testdata/finance-native-go-dto.json';

const mock = vi.hoisted(() => ({ state: { identity: { actor: 'ops:p1:' + 'A'.repeat(43), profile: 'azure', activation: '11111111-1111-4111-8111-111111111111', expires: 2000000000 } }, disclosure: vi.fn(), listeners: new Set<() => void>() }));
vi.mock('../lib/v2Session', () => ({ appSession: { snapshot: () => mock.state, subscribe: (listener: () => void) => { mock.listeners.add(listener); return () => mock.listeners.delete(listener); }, disclosure: mock.disclosure } }));
const now = Math.floor(Date.now() / 1000), id = '11111111-1111-4111-8111-111111111111', actor = mock.state.identity.actor, digest = 'sha256:' + 'a'.repeat(64);
beforeEach(() => { mock.state = { identity: { actor, profile: 'azure', activation: id, expires: now + 300 } }; mock.disclosure.mockReset(); });
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.useRealTimers(); vi.unstubAllGlobals(); });
const record = (level = 'full', resource = 'events') => ({ caseId: id, ruleId: 'events-only', recipientActorId: actor, creatorActorId: actor, dataOrganizationId: id, scope: { targetActors: [actor], resources: [resource], addresses: { mode: 'all-target-addresses', values: [] }, dataTime: { start: now - 100, end: now }, level }, caseReference: 'Events audit', purpose: 'Investigation', legalBasis: 'Mandate', status: 'open', version: 1, createdAt: now - 1, expiresAt: now + 900 });
const event = (level = 'full') => ({ eventId: 'de1.data.' + 'A'.repeat(21) + 'Q', blockNumber: '9007199254740993', transactionIndex: 2, logIndex: 7, blockTime: now - 1, emitter: level === 'full' ? '0x' + '1'.repeat(40) : level === 'redacted' ? '[PRIVATE]' : 'Address-' + 'A'.repeat(26), payload: level === 'full' ? 'disclosed' : 'withheld', ...(level === 'full' ? { transactionHash: '0x' + 'a'.repeat(64), blockHash: '0x' + 'b'.repeat(64), topics: ['0x' + 'c'.repeat(64)], data: '0xdeadbeef' } : {}) });
const envelope = (level = 'full', empty = false) => ({ v: 1, readId: id, caseId: id, resource: 'events', view: 'list', level, dataTime: record().scope.dataTime, effectiveUntil: now + 900, coverage: { status: 'RETAINED_AT_READ', populations: ['CURRENT_PROVED_ASSOCIATIONS', 'RETAINED_SAME_ORG_EVENTS_FROM_DISCLOSED_TRANSACTIONS'], sourceBasisDigest: digest, preparedAt: now }, result: { v: 1, population: 'EVENTS_FROM_DISCLOSED_TRANSACTIONS', items: empty ? [] : [event(level)], nextCursor: empty ? 'continue-opaque' : null } });
function fixture(level = 'full', resource = 'events') {
  const bodies: any[] = [];
  mock.disclosure.mockImplementation(async (path: string, init?: RequestInit) => {
    if (path.includes('/audit')) return { items: [], nextCursor: '' };
    if (init?.method !== 'POST') return record(level, resource);
    const body = JSON.parse(String(init.body)); bodies.push(body);
    if (resource === 'events') return { ...envelope(level, !body.cursor), dataTime: body.dataTime ?? record().scope.dataTime };
    const raw = structuredClone(body.view === 'report' ? financeDTO.report : financeDTO.list);
    return { ...raw, dataTime: body.dataTime ?? record().scope.dataTime, effectiveUntil: now + 900, coverage: { ...raw.coverage, preparedAt: now, sourceBasisDigest: body.cursor ? 'sha256:' + 'b'.repeat(64) : digest }, result: body.view === 'report' ? { ...financeDTO.report.result, finance: { ...financeDTO.report.result.finance, observedAt: new Date(now * 1000).toISOString() } } : { ...financeDTO.list.result, items: financeDTO.list.result.items.map(item => ({ ...item, time: now - 1 })), nextCursor: body.cursor ? null : 'next-finance', matchingRetainedCount: body.cursor ? '20002' : '20001', observedAt: new Date(now * 1000).toISOString() } };
  });
  return bodies;
}
async function open(resource = 'Events') {
  render(<MemoryRouter initialEntries={['/disclosures/cases/' + id]}><Routes><Route path="/disclosures/cases/:caseId" element={<DisclosureCaseV2 />} /></Routes></MemoryRouter>);
  fireEvent.click(await screen.findByRole('button', { name: resource }));
}
it.each(['full', 'pseudonymous', 'redacted'])('uses explicit Events-only %s authority and Continue without ordinary Explorer navigation', async level => {
  const bodies = fixture(level); await open();
  fireEvent.click(await screen.findByRole('button', { name: 'Continue' }));
  await screen.findByRole('cell', { name: '9007199254740993' });
  expect(bodies).toEqual([{ v: 1, resource: 'events', view: 'list', limit: 50 }, { v: 1, resource: 'events', view: 'list', limit: 50, cursor: 'continue-opaque' }]);
  expect(screen.queryByRole('link', { name: /Details|Explorer/ })).toBeNull();
  if (level === 'full') { fireEvent.click(screen.getByText('Raw payload')); expect(screen.getByText('0xdeadbeef')).toBeInTheDocument(); expect(screen.getByText('0x' + 'c'.repeat(64))).toBeInTheDocument(); expect(screen.getByText(/may include other parties/)).toBeInTheDocument(); }
  else { expect(screen.getByText('Payload withheld')).toBeInTheDocument(); expect(screen.queryByText('Raw payload')).toBeNull(); expect(screen.queryByText('0xdeadbeef')).toBeNull(); }
  fireEvent.click(screen.getByRole('button', { name: 'Previous page' })); expect(screen.getByText('No eligible events on this page.')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Continue' })); await screen.findByRole('cell', { name: '9007199254740993' }); expect(bodies).toHaveLength(2);
});
it('downloads exactly the visible Events page', async () => {
  const blobs: Blob[] = [], downloads: string[] = [];
  vi.stubGlobal('URL', class extends URL { static createObjectURL(blob: Blob) { blobs.push(blob); return 'blob:event-page'; } static revokeObjectURL = vi.fn(); });
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () { downloads.push(this.download); });
  fixture(); await open(); fireEvent.click(await screen.findByRole('button', { name: 'Continue' })); await screen.findByRole('cell', { name: '9007199254740993' });
  fireEvent.click(screen.getByRole('button', { name: 'Download this page' }));
  const content = await new Promise<any>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(JSON.parse(String(reader.result))); reader.onerror = reject; reader.readAsText(blobs[0]); });
  expect(content).toEqual(envelope()); expect(content.result.items).toHaveLength(1); expect(downloads).toEqual([`disclosure-${id}-events-page.json`]);
});
it('clears cached Events at the original shorter session expiry without a click', async () => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); vi.spyOn(Date, 'now').mockReturnValue(now * 1000); const bodies = fixture('redacted');
  await act(async () => { render(<MemoryRouter initialEntries={['/disclosures/cases/' + id]}><Routes><Route path="/disclosures/cases/:caseId" element={<DisclosureCaseV2 />} /></Routes></MemoryRouter>); });
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Events' })); });
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Continue' })); });
  expect(screen.getByText('Payload withheld')).toBeInTheDocument();
  vi.mocked(Date.now).mockReturnValue((now + 300) * 1000); await act(async () => { await vi.advanceTimersByTimeAsync(300000); });
  expect(screen.queryByText('Payload withheld')).toBeNull(); expect(screen.queryByRole('button', { name: 'Download this page' })).toBeNull(); expect(bodies).toHaveLength(2);
});
it('rechecks original authority after download materialization', async () => {
  const revoke = vi.fn(), click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  vi.spyOn(Date, 'now').mockReturnValue(now * 1000);
  vi.stubGlobal('URL', class extends URL { static createObjectURL() { vi.mocked(Date.now).mockReturnValue((now + 300) * 1000); return 'blob:event-expired'; } static revokeObjectURL = revoke; });
  const bodies = fixture(); await open(); fireEvent.click(await screen.findByRole('button', { name: 'Continue' })); await screen.findByRole('cell', { name: '9007199254740993' });
  fireEvent.click(screen.getByRole('button', { name: 'Download this page' }));
  expect(click).not.toHaveBeenCalled(); expect(revoke).toHaveBeenCalledWith('blob:event-expired'); expect(bodies).toHaveLength(2); expect(screen.queryByRole('button', { name: 'Download this page' })).toBeNull();
});
it('ignores a pending read after target narrowing and releases previous pages after a source failure', async () => {
  const bodies = fixture(), original = mock.disclosure.getMockImplementation()!; let finish!: (value: unknown) => void;
  mock.disclosure.mockImplementation((path, init) => init?.method === 'POST' ? new Promise(resolve => { finish = resolve; }) : original(path, init));
  await open(); await waitFor(() => expect(mock.disclosure.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(true));
  fireEvent.change(screen.getByLabelText('Target actor'), { target: { value: actor } });
  await act(async () => { finish(envelope()); }); expect(screen.queryByText('Raw payload')).toBeNull();
  mock.disclosure.mockImplementation(original); fireEvent.click(screen.getByRole('button', { name: 'Read permitted data' }));
  fireEvent.click(await screen.findByRole('button', { name: 'Continue' })); await screen.findByRole('cell', { name: '9007199254740993' });
  mock.disclosure.mockImplementation(async (path, init) => init?.method === 'POST' ? Promise.reject(Error('The source changed. Reload the case.')) : original(path, init));
  fireEvent.click(screen.getByRole('button', { name: 'Refresh results' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('source changed'); expect(screen.queryByText('Raw payload')).toBeNull(); expect(screen.queryByRole('button', { name: 'Previous page' })).toBeNull(); expect(bodies).toHaveLength(2);
});
it('opens v2 summary details with identical actor/time narrowing, accepts live changes, and refreshes newest', async () => {
  const bodies = fixture('full', 'compliance_report'); await open('Compliance'); await screen.findByRole('button', { name: 'View admission decisions' });
  const from = new Date((now - 50) * 1000).toISOString().slice(0, 19), through = new Date(now * 1000).toISOString().slice(0, 19);
  fireEvent.change(screen.getByLabelText('Target actor'), { target: { value: actor } }); fireEvent.change(screen.getByLabelText('From (UTC, inclusive)'), { target: { value: from } }); fireEvent.change(screen.getByLabelText('Through (UTC, inclusive)'), { target: { value: through } });
  fireEvent.click(screen.getByRole('button', { name: 'Read permitted data' })); fireEvent.click(await screen.findByRole('button', { name: 'View admission decisions' }));
  await screen.findByRole('button', { name: 'Download this page' });
  expect(bodies.at(-1)).toEqual({ v: 1, resource: 'compliance_report', view: 'list', limit: 50, targetActorIds: [actor], dataTime: { start: now - 50, end: now } });
  expect(bodies.at(-2)).toEqual({ v: 1, resource: 'compliance_report', view: 'report', targetActorIds: [actor], dataTime: { start: now - 50, end: now } });
  fireEvent.click(screen.getByRole('button', { name: 'Next page' })); await screen.findByText('20002'); expect(screen.queryByRole('alert')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: 'Refresh results' })); await screen.findByText('20001'); expect(bodies.at(-1)).not.toHaveProperty('cursor'); expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled();
});
