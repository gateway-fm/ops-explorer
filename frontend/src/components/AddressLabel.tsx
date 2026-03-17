import type { VisibilityReason } from '../lib/api';

interface AddressLabelProps {
  reason: VisibilityReason | undefined;
  className?: string;
}

const LABEL_CONFIG: Record<string, { text: string; classes: string }> = {
  own_address:      { text: 'Mine',     classes: 'bg-success-50 text-success-700' },
  disclosure_grant: { text: 'Disclosed', classes: 'bg-primary-100 text-primary-600' },
  rbac_group_member:{ text: 'My Org',   classes: 'bg-warning-50 text-warning-700' },
  public_address:   { text: 'Public',   classes: 'bg-neutral-100 text-neutral-500' },
  no_access:        { text: 'Private',  classes: 'bg-neutral-50 text-neutral-400' },
};

export function AddressLabel({ reason, className = '' }: AddressLabelProps) {
  if (!reason) return null;

  const c = LABEL_CONFIG[reason];
  if (!c) return null;

  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium whitespace-nowrap ${c.classes} ${className}`}>
      {c.text}
    </span>
  );
}
