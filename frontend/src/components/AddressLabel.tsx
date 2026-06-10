import type { AddressVisibility, VisibilityReason } from '../lib/api';
import { LABEL_CONFIG } from './addressLabelConfig';

interface AddressLabelProps {
  reason?: VisibilityReason;
  visibility?: AddressVisibility | null;
  className?: string;
}

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
    <span
      data-testid="address-label"
      data-reason={r}
      className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium whitespace-nowrap ${c.classes} ${className}`}
    >
      {c.text}
    </span>
  );
}
