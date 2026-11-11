import { beforeEach, describe, expect, it, vi } from 'vitest';
import { privateKeyToAccount } from 'viem/accounts';
import { hexToString, recoverMessageAddress, type Hex } from 'viem';
import { AppSession } from './v2Session';
import { ownWallets, walletChallenge, WalletController, type WalletProvider } from './v2Wallet';

const signer = privateKeyToAccount(('0x' + '17'.repeat(32)) as Hex), address = signer.address.toLowerCase();
const first = { protocol: 'ops-v2', authenticated: true, did: 'ops:p1:' + 'A'.repeat(43), source_profile: 'azure', session_id: '11111111-1111-1111-1111-111111111111', expires_at: 1300 };
const second = { ...first, did: 'ops:p1:' + 'B'.repeat(43), session_id: '22222222-2222-2222-2222-222222222222' };
const iso = (n: number) => new Date(n * 1000).toISOString().replace('.000Z', 'Z');
function challenge(actor = first.did, service = 'https://wallet-service.test') {
  const nonce = 'ab'.repeat(32), expiresAt = iso(1300);
  const message = ['OPS wallet link v1', 'Service: ' + service, 'Network: sha256:' + 'a'.repeat(64), 'Actor: ' + actor, 'Address: ' + address, 'Nonce: ' + nonce, 'IssuedAt: ' + iso(1000), 'ExpiresAt: ' + expiresAt, '', 'I authorize this wallet to be linked to this OPS identity. This does not authorize a transaction.'].join('\n');
  return { nonce, message, expiresAt };
}
const reply = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
const deferred = <T,>() => { let resolve!: (value: T) => void; const promise = new Promise<T>(r => { resolve = r; }); return { promise, resolve }; };
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); });
async function fixture(options: { sign?: (message: string) => Promise<string>; recover?: typeof recoverMessageAddress } = {}) {
  let status = first, linked = false, revoked = false;
  const actions: string[] = [];
  const transport = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    if (path.endsWith('/auth/status')) return reply(status);
    if (path.endsWith('/auth/login')) return new Promise<Response>(() => {});
    if (path.endsWith('/eth/link/challenge')) { actions.push('challenge'); return reply(challenge()); }
    if (path.endsWith('/eth/link/verify')) { actions.push('verify'); linked = true; return reply({ address, status: 'linked' }); }
    if (init?.method === 'DELETE') { actions.push('unlink'); linked = false; revoked = true; return reply({ address, status: 'revoked' }); }
    if (path.endsWith('/eth/addresses')) { actions.push('own'); return reply(linked && !revoked ? [{ address, verifiedAt: iso(1000), proofVersion: 2 }] : []); }
    throw Error('unexpected fixture route');
  });
  const session = new AppSession('/api', () => 1000, transport as typeof fetch); await session.check();
  const events = new Map<string, Set<() => void>>();
  const request = vi.fn(async ({ method, params }: { method: string; params?: unknown[] }) => {
    if (method === 'eth_accounts' || method === 'eth_requestAccounts') return [signer.address];
    if (method === 'personal_sign') {
      expect(params?.[1]).toBe(address); const message = hexToString(params?.[0] as Hex);
      expect(message).toBe(challenge().message);
      return options.sign ? options.sign(message) : signer.signMessage({ message });
    }
    throw Error('forbidden provider method');
  });
  const provider: WalletProvider = { request, on: (kind, callback) => { if (!events.has(kind)) events.set(kind, new Set()); events.get(kind)!.add(callback); }, removeListener: (kind, callback) => { events.get(kind)?.delete(callback); } };
  const controller = new WalletController({ session, provider: () => provider, now: () => 1000, recover: options.recover });
  return { controller, session, provider, events, actions, transport, switch: async (sameActor = false) => { status = sameActor ? { ...first, session_id: second.session_id } : second; await session.check(true); } };
}
async function ready(controller: WalletController) { await controller.connect(); await controller.select(address); }

describe('native wallet UI contract', () => {
  it('signs exact EIP191 bytes, explicitly confirms, lists own proof2 and explicitly revokes', async () => {
    const f = await fixture(); await ready(f.controller);
    expect(f.controller.snapshot().message).toContain('https://wallet-service.test');
    await f.controller.sign(); expect(f.actions).toEqual(['challenge']);
    expect(f.controller.snapshot().stage).toBe('confirm'); await f.controller.confirm();
    expect(f.actions).toEqual(['challenge', 'verify', 'own']); expect(f.controller.snapshot().links[0].address).toBe(address);
    f.controller.prepareUnlink(address); expect(f.actions).not.toContain('unlink'); await f.controller.unlink();
    expect(f.actions).toEqual(['challenge', 'verify', 'own', 'unlink', 'own']); expect(f.controller.snapshot().links).toEqual([]);
    expect(f.transport.mock.calls.filter(call => String(call[0]).includes('/eth/')).every(call => (call[1]?.headers as Headers).get('X-OPS-App-Session') === first.session_id)).toBe(true);
  });
  it('rejects V1 wrappers, inferred system proof, duplicate addresses and changed message bindings', () => {
    const link = { address, verifiedAt: iso(1000), proofVersion: 2 };
    for (const v of [{ addresses: [link] }, [{ ...link, proofVersion: 1 }], [link, link], [{ ...link, link_type: 'user' }]]) expect(() => ownWallets(v)).toThrow();
    const good = challenge(); expect(walletChallenge(good, first.did, address, 1000)).toEqual(good);
    for (const v of [{ ...good, nonce: '00'.repeat(32) }, { ...good, message: good.message + '\n' }, { ...good, expiresAt: iso(1301) }, challenge(second.did)]) expect(() => walletChallenge(v, first.did, address, 1000)).toThrow();
    expect(() => walletChallenge(good, first.did, address, 1300)).toThrow();
    // A source-owned service need not equal or be a browser-routable SSO URL.
    expect(walletChallenge(challenge(first.did, 'https://' + 'a'.repeat(2000)), first.did, address, 1000).message).toContain('https://');
  });
  it.each(['actor', 'sameActor', 'refresh', 'provider', 'dispose'])('held personal_sign cannot submit after %s changes', async kind => {
    const held = deferred<string>(); const f = await fixture({ sign: () => held.promise }); await ready(f.controller);
    const pending = f.controller.sign(); await vi.waitFor(() => expect(f.provider.request).toHaveBeenCalledWith(expect.objectContaining({ method: 'personal_sign' })));
    if (kind === 'actor' || kind === 'sameActor') await f.switch(kind === 'sameActor');
    else if (kind === 'refresh') { void f.session.login('/wallets', true); expect(f.controller.snapshot().message).toBeNull(); }
    else if (kind === 'provider') for (const callback of [...(f.events.get('accountsChanged') ?? [])]) callback();
    else f.controller.stop();
    held.resolve(await signer.signMessage({ message: challenge().message })); await pending; await f.controller.confirm();
    expect(f.actions).toEqual(['challenge']); expect(f.controller.snapshot().message).toBeNull(); expect(f.controller.snapshot().stage).toBe('idle');
  });
  it('held signature recovery cannot publish or verify after a queued native activation event', async () => {
    const held = deferred<`0x${string}`>(), recovering = vi.fn(() => held.promise); const f = await fixture({ recover: recovering }); await ready(f.controller);
    const pending = f.controller.sign(); await vi.waitFor(() => expect(recovering).toHaveBeenCalled());
    f.session.notice(new StorageEvent('storage', { key: 'ops-v2-app-activation', storageArea: localStorage, url: window.location.href }));
    held.resolve(signer.address); await pending; await f.controller.confirm();
    expect(f.actions).toEqual(['challenge']); expect(f.controller.snapshot().message).toBeNull();
  });
  it('an old prompt finally cannot erase a newer list scope', async () => {
    const held = deferred<string>(), f = await fixture({ sign: () => held.promise }); await ready(f.controller);
    const pending = f.controller.sign(); await vi.waitFor(() => expect(f.provider.request).toHaveBeenCalledWith(expect.objectContaining({ method: 'personal_sign' })));
    await f.controller.load(); const snapshot = f.controller.snapshot();
    held.resolve(await signer.signMessage({ message: challenge().message })); await pending;
    expect(f.controller.snapshot()).toBe(snapshot); expect(f.actions).toEqual(['challenge', 'own']);
  });
  it('an uncertain verify is never replayed and an explicit own read reconciles it', async () => {
    const f = await fixture(); await ready(f.controller); await f.controller.sign();
    const original = f.transport.getMockImplementation()!;
    f.transport.mockImplementationOnce(async (...args) => { await original(...args); throw Error('owned withheld reply'); });
    await f.controller.confirm(); await f.controller.confirm();
    expect(f.actions).toEqual(['challenge', 'verify']); expect(f.controller.snapshot().message).toBeNull();
    await f.controller.load(); expect(f.actions).toEqual(['challenge', 'verify', 'own']); expect(f.controller.snapshot().links[0].address).toBe(address);
  });
});

describe('one immutable AppSession wallet scope', () => {
  it('does not recapture B after A prompt, keeps exact wallet business409, and clears app409', async () => {
    let conflict = 'wallet state conflict';
    const transport = vi.fn(async (path: RequestInfo | URL) => String(path).endsWith('/auth/status') ? reply(first) : reply({ error: conflict }, 409));
    const session = new AppSession('/api', () => 1000, transport as typeof fetch); await session.check();
    const scope = session.captureOperation(); await expect(scope.request('/eth/link/verify', { method: 'POST' })).rejects.toMatchObject({ status: 409 }); expect(session.snapshot().identity?.activation).toBe(first.session_id);
    conflict = 'session_changed'; await expect(scope.request('/eth/addresses')).rejects.toThrow('App context changed'); expect(session.snapshot().identity).toBeNull();
    await expect(scope.request('/eth/addresses')).rejects.toThrow('App context changed'); expect(transport).toHaveBeenCalledTimes(3);
  });
  it('bounds response bytes, fails malformed409 closed and fences held body chunks', async () => {
    const held = deferred<void>(); let count = 0;
    const transport = vi.fn(async (path: RequestInfo | URL) => {
      if (String(path).endsWith('/auth/status')) return reply(first);
      count++;
      if (count === 1) return new Response(' '.repeat((1 << 20) + 1), { headers: { 'Content-Type': 'application/json' } });
      if (count === 2) return new Response(new ReadableStream({ async start(c) { c.enqueue(new TextEncoder().encode('[')); await held.promise; c.enqueue(new TextEncoder().encode(']')); c.close(); } }), { headers: { 'Content-Type': 'application/json' } });
      return new Response('broken', { status: 409 });
    });
    const session = new AppSession('/api', () => 1000, transport as typeof fetch); await session.check(); const scope = session.captureOperation();
    await expect(scope.request('/eth/addresses')).rejects.toThrow('too large');
    const pending = scope.request('/eth/addresses'); await Promise.resolve(); session.suspend(); held.resolve(); await expect(pending).rejects.toThrow('App context changed');
    const fresh = new AppSession('/api', () => 1000, transport as typeof fetch); await fresh.check(); await expect(fresh.captureOperation().request('/eth/addresses')).rejects.toThrow('App context changed'); expect(fresh.snapshot().identity).toBeNull();
  });
});
