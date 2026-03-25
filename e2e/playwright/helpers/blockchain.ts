const ANVIL_URL = process.env.ANVIL_URL || 'http://localhost:8545';
const EXPLORER_API_URL = process.env.EXPLORER_URL || 'http://localhost:3001';

/** Known Anvil pre-funded accounts (always available, unlocked). */
export const ANVIL_ACCOUNTS: { address: string }[] = [
  { address: '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266' },
  { address: '0x70997970C51812dc3A010C7d01b50e0d17dc79C8' },
  { address: '0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC' },
  { address: '0x90F79bf6EB2c4f870365E785982E1f101E93b906' },
  { address: '0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65' },
  { address: '0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc' },
  { address: '0x976EA74026E726554dB657fA54763abd0C3a0aa9' },
  { address: '0x14dC79964da2C08dA15Fd353d30a3d14002b962b' },
  { address: '0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f' },
  { address: '0xa0Ee7A142d267C1f36714E4a8F75612F20a79720' },
];

let rpcId = 1;

/**
 * Make a JSON-RPC call to Anvil.
 * Uses native fetch (Node.js 18+).
 */
export async function anvilRpc(method: string, params: unknown[] = []): Promise<unknown> {
  const id = rpcId++;
  const response = await fetch(ANVIL_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', method, params, id }),
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Anvil RPC ${method} HTTP error: ${response.status} - ${body}`);
  }

  const data = (await response.json()) as { result?: unknown; error?: { message: string; code: number } };
  if (data.error) {
    throw new Error(`Anvil RPC ${method} error: ${data.error.message} (code ${data.error.code})`);
  }

  return data.result;
}

/**
 * Send ETH between unlocked Anvil accounts using eth_sendTransaction.
 * Returns the transaction hash.
 */
export async function sendETH(from: string, to: string, valueWei: bigint): Promise<string> {
  const txHash = (await anvilRpc('eth_sendTransaction', [
    {
      from,
      to,
      value: '0x' + valueWei.toString(16),
    },
  ])) as string;
  return txHash;
}

/**
 * Wait for a transaction receipt by polling eth_getTransactionReceipt.
 * Polls every 500ms, up to 15s timeout.
 */
export async function waitForReceipt(
  txHash: string,
): Promise<{ blockNumber: string; contractAddress?: string | null }> {
  const maxAttempts = 30;
  for (let i = 0; i < maxAttempts; i++) {
    const receipt = (await anvilRpc('eth_getTransactionReceipt', [txHash])) as {
      blockNumber: string;
      contractAddress?: string | null;
    } | null;

    if (receipt) {
      return receipt;
    }

    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  throw new Error(`Timeout waiting for receipt of tx ${txHash} after ${maxAttempts * 500}ms`);
}

/**
 * Get the current block number from Anvil (as a decimal number).
 */
export async function getBlockNumber(): Promise<number> {
  const hex = (await anvilRpc('eth_blockNumber')) as string;
  return parseInt(hex, 16);
}

/**
 * Wait for the block-explorer indexer to reach a target block number.
 * Checks if the target block exists via the blocks API endpoint.
 * The sync status endpoint only updates every 10 blocks in realtime mode,
 * so checking the actual block is more reliable.
 */
export async function waitForIndexer(targetBlock: number, timeoutSeconds = 60): Promise<void> {
  const maxAttempts = timeoutSeconds;
  for (let i = 0; i < maxAttempts; i++) {
    try {
      const response = await fetch(`${EXPLORER_API_URL}/api/v1/blocks/${targetBlock}`);
      if (response.ok) {
        return;
      }
    } catch {
      // Network error, retry
    }

    await new Promise((resolve) => setTimeout(resolve, 1000));
  }

  throw new Error(
    `Timeout: block-explorer indexer did not reach block ${targetBlock} after ${timeoutSeconds}s`,
  );
}

/**
 * Generate ETH transfers between specific Anvil accounts to create transaction history.
 *
 * Uses account[0] as the sender for all transfers, cycling through recipients
 * starting from account[1]. This ensures predictable sender/recipient pairs.
 *
 * Each transfer sends 0.001 ETH.
 *
 * Returns the array of transaction hashes.
 */
export async function generateTransactions(count: number): Promise<string[]> {
  const hashes: string[] = [];
  const valueWei = BigInt('1000000000000000'); // 0.001 ETH

  for (let i = 0; i < count; i++) {
    // Sender is always account[0], recipient cycles through account[1..5]
    const recipientIndex = (i % (ANVIL_ACCOUNTS.length - 1)) + 1;
    const from = ANVIL_ACCOUNTS[0].address;
    const to = ANVIL_ACCOUNTS[recipientIndex].address;

    const txHash = await sendETH(from, to, valueWei);
    hashes.push(txHash);
  }

  // Wait for all receipts to ensure they are mined
  for (const hash of hashes) {
    await waitForReceipt(hash);
  }

  return hashes;
}

/**
 * Send ETH specifically from account[0] to account[2] to ensure account[2]
 * appears in the transaction list. Returns the transaction hash.
 */
export async function sendToAccount2(): Promise<string> {
  const from = ANVIL_ACCOUNTS[0].address;
  const to = ANVIL_ACCOUNTS[2].address;
  const valueWei = BigInt('2000000000000000'); // 0.002 ETH
  const txHash = await sendETH(from, to, valueWei);
  await waitForReceipt(txHash);
  return txHash;
}
