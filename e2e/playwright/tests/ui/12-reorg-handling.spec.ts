import { test, expect } from '@playwright/test';
import {
  anvilRpc,
  sendETH,
  waitForReceipt,
  getBlockNumber,
  waitForIndexer,
  ANVIL_ACCOUNTS,
} from '../../helpers/blockchain';

const EXPLORER_API_URL = process.env.EXPLORER_URL || 'http://localhost:3001';

async function explorerApi(path: string): Promise<unknown> {
  const res = await fetch(`${EXPLORER_API_URL}/api/v1${path}`);
  if (!res.ok) {
    throw new Error(`Explorer API ${path} returned ${res.status}`);
  }
  return res.json();
}

async function getExplorerBlock(number: number): Promise<{ block: { hash: string; number: number } } | null> {
  try {
    return (await explorerApi(`/blocks/${number}`)) as { block: { hash: string; number: number } };
  } catch {
    return null;
  }
}

async function getExplorerTransaction(hash: string): Promise<{ hash: string } | null> {
  try {
    return (await explorerApi(`/transactions/${hash}`)) as { hash: string };
  } catch {
    return null;
  }
}

test.describe('Reorg Handling', () => {
  test.describe.configure({ mode: 'serial', timeout: 120_000 });

  let snapshotBlock: number;
  let snapshotId: string;
  let preReorgTxHashes: string[] = [];
  let preReorgBlockHashes: Map<number, string> = new Map();

  test('setup: index initial blocks and take snapshot', async () => {
    const sender = ANVIL_ACCOUNTS[0].address;
    const recipient = ANVIL_ACCOUNTS[1].address;
    const value = BigInt('1000000000000000');

    const txHash = await sendETH(sender, recipient, value);
    await waitForReceipt(txHash);

    snapshotBlock = await getBlockNumber();
    await waitForIndexer(snapshotBlock);

    snapshotId = (await anvilRpc('evm_snapshot')) as string;
  });

  test('index post-snapshot transactions', async () => {
    const sender = ANVIL_ACCOUNTS[0].address;
    const recipient = ANVIL_ACCOUNTS[3].address;
    const value = BigInt('2000000000000000');

    for (let i = 0; i < 3; i++) {
      const txHash = await sendETH(sender, recipient, value);
      const receipt = await waitForReceipt(txHash);
      preReorgTxHashes.push(txHash);

      const blockNum = parseInt(receipt.blockNumber, 16);
      const blockData = await anvilRpc('eth_getBlockByNumber', [receipt.blockNumber, false]) as {
        hash: string;
      };
      preReorgBlockHashes.set(blockNum, blockData.hash);
    }

    const currentBlock = await getBlockNumber();
    await waitForIndexer(currentBlock);

    for (const txHash of preReorgTxHashes) {
      const tx = await getExplorerTransaction(txHash);
      expect(tx, `pre-reorg tx ${txHash} should exist in explorer`).not.toBeNull();
    }
  });

  test('revert to snapshot and mine alternate chain', async ({ page }) => {
    const success = (await anvilRpc('evm_revert', [snapshotId])) as boolean;
    expect(success, 'evm_revert should succeed').toBe(true);

    const blockAfterRevert = await getBlockNumber();
    expect(blockAfterRevert).toBeLessThanOrEqual(snapshotBlock);

    const sender = ANVIL_ACCOUNTS[0].address;
    const recipient = ANVIL_ACCOUNTS[4].address;
    const value = BigInt('3000000000000000');

    const newTxHashes: string[] = [];
    for (let i = 0; i < 3; i++) {
      const txHash = await sendETH(sender, recipient, value);
      await waitForReceipt(txHash);
      newTxHashes.push(txHash);
    }

    const newHead = await getBlockNumber();
    expect(newHead).toBeGreaterThan(snapshotBlock);

    for (const [blockNum, oldHash] of preReorgBlockHashes) {
      if (blockNum > snapshotBlock) {
        const newBlockData = await anvilRpc('eth_getBlockByNumber', [
          '0x' + blockNum.toString(16),
          false,
        ]) as { hash: string } | null;

        if (newBlockData) {
          expect(
            newBlockData.hash,
            `block ${blockNum} hash should differ after reorg`,
          ).not.toBe(oldHash);
        }
      }
    }

    const maxWaitMs = 30_000;
    const pollMs = 1_000;
    const start = Date.now();
    let reorgHandled = false;

    while (Date.now() - start < maxWaitMs) {
      let allGone = true;
      for (const txHash of preReorgTxHashes) {
        const tx = await getExplorerTransaction(txHash);
        if (tx !== null) {
          allGone = false;
          break;
        }
      }
      if (allGone) {
        reorgHandled = true;
        break;
      }
      await new Promise((r) => setTimeout(r, pollMs));
    }

    expect(reorgHandled, 'indexer should have removed reverted transactions').toBe(true);

    await waitForIndexer(newHead);

    for (const txHash of newTxHashes) {
      const tx = await getExplorerTransaction(txHash);
      expect(tx, `new post-reorg tx ${txHash} should be indexed`).not.toBeNull();
    }

    for (const [blockNum] of preReorgBlockHashes) {
      if (blockNum > snapshotBlock) {
        const block = await getExplorerBlock(blockNum);
        if (block) {
          const chainBlock = (await anvilRpc('eth_getBlockByNumber', [
            '0x' + blockNum.toString(16),
            false,
          ])) as { hash: string };
          expect(
            block.block.hash,
            `explorer block ${blockNum} should match new chain hash`,
          ).toBe(chainBlock.hash);
        }
      }
    }

    await page.goto('/transactions');
    await page.waitForLoadState('networkidle');

    const txContent = page.locator('text=/[Tt]ransaction/ >> visible=true');
    await expect(txContent.first()).toBeVisible({ timeout: 10_000 });

    const body = await page.locator('body').textContent();
    for (const txHash of preReorgTxHashes) {
      const shortHash = txHash.slice(0, 10);
      expect(body, `reverted tx ${shortHash} should not appear on page`).not.toContain(shortHash);
    }
  });
});
