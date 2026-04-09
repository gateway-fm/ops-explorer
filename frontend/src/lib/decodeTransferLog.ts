import type { SharedEventLog } from './api';

const TRANSFER_TOPIC = '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef';

export interface DecodedTransfer {
  from: string;
  to: string;
  amount: string; // raw hex for formatTokenValue
  tokenAddress: string;
  blockNumber: number;
  logIndex: number;
}

export function isTransferLog(log: SharedEventLog): boolean {
  return log.topic0?.toLowerCase() === TRANSFER_TOPIC.toLowerCase();
}

export function decodeTransferLog(log: SharedEventLog): DecodedTransfer | null {
  if (!isTransferLog(log) || !log.topic1 || !log.topic2) return null;

  const from = '0x' + log.topic1.slice(-40);
  const to = '0x' + log.topic2.slice(-40);

  // data is the uint256 amount as hex
  let amount = '0';
  if (log.data && log.data !== '0x' && log.data.length > 2) {
    amount = log.data;
  }

  return {
    from: from.toLowerCase(),
    to: to.toLowerCase(),
    amount,
    tokenAddress: log.address.toLowerCase(),
    blockNumber: log.blockNumber,
    logIndex: log.logIndex,
  };
}
