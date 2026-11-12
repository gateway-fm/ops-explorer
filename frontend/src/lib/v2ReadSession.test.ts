import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppSession, ReadHTTPError } from './v2Session';
const first = { protocol: 'ops-v2', authenticated: true, did: 'ops:p1:' + 'A'.repeat(43), source_profile: 'azure', session_id: '11111111-1111-1111-1111-111111111111', expires_at: 1300 };
const second = { ...first, did: 'ops:p1:' + 'B'.repeat(43), session_id: '22222222-2222-2222-2222-222222222222' };
const response = (v: unknown, status = 200) => new Response(JSON.stringify(v), { status, headers: { 'Content-Type': 'application/json' } });
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); });
afterEach(() => { vi.restoreAllMocks(); });
describe('ordinary read session ownership', () => {
  it('resets a stale cursor without clearing the verified app identity', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response({ error: { code: 'cursor_stale' } }, 409));
    const session = new AppSession('/api', () => 1000, transport); await session.check();
    await expect(session.read('/v2/explorer/transactions?cursor=old')).rejects.toMatchObject({ status: 409, code: 'cursor_stale' });
    expect(session.snapshot().identity?.activation).toBe(first.session_id);
  });
  it('still clears identity on an actual changed app capture', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response({ error: 'session_changed' }, 409));
    const session = new AppSession('/api', () => 1000, transport); await session.check();
    await expect(session.read('/v2/explorer/transactions')).rejects.toThrow('App context changed'); expect(session.snapshot().identity).toBeNull();
  });
  it('rejects a held read-body chunk after another account activates before its event is delivered', async () => {
    let body!: ReadableStreamDefaultController<Uint8Array>;
    const stream = new ReadableStream<Uint8Array>({ start(controller) { body = controller; } });
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce(new Response(stream, { headers: { 'Content-Type': 'application/json' } }));
    const a = new AppSession('/api', () => 1000, transport); await a.check(); const pending = a.read('/v2/explorer/transactions'); await Promise.resolve();
    const b = new AppSession('/api', () => 1000, vi.fn(async () => response(second))); await b.check();
    body.enqueue(new TextEncoder().encode('{"private":"A"}')); body.close();
    await expect(pending).rejects.toThrow('App context changed'); expect(a.snapshot().identity).toBeNull(); expect(b.snapshot().identity?.activation).toBe(second.session_id);
  });
  it('reports permission denial without dropping the app context', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response({ error: { code: 'denied' } }, 403));
    const session = new AppSession('/api', () => 1000, transport); await session.check(); await expect(session.read('/v2/explorer/transactions')).rejects.toBeInstanceOf(ReadHTTPError); expect(session.snapshot().identity?.activation).toBe(first.session_id);
  });
});
