export function TransferMethodBadge({ method }: { method: string }) {
  const tone =
    method === 'mint'
      ? 'bg-primary/10 text-primary'
      : method === 'burn'
      ? 'bg-error-50 text-error-700 dark:bg-error-500/10 dark:text-error-500'
      : 'bg-neutral-200/60 text-neutral-700';
  return (
    <span className={`inline-flex items-center rounded-md px-2 py-0.5 font-mono text-xs ${tone}`}>
      {method}
    </span>
  );
}
