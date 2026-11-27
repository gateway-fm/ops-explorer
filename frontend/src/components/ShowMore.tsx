// Long transaction lists (token transfers, logs, internal calls) are drawn a page at a
// time, so a transaction with thousands of rows does not mount thousands of rows, or
// start a lookup for each, until the reader asks for them.
export const ROWS_PER_PAGE = 50;

export function ShowMore({ shown, total, onMore, testId }: { shown: number; total: number; onMore: () => void; testId: string }) {
  if (shown >= total) return null;
  return (
    <button type="button" className="btn-secondary self-start text-sm" onClick={onMore} data-testid={testId}>
      Show {Math.min(ROWS_PER_PAGE, total - shown)} more ({total - shown} not shown)
    </button>
  );
}
