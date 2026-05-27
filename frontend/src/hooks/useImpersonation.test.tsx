import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import {
  ImpersonationProvider,
  ImpersonationTokenMirror,
  useImpersonation,
  getImpersonationTokenHeader,
} from './useImpersonation';

// Note: we hit real fetch via global, mocked with vi.spyOn.

function wrapper({ children }: { children: ReactNode }) {
  return createElement(ImpersonationProvider, null, [
    createElement(ImpersonationTokenMirror, { key: 'mirror' }),
    children,
  ]);
}

function mockFetch(responses: Array<{ status: number; body?: unknown }>) {
  let i = 0;
  return vi.fn(async () => {
    const r = responses[i++] ?? responses[responses.length - 1];
    return new Response(JSON.stringify(r.body ?? {}), { status: r.status });
  });
}

beforeEach(() => {
  // Reset URL between tests so ?as= doesn't bleed across cases.
  window.history.replaceState({}, '', '/');
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('useImpersonation', () => {
  it('starts a session and exposes target DID + token', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(
      mockFetch([
        {
          status: 200,
          body: {
            token: 'TOK-1',
            expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
            target_did: 'did:p:target',
          },
        },
      ]) as unknown as typeof fetch
    );

    const { result } = renderHook(() => useImpersonation(), { wrapper });
    expect(result.current.isActive).toBe(false);

    await act(async () => {
      await result.current.start('did:p:target');
    });

    expect(result.current.isActive).toBe(true);
    expect(result.current.token).toBe('TOK-1');
    expect(result.current.targetDID).toBe('did:p:target');
    // URL must carry ?as=TOK-1 so refreshes preserve view-as state.
    expect(new URLSearchParams(window.location.search).get('as')).toBe('TOK-1');
    // The module-level mirror used by fetch wrapper sees the same token.
    await waitFor(() => expect(getImpersonationTokenHeader()).toBe('TOK-1'));
  });

  it('rejects chained start while a session is active', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(
      mockFetch([
        {
          status: 200,
          body: {
            token: 'TOK-1',
            expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
            target_did: 'did:p:a',
          },
        },
      ]) as unknown as typeof fetch
    );
    const { result } = renderHook(() => useImpersonation(), { wrapper });
    await act(async () => {
      await result.current.start('did:p:a');
    });

    await act(async () => {
      await expect(result.current.start('did:p:b')).rejects.toThrow();
    });
    expect(result.current.targetDID).toBe('did:p:a');
  });

  it('stop() clears session + URL and calls DELETE', async () => {
    const calls: Array<{ url: string; method?: string }> = [];
    vi.spyOn(globalThis, 'fetch').mockImplementation(((url: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ url: String(url), method: init?.method });
      if (init?.method === 'DELETE') {
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({
            token: 'TOK-X',
            expires_at: new Date(Date.now() + 1000 * 60 * 60).toISOString(),
            target_did: 'did:p:x',
          }),
          { status: 200 }
        )
      );
    }) as unknown as typeof fetch);

    const { result } = renderHook(() => useImpersonation(), { wrapper });
    await act(async () => {
      await result.current.start('did:p:x');
    });
    expect(result.current.isActive).toBe(true);

    await act(async () => {
      await result.current.stop();
    });

    expect(result.current.isActive).toBe(false);
    expect(new URLSearchParams(window.location.search).get('as')).toBeNull();
    expect(calls.some((c) => c.url.includes('/api/impersonation/TOK-X') && c.method === 'DELETE')).toBe(true);
    await waitFor(() => expect(getImpersonationTokenHeader()).toBeNull());
  });

  it('surfaces friendly error on 404 from the BFF', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'Target user not found' }), { status: 404 })
    );

    const { result } = renderHook(() => useImpersonation(), { wrapper });

    await act(async () => {
      await expect(result.current.start('did:p:nonexistent')).rejects.toThrow(/not found/);
    });
    expect(result.current.isActive).toBe(false);
    expect(result.current.error).toMatch(/not found/i);
  });
});
