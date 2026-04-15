import type { AddressVisibility, VisibilityReason } from '../lib/api';

interface AddressLabelProps {
  reason?: VisibilityReason;
  visibility?: AddressVisibility | null;
  className?: string;
}

const LABEL_CONFIG: Record<string, { text: string; classes: string }> = {
  own_address:          { text: 'Mine',                 classes: 'bg-success-50 text-success-700' },
  rbac_group_member:    { text: 'My Org',               classes: 'bg-warning-50 text-warning-700' },
  public_address:       { text: 'Public',               classes: 'bg-neutral-100 text-neutral-500' },
  disclosure_grant:     { text: 'Disclosed',             classes: 'bg-purple-100 text-purple-700' },
  visible_to_grant:    { text: 'Shared',                classes: 'bg-blue-100 text-blue-700' },
  participant_override: { text: 'Counterparty',         classes: 'bg-primary-50 text-primary-700' },
};

export function AddressLabel({ reason, visibility, className = '' }: AddressLabelProps) {
  // If visibility data is available and address is not visible,
  // don't show a label. The address is either [PRIVATE] (self-evident)
  // or visible via participant override (no label needed).
  if (visibility && !visibility.visible) return null;

  const r = reason ?? visibility?.reason;
  if (!r) return null;

  const c = LABEL_CONFIG[r];
  if (!c) return null;

  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium whitespace-nowrap ${c.classes} ${className}`}>
      {c.text}
    </span>
  );
}
