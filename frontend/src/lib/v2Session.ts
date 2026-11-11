import { getConfig } from './runtimeConfig';

export const isV2 = () => import.meta.env.VITE_AUTH_PROTOCOL === 'ops-v2' || getConfig('VITE_AUTH_PROTOCOL') === 'ops-v2';
export interface AppIdentity { actor: string; profile: string; activation: string; expires: number }
export interface SessionState { identity: AppIdentity | null; loading: boolean; changed: boolean; error: string | null }
interface Status { protocol: 'ops-v2'; authenticated: boolean; did?: string; source_profile?: string; session_id?: string; expires_at?: number }
interface Intent { v: 1; actor: string; profile: string; prior: string; returnPath: string; expiresAt: number }
const intentKey = 'ops-v2-app-renewal';
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const actor = /^ops:p1:[A-Za-z0-9_-]{43}$/;
const exactKeys = (value: object, allowed: string[]) => Object.keys(value).every(key => allowed.includes(key));
const noticeKey = 'ops-v2-app-activation';
interface ActivationNotice { v: 1; revision: string; activation: string | null }
interface NoticeSnapshot { raw: string | null; value: ActivationNotice | null }
interface Owner { epoch: number; operation: number }
const validUUID = (value: unknown): value is string => typeof value === 'string' && uuid.test(value) && value !== '00000000-0000-0000-0000-000000000000';
function readNotice(): NoticeSnapshot {
  const raw = localStorage.getItem(noticeKey);
  if (raw === null) return { raw, value: null };
  if (raw.length > 256) throw Error('Invalid activation notice');
  const value: unknown = JSON.parse(raw);
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw Error('Invalid activation notice');
  const v = value as ActivationNotice;
  if (!exactKeys(v, ['v', 'revision', 'activation']) || Object.keys(v).length !== 3 || v.v !== 1 || !validUUID(v.revision) || v.activation !== null && !validUUID(v.activation) || JSON.stringify({ v: 1, revision: v.revision, activation: v.activation }) !== raw) throw Error('Invalid activation notice');
  return { raw, value: v };
}
export function parseStatus(value: unknown, now: number): AppIdentity | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw Error('Invalid app status');
  const v = value as Status;
  if (v.protocol !== 'ops-v2' || typeof v.authenticated !== 'boolean') throw Error('Wrong app protocol');
  if (!v.authenticated) {
    if (!exactKeys(v, ['protocol', 'authenticated'])) throw Error('Invalid app status');
    return null;
  }
  if (!exactKeys(v, ['protocol', 'authenticated', 'did', 'source_profile', 'session_id', 'expires_at']) || typeof v.did !== 'string' || !actor.test(v.did) || typeof v.source_profile !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(v.source_profile) || typeof v.session_id !== 'string' || !uuid.test(v.session_id) || v.session_id === '00000000-0000-0000-0000-000000000000' || !Number.isSafeInteger(v.expires_at) || v.expires_at! <= now || v.expires_at! > now + 330) throw Error('Invalid app status');
  return { actor: v.did, profile: v.source_profile, activation: v.session_id, expires: v.expires_at! };
}
export function localReturn(raw: string): boolean {
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.includes('\\') || new TextEncoder().encode(raw).length > 1024 || /[\u0000-\u001f\u007f]/.test(raw)) return false;
  try { const u = new URL(raw, window.location.origin); return u.origin === window.location.origin && !u.hash && u.pathname === raw.split('?')[0]; } catch { return false; }
}

// The single tab context owns every private request. These are opaque app
// activation identifiers, never access tokens, refresh tokens or OAuth state.
export class AppSession {
  private state: SessionState = { identity: null, loading: true, changed: false, error: null };
  private epoch = 0;
  private operation = 0;
  private live = true;
  private listeners = new Set<() => void>();
  private requests = new Set<AbortController>();
  private clearPrivate = () => {};
  private base: string;
  private now: () => number;
  private transport: typeof fetch;
  private renewalAttempted: string | null = null;
  private observedNotice: string | null | undefined;
  constructor(base = getConfig('VITE_API_URL', '/api'), now = () => Math.floor(Date.now() / 1000), transport: typeof fetch = globalThis.fetch.bind(globalThis)) { this.base = base; this.now = now; this.transport = transport; }
  snapshot = () => this.state;
  subscribe = (f: () => void) => { this.listeners.add(f); return () => { this.listeners.delete(f); }; };
  onClear(f: () => void) { this.clearPrivate = f; }
  private publish(next: SessionState) { this.state = next; this.listeners.forEach(f => f()); }
  private clear(changed = false, error: string | null = null) {
    ++this.epoch; ++this.operation;
    // Capture before abort/subscription/query callbacks can start another owner.
    const owner = { epoch: this.epoch, operation: this.operation };
    for (const request of this.requests) request.abort(); this.requests.clear();
    // Unmount first, then clear data/cursors before a new identity is published.
    this.publish({ identity: null, loading: false, changed, error });
    this.clearPrivate();
    return owner;
  }
  private current(epoch: number, activation?: string) { return this.live && this.epoch === epoch && (activation === undefined || this.state.identity?.activation === activation); }
  private noticeUnavailable() {
    this.observedNotice = undefined;
    this.clear(true, 'Shared app state is unavailable. Allow local storage, then verify the session again.');
  }
  private captureNotice(): NoticeSnapshot | null {
    try {
      const notice = readNotice();
      if (this.observedNotice !== undefined && this.observedNotice !== notice.raw) {
        this.observedNotice = notice.raw; this.clear(true); return null;
      }
      this.observedNotice = notice.raw; return notice;
    } catch { this.noticeUnavailable(); return null; }
  }
  private fence(owner: Owner, notice: NoticeSnapshot, activation?: string, operation = true): boolean {
    if (!this.current(owner.epoch, activation) || operation && owner.operation !== this.operation) return false;
    try {
      const current = readNotice();
      if (current.raw !== notice.raw || this.observedNotice !== notice.raw) {
        this.observedNotice = current.raw; this.clear(true); return false;
      }
      return true;
    } catch { this.noticeUnavailable(); return false; }
  }
  private writeNotice(owner: Owner, captured: NoticeSnapshot, activation: string | null): NoticeSnapshot | null {
    if (!this.fence(owner, captured)) return null;
    if (captured.value?.activation === activation) return captured;
    try {
      const revision = crypto.randomUUID();
      if (!validUUID(revision)) throw Error('Invalid activation revision');
      const value: ActivationNotice = { v: 1, revision, activation };
      const raw = JSON.stringify(value);
      if (raw.length > 256 || !this.fence(owner, captured)) return null;
      // Compare immediately before writing, but do not assume an atomic
      // cross-document read/write. Native competing events still invalidate.
      localStorage.setItem(noticeKey, raw);
      this.observedNotice = raw; // Writers receive no native storage event.
      const next = { raw, value };
      return this.fence(owner, next) ? next : null;
    } catch { this.noticeUnavailable(); return null; }
  }
  notice = (event: StorageEvent) => {
    if (!this.live || event.key !== noticeKey && event.key !== null) return;
    try {
      if (event.storageArea !== localStorage || new URL(event.url).origin !== window.location.origin) return;
      // The event is a foreign invalidation hint, even when our write has
      // overwritten its change. Read current storage for the next fence;
      // neither the event payload nor that current value authenticates anyone.
      const notice = readNotice();
      this.observedNotice = notice.raw; this.clear(true);
    } catch { this.noticeUnavailable(); }
  };
  private intent(): Intent | null {
    try { const raw = sessionStorage.getItem(intentKey); if (!raw || raw.length > 4096) return null; const v = JSON.parse(raw) as Intent; if (!v || typeof v !== 'object' || !exactKeys(v, ['v', 'actor', 'profile', 'prior', 'returnPath', 'expiresAt']) || v.v !== 1 || !actor.test(v.actor) || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(v.profile) || !uuid.test(v.prior) || !localReturn(v.returnPath) || !Number.isSafeInteger(v.expiresAt) || v.expiresAt <= this.now() || v.expiresAt > this.now() + 300) return null; return v; } catch { return null; }
  }
  async check(explicit = false) {
    if (!this.live) return;
    let notice = this.captureNotice(); if (!notice) return;
    let owner = { epoch: this.epoch, operation: ++this.operation };
    const selected = this.state.identity;
    try {
      const headers: Record<string, string> = {};
      if (selected && !explicit) headers['X-OPS-App-Session'] = selected.activation;
      if (!this.fence(owner, notice)) return;
      const res = await this.transport(this.base + '/auth/status', { credentials: 'include', cache: 'no-store', headers, redirect: 'error' });
      if (!this.fence(owner, notice)) return;
      if (res.status === 409) { this.clear(true); return; }
      if (!res.ok) throw Error('Cannot verify app session');
      const value: unknown = await res.json();
      if (!this.fence(owner, notice)) return;
      const identity = parseStatus(value, this.now());
      if (!identity) { this.clear(false); return; }
      const intent = this.intent();
      const ownedRenewal = !!intent && identity.actor === intent.actor && identity.profile === intent.profile;
      if (!explicit && selected && identity.activation !== selected.activation || !explicit && this.state.changed && !ownedRenewal) { this.clear(true); return; }
      // A fresh document bootstraps its own verified cookie context. A running
      // document may only switch through explicit acceptance or owned renewal.
      if (!selected || identity.activation !== selected.activation) {
        if (intent && !ownedRenewal) { this.clear(true, 'The renewed account does not match.'); return; }
        owner = this.clear(false);
        if (!this.fence(owner, notice)) return;
      }
      if (!this.fence(owner, notice)) return;
      sessionStorage.removeItem(intentKey);
      if (!this.fence(owner, notice)) return;
      const accepted = this.writeNotice(owner, notice, identity.activation); if (!accepted) return;
      notice = accepted;
      if (!this.fence(owner, notice)) return;
      this.publish({ identity, loading: false, changed: false, error: null });
    } catch { if (this.fence(owner, notice)) this.clear(false, 'Cannot verify app session. Try again.'); }
  }
  async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const identity = this.state.identity, epoch = this.epoch;
    if (!this.live || !identity || identity.expires <= this.now()) { this.clear(false); throw Error('Sign in required'); }
    const notice = this.captureNotice();
    if (!notice || !this.current(epoch, identity.activation)) throw Error('App context changed');
    const owner = { epoch, operation: this.operation };
    const abort = new AbortController(); this.requests.add(abort);
    const onAbort = () => abort.abort(); options.signal?.addEventListener('abort', onAbort, { once: true }); if (options.signal?.aborted) abort.abort();
    try {
      const headers = new Headers(options.headers); headers.set('X-OPS-App-Session', identity.activation); headers.delete('Authorization'); headers.delete('X-Impersonate-Token');
      if (!this.fence(owner, notice, identity.activation, false)) throw Error('App context changed');
      const res = await this.transport(this.base + endpoint, { ...options, credentials: 'include', cache: 'no-store', redirect: 'error', headers, signal: abort.signal });
      if (!this.fence(owner, notice, identity.activation, false) || identity.expires <= this.now()) throw Error('App context changed');
      if (res.status === 409 || res.status === 401) { this.clear(res.status === 409); throw Error('App context changed'); }
      if (!res.ok) throw Error(`API error: ${res.status}`);
      const value: unknown = await res.json();
      if (!this.fence(owner, notice, identity.activation, false) || identity.expires <= this.now()) throw Error('App context changed');
      return value as T;
    } catch (error) { this.fence(owner, notice, identity.activation, false); throw error;
    } finally { options.signal?.removeEventListener('abort', onAbort); this.requests.delete(abort); }
  }
  async logout() {
    const identity = this.state.identity;
    if (!identity || !this.live) return;
    let notice = this.captureNotice(); if (!notice) return;
    const owner = { epoch: this.epoch, operation: ++this.operation };
    try {
      const value: unknown = await this.request('/auth/logout', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
      if (!this.fence(owner, notice, identity.activation)) return;
      if (!value || typeof value !== 'object' || !exactKeys(value, ['logged_out']) || Object.keys(value).length !== 1 || (value as { logged_out?: unknown }).logged_out !== true) throw Error('Unverified logout');
      const accepted = this.writeNotice(owner, notice, null); if (!accepted) return;
      notice = accepted;
    } finally {
      if (this.fence(owner, notice, identity.activation)) {
        sessionStorage.removeItem(intentKey);
        if (this.fence(owner, notice, identity.activation)) this.clear();
      }
    }
  }
  async login(returnPath = window.location.pathname + window.location.search, renew = false) {
    if (!this.live || !localReturn(returnPath)) throw Error('Invalid return path');
    const notice = this.captureNotice(); if (!notice) return;
    const selected = this.state.identity, owner = { epoch: this.epoch, operation: ++this.operation };
    if (renew && !selected) throw Error('No owned app context');
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (renew && selected) headers['X-OPS-App-Session'] = selected.activation;
    const body = renew && selected ? { mode: 'renew', returnPath, expectedActorId: selected.actor, expectedProfile: selected.profile } : { mode: 'login', returnPath };
    try {
      if (!this.fence(owner, notice)) return;
      const res = await this.transport(this.base + '/auth/login', { method: 'POST', credentials: 'include', cache: 'no-store', redirect: 'error', headers, body: JSON.stringify(body) });
      if (!this.fence(owner, notice)) return;
      if (res.status === 409) { this.clear(true); return; }
      if (!res.ok) throw Error('Cannot start sign in');
      const value: unknown = await res.json();
      if (!this.fence(owner, notice)) return;
      if (!value || typeof value !== 'object' || !exactKeys(value, ['authorizationUrl']) || typeof (value as { authorizationUrl?: unknown }).authorizationUrl !== 'string') throw Error('Invalid sign-in destination');
      const destination = new URL((value as { authorizationUrl: string }).authorizationUrl);
      if (destination.protocol !== 'https:' || destination.username || destination.password || destination.hash || destination.pathname !== '/api/v2/sso/authorize') throw Error('Invalid sign-in destination');
      if (!this.fence(owner, notice)) return;
      if (renew && selected) sessionStorage.setItem(intentKey, JSON.stringify({ v: 1, actor: selected.actor, profile: selected.profile, prior: selected.activation, returnPath, expiresAt: this.now() + 300 } satisfies Intent));
      else sessionStorage.removeItem(intentKey);
      if (!this.fence(owner, notice)) return;
      window.location.assign(destination.href);
    } finally { this.fence(owner, notice); }
  }
  expire() { if (this.state.identity && this.state.identity.expires <= this.now()) this.clear(); }
  tick() {
    const identity = this.state.identity;
    if (this.live && identity && identity.expires > this.now() && identity.expires <= this.now() + 5 && this.renewalAttempted !== identity.activation && document.visibilityState === 'visible') {
      // A single original-activation attempt. An uncertain POST is never
      // automatically retried by a timer or another status observer.
      this.renewalAttempted = identity.activation;
      void this.login(undefined, true).catch(() => {});
    }
    this.expire();
  }
  suspend() { if (!this.live) return; this.clear(); this.live = false; }
  discard() { if (this.live) this.clear(); }
}
export const appSession = new AppSession();
