import { describe, expect, it, vi } from 'vitest';
import actualDTO from './testdata/disclosure-events-go-dto.json';
import financeDTO from './testdata/finance-native-go-dto.json';
import { parseDisclosureData, type DisclosureCase } from './v2Disclosure';

const now = Math.floor(Date.now() / 1000), id = '11111111-1111-4111-8111-111111111111', digest = 'sha256:' + 'a'.repeat(64);
const time = { start: now - 100, end: now };
const event = (level = 'full') => ({ eventId: 'de1.data.' + 'A'.repeat(21) + 'Q', blockNumber: '9007199254740993', transactionIndex: 2, logIndex: 7, blockTime: now - 1, emitter: level === 'full' ? '0x' + '1'.repeat(40) : level === 'redacted' ? '[PRIVATE]' : 'Address-' + 'A'.repeat(26), payload: level === 'full' ? 'disclosed' : 'withheld', ...(level === 'full' ? { transactionHash: '0x' + 'a'.repeat(64), blockHash: '0x' + 'b'.repeat(64), topics: ['0x' + 'c'.repeat(64)], data: '0xdeadbeef' } : {}) });
const envelope = (level = 'full', empty = false) => ({ v: 1, readId: id, caseId: id, resource: 'events', view: 'list', level, dataTime: time, effectiveUntil: now + 900, coverage: { status: 'RETAINED_AT_READ', populations: ['CURRENT_PROVED_ASSOCIATIONS', 'RETAINED_SAME_ORG_EVENTS_FROM_DISCLOSED_TRANSACTIONS'], sourceBasisDigest: digest, preparedAt: now }, result: { v: 1, population: 'EVENTS_FROM_DISCLOSED_TRANSACTIONS', items: empty ? [] : [event(level)], nextCursor: empty ? 'continue-opaque' : null } });
const decode = (raw: unknown) => parseDisclosureData(raw, id, 'events', { view: 'list', limit: 50 });
describe('explicit case Events and native finance DTOs', () => {
  it('accepts unchanged actual Go Full, pseudonymous, redacted and empty-advancing Events fixtures', () => {
    const clock = vi.spyOn(Date, 'now').mockReturnValue(actualDTO.full.coverage.preparedAt * 1000);
    try {
      for (const raw of Object.values(actualDTO)) expect(parseDisclosureData(raw, raw.caseId, 'events', { view: 'list', limit: 50, sessionExpiresAt: raw.coverage.preparedAt + 30 }).result).toEqual(raw.result);
    } finally { clock.mockRestore(); }
  });
  it.each(['full', 'pseudonymous', 'redacted'])('accepts the closed %s EventDTO and an empty advancing page', level => {
    expect(decode(envelope(level)).result).toEqual(envelope(level).result);
    expect(decode(envelope(level, true)).result).toEqual(envelope(level, true).result);
  });
  it('rejects all lower payload, decoded fields and invented wallet-emitter authority', () => {
    for (const level of ['redacted', 'pseudonymous']) for (const field of ['transactionHash', 'blockHash', 'topics', 'data', 'decoded', 'addressId', 'isMine', 'targetActorIds']) {
      const value = envelope(level); Object.assign(value.result.items[0], { [field]: null }); expect(() => decode(value)).toThrow();
    }
  });
  it('requires full raw fields, canonical IDs, bounds, order and exact Events populations', () => {
    const changes: ((value: any) => void)[] = [
      value => { delete value.result.items[0].topics; }, value => { delete value.result.items[0].data; },
      value => { value.result.items[0].blockHash = '0x' + '0'.repeat(64); }, value => { value.result.items[0].data = '0xabc'; },
      value => { value.result.items[0].topics = Array(5).fill('0x' + 'a'.repeat(64)); }, value => { value.result.items[0].logIndex = 4294967296; },
      value => { value.result.items[0].eventId = 'de1.data.' + 'A'.repeat(21) + 'B'; }, value => { value.result.items[0].blockTime = now + 1; },
      value => { value.result.items.push(value.result.items[0]); }, value => { value.result.privateProgress = 'secret'; },
      value => { value.coverage.populations[1] = 'RETAINED_AND_POSITIVELY_ATTRIBUTED_WALLET_HISTORY'; }, value => { value.coverage.sourceCutDigest = digest; },
      value => { value.result.population = 'ALL_WALLET_EVENTS'; }, value => { value.raw = 'secret'; },
    ];
    for (const change of changes) { const value = envelope(); change(value); expect(() => decode(value)).toThrow(); }
    const emptyPayload = envelope(); Object.assign(emptyPayload.result.items[0], { topics: [], data: '0x' }); expect(() => decode(emptyPayload)).not.toThrow();
  });
  it('accepts unchanged native Go finance serializer fixtures and rejects the obsolete inline report', () => {
    const clock = vi.spyOn(Date, 'now').mockReturnValue(financeDTO.report.coverage.preparedAt * 1000);
    try {
      for (const raw of [financeDTO.report, financeDTO.list]) {
        const record = { caseId: raw.caseId, expiresAt: raw.effectiveUntil, scope: { targetActors: [], resources: ['compliance_report'], level: raw.level, addresses: { mode: 'all-target-addresses', values: [] }, dataTime: raw.dataTime } } as unknown as DisclosureCase;
        expect(parseDisclosureData(raw, raw.caseId, 'compliance_report', { record, view: raw.view as 'list' | 'report', limit: 50 }).result).toEqual(raw.result);
      }
      expect(() => parseDisclosureData({ ...financeDTO.report, result: { activity: financeDTO.report.result.activity, sanctions: financeDTO.report.result.sanctions, kyc: 'not_evaluated', evidence: [] } }, id, 'compliance_report')).toThrow();
    } finally { clock.mockRestore(); }
  });
  it('keeps live counts above 16384 and nanosecond dates without cohort-equality claims', () => {
    const raw = structuredClone(financeDTO.list);
    raw.result.matchingRetainedCount = '9223372036854775807'; raw.result.observedAt = raw.result.observedAt.replace('Z', '.123456789Z');
    expect(parseDisclosureData(raw, id, 'compliance_report').result).toEqual(raw.result);
    raw.result.matchingRetainedCount = '9223372036854775808'; expect(() => parseDisclosureData(raw, id, 'compliance_report')).toThrow();
  });
});
