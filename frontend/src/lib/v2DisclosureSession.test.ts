import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppSession, DisclosureHTTPError } from './v2Session';
const id = '11111111-1111-1111-1111-111111111111';
const identity = { protocol: 'ops-v2', authenticated: true, did: 'ops:p1:' + 'A'.repeat(43), source_profile: 'azure', session_id: id, expires_at: 1300 };
const response = (v: unknown, status = 200) => new Response(JSON.stringify(v), { status, headers: { 'Content-Type': 'application/json' } });
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); });
describe('bounded protected case requests', () => {
  it('preserves identity only for the confirmed disclosure conflict', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(identity)).mockResolvedValueOnce(response({ error: 'disclosure context changed' }, 409));
    const session = new AppSession('/api', () => 1000, transport); await session.check();
    await expect(session.disclosure('/disclosure/cases/' + id + '/read', { method: 'POST', body: '{}' })).rejects.toMatchObject({ status: 409, conflict: true });
    expect(session.snapshot().identity?.activation).toBe(id);
  });
  it.each(['session_changed', 'another conflict'])('clears identity for unconfirmed app conflict %s', async message => {
    const transport = vi.fn().mockResolvedValueOnce(response(identity)).mockResolvedValueOnce(response({ error: message }, 409));
    const session = new AppSession('/api', () => 1000, transport); await session.check();
    await expect(session.disclosure('/disclosure/cases')).rejects.toThrow('App context changed'); expect(session.snapshot().identity).toBeNull();
  });
  it('permits the exact case and opaque alias queries without exposing bearer credentials', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(identity)).mockImplementation(async () => response({ items: [] }));
    const session = new AppSession('/api', () => 1000, transport); await session.check();
    await session.disclosure('/disclosure/cases?role=data-org-admin&organizationId=' + id + '&limit=100');
    await session.disclosure('/grant/' + id + '/da1.render.' + 'A'.repeat(22) + '/transactions?limit=100&cursor=opaque');
    const headers = transport.mock.calls.at(-1)?.[1]?.headers as Headers; expect(headers.get('Authorization')).toBeNull(); expect(headers.get('X-OPS-App-Session')).toBe(id);
  });
  it('rejects unsupported paths, operations, duplicate queries and oversized writes before transport', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(identity)); const session = new AppSession('/api', () => 1000, transport); await session.check();
    for (const endpoint of ['https://elsewhere.test/disclosure/cases', '/disclosure/cases?limit=1&limit=2', '/disclosure/cases?role=data-org-admin', '/viewable-addresses?limit=101', '/grant/' + id + '/activity?cursor=' + 'A'.repeat(2049), '/disclosure/cases/' + id + '/approve', '/disclosure/cases/' + id + '?actor=other']) await expect(session.disclosure(endpoint)).rejects.toThrow('Invalid disclosure');
    await expect(session.disclosure('/disclosure/cases', { method: 'POST', body: 'A'.repeat(131073) })).rejects.toThrow('Invalid disclosure');
    await expect(session.disclosure('/disclosure/cases', { method: 'DELETE' })).rejects.toThrow('Invalid disclosure'); expect(transport).toHaveBeenCalledTimes(1);
  });
  it('reports unavailable scope without dropping the verified app context', async () => {
    const transport = vi.fn().mockResolvedValueOnce(response(identity)).mockResolvedValueOnce(response({ error: 'protected data scope unavailable' }, 422));
    const session = new AppSession('/api', () => 1000, transport); await session.check(); await expect(session.disclosure('/disclosure/cases/' + id + '/read', { method: 'POST', body: '{}' })).rejects.toBeInstanceOf(DisclosureHTTPError); expect(session.snapshot().identity?.activation).toBe(id);
  });
});
