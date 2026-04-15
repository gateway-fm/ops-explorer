import { randomUUID } from 'crypto';
import { getAuthTokens } from './auth';

const PROXY_URL = process.env.PROXY_URL || 'http://localhost:8080';
const ADMIN_TOKEN = process.env.ADMIN_API_TOKEN || '';

// === Types ===

export type Claim = 'read' | 'write' | 'admin' | 'upgrade' | 'deploy';

// Proxy doesn't support wildcard '*' in allowed_methods — expand based on claims.
const READ_METHODS = [
  'eth_blockNumber', 'eth_chainId', 'eth_gasPrice', 'eth_call',
  'eth_getBalance', 'eth_getCode', 'eth_getLogs', 'eth_getStorageAt',
  'eth_getTransactionByHash', 'eth_getTransactionCount',
  'eth_getTransactionReceipt', 'eth_getBlockByNumber', 'eth_getBlockByHash',
  'eth_getBlockReceipts', 'net_version',
];
const WRITE_METHODS = ['eth_sendRawTransaction', 'eth_estimateGas', 'eth_sendTransaction'];

function expandMethodsForClaims(claims: Claim[]): string[] {
  const methods = [...READ_METHODS];
  if (claims.includes('write') || claims.includes('admin') || claims.includes('deploy') || claims.includes('upgrade')) {
    methods.push(...WRITE_METHODS);
  }
  return methods;
}

export interface Organization {
  id: string;
  slug: string;
  name: string;
  settings: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface Group {
  id: string;
  org_id: string;
  parent_id: string | null;
  slug: string;
  name: string;
  description: string;
  depth: number;
  path: string;
  created_at: string;
  updated_at: string;
}

export interface GroupAccess {
  id: string;
  group_id: string;
  allowed_methods: string[];
  claims: Claim[];
  rate_limit_rps: number | null;
  rate_limit_daily: number | null;
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  external_id: string;
  kyc: boolean;
  banned: boolean;
  note: string;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface UserMembership {
  id: string;
  user_id: string;
  group_id: string;
  source: string;
  zk_credential_ref: string;
  expires_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface Contract {
  id: string;
  org_id: string;
  address?: string;
  contract_address?: string;
  name?: string;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ContractGrant {
  id: string;
  contract_id: string;
  group_id: string;
  functions?: unknown[] | null;
  created_at: string;
  updated_at: string;
}

export interface DisclosureRequest {
  id: string;
  requester_user_id?: string | null;
  requester_did: string;
  target_user_id: string;
  org_id: string;
  scope: DisclosureScope;
  reason: string;
  legal_basis?: string;
  status: string;
  requested_at: string;
  expires_at?: string | null;
  decided_at?: string | null;
  decided_by_user_id?: string | null;
  decision_reason?: string;
}

export interface DisclosureScope {
  methods?: string[];
  addresses?: string[];
  date_range?: { start: string; end: string };
  disclosure_level?: string;
}

export interface DisclosureGrant {
  id: string;
  request_id: string;
  scope: DisclosureScope;
  granted_at: string;
  expires_at: string;
  revoked_at?: string | null;
  revoked_reason?: string;
}

export interface ApproveDisclosureResponse {
  grant: DisclosureGrant;
  message: string;
}

// === Helper functions ===

function adminHeaders(): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    'X-Admin-Token': ADMIN_TOKEN,
  };
}

function userHeaders(accessToken: string): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${accessToken}`,
  };
}

async function assertOk(response: Response, operation: string): Promise<void> {
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`${operation} failed: ${response.status} - ${body}`);
  }
}

// === API Client Functions ===

export async function createOrg(slug: string, name: string): Promise<Organization> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/orgs`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({ slug, name }),
  });
  await assertOk(response, `createOrg(${slug})`);
  return (await response.json()) as Organization;
}

export async function deleteOrg(orgId: string): Promise<void> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/orgs/${orgId}`, {
    method: 'DELETE',
    headers: adminHeaders(),
  });
  // 404 is acceptable during cleanup
  if (!response.ok && response.status !== 404) {
    const body = await response.text();
    throw new Error(`deleteOrg(${orgId}) failed: ${response.status} - ${body}`);
  }
}

export async function createGroup(
  orgId: string,
  slug: string,
  name: string,
  claims: Claim[],
): Promise<{ group: Group; access: GroupAccess }> {
  // Create the group
  const createResponse = await fetch(`${PROXY_URL}/api/v1/admin/orgs/${orgId}/groups`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({ slug, name }),
  });
  await assertOk(createResponse, `createGroup(${slug})`);
  const group = (await createResponse.json()) as Group;

  // Set access (claims + allowed_methods)
  const accessResponse = await fetch(
    `${PROXY_URL}/api/v1/admin/orgs/${orgId}/groups/${group.id}/access`,
    {
      method: 'PUT',
      headers: adminHeaders(),
      body: JSON.stringify({ claims, allowed_methods: expandMethodsForClaims(claims) }),
    },
  );
  await assertOk(accessResponse, `setGroupAccess(${slug})`);
  const access = (await accessResponse.json()) as GroupAccess;

  return { group, access };
}

export async function deleteGroup(orgId: string, groupId: string): Promise<void> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/orgs/${orgId}/groups/${groupId}`, {
    method: 'DELETE',
    headers: adminHeaders(),
  });
  if (!response.ok && response.status !== 404) {
    const body = await response.text();
    throw new Error(`deleteGroup(${groupId}) failed: ${response.status} - ${body}`);
  }
}

export async function getUserByDID(did: string): Promise<User | null> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/users?limit=1000`, {
    method: 'GET',
    headers: adminHeaders(),
  });
  await assertOk(response, 'listUsers');
  const body = (await response.json()) as { data: User[] };
  const users = body.data ?? [];
  return users.find((u) => u.external_id === did) ?? null;
}

/**
 * Ensure a user exists by logging them in (mock auth auto-creates the user),
 * then retrieve the internal user record.
 */
export async function ensureUser(did: string): Promise<{ user: User; accessToken: string }> {
  const tokens = await getAuthTokens(did);
  const user = await getUserByDID(did);
  if (!user) {
    throw new Error(`User not found after mock login: ${did}`);
  }
  // Set KYC=true so the user can call RPC methods
  const kycResp = await fetch(`${PROXY_URL}/api/v1/admin/users/${user.id}`, {
    method: 'PUT',
    headers: adminHeaders(),
    body: JSON.stringify({ kyc: true }),
  });
  await assertOk(kycResp, `setKYC(${user.id})`);
  user.kyc = true;
  return { user, accessToken: tokens.accessToken };
}

export async function addMembership(userId: string, groupId: string): Promise<UserMembership> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/users/${userId}/memberships`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({ group_id: groupId }),
  });
  await assertOk(response, `addMembership(${userId}, ${groupId})`);
  return (await response.json()) as UserMembership;
}

export async function deleteMembership(userId: string, membershipId: string): Promise<void> {
  const response = await fetch(
    `${PROXY_URL}/api/v1/admin/users/${userId}/memberships/${membershipId}`,
    {
      method: 'DELETE',
      headers: adminHeaders(),
    },
  );
  if (!response.ok && response.status !== 404) {
    const body = await response.text();
    throw new Error(`deleteMembership(${membershipId}) failed: ${response.status} - ${body}`);
  }
}

export async function createContract(
  orgId: string,
  address: string,
  name: string,
): Promise<Contract> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/orgs/${orgId}/contracts`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({ address, name }),
  });
  await assertOk(response, `createContract(${address})`);
  return (await response.json()) as Contract;
}

export async function deleteContract(orgId: string, address: string): Promise<void> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/orgs/${orgId}/contracts/${address}`, {
    method: 'DELETE',
    headers: adminHeaders(),
  });
  if (!response.ok && response.status !== 404) {
    const body = await response.text();
    throw new Error(`deleteContract(${address}) failed: ${response.status} - ${body}`);
  }
}

export async function createContractGrant(
  orgId: string,
  contractAddress: string,
  groupId: string,
): Promise<ContractGrant> {
  const response = await fetch(
    `${PROXY_URL}/api/v1/admin/orgs/${orgId}/contracts/${contractAddress}/grants`,
    {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ group_id: groupId }),
    },
  );
  await assertOk(response, `createContractGrant(${contractAddress}, ${groupId})`);
  return (await response.json()) as ContractGrant;
}

export async function deleteContractGrant(
  orgId: string,
  contractAddress: string,
  groupId: string,
): Promise<void> {
  const response = await fetch(
    `${PROXY_URL}/api/v1/admin/orgs/${orgId}/contracts/${contractAddress}/grants/${groupId}`,
    {
      method: 'DELETE',
      headers: adminHeaders(),
    },
  );
  if (!response.ok && response.status !== 404) {
    const body = await response.text();
    throw new Error(
      `deleteContractGrant(${contractAddress}, ${groupId}) failed: ${response.status} - ${body}`,
    );
  }
}

export async function createDisclosureRequest(
  requesterDid: string,
  targetUserId: string,
  reason: string,
  scope?: DisclosureScope,
): Promise<DisclosureRequest> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/disclosure/requests`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({
      requester_did: requesterDid,
      target_user_id: targetUserId,
      reason,
      scope: scope ?? { disclosure_level: 'full' },
    }),
  });
  await assertOk(response, 'createDisclosureRequest');
  return (await response.json()) as DisclosureRequest;
}

export async function deleteDisclosureRequest(requestId: string): Promise<void> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/disclosure/requests/${requestId}`, {
    method: 'DELETE',
    headers: adminHeaders(),
  });
  if (!response.ok && response.status !== 404) {
    const body = await response.text();
    throw new Error(`deleteDisclosureRequest(${requestId}) failed: ${response.status} - ${body}`);
  }
}

/**
 * Approve a disclosure request as the target user.
 * Uses the target user's own JWT in the Authorization header.
 */
export async function approveDisclosureRequest(
  requestId: string,
  userAccessToken: string,
  grantDurationHours = 24,
): Promise<ApproveDisclosureResponse> {
  const response = await fetch(
    `${PROXY_URL}/api/v1/me/disclosure/requests/${requestId}/approve`,
    {
      method: 'POST',
      headers: userHeaders(userAccessToken),
      body: JSON.stringify({ grant_duration_hours: grantDurationHours }),
    },
  );
  await assertOk(response, `approveDisclosureRequest(${requestId})`);
  return (await response.json()) as ApproveDisclosureResponse;
}

/**
 * Get all disclosure grants visible to the authenticated user.
 */
export async function getUserDisclosureGrants(
  userAccessToken: string,
): Promise<unknown[]> {
  const response = await fetch(`${PROXY_URL}/api/v1/me/disclosure/grants/all`, {
    method: 'GET',
    headers: userHeaders(userAccessToken),
  });
  await assertOk(response, 'getUserDisclosureGrants');
  return (await response.json()) as unknown[];
}

/**
 * Revoke a disclosure grant (admin endpoint).
 */
export async function revokeDisclosureGrant(
  grantId: string,
  reason?: string,
): Promise<void> {
  const response = await fetch(`${PROXY_URL}/api/v1/admin/disclosure/grants/${grantId}/revoke`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({ reason: reason ?? 'e2e test revocation' }),
  });
  await assertOk(response, `revokeDisclosureGrant(${grantId})`);
}

/**
 * Link a wallet address to a user via the challenge/verify flow.
 *
 * Requires MOCK_SIGNATURES=true on the proxy (set in docker-compose.e2e.yml).
 * The fake signature is accepted because MockSignatures mode skips verification.
 *
 * @param accessToken - The user's JWT access token (from ensureUser / getAuthTokens)
 * @param address     - The Ethereum address to link (must be a known Anvil account)
 */
export async function linkUserWallet(accessToken: string, address: string): Promise<void> {
  // Step 1: Get a challenge nonce for this user
  const challengeResp = await fetch(`${PROXY_URL}/eth/link/challenge`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${accessToken}` },
  });
  await assertOk(challengeResp, `eth link challenge for ${address}`);
  const { nonce } = (await challengeResp.json()) as { nonce: string; message: string };

  // Step 2: Submit verification with a dummy signature.
  // MockSignatures=true (docker-compose.e2e.yml) skips cryptographic verification.
  const verifyResp = await fetch(`${PROXY_URL}/eth/link/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${accessToken}` },
    body: JSON.stringify({
      nonce,
      address,
      signature: '0x' + 'aa'.repeat(65),
    }),
  });
  await assertOk(verifyResp, `eth link verify for ${address}`);
}

/**
 * Unlink (revoke) a wallet address from a user.
 * Silently ignores 404 (already unlinked or never linked).
 */
export async function unlinkUserWallet(accessToken: string, address: string): Promise<void> {
  const resp = await fetch(`${PROXY_URL}/eth/addresses/${address}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!resp.ok && resp.status !== 404) {
    const body = await resp.text();
    throw new Error(`unlinkUserWallet(${address}) failed: ${resp.status} - ${body}`);
  }
}

// === Fixture Class ===

interface TrackedOrg {
  id: string;
}

interface TrackedGroup {
  orgId: string;
  groupId: string;
}

interface TrackedMembership {
  userId: string;
  membershipId: string;
}

interface TrackedContract {
  orgId: string;
  address: string;
}

interface TrackedDisclosureRequest {
  id: string;
}

interface TrackedDisclosureGrant {
  id: string;
}

interface TrackedLinkedWallet {
  accessToken: string;
  address: string;
}

/**
 * ProxyAdminFixture provides test-isolated resource creation with automatic cleanup.
 * All resource names are prefixed with a short UUID to prevent collisions between
 * parallel test runs.
 */
export class ProxyAdminFixture {
  readonly testId: string;

  private orgs: TrackedOrg[] = [];
  private groups: TrackedGroup[] = [];
  private memberships: TrackedMembership[] = [];
  private contracts: TrackedContract[] = [];
  private disclosureRequests: TrackedDisclosureRequest[] = [];
  private disclosureGrants: TrackedDisclosureGrant[] = [];
  private linkedWallets: TrackedLinkedWallet[] = [];

  constructor() {
    this.testId = randomUUID().slice(0, 8);
  }

  /** Generate a test-isolated slug: `{base}_{testId}` */
  slug(base: string): string {
    return `${base}_${this.testId}`;
  }

  /** Generate a unique DID for test isolation. */
  did(): string {
    const unique = randomUUID().slice(0, 8);
    return `did:privado:e2e_${this.testId}_${unique}`;
  }

  /** Generate a random Ethereum address (0x + 40 hex chars). */
  address(): string {
    const hex1 = randomUUID().replace(/-/g, '');
    const hex2 = randomUUID().replace(/-/g, '');
    return `0x${(hex1 + hex2).slice(0, 40)}`;
  }

  async setup(): Promise<void> {
    // No-op: ensures the fixture is ready. Could be extended for health checks.
  }

  /**
   * Clean up all tracked resources in reverse creation order.
   * Silently ignores errors during cleanup to ensure all resources are attempted.
   */
  async cleanup(): Promise<void> {
    // Revoke grants first
    for (const g of [...this.disclosureGrants].reverse()) {
      try {
        await revokeDisclosureGrant(g.id, 'e2e cleanup');
      } catch {
        // Ignore cleanup errors
      }
    }

    // Delete pending disclosure requests
    for (const r of [...this.disclosureRequests].reverse()) {
      try {
        await deleteDisclosureRequest(r.id);
      } catch {
        // Ignore cleanup errors (may have been approved/rejected)
      }
    }

    // Delete contracts
    for (const c of [...this.contracts].reverse()) {
      try {
        await deleteContract(c.orgId, c.address);
      } catch {
        // Ignore cleanup errors
      }
    }

    // Delete memberships
    for (const m of [...this.memberships].reverse()) {
      try {
        await deleteMembership(m.userId, m.membershipId);
      } catch {
        // Ignore cleanup errors
      }
    }

    // Delete groups (children first due to reverse order)
    for (const g of [...this.groups].reverse()) {
      try {
        await deleteGroup(g.orgId, g.groupId);
      } catch {
        // Ignore cleanup errors
      }
    }

    // Delete orgs last
    for (const o of [...this.orgs].reverse()) {
      try {
        await deleteOrg(o.id);
      } catch {
        // Ignore cleanup errors
      }
    }

    // Unlink wallets
    for (const w of [...this.linkedWallets].reverse()) {
      try {
        await unlinkUserWallet(w.accessToken, w.address);
      } catch {
        // Ignore cleanup errors
      }
    }

    // Clear all tracking arrays
    this.disclosureGrants = [];
    this.disclosureRequests = [];
    this.contracts = [];
    this.memberships = [];
    this.groups = [];
    this.orgs = [];
    this.linkedWallets = [];
  }

  // === Tracked resource creation ===

  async createOrg(slugBase: string, name?: string): Promise<Organization> {
    const s = this.slug(slugBase);
    const org = await createOrg(s, name ?? `Test Org ${s}`);
    this.orgs.push({ id: org.id });
    return org;
  }

  async createGroup(
    orgId: string,
    slugBase: string,
    name: string,
    claims: Claim[],
  ): Promise<{ group: Group; access: GroupAccess }> {
    const s = this.slug(slugBase);
    const result = await createGroup(orgId, s, name, claims);
    this.groups.push({ orgId, groupId: result.group.id });
    return result;
  }

  async ensureUser(did: string): Promise<{ user: User; accessToken: string }> {
    return ensureUser(did);
  }

  async addMembership(userId: string, groupId: string): Promise<UserMembership> {
    const membership = await addMembership(userId, groupId);
    this.memberships.push({ userId, membershipId: membership.id });
    return membership;
  }

  async createContract(orgId: string, address: string, name: string): Promise<Contract> {
    const contract = await createContract(orgId, address, name);
    this.contracts.push({ orgId, address });
    return contract;
  }

  async createContractGrant(
    orgId: string,
    contractAddress: string,
    groupId: string,
  ): Promise<ContractGrant> {
    return createContractGrant(orgId, contractAddress, groupId);
  }

  /**
   * Link an Ethereum address to a user via mock signature flow.
   * Requires MOCK_SIGNATURES=true on the proxy (set in docker-compose.e2e.yml).
   */
  async linkUserWallet(accessToken: string, address: string): Promise<void> {
    await linkUserWallet(accessToken, address);
    this.linkedWallets.push({ accessToken, address });
  }

  async createDisclosureRequest(
    requesterDid: string,
    targetUserId: string,
    reason: string,
    scope?: DisclosureScope,
  ): Promise<DisclosureRequest> {
    const req = await createDisclosureRequest(requesterDid, targetUserId, reason, scope);
    this.disclosureRequests.push({ id: req.id });
    return req;
  }

  async approveDisclosureRequest(
    requestId: string,
    userAccessToken: string,
    grantDurationHours = 24,
  ): Promise<ApproveDisclosureResponse> {
    const result = await approveDisclosureRequest(requestId, userAccessToken, grantDurationHours);
    this.disclosureGrants.push({ id: result.grant.id });
    return result;
  }

  async revokeDisclosureGrant(grantId: string, reason?: string): Promise<void> {
    return revokeDisclosureGrant(grantId, reason);
  }
}
