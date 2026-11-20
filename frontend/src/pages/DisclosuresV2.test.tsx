import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { DisclosureCaseV2, DisclosuresV2 } from './DisclosuresV2';

const mock = vi.hoisted(() => {
  const actor = 'ops:p1:' + 'A'.repeat(43);
  const state = { identity: { actor, profile: 'azure', activation: '11111111-1111-4111-8111-111111111111', expires: 2000000000 }, loading: false, changed: false, error: null };
  return { state, disclosure: vi.fn(), request: vi.fn() };
});
vi.mock('../lib/v2Session', () => ({ appSession: { snapshot: () => mock.state, subscribe: () => () => {}, disclosure: mock.disclosure, request: mock.request } }));
afterEach(() => { cleanup(); vi.clearAllMocks(); vi.useRealTimers(); });

const caseId = '11111111-1111-4111-8111-111111111111';
function currentCase() {
  const now = Math.floor(Date.now()/1000);
  return {caseId, ruleId:'bounded-review', creatorActorId:mock.state.identity.actor, recipientActorId:mock.state.identity.actor, dataOrganizationId:caseId, scope:{targetActors:[mock.state.identity.actor],resources:['transactions'],addresses:{mode:'all-target-addresses',values:[]},dataTime:{start:now-60,end:now},level:'full'},caseReference:'Expiry fixture',purpose:'Review',legalBasis:'Mandate',status:'open',version:1,createdAt:now-1,expiresAt:now+3};
}
function mountCase() { return render(<MemoryRouter initialEntries={['/disclosures/cases/'+caseId]}><Routes><Route path="/disclosures/cases/:caseId" element={<DisclosureCaseV2/>}/></Routes></MemoryRouter>); }
describe('case review and recovery', () => {
  it('retries failed case history on the same page and restarts from the first page', async () => {
    let fail = true;
    mock.disclosure.mockImplementation(async (path: string) => {
      if (!path.includes('/audit')) return currentCase();
      if (fail) throw Error('History interrupted');
      return { items: [{ eventId: path.includes('cursor=') ? 'next' : 'first', time: 1900000000, action: path.includes('cursor=') ? 'Continued audit action' : 'First audit action', outcome: 'allowed' }], nextCursor: path.includes('cursor=') ? '' : 'next-history-page' };
    });
    mountCase();
    await screen.findByText('History interrupted');
    fireEvent.click(screen.getByText('Case history'));
    fail = false; fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    expect(await screen.findByText('First audit action')).toBeInTheDocument();
    fail = true; fireEvent.click(screen.getByRole('button', { name: 'Next history page' }));
    await screen.findByText('History interrupted');
    fail = false; fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    expect(await screen.findByText('Continued audit action')).toBeInTheDocument();
    expect(mock.disclosure.mock.calls.filter(([path]) => String(path).includes('/audit')).at(-1)![0]).toContain('cursor=next-history-page');
    fireEvent.click(screen.getByRole('button', { name: 'Restart history' }));
    expect(await screen.findByText('First audit action')).toBeInTheDocument();
    expect(mock.disclosure.mock.calls.filter(([path]) => String(path).includes('/audit')).at(-1)![0]).not.toContain('cursor=');
  });
  it('offers retry after the initial case and case-list requests fail', async () => {
    let fail = true;
    mock.disclosure.mockImplementation(async (path:string) => {
      if (path.includes('/audit')) return {items:[],nextCursor:''};
      if (fail) throw Error('Transport interrupted');
      return currentCase();
    });
    const view = mountCase(); await screen.findByRole('alert'); fail = false;
    fireEvent.click(screen.getByRole('button',{name:'Retry case'})); expect(await screen.findByText('Review')).toBeInTheDocument(); view.unmount();
    mock.request.mockResolvedValue([]); fail = true;
    mock.disclosure.mockImplementation(async () => { if(fail) throw Error('Transport interrupted'); return {items:[currentCase()],nextCursor:'',scanComplete:true}; });
    render(<MemoryRouter><DisclosuresV2/></MemoryRouter>); await screen.findByRole('alert'); fail = false;
    fireEvent.click(screen.getByRole('button',{name:'Retry cases'})); expect(await screen.findByRole('link',{name:'Open case'})).toBeInTheDocument();
  });
  it('shows returned scope identifiers, named wallet limits and inclusive time bounds without ordinary lookups', async () => {
    const address='0x'+'b'.repeat(40), record=currentCase(); record.scope.addresses={mode:'list',values:[address]};
    mock.disclosure.mockImplementation(async (path:string)=>path.includes('/audit')?{items:[],nextCursor:''}:record); mountCase();
    expect(await screen.findByText('bounded-review')).toBeInTheDocument();
    expect(document.querySelector('[data-label="Data organization"]')).toHaveTextContent(caseId);
    expect(document.querySelector('[data-label="Target actors"]')).toHaveTextContent(mock.state.identity.actor);
    expect(document.querySelector('[data-label="Wallet limit"]')).toHaveTextContent(address);
    expect(document.querySelector('[data-label="Data time (inclusive)"]')).toHaveTextContent('–');
    expect(document.querySelector(`[href*="${address}"]`)).toBeNull(); expect(mock.request).not.toHaveBeenCalled();
  });
  it('retires data and disables stored-open resource controls at the original case deadline, even after refresh returns a later expiry', async () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date('2030-03-17T00:00:00Z'));
    const record=currentCase(), hash='0x'+'a'.repeat(64); let refreshed=false;
    mock.disclosure.mockImplementation(async (path:string,options?:RequestInit)=>path.includes('/audit')?{items:[],nextCursor:''}:options?.method==='POST'?{v:1,readId:caseId,caseId,resource:'transactions',view:'list',level:'full',effectiveUntil:record.expiresAt,coverage:{status:'RETAINED_AT_READ',sourceBasisDigest:'sha256:'+'a'.repeat(64),preparedAt:record.createdAt},result:{items:[{blockNumber:'1',blockTime:record.createdAt,transactionIndex:0,gasUsed:'21000',status:1,direction:'outgoing',from:'0x'+'1'.repeat(40),to:null,transactionHash:hash,addressId:'da1.render-test.3AVmeb8xFDUr-vAhQHJYIg',value:'7'}],nextCursor:null}}:{...record,expiresAt:record.expiresAt+(refreshed?600:0)});
    mountCase(); await act(async()=>{await Promise.resolve();await Promise.resolve();}); fireEvent.click(screen.getByRole('button',{name:'Transactions',exact:true})); await act(async()=>{await Promise.resolve();await Promise.resolve();}); expect(screen.getByTestId('disclosure-transaction-row')).toBeInTheDocument();
    refreshed=true;fireEvent.click(screen.getByRole('button',{name:'Refresh case'}));await act(async()=>{await Promise.resolve();await Promise.resolve();});expect(screen.queryByTestId('disclosure-transaction-row')).toBeNull();
    act(()=>vi.advanceTimersByTime(3000)); expect(screen.getByRole('button',{name:'Transactions',exact:true})).toBeDisabled();expect(within(document.querySelector('[data-label="Status"]') as HTMLElement).getByText('Expired')).toBeInTheDocument();expect(screen.getByRole('status')).toHaveTextContent('original case or application session deadline');
    expect(screen.queryByText('Revoke this case')).toBeNull();
  });
  it('updates dashboard stored-open status when the case expires before the live identity', async () => {
    vi.useFakeTimers();vi.setSystemTime(new Date('2030-03-17T00:00:00Z'));const record=currentCase();
    mock.request.mockResolvedValue([]);mock.disclosure.mockResolvedValue({items:[record],nextCursor:'',scanComplete:true});render(<MemoryRouter><DisclosuresV2/></MemoryRouter>);
    await act(async()=>{await Promise.resolve();await Promise.resolve();});expect(screen.getByText('open')).toBeInTheDocument();act(()=>vi.advanceTimersByTime(3000));expect(screen.getByText('Expired')).toBeInTheDocument();
  });
});

describe('standing-rule case creation', () => {
  it.each([['transactions'], ['events', 'transactions']])('opens transaction details with declared resources %j without inventing unsupported reads', async (...resources) => {
    const id = '11111111-1111-4111-8111-111111111111', alias = 'da1.render-test.3AVmeb8xFDUr-vAhQHJYIg', hash = '0x' + 'a'.repeat(64);
    const record = { caseId: id, ruleId: 'transactions-only', creatorActorId: mock.state.identity.actor, recipientActorId: mock.state.identity.actor, dataOrganizationId: id, scope: { targetActors: [mock.state.identity.actor], resources, addresses: { mode: 'all-target-addresses', values: [] }, dataTime: { start: 1900000000, end: 1900000100 }, level: 'full' }, caseReference: 'audit-123', purpose: 'Investigation', legalBasis: 'Internal control', status: 'open', version: 1, createdAt: 1900000000, expiresAt: 1900000300 };
    mock.disclosure.mockImplementation(async (path: string, options?: RequestInit) => {
      if (path.includes('/audit')) return { items: [], nextCursor: '', scanComplete: true };
      if (options?.method !== 'POST') return record;
      return { v: 1, readId: id, caseId: id, resource: 'transactions', view: 'list', level: 'full', effectiveUntil: 1900000300, coverage: { status: 'RETAINED_AT_READ', sourceBasisDigest: 'sha256:' + 'a'.repeat(64), preparedAt: 1900000000 }, result: { items: [{ blockNumber: '1', blockTime: 1900000000, transactionIndex: 0, gasUsed: '21000', status: 1, direction: 'outgoing', from: '0x' + '1'.repeat(40), to: null, transactionHash: hash, addressId: alias, value: '7' }], nextCursor: null } };
    });
    render(<MemoryRouter initialEntries={['/disclosures/cases/' + id]}><Routes><Route path="/disclosures/cases/:caseId" element={<DisclosureCaseV2 />} /></Routes></MemoryRouter>);
    fireEvent.click(await screen.findByRole('button', { name: 'Transactions' }));
    const details = await screen.findByRole('link', { name: 'Details' });
    expect(details.getAttribute('href')).toBe(`/disclosures/cases/${id}/addresses/${alias}/transactions/${hash}`);
    expect(screen.queryByRole('button', { name: 'Addresses' })).toBeNull();
    if(resources.includes('events'))expect(screen.getByRole('button',{name:'Events'})).toBeEnabled();
    expect(mock.disclosure.mock.calls.every(([, options])=>!options?.body||JSON.parse(String(options.body)).resource!=='events')).toBe(true);
  });
  it('supports rules restricted to named wallets and preserves the request after an uncertain result', async () => {
    const bodies: string[] = [];
    mock.request.mockResolvedValue([]);
    mock.disclosure.mockImplementation(async (_path: string, options?: RequestInit) => {
      if (options?.method !== 'POST') return { items: [], nextCursor: '', scanComplete: true };
      bodies.push(String(options.body));
      if (bodies.length === 1) throw Error('The response was interrupted. Try the same request again.');
      const body = JSON.parse(String(options.body));
      return { ...body, caseId: '11111111-1111-4111-8111-111111111111', creatorActorId: mock.state.identity.actor, dataOrganizationId: '22222222-2222-4222-8222-222222222222', status: 'open', version: 1, createdAt: Math.floor(Date.now() / 1000) };
    });
    render(<MemoryRouter><DisclosuresV2 /></MemoryRouter>);
    fireEvent.click(screen.getByText('Open a disclosure case'));
    fireEvent.change(screen.getByLabelText('Approved rule ID'), { target: { value: 'named-wallet-audit' } });
    fireEvent.change(screen.getByLabelText('Client actor IDs'), { target: { value: mock.state.identity.actor } });
    fireEvent.change(screen.getByLabelText(/Limit to these wallet addresses/), { target: { value: '0x' + 'B'.repeat(40) + ',0x' + 'a'.repeat(40) } });
    fireEvent.change(screen.getByLabelText('Case reference'), { target: { value: 'audit-123' } });
    fireEvent.change(screen.getByLabelText('Purpose'), { target: { value: 'Internal investigation' } });
    fireEvent.change(screen.getByLabelText('Legal basis'), { target: { value: 'Approved internal control' } });
    fireEvent.click(screen.getByRole('button', { name: 'Open case' }));
    await screen.findByRole('alert');
    fireEvent.click(screen.getByRole('button', { name: 'Open case' }));
    await waitFor(() => expect(bodies).toHaveLength(2));
    expect(bodies[1]).toBe(bodies[0]);
    const body = JSON.parse(bodies[0]);
    expect(body.scope.addresses).toEqual({ mode: 'list', values: ['0x' + 'a'.repeat(40), '0x' + 'b'.repeat(40)] });
    expect(body.recipientActorId).toBe(mock.state.identity.actor);
    expect(body).not.toHaveProperty('approval');
    expect(body.idempotencyKey).toMatch(/^[0-9a-f-]{36}$/);
  });
});
