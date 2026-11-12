export const disclosureResources = ['addresses', 'transactions', 'activity_logs', 'activity_summary', 'sanctions_report', 'compliance_report'] as const;
export type DisclosureResource = typeof disclosureResources[number];
export type DisclosureLevel = 'redacted' | 'pseudonymous' | 'full';
export interface DisclosureScope { targetActors: string[]; resources: DisclosureResource[]; addresses: { mode: 'all-target-addresses' | 'list'; values: string[] }; dataTime: { start: number; end: number }; level: DisclosureLevel }
export interface DisclosureCase { caseId: string; ruleId: string; recipientActorId: string; creatorActorId: string; dataOrganizationId: string; scope: DisclosureScope; caseReference: string; purpose: string; legalBasis: string; status: 'open' | 'revoked'; version: number; createdAt: number; expiresAt: number }
export interface CasePage { items: DisclosureCase[]; nextCursor: string; scanComplete: boolean }
export interface DataAddress { addressId: string; display: string; address?: string; isMine?: boolean }
export interface DataTransaction { blockNumber: string; transactionIndex: number; blockTime: number; gasUsed: string; status: number; direction: string; from: string; to: string | null; transactionHash?: string; value?: string; gasPrice?: string; gasLimit?: string; nonce?: string; input?: string; type?: number }
export interface DataActivity { recordId: string; method: string; statusCode: number; time: number; actorId?: string; ipAddress?: string }
export interface DataSummary { total: string; successful: string; failed: string; methodBreakdown: { method: string; total: string; successful: string; failed: string }[] }
export interface DataSanctions { status: 'clear' | 'hit' | 'not-applicable'; scopedAddressCount: number; checkedAt: number; matches: { address: DataAddress; sourceId: string; reason?: string }[] }
export interface DataEvidence { id: string; time: number; kind: string; decision: string; monitored: boolean; coverage: string; transactionHash?: string; reason?: string }
export interface DataCompliance { activity: DataSummary; sanctions: DataSanctions; kyc: 'not_evaluated'; evidence: DataEvidence[] }
export interface DataPage<T> { items: T[]; nextCursor: string | null }
export interface DataEnvelope { v: 1; readId: string; caseId: string; resource: DisclosureResource; view: 'list' | 'resolve' | 'detail' | 'report'; level: DisclosureLevel; effectiveUntil: number; coverage: { status: 'COMPLETE_AT_CUT' | 'RETAINED_AT_READ'; preparedAt: number }; result: DataPage<DataAddress | DataTransaction | DataActivity> | { item: DataAddress | DataTransaction } | DataSummary | DataSanctions | DataCompliance }

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const actorPattern = /^ops:p1:[A-Za-z0-9_-]{43}$/;
const addressPattern = /^0x[0-9a-f]{40}$/;
const hashPattern = /^0x[0-9a-f]{64}$/;
const aliasPattern = /^da1\.[A-Za-z0-9_-]{1,64}\.[A-Za-z0-9_-]{22}$/;
const digestPattern = /^sha256:[0-9a-f]{64}$/;
const invalid = (): never => { throw Error('The disclosure response is invalid. Refresh the case before continuing.'); };
function object(value: unknown): Record<string, unknown> { if (!value || typeof value !== 'object' || Array.isArray(value)) return invalid(); return value as Record<string, unknown>; }
function text(value: unknown, max = 4096): string { if (typeof value !== 'string' || value.length > max) return invalid(); return value; }
function integer(value: unknown, max = 253402300799): number { if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0 || value > max) return invalid(); return value; }
function identifier(value: unknown): string { const s = text(value, 36); if (!uuidPattern.test(s) || s === '00000000-0000-0000-0000-000000000000') return invalid(); return s; }
function actor(value: unknown): string { const s = text(value, 50); if (!actorPattern.test(s)) return invalid(); return s; }
function list<T>(value: unknown, max: number, parse: (item: unknown) => T): T[] { if (!Array.isArray(value) || value.length > max) return invalid(); return value.map(parse); }
function level(value: unknown): DisclosureLevel { if (value !== 'full' && value !== 'pseudonymous' && value !== 'redacted') return invalid(); return value; }
function resource(value: unknown): DisclosureResource { if (!(disclosureResources as readonly unknown[]).includes(value)) return invalid(); return value as DisclosureResource; }
function quantity(value: unknown): string { const s = text(value, 78); if (!/^(?:0|[1-9][0-9]*)$/.test(s)) return invalid(); return s; }
function optional<T>(value: unknown, parse: (item: unknown) => T): T | undefined { return value === undefined ? undefined : parse(value); }
function pattern(value: unknown, re: RegExp, max: number): string { const s = text(value, max); if (!re.test(s)) return invalid(); return s; }
function scope(value: unknown): DisclosureScope {
  const v = object(value), addresses = object(v.addresses), time = object(v.dataTime);
  if (addresses.mode !== 'all-target-addresses' && addresses.mode !== 'list') return invalid();
  const out: DisclosureScope = { targetActors: list(v.targetActors, 256, actor), resources: list(v.resources, 6, resource), addresses: { mode: addresses.mode, values: list(addresses.values, 1024, item => pattern(item, addressPattern, 42)) }, dataTime: { start: integer(time.start), end: integer(time.end) }, level: level(v.level) };
  if (!out.targetActors.length || !out.resources.length || out.dataTime.start > out.dataTime.end) return invalid(); return out;
}
export function parseDisclosureCase(value: unknown): DisclosureCase {
  const v = object(value); if (v.status !== 'open' && v.status !== 'revoked') return invalid();
  return { caseId: identifier(v.caseId), ruleId: text(v.ruleId, 128), recipientActorId: actor(v.recipientActorId), creatorActorId: actor(v.creatorActorId), dataOrganizationId: identifier(v.dataOrganizationId), scope: scope(v.scope), caseReference: text(v.caseReference), purpose: text(v.purpose), legalBasis: text(v.legalBasis), status: v.status, version: integer(v.version, Number.MAX_SAFE_INTEGER), createdAt: integer(v.createdAt), expiresAt: integer(v.expiresAt) };
}
export function parseCasePage(value: unknown): CasePage { const v = object(value); if (typeof v.scanComplete !== 'boolean') return invalid(); return { items: list(v.items, 100, parseDisclosureCase), nextCursor: text(v.nextCursor, 8192), scanComplete: v.scanComplete }; }
function address(value: unknown, selected: DisclosureLevel): DataAddress {
  const v = object(value); if (selected !== 'full' && (v.address !== undefined || v.targetActorIds !== undefined || v.ensName !== undefined) || selected === 'redacted' && v.isMine !== undefined) return invalid();
  const mine = v.isMine; if (mine !== undefined && typeof mine !== 'boolean') return invalid();
  return { addressId: pattern(v.addressId, aliasPattern, 128), display: text(v.display, 256), address: optional(v.address, v => pattern(v, addressPattern, 42)), isMine: mine as boolean | undefined };
}
function transaction(value: unknown, selected: DisclosureLevel, detail = false): DataTransaction {
  const v = object(value); if (selected !== 'full' && (v.transactionHash !== undefined || v.value !== undefined) || detail && selected !== 'full') return invalid();
  if (!['incoming', 'outgoing', 'self'].includes(String(v.direction))) return invalid();
  const out: DataTransaction = { blockNumber: quantity(v.blockNumber), transactionIndex: integer(v.transactionIndex, 4294967295), blockTime: integer(v.blockTime), gasUsed: quantity(v.gasUsed), status: integer(v.status, 1), direction: String(v.direction), from: text(v.from, 256), to: v.to === null ? null : text(v.to, 256), transactionHash: optional(v.transactionHash, x => pattern(x, hashPattern, 66)), value: optional(v.value, quantity) };
  if (detail) { out.gasPrice = quantity(v.gasPrice); out.gasLimit = quantity(v.gasLimit); out.nonce = quantity(v.nonce); out.type = integer(v.type, 4); out.input = pattern(v.input, /^0x(?:[0-9a-f]{2})*$/, 1 << 20); }
  return out;
}
function activity(value: unknown, selected: DisclosureLevel): DataActivity { const v = object(value); if (selected !== 'full' && (v.actorId !== undefined || v.ipAddress !== undefined)) return invalid(); return { recordId: identifier(v.recordId), method: text(v.method, 100), statusCode: integer(v.statusCode, 599), time: integer(v.time), actorId: optional(v.actorId, actor), ipAddress: optional(v.ipAddress, x => text(x, 45)) }; }
function summary(value: unknown): DataSummary { const v = object(value); return { total: quantity(v.total), successful: quantity(v.successful), failed: quantity(v.failed), methodBreakdown: list(v.methodBreakdown, 512, item => { const m = object(item); return { method: text(m.method, 100), total: quantity(m.total), successful: quantity(m.successful), failed: quantity(m.failed) }; }) }; }
function sanctions(value: unknown, selected: DisclosureLevel): DataSanctions { const v = object(value); if (v.status !== 'clear' && v.status !== 'hit' && v.status !== 'not-applicable') return invalid(); return { status: v.status, scopedAddressCount: integer(v.scopedAddressCount, 4096), checkedAt: integer(v.checkedAt), matches: list(v.matches, 4096, item => { const match = object(item); if (selected !== 'full' && match.reason !== undefined) return invalid(); return { address: address(match.address, selected), sourceId: identifier(match.sourceId), reason: optional(match.reason, x => text(x, 1 << 20)) }; }) }; }
function compliance(value: unknown, selected: DisclosureLevel): DataCompliance { const v = object(value); if (v.kyc !== 'not_evaluated') return invalid(); return { activity: summary(v.activity), sanctions: sanctions(v.sanctions, selected), kyc: 'not_evaluated', evidence: list(v.evidence, 100, item => { const e = object(item); if (e.population !== 'ADMISSION_DECISION' || typeof e.monitored !== 'boolean' || selected !== 'full' && (e.transactionHash !== undefined || e.reason !== undefined || e.quote !== undefined || e.recordId !== undefined)) return invalid(); return { id: quantity(e.id), time: integer(e.time), kind: text(e.kind, 64), decision: text(e.decision, 16), monitored: e.monitored, coverage: text(e.coverage, 64), transactionHash: optional(e.transactionHash, x => pattern(x, hashPattern, 66)), reason: optional(e.reason, x => text(x, 2048)) }; }) }; }
export function parseDisclosureData(value: unknown, caseId: string, expected: DisclosureResource): DataEnvelope {
  const v = object(value), selected = level(v.level), actual = resource(v.resource), coverage = object(v.coverage);
  if (v.v !== 1 || v.caseId !== caseId || actual !== expected || !['list', 'resolve', 'detail', 'report'].includes(String(v.view))) return invalid();
  const retained = ['addresses', 'transactions', 'sanctions_report', 'compliance_report'].includes(actual);
  if (retained ? coverage.status !== 'RETAINED_AT_READ' || coverage.sourceCutDigest !== undefined || !digestPattern.test(String(coverage.sourceBasisDigest)) : coverage.status !== 'COMPLETE_AT_CUT' || coverage.sourceBasisDigest !== undefined || !digestPattern.test(String(coverage.sourceCutDigest))) return invalid();
  let result: DataEnvelope['result']; const body = object(v.result);
  if (v.view === 'list') { const parse: (item: unknown) => DataAddress | DataTransaction | DataActivity = actual === 'addresses' ? (x: unknown) => address(x, selected) : actual === 'transactions' ? (x: unknown) => transaction(x, selected) : actual === 'activity_logs' ? (x: unknown) => activity(x, selected) : invalid(); result = { items: list(body.items, 100, parse), nextCursor: body.nextCursor === null ? null : text(body.nextCursor, 2048) }; }
  else if (v.view === 'resolve' && actual === 'addresses') result = { item: address(body.item, selected) };
  else if (v.view === 'detail' && actual === 'transactions') result = { item: transaction(body.item, selected, true) };
  else if (v.view === 'report') result = actual === 'activity_summary' ? summary(body) : actual === 'sanctions_report' ? sanctions(body, selected) : actual === 'compliance_report' ? compliance(body, selected) : invalid();
  else return invalid();
  return { v: 1, readId: identifier(v.readId), caseId, resource: actual, view: v.view as DataEnvelope['view'], level: selected, effectiveUntil: integer(v.effectiveUntil), coverage: { status: coverage.status as DataEnvelope['coverage']['status'], preparedAt: integer(coverage.preparedAt) }, result };
}
export function displayDisclosureTime(seconds: number): string { return new Date(seconds * 1000).toLocaleString(); }
