import { formatEther } from 'viem';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function formatAddress(address: string, chars = 6): string {
  if (!address.startsWith('0x')) return address; // don't truncate [PRIVATE] or other placeholders
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

export function formatHash(hash: string, chars = 10): string {
  return `${hash.slice(0, chars + 2)}...`;
}

export function formatTimestamp(timestamp: number): string {
  const date = new Date(timestamp * 1000);
  const ago = formatTimeAgo(timestamp);

  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  const month = months[date.getUTCMonth()];
  const day = String(date.getUTCDate()).padStart(2, '0');
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours();
  const minutes = String(date.getUTCMinutes()).padStart(2, '0');
  const seconds = String(date.getUTCSeconds()).padStart(2, '0');
  const ampm = hours >= 12 ? 'PM' : 'AM';
  const hours12 = hours % 12 || 12;

  return `${ago} | ${month} ${day} ${year} ${hours12}:${minutes}:${seconds} ${ampm} (+00:00 UTC)`;
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

export function didIdentifier(did: string): string {
  const lastColon = did.lastIndexOf(':');
  if (lastColon === -1) return did;
  return did.slice(lastColon + 1);
}

export function formatDID(did: string, chars = 6): string {
  const id = didIdentifier(did);
  if (id.length <= chars * 2 + 3) return id;
  return `${id.slice(0, chars)}...${id.slice(-chars)}`;
}
