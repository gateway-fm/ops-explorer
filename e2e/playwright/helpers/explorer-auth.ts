import { Page, BrowserContext } from '@playwright/test';
import { getAuthTokens } from './auth';

const EXPLORER_API = process.env.EXPLORER_URL || 'http://localhost:5173';

/**
 * Check if user is authenticated in block-explorer by calling /api/auth/status.
 */
export async function isAuthenticated(context: BrowserContext): Promise<boolean> {
  const response = await context.request.get(`${EXPLORER_API}/api/auth/status`);
  if (!response.ok()) return false;
  const data = await response.json();
  return data.authenticated === true;
}

/**
 * Log in to block-explorer by directly setting the auth cookie.
 * This bypasses the OAuth redirect flow for tests that just need authenticated state.
 *
 * @param context - Playwright browser context
 * @param userDID - Optional DID (defaults to generated one)
 */
export async function loginViaCookie(context: BrowserContext, userDID?: string): Promise<void> {
  const did = userDID || `did:privado:e2e_${Date.now()}`;
  const tokens = await getAuthTokens(did);

  // Parse the explorer URL to get the domain for the cookie
  const url = new URL(EXPLORER_API);

  await context.addCookies([{
    name: 'explorer_auth',
    value: tokens.accessToken,
    domain: url.hostname,
    path: '/',
    httpOnly: true,
    sameSite: 'Lax',
  }]);
}

/**
 * Log out of block-explorer by clearing the auth cookie.
 */
export async function logout(context: BrowserContext): Promise<void> {
  await context.clearCookies({ name: 'explorer_auth' });
}
