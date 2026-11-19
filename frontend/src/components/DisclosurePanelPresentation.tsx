import type { ReactNode } from 'react';

export function DisclosurePanelPresentation({title,children}:{title:ReactNode;children:ReactNode}) {
  return <div className="card"><div className="px-4 py-3 border-b border-neutral-200"><span className="text-sm font-medium text-neutral-700" data-testid="disclosure-banner">{title}</span></div>{children}</div>;
}
