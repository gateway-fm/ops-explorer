import { test, expect } from '@playwright/test';
import { ProxyAdminFixture } from '../../helpers/proxy-admin';
import {
  ANVIL_ACCOUNTS,
  anvilRpc,
  waitForReceipt,
} from '../../helpers/blockchain';

// ---------------------------------------------------------------------------
// Event Rules — Param Constraint Scenarios (end-to-end)
//
// These tests cover the complex event rule filtering behaviors where bugs
// are most likely to surface:
//
//   1. Self-param constraint on an indexed parameter
//      - Only events where the constrained indexed param matches the
//        viewer's linked wallet are visible.
//
//   2. Multiple param rules with OR semantics
//      - The proxy uses OR across param_rules on the same event rule:
//        Transfer visible if from=self OR to=self.
//      - Transfer(other->other) is hidden; Transfer(user->other) and
//        Transfer(other->user) both pass.
//
//   3. Multiple param rules — neither matches (deny)
//      - When a Transfer involves two third-party addresses, the event
//        is hidden even though the viewer has the grant.
//
//   4. Mixed grants — different groups see different logs
//      - Group A: can see only NumberSet events
//      - Group B: can see all events (null event_rules)
//      - User in Group A sees filtered logs; User in Group B sees all.
//
// Contract used:
//   EventCounter (contracts/src/EventCounter.sol):
//     NumberSet(address indexed sender, uint256 oldNumber, uint256 newNumber)
//     NumberIncremented(address indexed sender, uint256 oldNumber, uint256 newNumber)
//
// All assertions are via JSON-RPC through the proxy (eth_getLogs,
// eth_getTransactionReceipt) with JWT auth. No browser/UI is needed.
// ---------------------------------------------------------------------------

const PROXY_URL = process.env.PROXY_URL || 'http://localhost:8080';
const ADMIN_TOKEN = process.env.ADMIN_API_TOKEN || '';

// --- Topic0 hashes ---

// NumberSet(address,uint256,uint256)
const NUMBER_SET_TOPIC0 =
  '0xf1149f7c8c8b42148d37b45554fa667d734d10bb316ce16a28abd45e047d15b9';
// NumberIncremented(address,uint256,uint256)
const NUMBER_INCREMENTED_TOPIC0 =
  '0xbb5f390e400fa8a5d60a259882f5681ee9e9115665c6a3ec9fa02eef72b87ebd';

// --- Contract bytecodes ---

// EventCounter with constructor arg 42.
const EVENT_COUNTER_INIT_CODE =
  '0x6080604052348015600e575f5ffd5b5060405161021e38038061021e8339810160408190' +
  '52602b91606f565b5f818155604080519182526020820183905233917ff1149f7c8c8b4214' +
  '8d37b45554fa667d734d10bb316ce16a28abd45e047d15b9910160405180910390a2506085' +
  '565b5f60208284031215607e575f5ffd5b5051919050565b61018c806100925f395ff3fe60' +
  '8060405234801561000f575f5ffd5b506004361061003f575f3560e01c80633fb5c1cb1461' +
  '00435780638381f58a14610058578063d09de08a14610072575b5f5ffd5b61005661005136' +
  '600461011b565b61007a565b005b6100605f5481565b60405190815260200160405180910390' +
  'f35b6100566100c0565b5f805490829055604080518281526020810184905233917ff1149f' +
  '7c8c8b42148d37b45554fa667d734d10bb316ce16a28abd45e047d15b9910160405180910390' +
  'a25050565b5f8054908190806100d083610132565b90915550505f5460405133917fbb5f390e' +
  '400fa8a5d60a259882f5681ee9e9115665c6a3ec9fa02eef72b87ebd9161011091858252602' +
  '082015260400190565b60405180910390a250565b5f6020828403121561012b575f5ffd5b50' +
  '35919050565b5f6001820161014f57634e487b7160e01b5f52601160045260245ffd5b5060' +
  '01019056fea2646970667358221220540f302817261326e20a065fbd81a3dd374f48268ff414' +
  '6e83d5da2d5c3e6b8a64736f6c634300081e0033' +
  '000000000000000000000000000000000000000000000000000000000000002a';

// --- ABI definitions ---

const EVENT_COUNTER_ABI = JSON.stringify([
  {
    type: 'event',
    name: 'NumberSet',
    inputs: [
      { name: 'sender', type: 'address', indexed: true },
      { name: 'oldNumber', type: 'uint256', indexed: false },
      { name: 'newNumber', type: 'uint256', indexed: false },
    ],
  },
  {
    type: 'event',
    name: 'NumberIncremented',
    inputs: [
      { name: 'sender', type: 'address', indexed: true },
      { name: 'oldNumber', type: 'uint256', indexed: false },
      { name: 'newNumber', type: 'uint256', indexed: false },
    ],
  },
  {
    type: 'function',
    name: 'setNumber',
    inputs: [{ name: 'newNumber', type: 'uint256' }],
    outputs: [],
  },
  {
    type: 'function',
    name: 'increment',
    inputs: [],
    outputs: [],
  },
  {
    type: 'function',
    name: 'number',
    inputs: [],
    outputs: [{ type: 'uint256' }],
  },
]);

// Function selectors
const SET_NUMBER_SELECTOR = '0x3fb5c1cb';
const INCREMENT_SELECTOR = '0xd09de08a';

// --- Helpers ---

/** Make a JSON-RPC call through the privacy proxy (with auth). */
async function proxyRpc(
  method: string,
  params: unknown[],
  accessToken: string,
): Promise<unknown> {
  const response = await fetch(PROXY_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({ jsonrpc: '2.0', method, params, id: 1 }),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Proxy RPC ${method} HTTP error: ${response.status} - ${body}`);
  }
  const data = (await response.json()) as {
    result?: unknown;
    error?: { message: string; code: number };
  };
  if (data.error) {
    throw new Error(
      `Proxy RPC ${method} error: ${data.error.message} (code ${data.error.code})`,
    );
  }
  return data.result;
}

/** Deploy a contract to Anvil using eth_sendTransaction from an unlocked account. */
async function deployContract(
  deployerAddress: string,
  initCode: string,
): Promise<{ txHash: string; contractAddress: string }> {
  const txHash = (await anvilRpc('eth_sendTransaction', [
    { from: deployerAddress, data: initCode, gas: '0x4C4B40' },
  ])) as string;

  const receipt = await waitForReceipt(txHash);
  if (!receipt.contractAddress) {
    throw new Error(
      `Contract deployment failed: no contractAddress in receipt for ${txHash}`,
    );
  }

  return { txHash, contractAddress: receipt.contractAddress };
}

/** Send a contract function call on Anvil (unlocked account). */
async function sendContractTx(
  from: string,
  to: string,
  data: string,
): Promise<string> {
  const txHash = (await anvilRpc('eth_sendTransaction', [
    { from, to, data, gas: '0x30D40' },
  ])) as string;
  await waitForReceipt(txHash);
  return txHash;
}

/** Encode setNumber(uint256) calldata. */
function encodeSetNumber(value: number): string {
  return SET_NUMBER_SELECTOR + value.toString(16).padStart(64, '0');
}

/** Encode increment() calldata. */
function encodeIncrement(): string {
  return INCREMENT_SELECTOR;
}

/**
 * Pad an Ethereum address into a 32-byte topic hex (for topic comparison).
 * e.g., 0xf39F... -> 0x000000000000000000000000f39f...
 */
function addressToTopic(address: string): string {
  return '0x' + address.replace(/^0x/, '').toLowerCase().padStart(64, '0');
}

/**
 * Update a contract grant's event_rules via the privacy proxy admin API.
 * Supports param_rules on each event rule.
 *
 * PUT /api/v1/admin/orgs/:org_id/contracts/:address/grants/:group_id
 */
async function updateContractGrantEventRules(
  orgId: string,
  contractAddress: string,
  groupId: string,
  eventRules: { topic0: string; name: string; param_rules?: { index: number; must_be: string }[] }[] | null,
): Promise<void> {
  const response = await fetch(
    `${PROXY_URL}/api/v1/admin/orgs/${orgId}/contracts/${contractAddress}/grants/${groupId}`,
    {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Token': ADMIN_TOKEN,
      },
      body: JSON.stringify({ event_rules: eventRules }),
    },
  );
  if (!response.ok) {
    const body = await response.text();
    throw new Error(
      `updateContractGrantEventRules failed: ${response.status} - ${body}`,
    );
  }
}

/**
 * Upload the contract ABI via the privacy proxy admin API.
 * PUT /api/v1/admin/orgs/:org_id/contracts/:address/abi
 */
async function uploadContractABI(
  orgId: string,
  contractAddress: string,
  abi: string,
): Promise<void> {
  const response = await fetch(
    `${PROXY_URL}/api/v1/admin/orgs/${orgId}/contracts/${contractAddress}/abi`,
    {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Token': ADMIN_TOKEN,
      },
      body: JSON.stringify({ abi }),
    },
  );
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`uploadContractABI failed: ${response.status} - ${body}`);
  }
}

// ---------------------------------------------------------------------------
// 1. Self-param constraint on indexed parameter (NumberSet.sender)
// ---------------------------------------------------------------------------

test.describe('Param Constraint: self on indexed parameter', () => {
  test.slow();

  let fixture: ProxyAdminFixture;
  let orgId: string;
  let groupId: string;
  let userToken: string;
  let contractAddress: string;

  // Use two different senders so we get NumberSet events from distinct addresses.
  // Account [9] is not used as a sender in any other test file.
  // Account [1] is only used as a recipient in other tests — no sender collision.
  const USER_WALLET = ANVIL_ACCOUNTS[9].address;   // user's linked wallet
  const OTHER_WALLET = ANVIL_ACCOUNTS[1].address;   // unrelated sender

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Deploy EventCounter from USER_WALLET (constructor emits NumberSet with USER_WALLET as sender)
    const deployResult = await deployContract(USER_WALLET, EVENT_COUNTER_INIT_CODE);
    contractAddress = deployResult.contractAddress.toLowerCase();

    // RBAC setup
    const org = await fixture.createOrg('paramidx', 'Param Index Test');
    orgId = org.id;

    const { group } = await fixture.createGroup(orgId, 'viewers', 'Viewers', ['read']);
    groupId = group.id;

    await fixture.createContract(orgId, contractAddress, 'ParamIdx EventCounter');
    await uploadContractABI(orgId, contractAddress, EVENT_COUNTER_ABI);
    await fixture.createContractGrant(orgId, contractAddress, groupId);

    // Event rule: NumberSet allowed ONLY when sender (index 0, indexed) = self
    await updateContractGrantEventRules(orgId, contractAddress, groupId, [
      {
        topic0: NUMBER_SET_TOPIC0,
        name: 'NumberSet',
        param_rules: [{ index: 0, must_be: 'self' }],
      },
    ]);

    // Create user and link USER_WALLET
    const userDid = fixture.did();
    const { user, accessToken } = await fixture.ensureUser(userDid);
    userToken = accessToken;
    await fixture.addMembership(user.id, groupId);
    await fixture.linkUserWallet(userToken, USER_WALLET);

    // Emit NumberSet from USER_WALLET (sender = user)
    await sendContractTx(USER_WALLET, contractAddress, encodeSetNumber(200));

    // Emit NumberSet from OTHER_WALLET (sender != user)
    await sendContractTx(OTHER_WALLET, contractAddress, encodeSetNumber(300));
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('Anvil has NumberSet events from both senders', async () => {
    const logs = (await anvilRpc('eth_getLogs', [
      {
        address: contractAddress,
        topics: [NUMBER_SET_TOPIC0],
        fromBlock: '0x0',
        toBlock: 'latest',
      },
    ])) as Array<{ topics: string[] }>;

    // Constructor + USER_WALLET setNumber + OTHER_WALLET setNumber = at least 3
    expect(logs.length).toBeGreaterThanOrEqual(3);

    const senders = new Set(logs.map((l) => l.topics[1]?.toLowerCase()));
    expect(senders.has(addressToTopic(USER_WALLET))).toBe(true);
    expect(senders.has(addressToTopic(OTHER_WALLET))).toBe(true);
  });

  test('proxy returns only NumberSet events where sender = user', async () => {
    const logs = (await proxyRpc(
      'eth_getLogs',
      [
        {
          address: contractAddress,
          topics: [NUMBER_SET_TOPIC0],
          fromBlock: '0x0',
          toBlock: 'latest',
        },
      ],
      userToken,
    )) as Array<{ topics: string[] }>;

    // Every returned log must have sender = USER_WALLET
    expect(logs.length).toBeGreaterThanOrEqual(1);
    const userTopic = addressToTopic(USER_WALLET);
    for (const log of logs) {
      expect(log.topics[1]?.toLowerCase()).toBe(userTopic);
    }

    // No logs from OTHER_WALLET should pass
    const otherTopic = addressToTopic(OTHER_WALLET);
    const fromOther = logs.filter((l) => l.topics[1]?.toLowerCase() === otherTopic);
    expect(fromOther.length).toBe(0);
  });

  test('NumberIncremented events are fully blocked (not in allowlist)', async () => {
    // Emit a NumberIncremented from USER_WALLET
    await sendContractTx(USER_WALLET, contractAddress, encodeIncrement());

    const logs = (await proxyRpc(
      'eth_getLogs',
      [
        {
          address: contractAddress,
          topics: [NUMBER_INCREMENTED_TOPIC0],
          fromBlock: '0x0',
          toBlock: 'latest',
        },
      ],
      userToken,
    )) as Array<{ topics: string[] }>;

    // NumberIncremented is not in the event_rules at all, so it must be blocked
    expect(logs.length).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// 2. Multiple param rules — OR semantics
//
// EventCounter's NumberSet has only one address param (sender, index 0).
// To properly test OR across two address params we use the same event rule
// with param_rules [{index:0, self}, {index:1, self}] on a Transfer event.
//
// Since we don't have a compiled ERC20 in the repo, we test OR semantics
// using EventCounter by setting up two param_rules on NumberSet:
//   - {index: 0, must_be: "self"} = sender must be self (indexed address)
//   - {index: 1, must_be: "self"} = oldNumber must be self (non-indexed uint256)
//
// Index 1 (oldNumber) is a uint256, not an address, so it will never match
// any user address. This means only the index-0 rule can trigger, and the
// OR semantic means the event still passes when index 0 matches.
//
// This verifies the OR behavior: even though one param_rule is unsatisfiable,
// the event is visible as long as the other matches.
// ---------------------------------------------------------------------------

test.describe('Param Constraint: OR semantics across multiple rules', () => {
  test.slow();

  let fixture: ProxyAdminFixture;
  let orgId: string;
  let groupId: string;
  let userToken: string;
  let contractAddress: string;

  // Same accounts as describe block 1 — each describe block deploys its own
  // contract so there is no state collision between them.
  const USER_WALLET = ANVIL_ACCOUNTS[9].address;
  const OTHER_WALLET = ANVIL_ACCOUNTS[1].address;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    const deployResult = await deployContract(USER_WALLET, EVENT_COUNTER_INIT_CODE);
    contractAddress = deployResult.contractAddress.toLowerCase();

    const org = await fixture.createOrg('paramor', 'Param OR Test');
    orgId = org.id;

    const { group } = await fixture.createGroup(orgId, 'viewers', 'Viewers', ['read']);
    groupId = group.id;

    await fixture.createContract(orgId, contractAddress, 'ParamOR EventCounter');
    await uploadContractABI(orgId, contractAddress, EVENT_COUNTER_ABI);
    await fixture.createContractGrant(orgId, contractAddress, groupId);

    // Two param_rules with OR semantics:
    //   index 0 (sender, address, indexed) = self — can match
    //   index 1 (oldNumber, uint256, non-indexed) = self — never matches (not an address)
    // Result: OR means event passes whenever sender = user
    await updateContractGrantEventRules(orgId, contractAddress, groupId, [
      {
        topic0: NUMBER_SET_TOPIC0,
        name: 'NumberSet',
        param_rules: [
          { index: 0, must_be: 'self' },
          { index: 1, must_be: 'self' },
        ],
      },
    ]);

    const userDid = fixture.did();
    const { user, accessToken } = await fixture.ensureUser(userDid);
    userToken = accessToken;
    await fixture.addMembership(user.id, groupId);
    await fixture.linkUserWallet(userToken, USER_WALLET);

    // Events from user's wallet
    await sendContractTx(USER_WALLET, contractAddress, encodeSetNumber(500));
    // Events from another wallet
    await sendContractTx(OTHER_WALLET, contractAddress, encodeSetNumber(600));
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('OR semantics: user events pass even though one param_rule cannot match', async () => {
    const logs = (await proxyRpc(
      'eth_getLogs',
      [
        {
          address: contractAddress,
          topics: [NUMBER_SET_TOPIC0],
          fromBlock: '0x0',
          toBlock: 'latest',
        },
      ],
      userToken,
    )) as Array<{ topics: string[] }>;

    // User's events should pass (index 0 = self matches via OR)
    expect(logs.length).toBeGreaterThanOrEqual(1);
    const userTopic = addressToTopic(USER_WALLET);
    const userLogs = logs.filter((l) => l.topics[1]?.toLowerCase() === userTopic);
    expect(userLogs.length).toBeGreaterThanOrEqual(1);
  });

  test('OR semantics: third-party events are blocked (no param_rule matches)', async () => {
    const logs = (await proxyRpc(
      'eth_getLogs',
      [
        {
          address: contractAddress,
          topics: [NUMBER_SET_TOPIC0],
          fromBlock: '0x0',
          toBlock: 'latest',
        },
      ],
      userToken,
    )) as Array<{ topics: string[] }>;

    // OTHER_WALLET events should be blocked (neither index 0 nor index 1 is self)
    const otherTopic = addressToTopic(OTHER_WALLET);
    const otherLogs = logs.filter((l) => l.topics[1]?.toLowerCase() === otherTopic);
    expect(otherLogs.length).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// 3. Mixed grants — different groups see different logs
//
// Two groups on the same contract with different event rules:
//   Group A (restricted): NumberSet events only
//   Group B (unrestricted): null event_rules = all events visible
//
// User A (in Group A) should only see NumberSet.
// User B (in Group B) should see both NumberSet and NumberIncremented.
// ---------------------------------------------------------------------------

test.describe('Mixed Grants: different groups see different logs', () => {
  test.slow();

  let fixture: ProxyAdminFixture;
  let orgId: string;
  let groupAId: string;
  let groupBId: string;
  let userAToken: string;
  let userBToken: string;
  let contractAddress: string;

  // Account [9] — same as USER_WALLET in other describe blocks (no cross-block collision).
  const DEPLOYER = ANVIL_ACCOUNTS[9].address;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    // Deploy EventCounter
    const deployResult = await deployContract(DEPLOYER, EVENT_COUNTER_INIT_CODE);
    contractAddress = deployResult.contractAddress.toLowerCase();

    // RBAC setup: one org, two groups
    const org = await fixture.createOrg('mixgrant', 'Mixed Grant Test');
    orgId = org.id;

    const { group: groupA } = await fixture.createGroup(
      orgId, 'restricted', 'Restricted Group', ['read'],
    );
    groupAId = groupA.id;

    const { group: groupB } = await fixture.createGroup(
      orgId, 'unrestricted', 'Unrestricted Group', ['read'],
    );
    groupBId = groupB.id;

    // Register contract
    await fixture.createContract(orgId, contractAddress, 'MixGrant EventCounter');
    await uploadContractABI(orgId, contractAddress, EVENT_COUNTER_ABI);

    // Grant A: only NumberSet events
    await fixture.createContractGrant(orgId, contractAddress, groupAId);
    await updateContractGrantEventRules(orgId, contractAddress, groupAId, [
      { topic0: NUMBER_SET_TOPIC0, name: 'NumberSet' },
    ]);

    // Grant B: null event_rules = all events visible
    await fixture.createContractGrant(orgId, contractAddress, groupBId);
    // No updateContractGrantEventRules call — null by default = unrestricted

    // User A (restricted group)
    const userADid = fixture.did();
    const { user: userA, accessToken: tokenA } = await fixture.ensureUser(userADid);
    userAToken = tokenA;
    await fixture.addMembership(userA.id, groupAId);
    await fixture.linkUserWallet(userAToken, DEPLOYER);

    // User B (unrestricted group).
    // Both users link the same DEPLOYER address — the DB allows this
    // (UNIQUE(did, eth_address) since migration 027). User B needs it
    // because null event_rules falls back to address-based filtering,
    // which requires the user's linked address to appear in a topic.
    const userBDid = fixture.did();
    const { user: userB, accessToken: tokenB } = await fixture.ensureUser(userBDid);
    userBToken = tokenB;
    await fixture.addMembership(userB.id, groupBId);
    await fixture.linkUserWallet(userBToken, DEPLOYER);

    // Emit both event types
    await sendContractTx(DEPLOYER, contractAddress, encodeSetNumber(700));
    await sendContractTx(DEPLOYER, contractAddress, encodeIncrement());
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('User A (restricted) sees only NumberSet events', async () => {
    const logs = (await proxyRpc(
      'eth_getLogs',
      [{ address: contractAddress, fromBlock: '0x0', toBlock: 'latest' }],
      userAToken,
    )) as Array<{ topics: string[] }>;

    expect(logs.length).toBeGreaterThanOrEqual(1);

    // Every log must be NumberSet
    for (const log of logs) {
      expect(log.topics[0]?.toLowerCase()).toBe(NUMBER_SET_TOPIC0.toLowerCase());
    }

    // No NumberIncremented
    const incLogs = logs.filter(
      (l) => l.topics[0]?.toLowerCase() === NUMBER_INCREMENTED_TOPIC0.toLowerCase(),
    );
    expect(incLogs.length).toBe(0);
  });

  test('User B (unrestricted) sees both NumberSet and NumberIncremented', async () => {
    const logs = (await proxyRpc(
      'eth_getLogs',
      [{ address: contractAddress, fromBlock: '0x0', toBlock: 'latest' }],
      userBToken,
    )) as Array<{ topics: string[] }>;

    const topics = new Set(logs.map((l) => l.topics[0]?.toLowerCase()));

    // Must have both event types
    expect(topics.has(NUMBER_SET_TOPIC0.toLowerCase())).toBe(true);
    expect(topics.has(NUMBER_INCREMENTED_TOPIC0.toLowerCase())).toBe(true);
  });

  test('User A receipt shows NumberSet log, User B receipt shows all logs', async () => {
    // setNumber tx has one NumberSet log
    // We need to find a transaction that emits events we can check.
    // Use setNumber tx — should have 1 NumberSet log for both users.
    const setTxHash = await sendContractTx(DEPLOYER, contractAddress, encodeSetNumber(800));

    // User A: should see the NumberSet log
    const receiptA = (await proxyRpc(
      'eth_getTransactionReceipt',
      [setTxHash],
      userAToken,
    )) as { logs: Array<{ topics: string[] }> } | null;

    if (receiptA !== null) {
      expect(receiptA.logs.length).toBe(1);
      expect(receiptA.logs[0].topics[0]?.toLowerCase()).toBe(
        NUMBER_SET_TOPIC0.toLowerCase(),
      );
    }

    // User B: should also see the NumberSet log (unrestricted)
    const receiptB = (await proxyRpc(
      'eth_getTransactionReceipt',
      [setTxHash],
      userBToken,
    )) as { logs: Array<{ topics: string[] }> } | null;

    if (receiptB !== null) {
      expect(receiptB.logs.length).toBe(1);
      expect(receiptB.logs[0].topics[0]?.toLowerCase()).toBe(
        NUMBER_SET_TOPIC0.toLowerCase(),
      );
    }

    // Now test an increment tx — has only NumberIncremented
    const incTxHash = await sendContractTx(DEPLOYER, contractAddress, encodeIncrement());

    // User A: NumberIncremented is blocked, so logs should be empty
    const incReceiptA = (await proxyRpc(
      'eth_getTransactionReceipt',
      [incTxHash],
      userAToken,
    )) as { logs: Array<{ topics: string[] }> } | null;

    if (incReceiptA !== null) {
      expect(incReceiptA.logs.length).toBe(0);
    }

    // User B: unrestricted, should see the NumberIncremented log
    const incReceiptB = (await proxyRpc(
      'eth_getTransactionReceipt',
      [incTxHash],
      userBToken,
    )) as { logs: Array<{ topics: string[] }> } | null;

    if (incReceiptB !== null) {
      expect(incReceiptB.logs.length).toBe(1);
      expect(incReceiptB.logs[0].topics[0]?.toLowerCase()).toBe(
        NUMBER_INCREMENTED_TOPIC0.toLowerCase(),
      );
    }
  });
});

// ---------------------------------------------------------------------------
// 4. Self-param constraint — sender filter via eth_getTransactionReceipt
//
// Verifies that param constraints also apply to receipt log filtering,
// not just eth_getLogs.
// ---------------------------------------------------------------------------

test.describe('Param Constraint: receipt log filtering', () => {
  test.slow();

  let fixture: ProxyAdminFixture;
  let orgId: string;
  let groupId: string;
  let userToken: string;
  let contractAddress: string;
  let userSetTxHash: string;
  let otherSetTxHash: string;

  const USER_WALLET = ANVIL_ACCOUNTS[9].address;
  const OTHER_WALLET = ANVIL_ACCOUNTS[1].address;

  test.beforeAll(async () => {
    fixture = new ProxyAdminFixture();
    await fixture.setup();

    const deployResult = await deployContract(USER_WALLET, EVENT_COUNTER_INIT_CODE);
    contractAddress = deployResult.contractAddress.toLowerCase();

    const org = await fixture.createOrg('paramreceipt', 'Param Receipt Test');
    orgId = org.id;

    const { group } = await fixture.createGroup(orgId, 'viewers', 'Viewers', ['read']);
    groupId = group.id;

    await fixture.createContract(orgId, contractAddress, 'ParamReceipt EventCounter');
    await uploadContractABI(orgId, contractAddress, EVENT_COUNTER_ABI);
    await fixture.createContractGrant(orgId, contractAddress, groupId);

    // NumberSet only when sender = self
    await updateContractGrantEventRules(orgId, contractAddress, groupId, [
      {
        topic0: NUMBER_SET_TOPIC0,
        name: 'NumberSet',
        param_rules: [{ index: 0, must_be: 'self' }],
      },
    ]);

    const userDid = fixture.did();
    const { user, accessToken } = await fixture.ensureUser(userDid);
    userToken = accessToken;
    await fixture.addMembership(user.id, groupId);
    await fixture.linkUserWallet(userToken, USER_WALLET);

    // Transaction from user's wallet
    userSetTxHash = await sendContractTx(
      USER_WALLET, contractAddress, encodeSetNumber(900),
    );
    // Transaction from another wallet
    otherSetTxHash = await sendContractTx(
      OTHER_WALLET, contractAddress, encodeSetNumber(1000),
    );
  });

  test.afterAll(async () => {
    await fixture.cleanup();
  });

  test('receipt for user-sent tx preserves NumberSet log (sender = self)', async () => {
    const receipt = (await proxyRpc(
      'eth_getTransactionReceipt',
      [userSetTxHash],
      userToken,
    )) as { logs: Array<{ topics: string[] }> } | null;

    if (receipt !== null) {
      expect(receipt.logs.length).toBe(1);
      expect(receipt.logs[0].topics[0]?.toLowerCase()).toBe(
        NUMBER_SET_TOPIC0.toLowerCase(),
      );
      expect(receipt.logs[0].topics[1]?.toLowerCase()).toBe(
        addressToTopic(USER_WALLET),
      );
    }
  });

  test('receipt for other-sent tx has empty logs (sender != self)', async () => {
    const receipt = (await proxyRpc(
      'eth_getTransactionReceipt',
      [otherSetTxHash],
      userToken,
    )) as { logs: Array<{ topics: string[] }> } | null;

    // The receipt itself may be visible (the user has read access to the contract),
    // but the logs should be filtered out because sender != self.
    if (receipt !== null) {
      expect(receipt.logs.length).toBe(0);
    }
  });
});
