import { useEffect, useState, useSyncExternalStore } from 'react';
import { flushSync } from 'react-dom';
import { appSession } from '../lib/v2Session';
import { WalletController } from '../lib/v2Wallet';

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
  return <section className="space-y-4" aria-label="Your wallet links">
    <h2>Wallets</h2>
    <p>Link an address you control to your OPS identity. Linking does not grant group membership or authorize a transaction.</p>
    {state.error && <p role="alert">{state.error}</p>}
    <div className="flex gap-4"><button disabled={state.busy} onClick={() => void wallet.load()}>Refresh links</button><button disabled={state.busy} onClick={() => void wallet.connect()}>Link wallet</button><button onClick={wallet.cancel}>Cancel wallet operation</button></div>
    {state.busy && <p role="status">Waiting for the wallet operation…</p>}
    {state.links.length ? <ul>{state.links.map(link => <li key={link.address}><code>{link.address}</code> · Verified {link.verifiedAt} <button disabled={state.busy} onClick={() => wallet.prepareUnlink(link.address)}>Unlink</button></li>)}</ul> : <p>No wallet links are currently displayed. Refresh links to check the current state.</p>}
    {state.stage === 'choose' && <section><h3>Choose the address to link</h3>{state.accounts.map(value => <button key={value} onClick={() => void wallet.select(value)}><code>{value}</code></button>)}</section>}
    {state.message && <section><h3>Wallet ownership proof</h3><pre className="whitespace-pre-wrap break-all">{state.message}</pre>{state.stage === 'sign' && <button disabled={state.busy} onClick={() => void wallet.sign()}>Sign ownership proof</button>}{state.stage === 'confirm' && <button disabled={state.busy} onClick={() => void wallet.confirm()}>Confirm link</button>}</section>}
    {state.unlink && <section role="alert"><p>Unlink <code>{state.unlink}</code>? This address cannot be linked to this OPS identity again under the current deployment rules. There is no Undo.</p><button disabled={state.busy} onClick={() => void wallet.unlink()}>Confirm permanent unlink</button></section>}
  </section>;
}
