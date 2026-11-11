import { useQuery } from '@tanstack/react-query';
import { getConfig } from '../lib/runtimeConfig';

export interface NetworkSetup { protocol: 'ops-v2'; source: 'approved-config'; chainId: string; chainIdDecimal: string; genesisHash: string; privacyProxyPublicUrl: string }
export async function networkSetup(): Promise<NetworkSetup> {
  const response = await fetch(getConfig('VITE_API_URL', '/api') + '/chain-info', { credentials: 'include', cache: 'no-store', redirect: 'error' });
  if (!response.ok) throw Error('Cannot load approved network setup');
  const v = await response.json() as NetworkSetup;
  if (!v || v.protocol !== 'ops-v2' || v.source !== 'approved-config' || typeof v.chainIdDecimal !== 'string' || !/^[1-9][0-9]{0,77}$/.test(v.chainIdDecimal) || BigInt(v.chainIdDecimal) >= (1n << 256n) || v.chainId !== '0x' + BigInt(v.chainIdDecimal).toString(16) || !/^0x[0-9a-f]{64}$/.test(v.genesisHash) || v.genesisHash === '0x' + '0'.repeat(64) || new URL(v.privacyProxyPublicUrl).protocol !== 'https:' || Object.keys(v).length !== 6) throw Error('Invalid network setup');
  return v;
}
export function PublicNetworkSetup() {
  const { data, error } = useQuery({ queryKey: ['approvedNetworkSetup'], queryFn: networkSetup, staleTime: Infinity, retry: false });
  return <section><h2>Approved network setup</h2><p>This describes the configured network. Live node status is available only through authenticated OPS reads.</p>{data ? <dl><dt>Chain ID</dt><dd>{data.chainIdDecimal} ({data.chainId})</dd><dt>Genesis hash</dt><dd className="break-all">{data.genesisHash}</dd><dt>OPS</dt><dd>{data.privacyProxyPublicUrl}</dd></dl> : <p>{error ? 'Network setup unavailable.' : 'Loading network setup…'}</p>}</section>;
}
