import { useState } from 'react';
import { Fingerprint, Loader2, RefreshCw, ShieldAlert } from 'lucide-react';
import type { SessionState } from '../lib/v2Session';
import { appSession } from '../lib/v2Session';
import { AuthenticationPrompt } from './AuthenticationPrompt';

export function NativeAuthenticationPrompt({ state }: { state: SessionState }) {
  const [pending, setPending] = useState<'login' | 'check' | null>(null);
  const [error, setError] = useState<string | null>(null);
  // A failed shared-state check invalidates identity too, but does not prove
  // that another account was selected. Offer verification recovery in that
  // case rather than asking the person to accept an unverified account.
  const accountChanged = state.changed && !state.error;
  const expired = !state.error && (state.reason === 'expired' || !!state.identity && state.identity.expires <= Math.floor(Date.now() / 1000));
  const checking = state.loading;
  const entryState = checking ? 'loading' : accountChanged ? 'changed' : expired ? 'expired' : state.error ? 'error' : 'anonymous';
  const title = checking ? 'Checking your session'
    : accountChanged ? 'Your account changed'
    : expired ? 'Session expired'
    : state.error ? 'Unable to verify your session'
    : 'Sign in to Explorer';
  const detail = checking ? 'Please wait while we verify your access to this network.'
    : accountChanged ? 'Your account changed in another tab. Confirm the current session or sign in again before viewing private data.'
    : expired ? 'Sign in again to continue viewing the network with your current permissions.'
    : state.error ? 'Try checking your session again, or sign in to choose an account.'
    : 'Sign in to view the blocks, transactions and accounts available to you.';

  async function run(action: 'login' | 'check', operation: () => Promise<unknown>) {
    if (pending) return;
    setPending(action); setError(null);
    try { await operation(); }
    catch { setError(action === 'login' ? 'Sign in could not start. Please try again.' : 'Your session could not be checked. Please try again.'); }
    finally { setPending(null); }
  }
  const busy = pending !== null;
  const login = (
    <button type="button" disabled={busy} aria-busy={pending === 'login'}
      onClick={() => { void run('login', () => appSession.login()); }}
      className={accountChanged ? 'btn-secondary' : 'btn-primary'} data-testid="entry-sign-in">
      {pending === 'login' ? <Loader2 className="w-4 h-4 animate-spin" aria-hidden="true" /> : <Fingerprint className="w-4 h-4" aria-hidden="true" />}
      {pending === 'login' ? 'Opening sign in…' : expired || accountChanged ? 'Sign in again' : 'Sign in to Explorer'}
    </button>
  );
  const actions = checking ? undefined : <>
    {accountChanged && <button type="button" className="btn-primary" disabled={busy} aria-busy={pending === 'check'}
      onClick={() => { void run('check', () => appSession.check(true)); }}>
      {pending === 'check' ? <Loader2 className="w-4 h-4 animate-spin" aria-hidden="true" /> : <RefreshCw className="w-4 h-4" aria-hidden="true" />}
      {pending === 'check' ? 'Checking session…' : 'Use current session'}
    </button>}
    {login}
    {!accountChanged && state.error && <button type="button" className="btn-secondary" disabled={busy} aria-busy={pending === 'check'}
      onClick={() => { void run('check', () => appSession.check()); }}>
      <RefreshCw className={`w-4 h-4 ${pending === 'check' ? 'animate-spin' : ''}`} aria-hidden="true" />
      {pending === 'check' ? 'Checking session…' : 'Check again'}
    </button>}
  </>;
  return <div data-testid="explorer-auth-entry" data-auth-state={entryState}><AuthenticationPrompt title={title} detail={detail} checking={checking} actions={actions}
    icon={checking ? <Loader2 className="w-8 h-8 text-primary animate-spin" aria-hidden="true" /> : accountChanged ? <ShieldAlert className="w-8 h-8 text-primary" aria-hidden="true" /> : undefined}>
    {(error || state.error) && <p role="alert" className="max-w-md px-4 text-center text-sm text-error-600">{error ?? state.error}</p>}
  </AuthenticationPrompt></div>;
}
