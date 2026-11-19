import type { ReactNode } from 'react';
export function BlockInfoRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="info-row" data-testid="info-row" data-label={label}>
      <div className="info-label">{label}</div>
      <div className="info-value">{value}</div>
    </div>
  );
}
