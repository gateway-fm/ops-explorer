import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { AlertTriangle, Eye, Loader2 } from 'lucide-react';
import { useImpersonation } from '../hooks/useImpersonation';
import { useAuth } from '../lib/auth';
import { redirectToLogin } from '../lib/login';

/**
 * RD-928 / RD-994 — cross-origin "View as user" entry point.
 *
 * The privacy-proxy admin dashboard (port 5173) is the primary initiator for
 * the View-as flow: an org admin clicks "View as in Explorer" on a user-list
 * row, which opens a new tab at /view-as?did=<target_did>&org=<org_id> here.
 * This route mints a session against the BFF on the admin's behalf (bound to
 * the explicit org) and lands them on the explorer home with the banner
 * active.
 *
 * Failure modes:
 *  - Not authenticated: redirect to login, resume here after login.
 *  - Missing/empty ?did=: render an instructional error.
 *  - Missing/empty ?org=: render an instructional error (RD-994 — the org is
 *    mandatory; the dashboard always supplies the currently-selected org).
 *  - Target-not-in-org / unknown target: the BFF returns 404 which surfaces
 *    here as a user-facing error (no info leak — same 404 shape that an
 *    attempt to impersonate a non-existent user produces).
 *  - Org not administered by the caller: the BFF returns 403.
 *  - Another session already active: stop() it first, then start the new
 *    one. The hook's no-chaining guard normally throws on this; the entry
 *    route is the one place where "switch targets" is the intended UX.
 */
export function ViewAsEntry() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const { isActive, start, stop } = useImpersonation();
  const [stage, setStage] = useState<'idle' | 'starting' | 'success' | 'error'>('idle');
  const [message, setMessage] = useState<string>('');

  const targetDID = params.get('did')?.trim() ?? '';
  const orgID = params.get('org')?.trim() ?? '';

  useEffect(() => {
    if (authLoading) return;

    if (!isAuthenticated) {
      // Preserve the current URL so login flow returns us here with ?did= and
      // ?org= intact.
      redirectToLogin(window.location.pathname + window.location.search);
      return;
    }

    if (!targetDID) {
      setStage('error');
      setMessage('Missing target DID. The URL must include a ?did=<target> parameter.');
      return;
    }

    if (!orgID) {
      setStage('error');
      setMessage('Missing organization. The URL must include an ?org=<org_id> parameter. Open View-as from the admin dashboard with an org selected.');
      return;
    }

    let cancelled = false;

    (async () => {
      setStage('starting');
      try {
        // If a session is already up, drop it first — switching targets is
        // the legitimate UX here. The "no chained impersonation" guard in
        // the hook is for in-app re-entry, not for the cross-origin entry.
        if (isActive) {
          await stop();
        }
        await start(targetDID, orgID);
        if (cancelled) return;
        setStage('success');
        // Hand off to the home page; the banner is already active because
        // start() wrote the token to context + URL.
        navigate('/', { replace: true });
      } catch (err) {
        if (cancelled) return;
        setStage('error');
        const msg = err instanceof Error ? err.message : 'Failed to start impersonation.';
        setMessage(msg);
      }
    })();

    return () => {
      cancelled = true;
    };
    // Intentionally key off targetDID + org + auth state. The hook callbacks
    // are stable references from useImpersonation/useAuth contexts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetDID, orgID, isAuthenticated, authLoading]);

  if (authLoading || stage === 'idle' || stage === 'starting') {
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <div className="w-16 h-16 rounded-full bg-amber-50 flex items-center justify-center border border-amber-200">
          <Loader2 className="w-8 h-8 text-amber-600 animate-spin" />
        </div>
        <h2 className="text-xl font-semibold text-neutral-900">Starting View-as session…</h2>
        {targetDID && (
          <p className="text-sm text-neutral-500 font-mono">{targetDID}</p>
        )}
      </div>
    );
  }

  if (stage === 'success') {
    // Navigated away; this branch is unreachable once navigate() fires, but
    // render a placeholder for the brief frame before the unmount.
    return (
      <div className="flex flex-col items-center justify-center py-16 space-y-4">
        <Eye className="w-12 h-12 text-amber-600" />
        <h2 className="text-xl font-semibold text-neutral-900">Session active</h2>
      </div>
    );
  }

  // stage === 'error'
  return (
    <div className="flex flex-col items-center justify-center py-16 space-y-4">
      <div className="w-16 h-16 rounded-full bg-error-50 flex items-center justify-center border border-error-200">
        <AlertTriangle className="w-8 h-8 text-error-600" />
      </div>
      <h2 className="text-xl font-semibold text-neutral-900">Could not start View-as</h2>
      <p className="text-neutral-500 text-center max-w-md">{message}</p>
      <button
        onClick={() => navigate('/', { replace: true })}
        className="btn-primary"
      >
        Back to Explorer
      </button>
    </div>
  );
}

export default ViewAsEntry;
