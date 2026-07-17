// RD-1149: advance a by-address feed's pagination without disturbing the rest of
// the query string. Prefer the opaque keyset cursor; fall back to the last row's
// block number for older proxies. `cursor` and `before` are kept mutually
// exclusive, and every other param — notably the active `tab` — is preserved.
// (Building a fresh param set per panel is what dropped `tab=transactions` and
// bounced "Load more" back to the details tab.)
export function nextPageSearchParams(
  prev: URLSearchParams,
  nextCursor: string | null | undefined,
  before: string | undefined,
): URLSearchParams {
  const next = new URLSearchParams(prev);
  next.delete('cursor');
  next.delete('before');
  if (nextCursor) next.set('cursor', nextCursor);
  else if (before !== undefined) next.set('before', before);
  return next;
}
