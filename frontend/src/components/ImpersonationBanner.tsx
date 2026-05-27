import { Eye, X } from 'lucide-react';
import { useImpersonation } from '../hooks/useImpersonation';

/**
 * Sticky amber banner shown while an admin is viewing the explorer "as" a
 * target user. Sits above all routed content (rendered in Layout) and acts
 * as the canonical reminder + exit affordance for the view-as session.
 *
 * Renders nothing when no impersonation session is active so it does not
 * take up layout space in the normal browsing flow.
 */
export function ImpersonationBanner() {
  const { isActive, targetDID, expiresAt, stop } = useImpersonation();

  if (!isActive) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      data-testid="impersonation-banner"
      className="sticky top-0 z-40 w-full bg-amber-400 text-amber-950 border-b border-amber-600 shadow-sm"
    >
      <div className="max-w-7xl mx-auto px-4 sm:px-6 py-2 flex items-center gap-3 text-sm">
        <Eye className="w-4 h-4 shrink-0" />
        <span className="font-semibold">View-as mode</span>
        <span className="text-amber-900 truncate">
          Viewing as <span className="font-mono">{truncateDID(targetDID)}</span>
          {expiresAt ? (
            <span className="ml-2 text-amber-800 hidden sm:inline">
              expires {formatExpiry(expiresAt)}
            </span>
          ) : null}
        </span>
        <button
          type="button"
          onClick={() => {
            void stop();
          }}
          className="ml-auto inline-flex items-center gap-1 px-3 py-1 rounded-md bg-amber-950/10 hover:bg-amber-950/20 transition-colors text-amber-950 font-medium"
        >
          <X className="w-4 h-4" />
          Stop viewing as
        </button>
      </div>
    </div>
  );
}

/**
 * Truncate the DID for display. The full DID is kept reachable via the
 * native `title` tooltip and in the impersonation context if any caller
 * needs it programmatically.
 */
function truncateDID(did: string | null): string {
  if (!did) return '(loading)';
  if (did.length <= 28) return did;
  return `${did.slice(0, 16)}…${did.slice(-8)}`;
}

function formatExpiry(d: Date): string {
  const ms = d.getTime() - Date.now();
  if (ms <= 0) return 'now';
  const minutes = Math.round(ms / 60000);
  if (minutes < 60) return `in ${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const rem = minutes % 60;
  return rem === 0 ? `in ${hours}h` : `in ${hours}h${rem}m`;
}
