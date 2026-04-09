const PROXY_URL = process.env.PROXY_URL || 'http://localhost:8080';

/** Default test DID used across e2e tests. */
export const MAIN_DID = 'did:privado:e2e_main_user';

/** Default test address (Anvil account #2) used across e2e tests. */
export const MAIN_ADDRESS = '0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC';

interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

/**
 * Get auth tokens from privacy-proxy using mock login.
 * Requires privacy-proxy running with ALLOW_MOCK_LOGIN=true.
 */
export async function getAuthTokens(userDID: string): Promise<AuthTokens> {
  // Step 1: Create auth request to get session ID
  const authRequestResponse = await fetch(`${PROXY_URL}/auth/request`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  });

  if (!authRequestResponse.ok) {
    const body = await authRequestResponse.text();
    throw new Error(`Failed to create auth request: ${authRequestResponse.status} - ${body}`);
  }

  const { session_id } = await authRequestResponse.json() as { session_id: string };

  // Step 2: Verify with mock JWZ token (dev mode only)
  const verifyResponse = await fetch(`${PROXY_URL}/auth/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      session_id,
      jwz_token: `mock.${userDID}`,
    }),
  });

  if (!verifyResponse.ok) {
    const body = await verifyResponse.text();
    throw new Error(`Failed to verify auth: ${verifyResponse.status} - ${body}`);
  }

  const data = await verifyResponse.json() as { access_token: string; refresh_token: string; expires_in: number };
  return {
    accessToken: data.access_token,
    refreshToken: data.refresh_token,
    expiresIn: data.expires_in,
  };
}
