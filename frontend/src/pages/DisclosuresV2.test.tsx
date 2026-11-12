import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { DisclosureCaseV2, DisclosuresV2 } from './DisclosuresV2';

const mock = vi.hoisted(() => {
  const actor = 'ops:p1:' + 'A'.repeat(43);
  const state = { identity: { actor, profile: 'azure', activation: '11111111-1111-4111-8111-111111111111', expires: 2000000000 }, loading: false, changed: false, error: null };
  return { state, disclosure: vi.fn(), request: vi.fn() };
});
vi.mock('../lib/v2Session', () => ({ appSession: { snapshot: () => mock.state, subscribe: () => () => {}, disclosure: mock.disclosure, request: mock.request } }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe('standing-rule case creation', () => {
  it('opens transaction details when the case grants transactions without address browsing', async () => {
    const id = '11111111-1111-4111-8111-111111111111', alias = 'da1.render-test.3AVmeb8xFDUr-vAhQHJYIg', hash = '0x' + 'a'.repeat(64);
    const record = { caseId: id, ruleId: 'transactions-only', creatorActorId: mock.state.identity.actor, recipientActorId: mock.state.identity.actor, dataOrganizationId: id, scope: { targetActors: [mock.state.identity.actor], resources: ['transactions'], addresses: { mode: 'all-target-addresses', values: [] }, dataTime: { start: 1900000000, end: 1900000100 }, level: 'full' }, caseReference: 'audit-123', purpose: 'Investigation', legalBasis: 'Internal control', status: 'open', version: 1, createdAt: 1900000000, expiresAt: 1900000300 };
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
