import { describe, expect, it } from 'vitest';
import { parseCasePage, parseDisclosureData, type DataPage, type DataTransaction } from './v2Disclosure';

const id = '11111111-1111-4111-8111-111111111111', actor = 'ops:p1:' + 'A'.repeat(43), digest = 'sha256:' + 'a'.repeat(64);
const alias = 'da1.render-test.3AVmeb8xFDUr-vAhQHJYIg', hash = '0x' + 'a'.repeat(64);
function envelope(resource: string, view: string, result: unknown, level = 'full') {
  const retained = ['addresses', 'transactions', 'sanctions_report', 'compliance_report'].includes(resource);
  return { v: 1, readId: id, caseId: id, resource, view, level, effectiveUntil: 1900000300, coverage: { status: retained ? 'RETAINED_AT_READ' : 'COMPLETE_AT_CUT', [retained ? 'sourceBasisDigest' : 'sourceCutDigest']: digest, populations: ['TEST_POPULATION'], preparedAt: 1900000000 }, result };
}
const summary = { total: '18446744073709551615', successful: '18446744073709551615', failed: '0', methodBreakdown: [] };
const sanctions = { status: 'clear', scopedAddressCount: 0, checkedAt: 1900000000, sourceDigest: digest, matches: [] };
const transaction = { blockNumber: '9007199254740993', transactionIndex: 0, blockTime: 1900000000, gasUsed: '21000', status: 1, direction: 'outgoing', from: '0x' + '1'.repeat(40), to: null, transactionHash: hash, value: '115792089237316195423570985008687907853269984665640564039457584007913129639935' };
describe('protected disclosure browser wire', () => {
  it('reads all six resources and preserves huge amounts and empty-page continuations', () => {
    const address = { addressId: alias, display: 'Address-private', isMine: false };
    const entries: [Parameters<typeof parseDisclosureData>[2], string, unknown][] = [
      ['addresses', 'list', { items: [address], nextCursor: null }],
      ['transactions', 'list', { items: [transaction], nextCursor: 'opaque' }],
      ['activity_logs', 'list', { items: [{ recordId: id, method: 'explorer.transactions', statusCode: 200, time: 1900000000, actorId: actor }], nextCursor: null }],
      ['activity_summary', 'report', summary], ['sanctions_report', 'report', sanctions],
      ['compliance_report', 'report', { activity: summary, sanctions, kyc: 'not_evaluated', evidence: [] }],
    ];
    for (const [resource, view, result] of entries) expect(parseDisclosureData(envelope(resource, view, result), id, resource).resource).toBe(resource);
    const parsed = parseDisclosureData(envelope('transactions', 'list', entries[1][2]), id, 'transactions').result as DataPage<DataTransaction>;
    expect(parsed.items[0].value).toBe(transaction.value); expect(parsed.items[0].blockNumber).toBe(transaction.blockNumber); expect(parsed.items[0].to).toBeNull(); expect(parsed.nextCursor).toBe('opaque');
    expect(parseDisclosureData(envelope('addresses', 'list', { items: [], nextCursor: 'keep-going' }), id, 'addresses').result).toEqual({ items: [], nextCursor: 'keep-going' });
  });
  it('supports opaque resolution and Full-only mined detail without converting gas/nonce to Number', () => {
    expect(parseDisclosureData(envelope('addresses', 'resolve', { item: { addressId: alias, display: 'Address-private' } }), id, 'addresses').view).toBe('resolve');
    const detail = envelope('transactions', 'detail', { item: { ...transaction, gasPrice: '9007199254740993', gasLimit: '30000', nonce: '9007199254740994', type: 2, input: '0x', maxFeePerGas: '9999999999999999999' } });
    expect((parseDisclosureData(detail, id, 'transactions').result as { item: DataTransaction }).item.nonce).toBe('9007199254740994');
    expect(() => parseDisclosureData({ ...detail, level: 'pseudonymous' }, id, 'transactions')).toThrow();
  });
  it('rejects redaction leaks, wrong case/resource and false immutable-history coverage', () => {
    const raw = envelope('transactions', 'list', { items: [transaction], nextCursor: null });
    expect(() => parseDisclosureData({ ...raw, level: 'pseudonymous' }, id, 'transactions')).toThrow();
    expect(() => parseDisclosureData(raw, '22222222-2222-4222-8222-222222222222', 'transactions')).toThrow();
    expect(() => parseDisclosureData(raw, id, 'addresses')).toThrow();
    expect(() => parseDisclosureData({ ...raw, coverage: { status: 'COMPLETE_AT_CUT', sourceCutDigest: digest, preparedAt: 1900000000 } }, id, 'transactions')).toThrow();
    expect(() => parseDisclosureData(envelope('transactions', 'list', { items: [{ ...transaction, value: 9007199254740992 }], nextCursor: null }), id, 'transactions')).toThrow();
  });
  it('keeps operational case pagination separate from protected data pagination', () => {
    const record = { caseId: id, ruleId: 'audit', recipientActorId: actor, creatorActorId: actor, dataOrganizationId: id, scope: { targetActors: [actor], resources: ['transactions'], addresses: { mode: 'all-target-addresses', values: [] }, dataTime: { start: 1899999000, end: 1900000000 }, level: 'full' }, caseReference: 'Bank audit', purpose: 'Investigate', legalBasis: 'Internal control', status: 'open', version: 1, createdAt: 1900000000, expiresAt: 1900000300 };
    expect(parseCasePage({ items: [record], nextCursor: 'case-page', scanComplete: false }).items[0].scope.resources).toEqual(['transactions']);
    expect(() => parseCasePage({ items: [record], nextCursor: null, scanComplete: false })).toThrow();
  });
});
