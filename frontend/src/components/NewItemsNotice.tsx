interface NewItemsNoticeProps {
  count: number;
  type: 'block' | 'transaction';
  onClick: () => void;
}

export function NewItemsNotice({ count, type, onClick }: NewItemsNoticeProps) {
  if (count <= 0) return null;

  const label = `${count.toLocaleString()} new ${type}${count === 1 ? '' : 's'}`;

  return (
    <button
      onClick={onClick}
      className="w-full px-4 py-2.5 text-sm font-medium text-primary-700 dark:text-primary-400 bg-primary-50 dark:bg-primary-900/20 border-b border-primary-200 dark:border-primary-800 hover:bg-primary-100 dark:hover:bg-primary-900/30 transition-colors cursor-pointer text-center"
    >
      {label} — click to load
    </button>
  );
}
