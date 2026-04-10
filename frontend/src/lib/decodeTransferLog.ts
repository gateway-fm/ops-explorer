import type { SharedEventLog } from './api';
import { decodeAddress, decodeUint256 } from './eventDecoder';

const TRANSFER_TOPIC = '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef';

export interface DecodedTransfer {
  from: string;
  to: string;
  amount: string; // decimal string for formatTokenValue
  tokenAddress: string;
}

export function isTransferLog(log: SharedEventLog): boolean {
  return log.topic0?.toLowerCase() === TRANSFER_TOPIC.toLowerCase();
}

export function decodeTransferLog(log: SharedEventLog): DecodedTransfer | null {
  if (!isTransferLog(log) || !log.topic1 || !log.topic2) return null;

  return {
    from: decodeAddress(log.topic1),
    to: decodeAddress(log.topic2),
    amount: decodeUint256(log.data),
    tokenAddress: log.address.toLowerCase(),
  };
}
