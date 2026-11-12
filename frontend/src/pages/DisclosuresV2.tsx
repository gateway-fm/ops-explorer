import { useEffect, useRef, useState, useSyncExternalStore, type FormEvent, type ReactNode } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { appSession } from '../lib/v2Session';
import { ownWallets, type OwnWallet } from '../lib/v2Wallet';
import { disclosureResources, displayDisclosureTime, parseCasePage, parseDisclosureCase, parseDisclosureData, type CasePage, type DataActivity, type DataAddress, type DataCompliance, type DataEnvelope, type DataPage, type DataSanctions, type DataSummary, type DataTransaction, type DisclosureCase, type DisclosureLevel, type DisclosureResource } from '../lib/v2Disclosure';

const labels: Record<DisclosureResource, string> = { addresses: 'Addresses', transactions: 'Transactions', activity_logs: 'Activity', activity_summary: 'Activity summary', sanctions_report: 'Sanctions', compliance_report: 'Compliance' };
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
function Status({ loading, error }: { loading: boolean; error?: string }) { return <>{loading && <p role="status">Loading…</p>}{error && <p role="alert">{error}</p>}</>; }
function Table({ headings, children }: { headings: string[]; children: ReactNode }) { return <div className="overflow-x-auto"><table className="w-full text-left"><thead><tr>{headings.map(h => <th className="p-2 border-b" key={h}>{h}</th>)}</tr></thead><tbody>{children}</tbody></table></div>; }
function Cell({ children }: { children?: ReactNode }) { return <td className="p-2 border-b align-top break-all">{children ?? '—'}</td>; }
function query(cursor: string | null) { return cursor ? `?limit=50&cursor=${encodeURIComponent(cursor)}` : '?limit=50'; }

export function DisclosuresV2() { const identity = useIdentity(); return identity ? <Dashboard key={identity.activation} /> : null; }
function Dashboard() {
  const [cursor, setCursor] = useState<string | null>(null);
  const cases = useRemote<CasePage>('/viewable-addresses' + query(cursor), parseCasePage);
  const wallets = useRemote<OwnWallet[]>('/eth/addresses', ownWallets, undefined, true);
  return <section className="space-y-5"><h2 className="text-xl">Disclosures</h2><p>Access follows approved rules. Each permitted disclosure is checked and recorded without asking the client to approve it.</p>
    <section><h3>Your wallets</h3><Status {...wallets} />{wallets.data && <ul>{wallets.data.map(w => <li key={w.address}><Link to={'/addresses/' + w.address}><code>{w.address}</code></Link></li>)}</ul>}<Link to="/wallets">Manage wallet links</Link></section>
    <section><h3>Your cases</h3><Status {...cases} />{cases.data && <><Table headings={['Reference', 'Purpose', 'Status', 'Expires', '']}>
      {cases.data.items.map(c => <tr key={c.caseId}><Cell>{c.caseReference}</Cell><Cell>{c.purpose}</Cell><Cell>{c.status}</Cell><Cell>{displayDisclosureTime(c.expiresAt)}</Cell><Cell><Link to={'/disclosures/cases/' + c.caseId}>Open case</Link></Cell></tr>)}
    </Table>{cases.data.items.length === 0 && <p>No cases on this page.</p>}<div className="flex gap-4"><button onClick={() => { setCursor(null); cases.reload(); }}>Refresh cases</button><button disabled={!cases.data.nextCursor} onClick={() => setCursor(cases.data!.nextCursor)}>Next cases</button></div></>}</section>
    <CreateCase onCreated={() => { setCursor(null); cases.reload(); }} />
  </section>;
}

function localDate(seconds: number) { const date = new Date(seconds * 1000); return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16); }
function CreateCase({ onCreated }: { onCreated: () => void }) {
  const identity = useIdentity(), navigate = useNavigate(), ticket = useRef(0), attempt = useRef<{ signature: string; body: string }>(undefined);
  const now = Math.floor(Date.now() / 1000);
  const [rule, setRule] = useState(''), [targets, setTargets] = useState(''), [addresses, setAddresses] = useState(''), [reference, setReference] = useState(''), [purpose, setPurpose] = useState(''), [legal, setLegal] = useState('');
  const [from, setFrom] = useState(localDate(now - 3600)), [through, setThrough] = useState(localDate(now)), [minutes, setMinutes] = useState(15), [selected, setSelected] = useState<DisclosureResource[]>([...disclosureResources]), [level, setLevel] = useState<DisclosureLevel>('pseudonymous');
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
  return <details><summary>Open a disclosure case</summary><form className="space-y-3 pt-3" onSubmit={submit}>
    <p>The approved rule limits the clients, dates and detail level you can request.</p>
    <label className="block">Approved rule ID<input required maxLength={128} value={rule} onChange={e => setRule(e.target.value)} className="block border p-2 w-full" /></label>
    <label className="block">Client actor IDs<textarea required value={targets} onChange={e => setTargets(e.target.value)} placeholder="ops:p1:…" className="block border p-2 w-full" /></label>
    <label className="block">Limit to these wallet addresses (optional)<textarea value={addresses} onChange={e => setAddresses(e.target.value)} placeholder="0x…" className="block border p-2 w-full" /><span>Leave empty to request all eligible wallets of these clients. For a rule limited to named wallets, enter the permitted addresses.</span></label>
    <label className="block">Case reference<input required maxLength={256} value={reference} onChange={e => setReference(e.target.value)} className="block border p-2 w-full" /></label>
    <label className="block">Purpose<input required maxLength={1024} value={purpose} onChange={e => setPurpose(e.target.value)} className="block border p-2 w-full" /></label>
    <label className="block">Legal basis<input required maxLength={1024} value={legal} onChange={e => setLegal(e.target.value)} className="block border p-2 w-full" /></label>
    <div className="flex flex-wrap gap-4"><label>From (local time)<input required type="datetime-local" value={from} onChange={e => setFrom(e.target.value)} className="block border p-2" /></label><label>Through (local time)<input required type="datetime-local" value={through} onChange={e => setThrough(e.target.value)} className="block border p-2" /></label><label>Case expires in minutes<input required type="number" min={1} max={43200} value={minutes} onChange={e => setMinutes(Number(e.target.value))} className="block border p-2" /></label></div>
    <fieldset><legend>Data</legend><div className="flex flex-wrap gap-4">{disclosureResources.map(r => <label key={r}><input type="checkbox" checked={selected.includes(r)} onChange={e => setSelected(old => e.target.checked ? [...old, r] : old.filter(x => x !== r))} /> {labels[r]}</label>)}</div></fieldset>
    <label>Detail level<select value={level} onChange={e => setLevel(e.target.value as DisclosureLevel)} className="border p-2 ml-2"><option value="redacted">Redacted</option><option value="pseudonymous">Pseudonymous</option><option value="full">Full</option></select></label>
    {error && <p role="alert">{error}</p>}<button disabled={busy} type="submit">{busy ? 'Opening case…' : 'Open case'}</button>
  </form></details>;
}

export function DisclosureCaseV2() { const identity = useIdentity(), { caseId = '' } = useParams(); return identity ? <CasePanel key={identity.activation + caseId} caseId={caseId} /> : null; }
function CasePanel({ caseId }: { caseId: string }) {
  const result = useRemote('/disclosure/cases/' + encodeURIComponent(caseId), parseDisclosureCase), [resource, setResource] = useState<DisclosureResource>(), [reason, setReason] = useState(''), [error, setError] = useState<string>(), [busy, setBusy] = useState(false);
  const revoke = async () => { if (busy || !reason.trim()) return; setBusy(true); setError(undefined); try { await appSession.disclosure('/disclosure/cases/' + caseId + '/revoke', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ reason }) }); setResource(undefined); result.reload(); } catch (e) { setError(failure(e)); } finally { setBusy(false); } };
  return <section className="space-y-4"><Link to="/disclosures">All disclosures</Link><Status {...result} />{result.data && <><h2 className="text-xl">{result.data.caseReference}</h2><p>{result.data.purpose}</p><p>Legal basis: {result.data.legalBasis}</p><p>{result.data.status} · Expires {displayDisclosureTime(result.data.expiresAt)} · {result.data.scope.level} detail</p>
    <div className="flex flex-wrap gap-4">{result.data.scope.resources.map(r => <button key={r} disabled={result.data?.status !== 'open'} onClick={() => setResource(r)}>{labels[r]}</button>)}<button onClick={() => { setResource(undefined); result.reload(); }}>Refresh case</button></div>
    {resource && <ResourcePanel key={resource + result.data.version} record={result.data} resource={resource} />}
    <CaseAudit caseId={caseId} />
    {result.data.status === 'open' && <details><summary>Revoke this case</summary><p>Revocation ends access through this case.</p><input aria-label="Revocation reason" value={reason} maxLength={1024} onChange={e => setReason(e.target.value)} /><button disabled={busy || !reason.trim()} onClick={() => void revoke()}>Revoke case</button>{error && <p role="alert">{error}</p>}</details>}
  </>}</section>;
}

interface ParsedRead { display: DataEnvelope; raw: unknown }
function ResourcePanel({ record, resource, addressId }: { record: DisclosureCase; resource: DisclosureResource; addressId?: string }) {
  const [cursor, setCursor] = useState<string | null>(null), identity = useIdentity();
  const view = ['addresses', 'transactions', 'activity_logs'].includes(resource) ? 'list' : 'report';
  const body = JSON.stringify({ v: 1, resource, view, ...(view === 'list' ? { limit: 50, ...(cursor ? { cursor } : {}) } : {}), ...(addressId ? { addressId } : {}) });
  const endpoint = addressId && resource === 'transactions' ? `/grant/${record.caseId}/${addressId}/transactions${query(cursor)}` : resource === 'activity_logs' ? `/grant/${record.caseId}/activity${query(cursor)}` : `/disclosure/cases/${record.caseId}/read`;
  const result = useRemote<ParsedRead>(endpoint, raw => ({ display: parseDisclosureData(raw, record.caseId, resource), raw }), endpoint.includes('/grant/') ? undefined : body);
  const data = result.data?.display, page = data && data.view === 'list' ? data.result as DataPage<DataAddress | DataTransaction | DataActivity> : null;
  const download = () => { if (!result.data || !identity || appSession.snapshot().identity?.activation !== identity.activation) return; const blob = new Blob([JSON.stringify(result.data.raw, null, 2)], { type: 'application/json' }), url = URL.createObjectURL(blob), a = document.createElement('a'); a.href = url; a.download = `disclosure-${record.caseId}-${resource}.json`; a.click(); URL.revokeObjectURL(url); };
  return <section className="space-y-3"><h3>{labels[resource]}</h3><Status {...result} />{data && <><p>{data.coverage.status === 'RETAINED_AT_READ' ? 'These results cover retained records available when the request was prepared.' : 'These results cover recorded activity up to the selected audit snapshot.'} Prepared {displayDisclosureTime(data.coverage.preparedAt)}.</p>
    {resource === 'addresses' && page && <ul>{(page.items as DataAddress[]).map(a => <li key={a.addressId}><Link to={`/disclosures/cases/${record.caseId}/addresses/${a.addressId}`}>{a.display}</Link>{a.isMine ? ' · Your wallet' : ''}</li>)}</ul>}
    {resource === 'transactions' && page && <Transactions rows={page.items as DataTransaction[]} record={record} addressId={addressId} />}
    {resource === 'activity_logs' && page && <Table headings={['Time', 'Method', 'Status', 'Actor']}>{(page.items as DataActivity[]).map(a => <tr key={a.recordId}><Cell>{displayDisclosureTime(a.time)}</Cell><Cell>{a.method}</Cell><Cell>{a.statusCode}</Cell><Cell>{a.actorId}</Cell></tr>)}</Table>}
    {resource === 'activity_summary' && <Summary data={data.result as DataSummary} />}
    {resource === 'sanctions_report' && <Sanctions data={data.result as DataSanctions} />}
    {resource === 'compliance_report' && <Compliance data={data.result as DataCompliance} />}
    {page && page.items.length === 0 && <p>No records on this page.</p>}
    <div className="flex gap-4"><button onClick={() => { setCursor(null); result.reload(); }}>Refresh results</button>{page && <button disabled={!page.nextCursor} onClick={() => setCursor(page.nextCursor)}>Next page</button>}<button onClick={download}>Download results</button></div>
  </>}</section>;
}
function Transactions({ rows, record, addressId }: { rows: DataTransaction[]; record: DisclosureCase; addressId?: string }) { return <Table headings={['Block', 'Time', 'From', 'To', 'Value', 'Status', '']}>{rows.map((t, i) => <tr key={t.transactionHash ?? `${t.blockNumber}:${t.transactionIndex}:${i}`}><Cell>{t.blockNumber}</Cell><Cell>{displayDisclosureTime(t.blockTime)}</Cell><Cell>{t.from}</Cell><Cell>{t.to ?? 'Contract creation'}</Cell><Cell>{t.value ?? 'Private'}</Cell><Cell>{t.status === 1 ? 'Success' : 'Failed'}</Cell><Cell>{addressId && t.transactionHash && record.scope.level === 'full' && <Link to={`/disclosures/cases/${record.caseId}/addresses/${addressId}/transactions/${t.transactionHash}`}>Details</Link>}</Cell></tr>)}</Table>; }
function Summary({ data }: { data: DataSummary }) { return <><p>Total {data.total} · Successful {data.successful} · Failed {data.failed}</p><Table headings={['Method', 'Total', 'Successful', 'Failed']}>{data.methodBreakdown.map(m => <tr key={m.method}><Cell>{m.method}</Cell><Cell>{m.total}</Cell><Cell>{m.successful}</Cell><Cell>{m.failed}</Cell></tr>)}</Table></>; }
function Sanctions({ data }: { data: DataSanctions }) { return <><p>{data.status === 'hit' ? 'Approved-list match found' : data.status === 'clear' ? 'No approved-list match in the checked addresses' : 'No eligible addresses to check'} · {data.scopedAddressCount} addresses · Checked {displayDisclosureTime(data.checkedAt)}</p><ul>{data.matches.map(m => <li key={m.sourceId}>{m.address.display}{m.reason ? ` · ${m.reason}` : ''}</li>)}</ul></>; }
function Compliance({ data }: { data: DataCompliance }) { return <div className="space-y-3"><Summary data={data.activity} /><Sanctions data={data.sanctions} /><p>KYC: not evaluated by this report.</p><Table headings={['Time', 'Check', 'Decision', 'Mode', 'Coverage']}>{data.evidence.map(e => <tr key={e.id}><Cell>{displayDisclosureTime(e.time)}</Cell><Cell>{e.kind}</Cell><Cell>{e.decision}</Cell><Cell>{e.monitored ? 'Monitor' : 'Enforce'}</Cell><Cell>{e.coverage}</Cell></tr>)}</Table></div>; }
function CaseAudit({ caseId }: { caseId: string }) { const [cursor, setCursor] = useState<string | null>(null), result = useRemote(`/disclosure/cases/${caseId}/audit${query(cursor)}`, value => { const v = value as { items: { eventId: string; time: number; action: string; outcome: string }[]; nextCursor: string }; if (!v || !Array.isArray(v.items) || v.items.length > 100 || typeof v.nextCursor !== 'string') throw Error('Invalid case history'); return v; }); return <details><summary>Case history</summary><Status {...result} />{result.data && <><Table headings={['Time', 'Action', 'Outcome']}>{result.data.items.map(e => <tr key={e.eventId}><Cell>{displayDisclosureTime(e.time)}</Cell><Cell>{e.action}</Cell><Cell>{e.outcome}</Cell></tr>)}</Table><button disabled={!result.data.nextCursor} onClick={() => setCursor(result.data!.nextCursor)}>Next history page</button></>}</details>; }

export function DisclosureAddressV2() { const identity = useIdentity(), { caseId = '', addressId = '' } = useParams(); return identity ? <AddressPanel key={identity.activation + caseId + addressId} caseId={caseId} addressId={addressId} /> : null; }
function AddressPanel({ caseId, addressId }: { caseId: string; addressId: string }) { const record = useRemote('/disclosure/cases/' + encodeURIComponent(caseId), parseDisclosureCase), result = useRemote(`/grant/${caseId}/resolve/${encodeURIComponent(addressId)}`, value => parseDisclosureData(value, caseId, 'addresses')); const address = result.data ? (result.data.result as { item: DataAddress }).item : undefined; return <section className="space-y-4"><Link to={'/disclosures/cases/' + caseId}>Back to case</Link><Status {...record} /><Status {...result} />{address && <h2>{address.display}</h2>}{record.data?.scope.resources.includes('transactions') && <ResourcePanel record={record.data} resource="transactions" addressId={addressId} />}</section>; }
export function DisclosureTransactionV2() { const identity = useIdentity(), { caseId = '', addressId = '', hash = '' } = useParams(); return identity ? <TransactionPanel key={identity.activation + caseId + addressId + hash} caseId={caseId} addressId={addressId} hash={hash} /> : null; }
function TransactionPanel({ caseId, addressId, hash }: { caseId: string; addressId: string; hash: string }) { const result = useRemote(`/grant/${caseId}/${encodeURIComponent(addressId)}/transactions/${encodeURIComponent(hash)}`, value => parseDisclosureData(value, caseId, 'transactions')), tx = result.data ? (result.data.result as { item: DataTransaction }).item : undefined; return <section className="space-y-4"><Link to={`/disclosures/cases/${caseId}/addresses/${addressId}`}>Back to address</Link><Status {...result} />{tx && <><h2>Transaction</h2><dl className="space-y-2">{[['Hash', tx.transactionHash], ['From', tx.from], ['To', tx.to ?? 'Contract creation'], ['Value', tx.value], ['Block', tx.blockNumber], ['Gas used', tx.gasUsed], ['Gas limit', tx.gasLimit], ['Gas price', tx.gasPrice], ['Nonce', tx.nonce]].map(([label, value]) => <div key={label}><dt>{label}</dt><dd className="break-all">{value}</dd></div>)}</dl><h3>Input</h3><pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all">{tx.input}</pre></>}</section>; }
