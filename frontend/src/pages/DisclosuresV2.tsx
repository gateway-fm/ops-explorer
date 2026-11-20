import { PageHeader } from '../components/PageHeader';
import { InfoRow } from '../components/TransactionDetailPresentation';
import { DisclosurePanelPresentation } from '../components/DisclosurePanelPresentation';
import { DisclosureEventCards } from '../components/DisclosureEventCards';
import { useEffect, useRef, useState, useSyncExternalStore, type FormEvent, type ReactNode } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { appSession } from '../lib/v2Session';
import { ownWallets, type OwnWallet } from '../lib/v2Wallet';
import { disclosureResources, displayDisclosureTime, parseCasePage, parseDisclosureCase, parseDisclosureData, type CasePage, type DataActivity, type DataAddress, type DataCompliance, type DataEvent, type DataFinancePage, type DataEnvelope, type DataPage, type DataSanctions, type DataSummary, type DataTransaction, type DisclosureCase, type DisclosureLevel, type DisclosureResource } from '../lib/v2Disclosure';

import { readNarrowing } from '../lib/v2DisclosureFinance';

const labels: Record<DisclosureResource, string> = { addresses: 'Addresses', transactions: 'Transactions', activity_logs: 'Activity', activity_summary: 'Activity summary', sanctions_report: 'Sanctions', compliance_report: 'Compliance', events: 'Events' };
function failure(error: unknown): string { return error instanceof Error ? error.message : 'The request did not complete. Refresh before trying again.'; }
function useIdentity() { return useSyncExternalStore(appSession.subscribe, appSession.snapshot).identity; }
function useRemote<T>(endpoint: string, parse: (value: unknown) => T, body?: string, wallet = false) {
  const identity = useIdentity(), parser = useRef(parse); parser.current = parse;
  const [version, setVersion] = useState(0), [state, setState] = useState<{ data?: T; error?: string; loading: boolean }>({ loading: true });
  useEffect(() => {
    const abort = new AbortController(); let live = true;
    setState({ loading: true });
    const options: RequestInit = body === undefined ? { signal: abort.signal } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body, signal: abort.signal };
    const request = wallet ? appSession.request<unknown>(endpoint, options) : appSession.disclosure<unknown>(endpoint, options);
    void request.then(value => parser.current(value)).then(data => { if (live) setState({ data, loading: false }); }).catch(error => { if (live) setState({ error: failure(error), loading: false }); });
    return () => { live = false; abort.abort(); };
  }, [endpoint, body, wallet, identity?.activation, version]);
  return { ...state, reload: () => setVersion(n => n + 1) };
}
function Status({ loading, error }: { loading: boolean; error?: string }) { return <>{loading && <p role="status" className="text-sm text-neutral-500">Loading…</p>}{error && <p role="alert" className="rounded-lg border border-error-200 bg-error-50 p-3 text-sm text-error-700">{error}</p>}</>; }
function Table({ headings, children }: { headings: string[]; children: ReactNode }) { return <div className="table-container"><table className={headings.length > 4 ? 'table min-w-[48rem]' : 'table'}><thead><tr>{headings.map(h => <th className="px-4 py-3" key={h}>{h}</th>)}</tr></thead><tbody>{children}</tbody></table></div>; }
function Cell({ children }: { children?: ReactNode }) { return <td className="px-4 py-3 align-top break-all text-sm">{children ?? '—'}</td>; }
function query(cursor: string | null) { return cursor ? `?limit=50&cursor=${encodeURIComponent(cursor)}` : '?limit=50'; }

export function DisclosuresV2() { const identity = useIdentity(); return identity ? <Dashboard key={identity.activation} /> : null; }
function Dashboard() {
  const [cursor, setCursor] = useState<string | null>(null);
  const cases = useRemote<CasePage>('/viewable-addresses' + query(cursor), parseCasePage);
  const wallets = useRemote<OwnWallet[]>('/eth/addresses', ownWallets, undefined, true);
  return <section className="space-y-5"><PageHeader title="Auditor" subtitle="Approved disclosure cases available to your current identity"/><p className="text-sm text-neutral-500">Access follows approved rules. Each permitted disclosure is checked and recorded without asking the client to approve it.</p>
    <DisclosurePanelPresentation title="Your wallets"><div className="p-4 space-y-3 text-sm"><Status {...wallets} />{wallets.data && <ul>{wallets.data.map(w => <li key={w.address} className="break-all"><Link className="text-primary hover:underline" to={'/addresses/' + w.address}><code>{w.address}</code></Link></li>)}</ul>}<Link className="text-primary" to="/wallets">Manage wallet links</Link></div></DisclosurePanelPresentation>
    <DisclosurePanelPresentation title="Your cases"><div className="p-4 space-y-3 text-sm"><Status {...cases} />{cases.data && <><Table headings={['Reference', 'Purpose', 'Status', 'Expires', '']}>
      {cases.data.items.map(c => <tr key={c.caseId}><Cell>{c.caseReference}</Cell><Cell>{c.purpose}</Cell><Cell>{c.status}</Cell><Cell>{displayDisclosureTime(c.expiresAt)}</Cell><Cell><Link className="btn-secondary whitespace-nowrap" to={'/disclosures/cases/' + c.caseId}>Open case</Link></Cell></tr>)}
    </Table>{cases.data.items.length === 0 && <p>No cases on this page.</p>}<div className="flex flex-wrap gap-2"><button className="btn-secondary" onClick={() => { setCursor(null); cases.reload(); }}>Refresh cases</button><button className="btn-secondary" disabled={!cases.data.nextCursor} onClick={() => setCursor(cases.data!.nextCursor)}>Next cases</button></div></>}</div></DisclosurePanelPresentation>
    <CreateCase onCreated={() => { setCursor(null); cases.reload(); }} />
  </section>;
}

function localDate(seconds: number) { const date = new Date(seconds * 1000); return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16); }
function CreateCase({ onCreated }: { onCreated: () => void }) {
  const identity = useIdentity(), navigate = useNavigate(), ticket = useRef(0), attempt = useRef<{ signature: string; body: string }>(undefined);
  const now = Math.floor(Date.now() / 1000);
  const [rule, setRule] = useState(''), [targets, setTargets] = useState(''), [addresses, setAddresses] = useState(''), [reference, setReference] = useState(''), [purpose, setPurpose] = useState(''), [legal, setLegal] = useState('');
  const [from, setFrom] = useState(localDate(now - 3600)), [through, setThrough] = useState(localDate(now)), [minutes, setMinutes] = useState(15), [selected, setSelected] = useState<DisclosureResource[]>(disclosureResources.filter(resource => resource !== 'events')), [level, setLevel] = useState<DisclosureLevel>('pseudonymous');
  const [busy, setBusy] = useState(false), [error, setError] = useState<string>();
  useEffect(() => () => { ++ticket.current; }, []);
  const submit = async (event: FormEvent) => {
    event.preventDefault(); if (!identity || busy) return; const current = ++ticket.current; setBusy(true); setError(undefined);
    try {
      const start = Math.floor(new Date(from).getTime() / 1000), end = Math.floor(new Date(through).getTime() / 1000), actors = [...new Set(targets.split(/[\s,]+/).filter(Boolean))].sort();
      if (!actors.length || actors.length > 256 || actors.some(a => !/^ops:p1:[A-Za-z0-9_-]{43}$/.test(a)) || !selected.length || !Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start > end || !Number.isSafeInteger(minutes) || minutes < 1 || minutes > 43200) throw Error('Check the client identifiers, dates, resources and expiry.');
      const values = [...new Set(addresses.split(/[\s,]+/).filter(Boolean).map(a => a.toLowerCase()))].sort();
      if (values.length > 1024 || values.some(a => !/^0x[0-9a-f]{40}$/.test(a))) throw Error('Use complete wallet addresses for the address limit.');
      const request = { ruleId: rule.trim(), recipientActorId: identity.actor, caseReference: reference, purpose, legalBasis: legal, scope: { targetActors: actors, resources: [...selected].sort(), addresses: { mode: values.length ? 'list' : 'all-target-addresses', values }, dataTime: { start, end }, level } };
      const signature = JSON.stringify({ request, minutes });
      if (attempt.current?.signature !== signature) attempt.current = { signature, body: JSON.stringify({ ...request, expiresAt: Math.floor(Date.now() / 1000) + minutes * 60, idempotencyKey: crypto.randomUUID() }) };
      const raw = await appSession.disclosure<unknown>('/disclosure/cases', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: attempt.current.body });
      const result = parseDisclosureCase(raw); if (current !== ticket.current) return; onCreated(); navigate('/disclosures/cases/' + result.caseId);
    } catch (e) { if (current === ticket.current) setError(failure(e)); }
    finally { if (current === ticket.current) setBusy(false); }
  };
  return <details className="card p-5"><summary className="cursor-pointer font-medium text-primary">Open a disclosure case</summary><form className="space-y-4 pt-4 text-sm" onSubmit={submit}>
    <p>The approved rule limits the clients, dates and detail level you can request.</p>
    <label className="block text-sm font-medium text-neutral-700">Approved rule ID<input required maxLength={128} value={rule} onChange={e => setRule(e.target.value)} className="input mt-1" /></label>
    <label className="block text-sm font-medium text-neutral-700">Client actor IDs<textarea required value={targets} onChange={e => setTargets(e.target.value)} placeholder="ops:p1:…" className="input mt-1" /></label>
    <label className="block text-sm font-medium text-neutral-700">Limit to these wallet addresses (optional)<textarea value={addresses} onChange={e => setAddresses(e.target.value)} placeholder="0x…" className="input mt-1" /><span className="mt-1 block text-xs font-normal text-neutral-500">Leave empty to request all eligible wallets of these clients. For a rule limited to named wallets, enter the permitted addresses.</span></label>
    <label className="block text-sm font-medium text-neutral-700">Case reference<input required maxLength={256} value={reference} onChange={e => setReference(e.target.value)} className="input mt-1" /></label>
    <label className="block text-sm font-medium text-neutral-700">Purpose<input required maxLength={1024} value={purpose} onChange={e => setPurpose(e.target.value)} className="input mt-1" /></label>
    <label className="block text-sm font-medium text-neutral-700">Legal basis<input required maxLength={1024} value={legal} onChange={e => setLegal(e.target.value)} className="input mt-1" /></label>
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3"><label>From (local time)<input required type="datetime-local" value={from} onChange={e => setFrom(e.target.value)} className="input mt-1" /></label><label>Through (local time)<input required type="datetime-local" value={through} onChange={e => setThrough(e.target.value)} className="input mt-1" /></label><label>Case expires in minutes<input required type="number" min={1} max={43200} value={minutes} onChange={e => setMinutes(Number(e.target.value))} className="input mt-1" /></label></div>
    <fieldset><legend>Data</legend><div className="flex flex-wrap gap-4">{disclosureResources.map(r => <label key={r}><input type="checkbox" checked={selected.includes(r)} onChange={e => setSelected(old => e.target.checked ? [...old, r] : old.filter(x => x !== r))} /> {labels[r]}</label>)}</div></fieldset>
    {selected.includes('events') && <p className="text-neutral-500">Events needs explicit authority in the approved rule and case. It covers same-organization emitters in transactions whose sender or recipient is a target wallet. {level === 'full' ? 'Full Events reveals the whole event payload and may include other parties’ data.' : 'Topics, raw data and hashes will be withheld.'}</p>}
    <label>Detail level<select value={level} onChange={e => setLevel(e.target.value as DisclosureLevel)} className="input mt-1"><option value="redacted">Redacted</option><option value="pseudonymous">Pseudonymous</option><option value="full">Full</option></select></label>
    {error && <p role="alert" className="rounded-lg border border-error-200 bg-error-50 p-3 text-sm text-error-700">{error}</p>}<button className="btn-primary" disabled={busy} type="submit">{busy ? 'Opening case…' : 'Open case'}</button>
  </form></details>;
}

export function DisclosureCaseV2() { const identity = useIdentity(), { caseId = '' } = useParams(); return identity ? <CasePanel key={identity.activation + caseId} caseId={caseId} /> : null; }
function CasePanel({ caseId }: { caseId: string }) {
  const result = useRemote('/disclosure/cases/' + encodeURIComponent(caseId), parseDisclosureCase), [resource, setResource] = useState<DisclosureResource>(), [reason, setReason] = useState(''), [error, setError] = useState<string>(), [busy, setBusy] = useState(false);
  const revoke = async () => { if (busy || !reason.trim()) return; setBusy(true); setError(undefined); try { await appSession.disclosure('/disclosure/cases/' + caseId + '/revoke', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ reason }) }); setResource(undefined); result.reload(); } catch (e) { setError(failure(e)); } finally { setBusy(false); } };
  return <section className="space-y-5">
    <PageHeader title={result.data?.caseReference ?? 'Disclosure case'} subtitle="Access through an approved disclosure case"><Link className="btn-secondary" to="/disclosures">All disclosures</Link></PageHeader>
    <Status {...result} />{result.data && <>
    <DisclosurePanelPresentation title="Case details"><div className="divide-y divide-neutral-100">
      <InfoRow label="Purpose" value={result.data.purpose} />
      <InfoRow label="Legal basis" value={result.data.legalBasis} />
      <InfoRow label="Status" value={<span className={result.data.status === 'open' ? 'badge-success' : 'badge-neutral'}>{result.data.status}</span>} />
      <InfoRow label="Expires" value={displayDisclosureTime(result.data.expiresAt)} />
      <InfoRow label="Detail level" value={result.data.scope.level} />
    </div></DisclosurePanelPresentation>
    <div className="flex flex-wrap gap-2" role="group" aria-label="Case data">{result.data.scope.resources.map(r => <button className={resource === r ? 'btn-primary' : 'btn-secondary'} aria-pressed={resource === r} key={r} disabled={result.data?.status !== 'open'} onClick={() => setResource(r)}>{labels[r]}</button>)}<button className="btn-secondary" onClick={() => { setResource(undefined); result.reload(); }}>Refresh case</button></div>
    {resource && <ResourcePanel key={resource + result.data.version} record={result.data} resource={resource} />}
    <CaseAudit caseId={caseId} />
    {result.data.status === 'open' && <details className="card p-5"><summary className="cursor-pointer font-medium text-error-600">Revoke this case</summary><div className="mt-4 space-y-3"><p className="text-sm text-neutral-500">Revocation ends access through this case.</p><label className="block text-sm font-medium text-neutral-700">Revocation reason<input className="input mt-1" aria-label="Revocation reason" value={reason} maxLength={1024} onChange={e => setReason(e.target.value)} /></label><button className="btn-secondary text-error-600" disabled={busy || !reason.trim()} onClick={() => void revoke()}>Revoke case</button>{error && <p role="alert" className="rounded-lg border border-error-200 bg-error-50 p-3 text-sm text-error-700">{error}</p>}</div></details>}
  </>}</section>;
}

interface ParsedRead { display: DataEnvelope; raw: unknown }
function ResourcePanel({ record, resource, addressId }: { record: DisclosureCase; resource: DisclosureResource; addressId?: string }) {
  const identity = useIdentity(), originalExpiry = useRef(identity?.expires ?? 0), revision = useRef(0), live = useRef(true), pending = useRef<AbortController | null>(null);
  const [readView, setReadView] = useState<'list' | 'report'>(['addresses', 'transactions', 'activity_logs', 'events'].includes(resource) ? 'list' : 'report');
  const [target, setTarget] = useState(''), [from, setFrom] = useState(''), [through, setThrough] = useState('');
  const [state, setState] = useState<{ data?: ParsedRead; history: ParsedRead[]; index: number; trimmed: boolean; loading: boolean; error?: string }>({ history: [], index: 0, trimmed: false, loading: false });
  const clear = () => { ++revision.current; pending.current?.abort(); setState({ history: [], index: 0, trimmed: false, loading: false }); };
  const owned = () => !!identity && record.status === 'open' && record.recipientActorId === identity.actor && record.scope.resources.includes(resource) && appSession.snapshot().identity?.activation === identity.activation && appSession.snapshot().identity?.actor === identity.actor && Math.min(originalExpiry.current, record.expiresAt) > Date.now() / 1000;
  const read = async (cursor = '', view = readView) => {
    if (!owned()) { clear(); return; }
    const current = ++revision.current, abort = new AbortController(); pending.current?.abort(); pending.current = abort;
    setState(old => ({ ...old, data: undefined, loading: true, error: undefined, ...(!cursor ? { history: [], index: 0, trimmed: false } : {}) }));
    try {
      const narrowed = readNarrowing(record.scope, target, from, through);
      const body = JSON.stringify({ v: 1, resource, view, ...(view === 'list' ? { limit: 50, ...(cursor ? { cursor } : {}) } : {}), ...(addressId ? { addressId } : {}), ...narrowed.body });
      const endpoint = addressId && resource === 'transactions' ? `/grant/${record.caseId}/${addressId}/transactions${query(cursor || null)}` : resource === 'activity_logs' ? `/grant/${record.caseId}/activity${query(cursor || null)}` : `/disclosure/cases/${record.caseId}/read`;
      const raw = await appSession.disclosure<unknown>(endpoint, endpoint.includes('/grant/') ? { signal: abort.signal } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body, signal: abort.signal });
      if (!live.current || current !== revision.current) return;
      if (!owned()) { clear(); return; }
      const display = parseDisclosureData(raw, record.caseId, resource, { record, view, dataTime: narrowed.dataTime, limit: 50, sessionExpiresAt: originalExpiry.current });
      const data = { raw, display }, history = cursor ? [...state.history.slice(0, state.index + 1), data] : [data];
      setState({ data, history: history.slice(-10), index: Math.min(history.length, 10) - 1, trimmed: history.length > 10 || !!cursor && state.trimmed, loading: false });
    } catch (error) {
      if (live.current && current === revision.current) setState({ history: [], index: 0, trimmed: false, loading: false, error: failure(error) });
    }
  };
  useEffect(() => { live.current = true; void read(); return () => { live.current = false; ++revision.current; pending.current?.abort(); }; }, []);
  useEffect(() => {
    if (!state.history.length) return;
    const deadline = Math.min(originalExpiry.current, record.expiresAt, ...state.history.map(page => page.display.effectiveUntil));
    const timer = setTimeout(clear, Math.min(2147483647, Math.max(1, deadline * 1000 - Date.now())));
    return () => clearTimeout(timer);
  }, [state.history, record.expiresAt]);
  const cached = (index: number) => {
    const data = state.history[index];
    if (!owned() || !data || data.display.effectiveUntil <= Date.now() / 1000) { clear(); return; }
    setState(old => ({ ...old, index, data }));
  };
  const data = state.data?.display, page = data?.view === 'list' ? data.result as DataPage<DataAddress | DataTransaction | DataActivity | DataEvent | DataFinancePage['items'][number]> : undefined;
  const download = () => {
    if (!state.data || !owned() || state.data.display.effectiveUntil <= Date.now() / 1000) { clear(); return; }
    const blob = new Blob([JSON.stringify(state.data.raw, null, 2)], { type: 'application/json' }), url = URL.createObjectURL(blob), a = document.createElement('a');
    try {
      if (!owned() || state.data.display.effectiveUntil <= Date.now() / 1000) { clear(); return; }
      a.href = url; a.download = `disclosure-${record.caseId}-${resource}-${data?.view === 'list' ? 'page' : 'summary'}.json`; a.click();
    } finally { URL.revokeObjectURL(url); }
  };
  const narrow = resource === 'events' || resource === 'compliance_report';
  return <section className="card p-5 space-y-3"><h3 className="text-lg font-medium">{resource === 'compliance_report' ? readView === 'list' ? 'Admission decisions' : 'Compliance summary report' : labels[resource]}</h3>
    {resource === 'events' && <p className="text-sm text-neutral-500">Events from transactions covered by this case, emitted by contracts attributed to the data organization. This covers transactions whose sender or recipient is a target wallet; events mentioning a target only in their payload are outside this population. {record.scope.level === 'full' ? 'Full Events reveals the whole event payload and may include other parties’ data.' : 'Topics, raw data and hashes are withheld at this privacy level.'}</p>}
    {resource === 'compliance_report' && <label>Compliance view<select aria-label="Compliance view" className="input mt-1" value={readView} disabled={state.loading} onChange={event => { clear(); const view = event.target.value as 'list' | 'report'; setReadView(view); void read('', view); }}><option value="report">Summary report</option><option value="list">Admission decisions</option></select></label>}
    {narrow && <fieldset className="space-y-3 rounded-lg border border-neutral-200 p-3"><legend>Narrow this read</legend><div className="grid gap-3 sm:grid-cols-3"><label>Target actor<select aria-label="Target actor" className="input mt-1" value={target} onChange={event => { clear(); setTarget(event.target.value); }}><option value="">All targets in this case</option>{record.scope.targetActors.map(actor => <option key={actor} value={actor}>{actor}</option>)}</select></label><label>From (UTC, inclusive)<input aria-label="From (UTC, inclusive)" type="datetime-local" step="1" className="input mt-1" value={from} onChange={event => { clear(); setFrom(event.target.value); }} /></label><label>Through (UTC, inclusive)<input aria-label="Through (UTC, inclusive)" type="datetime-local" step="1" className="input mt-1" value={through} onChange={event => { clear(); setThrough(event.target.value); }} /></label></div><button className="btn-secondary" disabled={state.loading} onClick={() => void read()}>Read permitted data</button></fieldset>}
    <Status {...state} />{data && <><p>{data.coverage.status === 'RETAINED_AT_READ' ? 'These results cover retained records available when the request was prepared.' : 'These results cover recorded activity up to the selected audit snapshot.'} Prepared {displayDisclosureTime(data.coverage.preparedAt)}. Available in this session until {displayDisclosureTime(Math.min(data.effectiveUntil, originalExpiry.current))}.</p>
    {resource === 'addresses' && page && <ul>{(page.items as DataAddress[]).map(a => <li key={a.addressId}><Link className="text-primary hover:underline break-all" to={`/disclosures/cases/${record.caseId}/addresses/${a.addressId}`}>{a.display}</Link>{a.isMine ? ' · Your wallet' : ''}</li>)}</ul>}
    {resource === 'transactions' && page && <Transactions rows={page.items as DataTransaction[]} record={record} addressId={addressId} />}
    {resource === 'events' && page && <DisclosureEventCards rows={page.items as DataEvent[]} level={record.scope.level} />}
    {resource === 'activity_logs' && page && <Table headings={['Time', 'Method', 'Status', 'Actor']}>{(page.items as DataActivity[]).map(a => <tr key={a.recordId}><Cell>{displayDisclosureTime(a.time)}</Cell><Cell>{a.method}</Cell><Cell>{a.statusCode}</Cell><Cell>{a.actorId}</Cell></tr>)}</Table>}
    {resource === 'activity_summary' && <Summary data={data.result as DataSummary} />}
    {resource === 'sanctions_report' && <Sanctions data={data.result as DataSanctions} />}
    {resource === 'compliance_report' && data.view === 'report' && <><Compliance data={data.result as DataCompliance} /><button className="btn-secondary" onClick={() => { clear(); setReadView('list'); void read('', 'list'); }}>View admission decisions</button></>}
    {resource === 'compliance_report' && page && <><p>Matching retained admission decisions: <strong>{(data.result as DataFinancePage).matchingRetainedCount}</strong> as of <time>{(data.result as DataFinancePage).observedAt}</time>. Each page is a live retained read; counts can change.</p><Evidence rows={page.items as DataFinancePage['items']} /></>}
    {page && page.items.length === 0 && <p>{resource === 'events' ? 'No eligible events on this page.' : 'No records on this page.'}</p>}
    {page && <p className="text-xs text-neutral-500">{state.trimmed ? 'Earlier pages were released; the latest 10 pages are kept in this tab.' : 'Up to 10 loaded pages are kept in this tab until expiry.'} Previous shows an already loaded page.</p>}
    <div className="flex flex-wrap gap-2"><button className="btn-secondary" onClick={() => void read()}>Refresh results</button>{page && <><button className="btn-secondary" disabled={state.index === 0} onClick={() => cached(state.index - 1)}>Previous page</button><button className="btn-secondary" disabled={!page.nextCursor && state.index + 1 >= state.history.length} onClick={() => { if (state.index + 1 < state.history.length) cached(state.index + 1); else if (page.nextCursor) void read(page.nextCursor); }}>{resource === 'events' && page.items.length === 0 ? 'Continue' : 'Next page'}</button></>}<button className="btn-secondary" onClick={download}>{page ? 'Download this page' : 'Download summary report'}</button></div>
  </>}</section>;
}
function Evidence({ rows }: { rows: DataFinancePage['items'] }) { return <Table headings={['Time', 'Check', 'Decision', 'Mode', 'Coverage']}>{rows.map(e => <tr key={e.id}><Cell>{displayDisclosureTime(e.time)}</Cell><Cell>{e.kind}</Cell><Cell>{e.decision}</Cell><Cell>{e.monitored ? 'Monitor' : 'Enforce'}</Cell><Cell>{e.coverage}</Cell></tr>)}</Table>; }
function Transactions({ rows, record, addressId }: { rows: DataTransaction[]; record: DisclosureCase; addressId?: string }) { return <Table headings={['Block', 'Time', 'From', 'To', 'Value', 'Status', '']}>{rows.map((t, i) => <tr key={t.transactionHash ?? `${t.blockNumber}:${t.transactionIndex}:${i}`}><Cell>{t.blockNumber}</Cell><Cell>{displayDisclosureTime(t.blockTime)}</Cell><Cell>{t.from}</Cell><Cell>{t.to ?? 'Contract creation'}</Cell><Cell>{t.value ?? 'Private'}</Cell><Cell>{t.status === 1 ? 'Success' : 'Failed'}</Cell><Cell>{(addressId || t.addressId) && t.transactionHash && record.scope.level === 'full' && <Link className="text-primary hover:underline" to={`/disclosures/cases/${record.caseId}/addresses/${addressId || t.addressId}/transactions/${t.transactionHash}`}>Details</Link>}</Cell></tr>)}</Table>; }
function Summary({ data }: { data: DataSummary }) { return <><p>Total {data.total} · Successful {data.successful} · Failed {data.failed}</p><Table headings={['Method', 'Total', 'Successful', 'Failed']}>{data.methodBreakdown.map(m => <tr key={m.method}><Cell>{m.method}</Cell><Cell>{m.total}</Cell><Cell>{m.successful}</Cell><Cell>{m.failed}</Cell></tr>)}</Table></>; }
function Sanctions({ data }: { data: DataSanctions }) { return <><p>{data.status === 'hit' ? 'Approved-list match found' : data.status === 'clear' ? 'No approved-list match in the checked addresses' : 'No eligible addresses to check'} · {data.scopedAddressCount} addresses · Checked {displayDisclosureTime(data.checkedAt)}</p><ul>{data.matches.map(m => <li key={m.sourceId}>{m.address.display}{m.reason ? ` · ${m.reason}` : ''}</li>)}</ul></>; }
function Compliance({ data }: { data: DataCompliance }) { return <div className="space-y-3"><Summary data={data.activity} /><Sanctions data={data.sanctions} /><p>KYC: not evaluated by this report.</p><p>Matching retained admission decisions: <strong>{data.finance.matchingRetainedCount}</strong> as of <time>{data.finance.observedAt}</time>. This summary includes aggregate counts. Open admission decisions to read individual records with these same filters.</p></div>; }
function CaseAudit({ caseId }: { caseId: string }) { const [cursor, setCursor] = useState<string | null>(null), result = useRemote(`/disclosure/cases/${caseId}/audit${query(cursor)}`, value => { const v = value as { items: { eventId: string; time: number; action: string; outcome: string }[]; nextCursor: string }; if (!v || !Array.isArray(v.items) || v.items.length > 100 || typeof v.nextCursor !== 'string') throw Error('Invalid case history'); return v; }); return <details className="card p-5"><summary className="cursor-pointer font-medium text-primary">Case history</summary><Status {...result} />{result.data && <><Table headings={['Time', 'Action', 'Outcome']}>{result.data.items.map(e => <tr key={e.eventId}><Cell>{displayDisclosureTime(e.time)}</Cell><Cell>{e.action}</Cell><Cell>{e.outcome}</Cell></tr>)}</Table><button className="btn-secondary mt-3" disabled={!result.data.nextCursor} onClick={() => setCursor(result.data!.nextCursor)}>Next history page</button></>}</details>; }

export function DisclosureAddressV2() { const identity = useIdentity(), { caseId = '', addressId = '' } = useParams(); return identity ? <AddressPanel key={identity.activation + caseId + addressId} caseId={caseId} addressId={addressId} /> : null; }
function AddressPanel({ caseId, addressId }: { caseId: string; addressId: string }) { const record = useRemote('/disclosure/cases/' + encodeURIComponent(caseId), parseDisclosureCase), result = useRemote(`/grant/${caseId}/resolve/${encodeURIComponent(addressId)}`, value => parseDisclosureData(value, caseId, 'addresses')); const address = result.data ? (result.data.result as { item: DataAddress }).item : undefined; return <section className="space-y-5"><PageHeader title="Disclosed Address" subtitle="Address data available through this case"><Link className="btn-secondary" to={'/disclosures/cases/' + caseId}>Back to case</Link></PageHeader><Status {...record} /><Status {...result} />{address && <DisclosurePanelPresentation title="Address"><InfoRow label="Address" value={<code className="break-all">{address.display}</code>} /></DisclosurePanelPresentation>}{record.data?.scope.resources.includes('transactions') && <ResourcePanel record={record.data} resource="transactions" addressId={addressId} />}</section>; }
export function DisclosureTransactionV2() { const identity = useIdentity(), { caseId = '', addressId = '', hash = '' } = useParams(); return identity ? <TransactionPanel key={identity.activation + caseId + addressId + hash} caseId={caseId} addressId={addressId} hash={hash} /> : null; }
function TransactionPanel({ caseId, addressId, hash }: { caseId: string; addressId: string; hash: string }) { const result = useRemote(`/grant/${caseId}/${encodeURIComponent(addressId)}/transactions/${encodeURIComponent(hash)}`, value => parseDisclosureData(value, caseId, 'transactions')), tx = result.data ? (result.data.result as { item: DataTransaction }).item : undefined; return <section className="space-y-5"><PageHeader title="Transaction Details" subtitle="Transaction data available through this case"><Link className="btn-secondary" to={`/disclosures/cases/${caseId}`}>Back to case</Link></PageHeader><Status {...result} />{tx && <><DisclosurePanelPresentation title="Transaction"><div className="divide-y divide-neutral-100">{[['Hash', tx.transactionHash], ['From', tx.from], ['To', tx.to ?? 'Contract creation'], ['Value', tx.value], ['Block', tx.blockNumber], ['Gas used', tx.gasUsed], ['Gas limit', tx.gasLimit], ['Gas price', tx.gasPrice], ['Nonce', tx.nonce]].map(([label, value]) => <InfoRow key={label} label={label!} value={<span className="break-all font-mono text-xs">{value ?? 'Unavailable in this disclosure'}</span>} />)}</div></DisclosurePanelPresentation><DisclosurePanelPresentation title="Input Data"><pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all p-4 font-mono text-xs">{tx.input ?? 'Unavailable in this disclosure'}</pre></DisclosurePanelPresentation></>}</section>; }
