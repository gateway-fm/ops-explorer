import type { PseudonymizedTransaction, PseudonymizedTransactionsResponse, Transaction } from '../lib/api';

export interface GrantSource { grantId: string; addressId: string; level: string }
export interface DisclosedRow { source: GrantSource; tx: PseudonymizedTransaction }
interface Position { block: number; index?: number }
export interface GrantWindow { upper?: Position; lower?: Position }
export interface GrantBoundary { upper?: Position; nextPage?: number }

export function grantWindow(rows: Transaction[], previousLast: Transaction | undefined, finalPage: boolean): GrantWindow | undefined {
  const last = rows.at(-1);
  // Empty filtered pages cannot close a range. Defer their disclosures to
  // the next non-empty page, or the final page, instead of overlapping it.
  if (!last && !finalPage) return undefined;
  return {
    ...(previousLast ? { upper: { block: previousLast.blockNumber, index: previousLast.txIndex } } : {}),
    ...(!finalPage && last ? { lower: { block: last.blockNumber, index: last.txIndex } } : {}),
  };
}

export async function loadGrantBoundary(
  page: number,
  readPage: (page: number, signal: AbortSignal) => Promise<Transaction[]>,
  signal: AbortSignal,
  maxPages = 5,
): Promise<GrantBoundary> {
  for (let checked = 0; page > 0 && checked < maxPages; checked++, page--) {
    signal.throwIfAborted();
    const last = (await readPage(page, signal)).at(-1);
    signal.throwIfAborted();
    if (last) return { upper: { block: last.blockNumber, index: last.txIndex } };
  }
  // Totals can include fully filtered pages. Preserve progress instead of
  // treating an exhausted request budget as an unbounded window.
  return page > 0 ? { nextPage: page } : {};
}

// Index-less responses from older proxies belong to the first page covering
// that block. This avoids repeating the entire block across regular pages.
function compare(tx: PseudonymizedTransaction, edge: Position): number {
  return tx.block_number - edge.block || (tx.tx_index ?? Infinity) - (edge.index ?? Infinity) || 0;
}

export function inGrantWindow(tx: PseudonymizedTransaction, window: GrantWindow): boolean {
  return (!window.upper || compare(tx, window.upper) < 0)
    && (!window.lower || compare(tx, window.lower) >= 0);
}

type FetchGrant = (grantId: string, addressId: string, limit: number, before?: number, cursor?: string, signal?: AbortSignal) => Promise<PseudonymizedTransactionsResponse>;

export async function loadGrantWindow(
  source: GrantSource, window: GrantWindow, fetch: FetchGrant, cursor?: string, maxBatches = 5, signal?: AbortSignal,
): Promise<{ rows: DisclosedRow[]; nextCursor?: string }> {
  const rows: DisclosedRow[] = [];
  const seen = new Set<string>();
  for (let batch = 0; batch < maxBatches; batch++) {
    signal?.throwIfAborted();
    if (cursor) seen.add(cursor);
    const response = await fetch(source.grantId, source.addressId, 100, window.upper ? window.upper.block + 1 : undefined, cursor, signal);
    signal?.throwIfAborted();
    const level = response.disclosure_level ?? source.level;
    // A response can observe a grant change after metadata was loaded. Do not
    // append it to earlier pages authorized at another level, even if empty.
    // The query's error state hides every cached page until metadata is retried.
    if (level !== source.level) throw new Error('Disclosure level changed');
    for (const tx of response.transactions) {
      if (inGrantWindow(tx, window)) rows.push({ source: { ...source, level }, tx });
    }
    const last = response.transactions.at(-1);
    if (!response.next_cursor || (last && window.lower && compare(last, window.lower) < 0)) return { rows };
    if (seen.has(response.next_cursor)) throw new Error('Disclosure pagination did not advance');
    cursor = response.next_cursor;
  }
  // Empty, scope-filtered batches may still carry a cursor. Preserve it so a
  // bounded fetch never silently hides older matches, including grant-only feeds.
  return { rows, nextCursor: cursor };
}

export type ComposedTransaction = (
  { kind: 'regular'; tx: Transaction } | { kind: 'disclosed'; tx: PseudonymizedTransaction; source: GrantSource }
) & { key: string; blockNumber: number; txIndex?: number };

export function mergeTransactions(regular: Transaction[], disclosed: DisclosedRow[]): ComposedTransaction[] {
  const hashes = new Set<string>();
  const positions = new Set<string>();
  const rows: ComposedTransaction[] = [];
  for (const tx of regular) {
    hashes.add(tx.hash.toLowerCase());
    if (tx.txIndex != null) positions.add(`${tx.blockNumber}:${tx.txIndex}`);
    const masked = tx.from?.startsWith('[') || tx.to?.startsWith('[') || tx.value == null || tx.value === '' || tx.value === 'hidden'
      || Object.values(tx.addressMetadata ?? {}).some((reason) => reason === 'no_access');
    const full = masked ? disclosed.find((grant) => grant.source.level === 'full' && (
      grant.tx.tx_hash?.toLowerCase() === tx.hash.toLowerCase()
      || (tx.txIndex != null && grant.tx.tx_index != null && tx.blockNumber === grant.tx.block_number && tx.txIndex === grant.tx.tx_index)
    )) : undefined;
    if (full) {
      if (full.tx.tx_hash) hashes.add(full.tx.tx_hash.toLowerCase());
      rows.push({ kind: 'disclosed', ...full, key: `grant:${full.source.grantId}:${tx.hash}`, blockNumber: tx.blockNumber, txIndex: tx.txIndex });
    } else {
      rows.push({ kind: 'regular', tx, key: tx.hash, blockNumber: tx.blockNumber, txIndex: tx.txIndex });
    }
  }
  // Prefer the richer disclosure when the same transaction occurs in several
  // grants. A visible regular row wins; a Full grant can replace a masked
  // regular row above, keeping its richer detail exclusively on the grant route.
  const rank = (level: string) => level === 'full' ? 2 : level === 'pseudonymous' ? 1 : 0;
  for (const [i, { source, tx }] of [...disclosed].sort((a, b) => rank(b.source.level) - rank(a.source.level)).entries()) {
    const hash = tx.tx_hash?.toLowerCase();
    const position = tx.tx_index == null ? undefined : `${tx.block_number}:${tx.tx_index}`;
    if (hash && hashes.has(hash) || position && positions.has(position)) continue;
    if (!hash && position === undefined && (tx.from === 'Mine' || tx.to === 'Mine')) continue;
    if (hash) hashes.add(hash);
    if (position) positions.add(position);
    rows.push({ kind: 'disclosed', source, tx, key: `grant:${source.grantId}:${source.addressId}:${hash ?? position ?? i}`, blockNumber: tx.block_number, txIndex: tx.tx_index });
  }
  return rows.sort((a, b) => b.blockNumber - a.blockNumber || (b.txIndex ?? Infinity) - (a.txIndex ?? Infinity) || 0);
}

export function grantLink(source: GrantSource, tx?: PseudonymizedTransaction): string {
  const base = `/grant/${encodeURIComponent(source.grantId)}/${encodeURIComponent(source.addressId)}`;
  return source.level === 'full' && tx?.tx_hash ? `${base}/tx/${encodeURIComponent(tx.tx_hash)}` : base;
}
