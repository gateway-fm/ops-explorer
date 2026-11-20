import type { ReactNode } from 'react';

// The transaction overview's original compact row. Controllers supply already
// authorized address controls and exact quantities; this view performs no reads.
export function InlineTokenTransferPresentation({from,to,quantity,token,details}: {
  from: ReactNode; to: ReactNode; quantity: ReactNode; token: ReactNode; details?: ReactNode;
}) {
  return <div data-testid="transfer-row" className="flex items-center gap-1.5 flex-wrap text-sm">
    <span className="text-neutral-500">From</span>{from}
    <span className="text-neutral-500">To</span>{to}
    <span className="text-neutral-500">For</span>
    <span className="font-mono text-success-600 font-medium">{quantity}</span>
    {token}{details}
  </div>;
}
