import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppSession, parseStatus } from './v2Session';

const first = { protocol: 'ops-v2', authenticated: true, did: 'ops:p1:' + 'A'.repeat(43), source_profile: 'azure', session_id: '11111111-1111-1111-1111-111111111111', expires_at: 1300 };
const second = { ...first, did: 'ops:p1:' + 'B'.repeat(43), session_id: '22222222-2222-2222-2222-222222222222' };
const response = (v: unknown, status = 200) => new Response(JSON.stringify(v), { status, headers: { 'Content-Type': 'application/json' } });
const deferred = <T,>() => { let resolve!: (v: T) => void; const promise = new Promise<T>(r => { resolve = r; }); return { promise, resolve }; };
const noticeKey = 'ops-v2-app-activation';
const intentKey = 'ops-v2-app-renewal';
const notice = (activation: string | null, revision = '33333333-3333-3333-3333-333333333333') => JSON.stringify({ v: 1, revision, activation });
const event = (newValue: string | null, key: string | null = noticeKey, area: Storage = localStorage, url = window.location.href) => new StorageEvent('storage', { key, newValue, storageArea: area, url });
const freshStatus = (value = first) => vi.fn(async () => response(value));
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); });
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); document.body.replaceChildren(); });

describe('one original app activation owns private requests', () => {
  it('rejects stale query results after an awaited body and explicit account adoption', async () => {
    const body = deferred<unknown>();
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce({ ok: true, status: 200, json: () => body.promise }).mockResolvedValueOnce(response({ error: 'session_changed' }, 409)).mockResolvedValueOnce(response(second));
    const session = new AppSession('/api', () => 1000, transport as typeof fetch), clear = vi.fn(); session.onClear(clear);
    await session.check(); const result = session.request('/transactions');
    await Promise.resolve(); await session.check();
    expect(session.snapshot().changed).toBe(true); expect(session.snapshot().identity).toBeNull();
    await session.check(true); expect(session.snapshot().identity?.actor).toBe(second.did);
    body.resolve({ private: 'old actor' }); await expect(result).rejects.toThrow('App context changed');
    expect(session.snapshot().identity?.actor).toBe(second.did); expect(clear).toHaveBeenCalled();
    expect((transport.mock.calls[1][1].headers as Headers).get('X-OPS-App-Session')).toBe(first.session_id);
  });
  it('does not let an older logout finally clear a newly accepted context', async () => {
    const pending = deferred<Response>();
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockReturnValueOnce(pending.promise).mockResolvedValueOnce(response({}, 409)).mockResolvedValueOnce(response(second));
    const session = new AppSession('/api', () => 1000, transport as typeof fetch); await session.check();
    const logout = session.logout(); await session.check(); await session.check(true);
    pending.resolve(response({ logged_out: true })); await expect(logout).rejects.toThrow('App context changed');
    expect(session.snapshot().identity?.activation).toBe(second.session_id);
  });
  it('clears every original token activation, including same-actor renewal, and suspends permanently', async () => {
    const next = { ...first, session_id: second.session_id };
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response({}, 409)).mockResolvedValueOnce(response(next));
    const session = new AppSession('/api', () => 1000, transport as typeof fetch), clear = vi.fn(); session.onClear(clear);
    await session.check(); await session.check(); await session.check(true); expect(clear).toHaveBeenCalledTimes(3);
    session.suspend(); expect(session.snapshot().identity).toBeNull(); await session.check(true);
    expect(transport).toHaveBeenCalledTimes(3); await expect(session.request('/blocks')).rejects.toThrow('Sign in required');
  });
  it('rejects wrong protocols, expired values, extra identity on false and malformed session IDs', () => {
    for (const bad of [{ ...first, protocol: 'v1' }, { ...first, expires_at: 1000 }, { ...first, expires_at: 1331 }, { protocol: 'ops-v2', authenticated: false, did: first.did }, { ...first, session_id: 'not-a-session' }]) expect(() => parseStatus(bad, 1000)).toThrow();
    expect(parseStatus(first, 1000)?.actor).toBe(first.did);
  });
});

describe('single opaque cross-document activation notice', () => {
  it('clears displayed A/cache/request epoch when B is accepted without focus, and never adopts B', async () => {
    const held = deferred<unknown>(), node = document.createElement('p'); document.body.append(node);
    const aTransport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce({ ok: true, status: 200, json: () => held.promise });
    const a = new AppSession('/api', () => 1000, aTransport as typeof fetch), b = new AppSession('/api', () => 1000, freshStatus(second) as typeof fetch);
    const cache = new Map([['private-A', 'value']]); a.onClear(() => cache.clear()); a.subscribe(() => { node.textContent = a.snapshot().identity?.actor ?? ''; });
    await a.check(); cache.set('private-A', 'value'); const pending = a.request('/private'); await Promise.resolve();
    await b.check(); const bNotice = localStorage.getItem(noticeKey)!;
    a.notice(event(bNotice)); expect(node.textContent).toBe(''); expect(cache.size).toBe(0); expect(a.snapshot().changed).toBe(true);
    expect((aTransport.mock.calls[1][1] as RequestInit).signal?.aborted).toBe(true);
    held.resolve({ private: 'A' }); await expect(pending).rejects.toThrow('App context changed'); expect(a.snapshot().identity).toBeNull();
    expect(b.snapshot().identity?.actor).toBe(second.did); expect(localStorage.getItem(noticeKey)).toBe(bNotice);
  });
  it.each(['response', 'body'])('fences held status %s before it can overwrite B notice or publish A', async where => {
    const heldResponse = deferred<Response>(), heldBody = deferred<unknown>();
    const transport = vi.fn().mockResolvedValueOnce(response(first));
    if (where === 'response') transport.mockReturnValueOnce(heldResponse.promise);
    else transport.mockResolvedValueOnce({ ok: true, status: 200, json: () => heldBody.promise });
    const a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check(); const pending = a.check(); await Promise.resolve();
    const b = new AppSession('/api', () => 1000, freshStatus(second) as typeof fetch); await b.check(); const current = localStorage.getItem(noticeKey);
    if (where === 'response') heldResponse.resolve(response(first)); else heldBody.resolve(first);
    await pending; expect(a.snapshot().identity).toBeNull(); expect(a.snapshot().changed).toBe(true); expect(localStorage.getItem(noticeKey)).toBe(current);
  });
  it.each(['response', 'body'])('fences a held private %s even before its queued storage event arrives', async where => {
    const heldResponse = deferred<Response>(), heldBody = deferred<unknown>(), transport = vi.fn().mockResolvedValueOnce(response(first));
    if (where === 'response') transport.mockReturnValueOnce(heldResponse.promise); else transport.mockResolvedValueOnce({ ok: true, status: 200, json: () => heldBody.promise });
    const a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check(); const pending = a.request('/private'); await Promise.resolve();
    const b = new AppSession('/api', () => 1000, freshStatus(second) as typeof fetch); await b.check(); const current = localStorage.getItem(noticeKey);
    if (where === 'response') heldResponse.resolve(response({ private: 'A' })); else heldBody.resolve({ private: 'A' });
    await expect(pending).rejects.toThrow('App context changed'); expect(a.snapshot().identity).toBeNull(); expect(localStorage.getItem(noticeKey)).toBe(current);
  });
  it('does not let held successful logout remove B notice or a newer tab intent', async () => {
    const held = deferred<unknown>(), transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce({ ok: true, status: 200, json: () => held.promise });
    const a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check(); const pending = a.logout(); await Promise.resolve();
    const b = new AppSession('/api', () => 1000, freshStatus(second) as typeof fetch); await b.check(); const current = localStorage.getItem(noticeKey);
    sessionStorage.setItem(intentKey, 'newer-tab-intent'); held.resolve({ logged_out: true }); await expect(pending).rejects.toThrow('App context changed');
    expect(localStorage.getItem(noticeKey)).toBe(current); expect(sessionStorage.getItem(intentKey)).toBe('newer-tab-intent'); expect(b.snapshot().identity?.actor).toBe(second.did);
  });
  it('fences held login JSON before an old renewal intent or navigation can publish', async () => {
    const held = deferred<unknown>(), transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce({ ok: true, status: 200, json: () => held.promise });
    const a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check(); const pending = a.login('/', true); await Promise.resolve();
    const b = new AppSession('/api', () => 1000, freshStatus(second) as typeof fetch); await b.check(); const current = localStorage.getItem(noticeKey);
    sessionStorage.setItem(intentKey, 'newer-tab-intent'); held.resolve({ authorizationUrl: 'https://ops.test/api/v2/sso/authorize' }); await pending;
    expect(localStorage.getItem(noticeKey)).toBe(current); expect(sessionStorage.getItem(intentKey)).toBe('newer-tab-intent'); expect(a.snapshot().identity).toBeNull();
  });
  it('checks synchronous clear callbacks before a notice write or identity publication', async () => {
    const current = notice(second.session_id); let once = false;
    const a = new AppSession('/api', () => 1000, freshStatus() as typeof fetch);
    a.onClear(() => { if (!once) { once = true; localStorage.setItem(noticeKey, current); } });
    await a.check(); expect(a.snapshot().identity).toBeNull(); expect(a.snapshot().changed).toBe(true); expect(localStorage.getItem(noticeKey)).toBe(current);
  });
  it('rereads current storage but conservatively invalidates on queued foreign events', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response(second)), a = new AppSession('/api', () => 1000, transport as typeof fetch);
    const clear = vi.fn(); a.onClear(clear); await a.check(); const old = localStorage.getItem(noticeKey);
    const b = new AppSession('/api', () => 1000, freshStatus(second) as typeof fetch); await b.check(); const current = localStorage.getItem(noticeKey);
    a.notice(event(old)); expect(a.snapshot().identity).toBeNull(); await a.check(true); expect(a.snapshot().identity?.actor).toBe(second.did);
    const count = clear.mock.calls.length; a.notice(event(old)); expect(clear).toHaveBeenCalledTimes(count + 1); expect(a.snapshot().identity).toBeNull(); expect(a.snapshot().changed).toBe(true); expect(localStorage.getItem(noticeKey)).toBe(current);
    expect(transport).toHaveBeenCalledTimes(2);
  });
  it('does not lose a queued competing event when its own final write overwrites that change', async () => {
    const original = localStorage, rawB = notice(second.session_id); let interleaved = false;
    const boundary = { getItem: original.getItem.bind(original), setItem: (key: string, value: string) => { if (key === noticeKey && !interleaved) { original.setItem(key, rawB); interleaved = true; } original.setItem(key, value); } };
    vi.stubGlobal('localStorage', boundary);
    const transport = freshStatus(), a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check();
    expect(interleaved).toBe(true); expect(a.snapshot().identity?.actor).toBe(first.did); const own = original.getItem(noticeKey); expect(JSON.parse(own!).activation).toBe(first.session_id);
    // Deliver the complete foreign native-event shape after both atomic writes.
    // The current key is A, but the queued B change must still invalidate A.
    a.notice({ key: noticeKey, storageArea: boundary, url: window.location.href, oldValue: null, newValue: rawB } as unknown as StorageEvent);
    expect(a.snapshot().identity).toBeNull(); expect(a.snapshot().changed).toBe(true); expect(original.getItem(noticeKey)).toBe(own); expect(transport).toHaveBeenCalledTimes(1);
  });
  it('emits no loop for same-activation status and only a known logout emits null', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response({ logged_out: true })), a = new AppSession('/api', () => 1000, transport as typeof fetch);
    await a.check(); const original = localStorage.getItem(noticeKey); await a.check(); expect(localStorage.getItem(noticeKey)).toBe(original);
    await a.logout(); const raw = localStorage.getItem(noticeKey)!; const marker = JSON.parse(raw); expect(marker.activation).toBeNull(); expect(marker.revision).not.toBe(JSON.parse(original!).revision);
    expect(Object.keys(marker)).toEqual(['v', 'revision', 'activation']); expect(new TextEncoder().encode(raw).length).toBeLessThanOrEqual(256); expect(raw).not.toContain('ops:p1:'); expect(a.snapshot().identity).toBeNull();
  });
  it.each(['unknown', 'malformed'])('does not broadcast a %s logout result', async kind => {
    const transport = vi.fn().mockResolvedValueOnce(response(first)); if (kind === 'unknown') transport.mockRejectedValueOnce(Error('network result unknown')); else transport.mockResolvedValueOnce(response({ logged_out: false }));
    const a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check(); const original = localStorage.getItem(noticeKey); await expect(a.logout()).rejects.toThrow();
    expect(localStorage.getItem(noticeKey)).toBe(original); expect(a.snapshot().identity).toBeNull();
  });
  it.each(['expiry', 'pagehide', 'dispose'])('does not broadcast local %s', async kind => {
    let now = 1000; const a = new AppSession('/api', () => now, freshStatus() as typeof fetch); await a.check(); const original = localStorage.getItem(noticeKey);
    if (kind === 'expiry') { now = 1300; a.expire(); } else if (kind === 'pagehide') a.suspend(); else a.discard();
    expect(a.snapshot().identity).toBeNull(); expect(localStorage.getItem(noticeKey)).toBe(original);
  });
  it('ignores foreign origin/key/storage area and only invalidates on actual change/removal', async () => {
    const a = new AppSession('/api', () => 1000, freshStatus() as typeof fetch); await a.check(); const newer = notice(second.session_id); localStorage.setItem(noticeKey, newer);
    a.notice(event(newer, noticeKey, sessionStorage)); a.notice(event(newer, 'another-key')); a.notice(event(newer, noticeKey, localStorage, 'https://other.test/'));
    expect(a.snapshot().identity?.actor).toBe(first.did); a.notice(event(newer)); expect(a.snapshot().identity).toBeNull();
    localStorage.removeItem(noticeKey); a.notice(event(null)); expect(a.snapshot().changed).toBe(true);
  });
  it('does not authenticate from a forged opaque notice', async () => {
    localStorage.setItem(noticeKey, notice(second.session_id)); const transport = vi.fn(async () => response({ protocol: 'ops-v2', authenticated: false }));
    const a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check(); expect(a.snapshot().identity).toBeNull(); await expect(a.request('/private')).rejects.toThrow('Sign in required'); expect(transport).toHaveBeenCalledTimes(1);
  });
  it.each(['duplicate', 'extra', 'oversized', 'zero', 'bad-json'])('fails closed for %s marker', async kind => {
    let raw = notice(first.session_id); if (kind === 'duplicate') raw = raw.replace('"v":1', '"v":1,"v":1'); if (kind === 'extra') raw = raw.replace('"v":1', '"v":1,"actor":"forbidden"'); if (kind === 'oversized') raw = 'x'.repeat(257); if (kind === 'zero') raw = notice('00000000-0000-0000-0000-000000000000'); if (kind === 'bad-json') raw = '{';
    localStorage.setItem(noticeKey, raw); const transport = freshStatus(), a = new AppSession('/api', () => 1000, transport as typeof fetch); await a.check(); expect(transport).not.toHaveBeenCalled(); expect(a.snapshot().identity).toBeNull(); expect(a.snapshot().error).toContain('Shared app state'); expect(localStorage.getItem(noticeKey)).toBe(raw);
  });
  it('fails locally for unavailable reads and quota-failed writes, then requires verified recovery', async () => {
    const transport = freshStatus(), a = new AppSession('/api', () => 1000, transport as typeof fetch), original = localStorage;
    // Happy DOM binds methods onto its Storage instance; prototype spies do
    // not exercise the actual caller. Replace only this browser API boundary.
    vi.stubGlobal('localStorage', { getItem: () => { throw Error('blocked'); } });
    await a.check(); expect(a.snapshot().identity).toBeNull(); expect(transport).not.toHaveBeenCalled();
    vi.stubGlobal('localStorage', { getItem: original.getItem.bind(original), setItem: () => { throw Error('quota'); } }); await a.check(true); expect(a.snapshot().identity).toBeNull(); expect(a.snapshot().error).toContain('Shared app state'); vi.stubGlobal('localStorage', original);
    await a.check(true); expect(a.snapshot().identity?.actor).toBe(first.did);
  });
});
