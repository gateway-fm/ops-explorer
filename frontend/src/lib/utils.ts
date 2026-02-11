import { formatEther } from 'viem';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function formatAddress(address: string, chars = 6): string {
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

export function formatHash(hash: string, chars = 8): string {
  return `${hash.slice(0, chars + 2)}...${hash.slice(-chars)}`;
}

export function formatTimestamp(timestamp: number): string {
  return new Date(timestamp * 1000).toLocaleString();
}

export function formatTimeAgo(timestamp: number): string {
  const seconds = Math.floor(Date.now() / 1000 - timestamp);
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

export function formatWei(wei: string | number): string {
  try {
    // Handle both string and number values from API
    const value = typeof wei === 'string' ? BigInt(wei) : BigInt(Math.floor(wei));
    return formatEther(value);
  } catch {
    return '0';
  }
}

export function formatGas(gas: number): string {
  if (gas >= 1_000_000) return `${(gas / 1_000_000).toFixed(2)}M`;
  if (gas >= 1_000) return `${(gas / 1_000).toFixed(1)}K`;
  return String(gas);
}

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
