// Shared react-query retry predicate.
//
// RD-1063: don't retry doomed requests. A request that failed with a
// non-retryable client error (4xx) will fail the same way on every retry, so
// retrying just storms the backend (and wastes the user's battery/network).
// This is the belt-and-suspenders backstop: the real storm-stopper for the
// charts/gas/price surfaces is the page early-return + Layout `enabled:false`
// gates (frontend) plus the backend route→404 (authoritative). This predicate
// additionally suppresses retries on 403/404 (and other 4xx) so a leaked or
// restricted request doesn't get hammered three more times.
//
// The thrown error is `Error("API error: <status>")` (optionally prefixed with
// a detail string) — see fetchAPI in api.ts. We parse the status out of the
// message rather than depending on a structured error shape (kept minimal to
// avoid churn with the separate structured-error refactor).

const STATUS_RE = /API error: (\d{3})/;

// 4xx statuses that ARE worth retrying (transient client-side conditions):
//   408 Request Timeout, 429 Too Many Requests.
const RETRYABLE_4XX = new Set([408, 429]);

const MAX_ATTEMPTS = 3;

function parseStatus(error: unknown): number | null {
  const message = error instanceof Error ? error.message : '';
  const match = STATUS_RE.exec(message);
  return match ? Number(match[1]) : null;
}

/**
 * react-query `retry` predicate.
 *
 * @param failureCount number of times the query has already failed (0-based on
 *        the first call, per react-query semantics).
 * @param error the thrown error from the query function.
 * @returns true to retry, false to give up.
 */
export function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  const status = parseStatus(error);

  // Non-retryable client errors (4xx) except the transient ones: give up
  // immediately so we don't hammer a doomed/forbidden endpoint.
  if (status !== null && status >= 400 && status < 500 && !RETRYABLE_4XX.has(status)) {
    return false;
  }

  // 5xx, network/parse errors (no status), and the retryable 4xx: retry up to
  // the cap (matching react-query's default of 3 attempts).
  return failureCount < MAX_ATTEMPTS;
}
