import { useEffect, useState, useSyncExternalStore } from 'react';
import { flushSync } from 'react-dom';
import { appSession } from '../lib/v2Session';
import { WalletController } from '../lib/v2Wallet';
import { PageHeader } from '../components/PageHeader';
import { DisclosurePanelPresentation } from '../components/DisclosurePanelPresentation';
import { LinkedWalletAddress } from '../components/LinkedWalletPresentation';
import { CheckCircle2, Wallet } from 'lucide-react';

export function WalletsV2() {
  const session = useSyncExternalStore(appSession.subscribe, appSession.snapshot);
  return session.identity ? <WalletPanel key={session.identity.activation} /> : null;
}
function WalletPanel() {
  const [wallet] = useState(() => new WalletController({ flush: work => flushSync(work) }));
  const state = useSyncExternalStore(wallet.subscribe, wallet.snapshot);
  useEffect(() => {
    void wallet.load();
    // Page/session suspension clears proof facts through the original scope.
    const hide = () => { flushSync(wallet.cancel); };
    window.addEventListener('pagehide', hide);
    return () => { window.removeEventListener('pagehide', hide); wallet.cancel(); };
  }, [wallet]);
  const activeStep = state.stage !== 'idle' || state.unlink !== null;
  // A confirm/unlink can already have reached the service. Do not offer a
  // local cancel control that would look like an undo for that request.
  const saving = state.busy && (state.stage === 'confirm' || state.unlink !== null);
  return <section className="space-y-5" aria-label="Your wallet links">
    <PageHeader title="Wallets" subtitle="Addresses linked to your current OPS identity" />
    <p className="text-sm text-neutral-500">Link an address you control to your OPS identity. Linking does not grant group membership or authorize a transaction.</p>
    {state.error && <p role="alert" className="rounded-lg border border-error-200 bg-error-50 p-3 text-sm text-error-700">{state.error}</p>}
    <div className="flex flex-wrap gap-2">
      <button className="btn-secondary" disabled={state.busy || activeStep} onClick={() => void wallet.load()}>Refresh links</button>
      <button className="btn-primary" disabled={state.busy || activeStep} onClick={() => void wallet.connect()}>Link wallet</button>
      {activeStep && !saving && <button className="btn-secondary" onClick={wallet.cancel}>{state.unlink ? 'Cancel unlinking' : 'Cancel linking'}</button>}
    </div>
    {state.busy && <p role="status" className="text-sm text-neutral-500">Waiting for the wallet operation…</p>}
    {saving && <p className="text-sm text-neutral-500">Saving is in progress. Closing this page cannot undo a request already sent. Wait for its result, then refresh links before starting another action.</p>}
    {activeStep && !saving && state.busy && <p className="text-xs text-neutral-500">Canceling here stops this OPS step. Dismiss any open wallet prompt separately.</p>}
    <DisclosurePanelPresentation title="Linked addresses">
      {state.links.length ? <ul className="divide-y divide-neutral-100">{state.links.map(link => <li key={link.address} className="flex flex-wrap items-center justify-between gap-3 p-4"><div className="min-w-0"><div className="flex items-center gap-1.5"><Wallet className="h-4 w-4 shrink-0 text-primary"/><LinkedWalletAddress address={link.address}/></div><p className="mt-1 flex items-center gap-1 text-xs text-neutral-500"><CheckCircle2 className="h-3.5 w-3.5 text-success-600"/>Verified <time dateTime={link.verifiedAt}>{new Date(link.verifiedAt).toLocaleString()}</time></p></div><button className="btn-secondary" disabled={state.busy || activeStep} onClick={() => wallet.prepareUnlink(link.address)}>Unlink</button></li>)}</ul> : <p className="p-4 text-sm text-neutral-500">No wallet links are currently displayed. Refresh links to check the current state.</p>}
    </DisclosurePanelPresentation>
    {state.stage === 'choose' && <DisclosurePanelPresentation title="Choose the address to link"><div className="space-y-2 p-4">{state.accounts.map(value => <button className="btn-secondary w-full" key={value} disabled={state.busy} onClick={() => void wallet.select(value)}><code className="break-all text-xs">{value}</code></button>)}</div></DisclosurePanelPresentation>}
    {state.message && <DisclosurePanelPresentation title="Wallet ownership proof"><div className="space-y-4 p-4"><p className="text-sm text-neutral-500">Sign a message to prove ownership. This does not authorize a transaction.</p><details><summary className="cursor-pointer text-sm font-medium text-primary">Ownership proof details</summary><pre className="mt-3 max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-neutral-100 p-3 font-mono text-xs">{state.message}</pre></details>{state.stage === 'sign' && <button className="btn-primary" disabled={state.busy} onClick={() => void wallet.sign()}>Sign ownership proof</button>}{state.stage === 'confirm' && <button className="btn-primary" disabled={state.busy} onClick={() => void wallet.confirm()}>Confirm link</button>}</div></DisclosurePanelPresentation>}
    {state.unlink && <section className="card space-y-4 border-warning-200 p-5" role="alert"><h2 className="text-base font-semibold text-neutral-900">Permanently unlink this address?</h2><code className="block break-all text-xs">{state.unlink}</code><p className="text-sm text-neutral-500">This address cannot be linked to this OPS identity again under the current deployment rules. There is no Undo.</p><button className="btn-secondary text-error-600" disabled={state.busy} onClick={() => void wallet.unlink()}>Confirm permanent unlink</button></section>}
  </section>;
}
