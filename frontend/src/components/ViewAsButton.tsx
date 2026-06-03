import { useState } from 'react';
import { Eye } from 'lucide-react';
import { useImpersonation } from '../hooks/useImpersonation';

interface ViewAsButtonProps {
  /** Target user DID to start a view-as session for. */
  targetDID: string;
  /** RD-994: org the view-as session is anchored to. Mandatory — the proxy
   *  resolves the impersonated view against this org. */
  orgID: string;
  /** Visual variant. "button" is full-size; "inline" is a smaller chip-like
   *  affordance suitable for being placed next to a DID link. */
  variant?: 'button' | 'inline';
  /** Optional className override / additional classes. */
  className?: string;
  /** Called when start succeeds. Useful for closing menus / toasts. */
  onStarted?: () => void;
}

/**
 * "View as user" affordance. Only renders if the user is signed in — the
 * BFF gates tier-2 admin checking on the privacy-proxy probe, so we don't
 * need to know the admin status client-side. A non-admin who clicks gets
 * an error toast (currently surfaced via the impersonation context's
 * `error` state) and no session.
 *
 * Refuses to start a second session over an existing one; UI prompts the
 * admin to stop the current view-as first.
 */
export function ViewAsButton({
  targetDID,
  orgID,
  variant = 'button',
  className,
  onStarted,
}: ViewAsButtonProps) {
  const { start, isActive } = useImpersonation();
  const [pending, setPending] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);

  const handleClick = async () => {
    setLocalError(null);
    if (isActive) {
      setLocalError('Stop the current view-as session first.');
      return;
    }
    setPending(true);
    try {
      await start(targetDID, orgID);
      onStarted?.();
    } catch (e) {
      // start() already populated the context error; mirror locally so the
      // button can show inline feedback without depending on a global toast
      // host being present.
      setLocalError(e instanceof Error ? e.message : 'Failed to start view-as');
    } finally {
      setPending(false);
    }
  };

  const base = 'inline-flex items-center gap-1.5 font-medium transition-colors';
  const styled =
    variant === 'inline'
      ? `${base} px-2 py-1 text-xs rounded-md border border-amber-300 text-amber-800 bg-amber-50 hover:bg-amber-100`
      : `${base} px-3 py-1.5 text-sm rounded-lg border border-amber-300 text-amber-800 bg-amber-50 hover:bg-amber-100`;

  return (
    <div className={className}>
      <button
        type="button"
        onClick={() => {
          void handleClick();
        }}
        disabled={pending || isActive}
        title={
          isActive
            ? 'A view-as session is already active — stop it first'
            : 'Start a read-only view as this user'
        }
        data-testid="view-as-button"
        className={`${styled} disabled:opacity-50 disabled:cursor-not-allowed`}
      >
        <Eye className={variant === 'inline' ? 'w-3 h-3' : 'w-4 h-4'} />
        {pending ? 'Starting…' : 'View as user'}
      </button>
      {localError ? (
        <div className="mt-1 text-xs text-red-600" role="alert">
          {localError}
        </div>
      ) : null}
    </div>
  );
}
