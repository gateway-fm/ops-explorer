import { createContext, useContext, type ReactNode } from 'react';
import { useNativeMetadata } from '../hooks/useNativeMetadata';
import { decodeContract, decodeToken } from '../lib/nativeReadModels';
import type { AddressMetadata } from '../lib/v2ReadTypes';
export const nativeTokenPath = (address: string) => `/v2/explorer/tokens/${address}`;
export function mayEnrich(address: string, metadata?: AddressMetadata): boolean {
  return /^0x[0-9a-f]{40}$/.test(address) && address !== '0x' + '0'.repeat(40) && !['no_access', 'participant_override'].includes(metadata?.[address] ?? '');
}
function decodeName(raw: unknown, path?: string) {
  if (path?.startsWith('/v2/explorer/tokens/')) { const value = decodeToken(raw); return value ? {address: value.address, name: value.name || value.symbol} : undefined; }
  const value = decodeContract(raw); return value ? {address: value.address, name: value.contractName} : undefined;
}
const Names = createContext<Record<string, string>>({});
export const useNativeName = (address: string) => useContext(Names)[address];
export function NativeNames({items, children}: {items: {address: string; metadata?: AddressMetadata}[]; children: ReactNode}) {
  const addresses = [...new Set(items.filter(item => mayEnrich(item.address, item.metadata)).map(item => item.address))];
  // Deterministic interleaving leaves some room for both token and contract names.
  const paths = addresses.flatMap(address => [nativeTokenPath(address), `/v2/explorer/addresses/${address}/contract`]);
  const records = useNativeMetadata(paths, decodeName), inherited = useContext(Names), names = {...inherited};
  for (const address of addresses) for (const path of [nativeTokenPath(address), `/v2/explorer/addresses/${address}/contract`]) {
    const record = records[path]; if (record?.address === address && record.name) { names[address] = record.name; break; }
  }
  return <Names.Provider value={names}>{children}</Names.Provider>;
}
