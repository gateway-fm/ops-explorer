import { test, expect } from '@playwright/test';

test.describe('Auth API', () => {
  test('GET /api/auth/status returns unauthenticated without cookie', async ({ request }) => {
    const response = await request.get('/api/auth/status');
    expect(response.ok()).toBe(true);

    const data = await response.json();
    expect(data.authenticated).toBe(false);
  });

  test('POST /api/auth/logout clears auth', async ({ request }) => {
    const response = await request.post('/api/auth/logout');
    expect(response.ok()).toBe(true);

    const data = await response.json();
    expect(data.logged_out).toBe(true);
  });

  test('GET /api/auth/login returns 302 redirect when SSO enabled', async ({ request }) => {
    const response = await request.get('/api/auth/login', {
      maxRedirects: 0,
    });

    // Either 302 (SSO enabled) or 503 (SSO not configured)
    const status = response.status();
    expect([302, 503]).toContain(status);

    if (status === 302) {
      const location = response.headers()['location'];
      expect(location).toContain('/oauth/authorize');
    }
  });

  test('GET /api/auth/callback rejects missing code', async ({ request }) => {
    const response = await request.get('/api/auth/callback?state=test');
    expect(response.status()).toBe(400);
  });

  test('GET /api/auth/callback rejects missing state', async ({ request }) => {
    const response = await request.get('/api/auth/callback?code=test');
    expect(response.status()).toBe(400);
  });

  test('GET /api/auth/callback rejects invalid state', async ({ request }) => {
    const response = await request.get('/api/auth/callback?code=test&state=invalid');
    expect(response.status()).toBe(400);
  });
});
