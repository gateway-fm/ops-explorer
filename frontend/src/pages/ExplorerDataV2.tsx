import { useEffect, useState, type ReactNode } from 'react';
import { Link, NavLink, Outlet, useNavigate, useParams } from 'react-router-dom';
import { appSession, ReadHTTPError } from '../lib/v2Session';
import { continuationMessage, readItem, readPage, readObservedStats, readObservedHistory, readTarget, searchSuggestionTarget, type ReadItem } from '../lib/v2Reads';
import type { PageInfo } from '../lib/v2ReadTypes';
import { ObservedStats, ObservedHistoryChart } from './ExplorerSummaryV2';

const quantity = /^(0|[1-9][0-9]{0,77})$/;
const address = /^0x[0-9a-f]{40}$/;
const hash = /^0x[0-9a-f]{64}$/;
function label(v: string) { return v.replace(/([A-Z])/g, ' $1').replace(/^./, s => s.toUpperCase()); }
function Cell({ name, value, row }: { name: string; value: unknown; row?: ReadItem }) {
  if (value === null) return <span>{name === 'value' ? 'Hidden' : 'Unknown'}</span>;
  if (typeof value === 'boolean') return <span>{value ? 'Yes' : 'No'}</span>;
  if (typeof value === 'string') {
    let target = row && name === 'value' ? searchSuggestionTarget(row) : undefined;
    if (hash.test(value) && ['hash', 'txHash', 'creationTx'].includes(name)) target = name === 'hash' && row?.parentHash ? '/blocks/hash/' + value : '/transactions/' + value;
    if (!target && address.test(value) && value !== '0x' + '0'.repeat(40)) target = ['tokenAddress', 'l1Address'].includes(name) ? '/tokens/' + value : '/addresses/' + value;
    if (quantity.test(value) && (name === 'blockNumber' || name === 'number' && row?.parentHash)) target = '/blocks/' + value;
    if (target) return <Link className="underline break-all" to={target}>{value}</Link>;
    if (value.length > 256) return <details><summary>{value.slice(0, 96)}…</summary><pre className="break-all whitespace-pre-wrap">{value}</pre></details>;
    return <span className="break-all">{value || 'Empty'}</span>;
  }
  if (typeof value === 'number') return <span>{value}</span>; // only finite wire enums/indexes
  return <pre className="break-all whitespace-pre-wrap text-xs">{JSON.stringify(value, null, 2)}</pre>;
}
function RecordView({ item }: { item: ReadItem }) {
  if (!Object.keys(item).length) return <p>No metrics are available for your current permissions.</p>;
  return <dl className="grid sm:grid-cols-[minmax(9rem,1fr)_4fr] gap-3">{Object.entries(item).map(([name, value]) => <div key={name} className="contents"><dt className="font-medium">{label(name)}</dt><dd><Cell name={name} value={value} row={item} /></dd></div>)}</dl>;
}
function ItemTable({ items }: { items: ReadItem[] }) {
  if (!items.length) return <p>No records on this page.</p>;
  const preferred = ['hash', 'number', 'address', 'txHash', 'blockNumber', 'timestamp', 'from', 'to', 'value', 'balance', 'symbol', 'name', 'status', 'count', 'type', 'label'];
  const columns = preferred.filter(k => items.some(item => k in item));
  if (!columns.length) columns.push(...Object.keys(items[0]).slice(0, 6));
  return <div className="overflow-x-auto"><table className="w-full text-sm"><thead><tr>{columns.map(k => <th className="text-left p-2" key={k}>{label(k)}</th>)}<th className="text-left p-2">Details</th></tr></thead><tbody>{items.map((item, index) => <tr className="border-t align-top" key={typeof item.id === 'string' ? item.id : index}>{columns.map(k => <td className="p-2 max-w-72" key={k}>{k in item ? <Cell name={k} value={item[k]} row={item} /> : 'Unknown'}</td>)}<td className="p-2"><details><summary>Show</summary><RecordView item={item} /></details></td></tr>)}</tbody></table></div>;
}
type Result = { item?: ReadItem; items?: ReadItem[]; unavailable?: boolean; error?: string; stale?: boolean };
function ReadResult({ endpoint, collection, summary, onPage }: { endpoint: string; collection: boolean; summary?: 'stats' | 'chart'; onPage?: (p: PageInfo | null) => void }) {
  const [result, setResult] = useState<Result | null>(null);
  useEffect(() => {
    let active = true;
    let scope: ReturnType<typeof appSession.captureOperation> | undefined;
    try {
      scope = appSession.captureOperation();
      const owner = scope;
      void appSession.read<unknown>(endpoint, owner.signal).then(raw => {
        owner.check();
        const page = collection ? summary === 'chart' ? readObservedHistory(raw) : readPage(raw) : undefined;
        const item = !collection && raw !== null ? summary === 'stats' ? readObservedStats(raw) : readItem(raw) : undefined;
        const next: Result = page ? { items: page.items } : item ? { item } : { unavailable: true };
        owner.check();
        if (active) { onPage?.(page?.page ?? (summary === 'stats' && item ? item.page as PageInfo : null)); setResult(next); }
      }).catch(error => {
        if (!active || owner.signal.aborted) return;
        try { owner.check(); } catch { return; }
        onPage?.(null);
        setResult({ error: error instanceof ReadHTTPError ? error.message : 'The read could not be completed. Try again.', stale: error instanceof ReadHTTPError && error.code === 'cursor_stale' });
      });
    } catch { setResult({ error: 'Verify your app session before reading data.' }); }
    return () => { active = false; scope?.dispose(); };
  }, [endpoint, collection, summary, onPage]);
  if (!result) return <p role="status">Loading current data…</p>;
  if (result.error) return <p role="alert">{result.error}</p>;
  if (result.unavailable) return <p role="status">This record is unavailable.</p>;
  return result.items ? summary === 'chart' ? <ObservedHistoryChart items={result.items} /> : <ItemTable items={result.items} /> : result.item ? summary === 'stats' ? <ObservedStats item={result.item} /> : <RecordView item={result.item} /> : null;
}
export function ReadPanel({ path, title, collection = false, filters = '', summary }: { path: string; title: string; collection?: boolean; filters?: string; summary?: 'stats' | 'chart' }) {
  const paged = collection || summary === 'stats';
  const [query, setQuery] = useState(new URLSearchParams(paged ? { limit: filters === 'chart' ? '30' : '25' } : {}));
  const [cursor, setCursor] = useState<string>();
  const [previous, setPrevious] = useState<(string | undefined)[]>([]);
  const [page, setPage] = useState<PageInfo | null>(null);
  const [revision, setRevision] = useState(0);
  const endpoint = readTarget(path, query, cursor);
  const reset = () => { setCursor(undefined); setPrevious([]); setPage(null); setRevision(v => v + 1); };
  return <section className="space-y-4"><h2 className="text-xl">{title}</h2><p className="text-sm">Only data available under your current permissions is shown. Unavailable metrics stay unknown.</p>
    {paged && <form className="flex flex-wrap gap-3" onSubmit={event => { event.preventDefault(); const values = new FormData(event.currentTarget), next = new URLSearchParams(); values.forEach((v, k) => { if (typeof v === 'string' && v.trim()) next.set(k, v.trim()); }); setQuery(next); reset(); }}>
      <label>{summary ? 'Transactions per page ' : 'Limit '}<input className="border p-1 w-20" name="limit" defaultValue={filters === 'chart' ? '30' : '25'} inputMode="numeric" pattern="[1-9][0-9]*" /></label>
      {filters === 'transactions' && <label>Category <select name="category"><option value="">All available</option><option value="coin_transfer">Coin transfer</option><option value="contract_creation">Contract creation</option><option value="contract_call">Contract call</option><option value="token_transfer">Token transfer</option></select></label>}
      {filters === 'logs' && (path.startsWith('/addresses/') ? ['topic0', 'from', 'to'] : ['address', 'topic0', 'from', 'to']).map(name => <label key={name}>{label(name)} <input className="border p-1" name={name} /></label>)}
      {filters === 'tokens' && <label>Type <select name="type"><option value="">All available</option>{['ERC20', 'ERC721', 'ERC1155'].map(v => <option key={v}>{v}</option>)}</select></label>}
      {filters === 'chart' && <label>Interval <select name="interval"><option value="day">Day</option><option value="hour">Hour</option><option value="week">Week</option></select></label>}
      <button type="submit">Apply filters</button></form>}
    <button onClick={reset}>Refresh / first page</button>
    <ReadResult key={endpoint + ':' + revision} endpoint={endpoint} collection={collection} summary={summary} onPage={paged ? setPage : undefined} />
    {paged && page && <div className="flex flex-wrap gap-3 items-center"><span>{summary ? 'This page is not a chain or period total.' : page.total !== undefined ? `Total: ${page.total}` : 'Total unavailable'}</span><span>{continuationMessage(page)}</span><button disabled={!previous.length} onClick={() => { setPage(null); setCursor(previous.at(-1)); setPrevious(v => v.slice(0, -1)); }}>Previous</button><button disabled={!page.next} onClick={() => { if (page.next) { setPrevious(v => [...v.slice(-63), cursor]); setCursor(page.next); setPage(null); } }}>Continue</button></div>}
  </section>;
}
export function OverviewV2() { return <div className="space-y-8"><ReadPanel path="/stats" title="Visible activity on this page" summary="stats" /><ReadPanel path="/blocks/latest/number" title="Current block" /><ReadPanel path="/chain-id" title="Chain" /><ReadPanel path="/stats/tx-history" title="Visible transaction history" collection filters="chart" summary="chart" /></div>; }
function Tabs({ children }: { children: ReactNode }) { return <nav className="flex flex-wrap gap-4 border-b pb-3">{children}</nav>; }
export function BlockV2() {
  const { number, hash: blockHash } = useParams(); const base = blockHash ? '/blocks/hash/' + blockHash : '/blocks/' + number;
  return <div className="space-y-5"><ReadPanel key={base} path={base} title="Block" />{number && <><ReadPanel key={base + '/transactions'} path={base + '/transactions'} title="Available block transactions" collection filters="transactions" /><ReadPanel key={base + '/internal'} path={base + '/internal'} title="Available internal transactions" collection /></>}</div>;
}
export function TransactionV2() { const { hash: tx } = useParams(); return <div className="space-y-5"><h2 className="break-all">Transaction {tx}</h2><Tabs>{[['', 'Details'], ['/transfers', 'Transfers'], ['/logs', 'Logs'], ['/internal', 'Internal calls']].map(([path, text]) => <NavLink key={path} end={!path} to={'/transactions/' + tx + path}>{text}</NavLink>)}</Tabs><Outlet /></div>; }
export function TransactionReadV2({ suffix = '', collection = false, filters = '' }: { suffix?: string; collection?: boolean; filters?: string }) { const { hash: tx } = useParams(); const path = '/transactions/' + tx + suffix; return <ReadPanel key={path} path={path} title={suffix ? label(suffix.slice(1)) : 'Transaction details'} collection={collection} filters={filters} />; }
export function AddressV2() { const { address: id } = useParams(); return <div className="space-y-5"><h2 className="break-all">Address {id}</h2><Tabs>{['stats', 'transactions', 'balance', 'code', 'balances', 'transfers', 'internal', 'logs', 'contract', 'is-contract'].map(path => <NavLink key={path} to={'/addresses/' + id + '/' + path}>{label(path)}</NavLink>)}</Tabs><Outlet /></div>; }
export function AddressReadV2({ suffix, collection = false, filters = '' }: { suffix: string; collection?: boolean; filters?: string }) { const { address: id } = useParams(); const path = '/addresses/' + id + '/' + suffix; return <ReadPanel key={path} path={path} title={label(suffix)} collection={collection} filters={filters} />; }
export function TokenV2() { const { address: id } = useParams(); return <div className="space-y-5"><h2 className="break-all">Token {id}</h2><Tabs>{[['', 'Details'], ['/holders', 'Holders'], ['/transfers', 'Transfers']].map(([path, text]) => <NavLink key={path} end={!path} to={'/tokens/' + id + path}>{text}</NavLink>)}</Tabs><Outlet /></div>; }
export function TokenReadV2({ suffix = '', collection = false }: { suffix?: string; collection?: boolean }) { const { address: id } = useParams(); const path = '/tokens/' + id + suffix; return <ReadPanel key={path} path={path} title={suffix ? label(suffix.slice(1)) : 'Token details'} collection={collection} />; }
export function SyncV2() { return <div className="space-y-8">{['status', 'indexer-progress', 'catchup'].map(suffix => <ReadPanel key={suffix} path={'/sync/' + suffix} title={label(suffix)} />)}</div>; }
export function SearchV2() {
  const [query, setQuery] = useState<string>(); const navigate = useNavigate();
  return <section className="space-y-4"><h2>Search available records</h2><form onSubmit={event => { event.preventDefault(); const value = String(new FormData(event.currentTarget).get('q') ?? '').trim(); if (address.test(value)) { navigate('/addresses/' + value + '/stats'); return; } if (quantity.test(value) && value.length <= 19) { navigate('/blocks/' + value); return; } setQuery(value); }}><label>Address, transaction hash, block number or token <input name="q" maxLength={128} required className="border p-1" /></label><button>Search</button></form>{query && <ReadResult key={query} endpoint={readTarget('/search/suggestions', new URLSearchParams({ q: query, limit: '20' }))} collection />}</section>;
}
