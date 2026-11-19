import { useId, type ReactNode } from 'react';
import { Fingerprint } from 'lucide-react';

// Shared from the original Auditor and disclosed-address signed-out screens.
// This presentation owns no authentication or data requests.
export function AuthenticationPrompt({ title, detail, actions, icon, children, checking = false }: {
  title: string;
  detail: string;
  actions?: ReactNode;
  icon?: ReactNode;
  children?: ReactNode;
  checking?: boolean;
}) {
  const heading = useId();
  return (
    <section aria-labelledby={heading} className="flex flex-col items-center justify-center py-16 space-y-4">
      <div className="w-16 h-16 rounded-full bg-primary-50 flex items-center justify-center border border-primary-200">
        {icon ?? <Fingerprint className="w-8 h-8 text-primary" aria-hidden="true" />}
      </div>
      <div role={checking ? 'status' : undefined} aria-live={checking ? 'polite' : undefined} className="space-y-4 text-center">
        <h2 id={heading} className="text-xl font-semibold text-neutral-900">{title}</h2>
        <p className="text-neutral-500 text-center max-w-md px-4">{detail}</p>
      </div>
      {children}
      {actions && <div className="mt-4 flex flex-wrap items-center justify-center gap-3 px-4">{actions}</div>}
    </section>
  );
}
