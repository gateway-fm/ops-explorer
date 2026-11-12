import type { ReadItem } from '../lib/v2Reads';

export function ObservedStats({ item }: { item: ReadItem }) {
  return <div className="space-y-3"><p>Counts cover the visible transactions on this page, not the whole chain or all retained history.</p><dl className="grid gap-4 sm:grid-cols-3">{[['observedTransactions', 'Transactions on this page'], ['observedBlocks', 'Blocks represented'], ['observedParticipantAddresses', 'Visible participant addresses']].map(([field, title]) => <div key={field} className="border rounded p-4"><dt>{title}</dt><dd className="text-2xl break-all">{String(item[field])}</dd></div>)}</dl></div>;
}

function utcTime(stamp: string): string {
  const n = BigInt(stamp);
  return n <= 8640000000000n ? new Date(Number(n) * 1000).toISOString().replace('T', ' ').replace('.000Z', ' UTC') : stamp + ' Unix seconds';
}
export function ObservedHistoryChart({ items }: { items: ReadItem[] }) {
  if (!items.length) return <p>No visible transactions on this page. Continue if another page is available.</p>;
  const points = items.map(item => ({ timestamp: String(item.timestamp), count: String(item.count) }));
  const max = points.reduce((n, point) => BigInt(point.count) > n ? BigInt(point.count) : n, 1n), width = 640, height = 220, step = 600 / points.length;
  return <div className="space-y-3"><p>UTC bucket counts include only transactions on this page. A bucket can continue on another page; missing buckets are not reported as zero.</p><svg role="img" aria-label="Visible transaction counts on this page" viewBox={`0 0 ${width} ${height}`} className="w-full max-h-72 border-b">
    {points.map((point, i) => { const bar = Number(BigInt(point.count) * 16000n / max) / 100; return <g key={point.timestamp}><title>{utcTime(point.timestamp)}: {point.count} transactions on this page</title><rect x={20 + i * step} y={180 - bar} width={Math.max(1, step - 3)} height={bar} fill="currentColor" className="text-blue-600" /><text x={20 + i * step + step / 2} y={175 - bar} textAnchor="middle" fontSize="10">{point.count}</text></g>; })}
    <text x="20" y="205" fontSize="11">{utcTime(points[0].timestamp)}</text><text x="620" y="205" textAnchor="end" fontSize="11">{utcTime(points.at(-1)!.timestamp)}</text>
  </svg><details><summary>Bucket values</summary><table className="w-full text-left"><thead><tr><th>Bucket start (UTC)</th><th>Transactions on this page</th></tr></thead><tbody>{points.map(point => <tr key={point.timestamp}><td>{utcTime(point.timestamp)}</td><td>{point.count}</td></tr>)}</tbody></table></details></div>;
}
