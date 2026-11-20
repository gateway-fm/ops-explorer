interface NewItemsNoticeProps {
  count?: number;
  changed?: boolean;
  observedWindow?: boolean;
  type: 'block' | 'transaction';
  onClick: () => void;
  // When true, render the count as "N+" — used when the live window can
  // only confirm a lower bound (e.g. snapshot has fallen off the page).
  approximate?: boolean;
}

export function NewItemsNotice({ count, type, onClick, approximate = false, changed = false, observedWindow = false }: NewItemsNoticeProps) {
  if (!changed && (count === undefined || count <= 0)) return null;

  const countLabel = approximate ? `${count?.toLocaleString()}+` : count?.toLocaleString();
  const label = changed && count === undefined ? 'Visible list changed' : observedWindow ? `${countLabel} newly observed ${type}${count === 1 ? '' : 's'} in this window` : `${countLabel} new ${type}${count === 1 ? '' : 's'}`;

  return (
    <button
      onClick={onClick}
      data-testid="new-items-notice"
      className="w-full px-4 py-2.5 text-sm font-medium text-primary-700 dark:text-primary-400 bg-primary-50 dark:bg-primary-900/20 border-b border-primary-200 dark:border-primary-800 hover:bg-primary-100 dark:hover:bg-primary-900/30 transition-colors cursor-pointer text-center"
    >
      {label} — {changed ? 'refresh' : 'click to load'}
    </button>
  );
}
