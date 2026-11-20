import { useQuery } from '@tanstack/react-query';
import { getConfig } from '../lib/runtimeConfig';
import { PageHeader } from '../components/PageHeader';
import { ChainInfoRow as InfoRow } from '../components/ChainInfoRow';
import { CopyButton } from '../components/CopyButton';
import { StateMessage } from '../components/StateMessage';

export interface NetworkSetup { protocol: 'ops-v2'; source: 'approved-config'; chainId: string; chainIdDecimal: string; genesisHash: string; privacyProxyPublicUrl: string }
export async function networkSetup(): Promise<NetworkSetup> {
  const response = await fetch(getConfig('VITE_API_URL', '/api') + '/chain-info', { credentials: 'include', cache: 'no-store', redirect: 'error' });
  if (!response.ok) throw Error('Cannot load approved network setup');
  const v = await response.json() as NetworkSetup;
  if (!v || v.protocol !== 'ops-v2' || v.source !== 'approved-config' || typeof v.chainIdDecimal !== 'string' || !/^[1-9][0-9]{0,77}$/.test(v.chainIdDecimal) || BigInt(v.chainIdDecimal) >= (1n << 256n) || v.chainId !== '0x' + BigInt(v.chainIdDecimal).toString(16) || !/^0x[0-9a-f]{64}$/.test(v.genesisHash) || v.genesisHash === '0x' + '0'.repeat(64) || new URL(v.privacyProxyPublicUrl).protocol !== 'https:' || Object.keys(v).length !== 6) throw Error('Invalid network setup');
  return v;
}
export function PublicNetworkSetup() {
  const { data, error, refetch, isFetching } = useQuery({ queryKey: ['approvedNetworkSetup'], queryFn: networkSetup, staleTime: Infinity, retry: false });
  return <section className="space-y-6">
    <PageHeader title="Network setup" subtitle="Connection details from the approved network configuration" />
    <div className="card p-5 divide-y divide-neutral-100">
      {data ? <>
        <InfoRow label="Chain ID" value={<span className="font-mono">{data.chainIdDecimal} ({data.chainId}) <CopyButton text={data.chainIdDecimal}/></span>} />
        <InfoRow label="Genesis hash" value={<span className="font-mono text-sm break-all">{data.genesisHash} <CopyButton text={data.genesisHash}/></span>} />
        <InfoRow label="OPS endpoint" value={<span className="font-mono text-sm break-all">{data.privacyProxyPublicUrl} <CopyButton text={data.privacyProxyPublicUrl}/></span>} />
      </> : <StateMessage variant={error ? 'error' : 'loading'} title={error ? 'Network setup unavailable' : 'Loading network setup…'} action={error ? <button className="btn-secondary" disabled={isFetching} onClick={() => void refetch()}>Try again</button> : undefined} />}
    </div>
    <p className="text-sm text-neutral-500">These are connection details, not live node status. Sign in to inspect the network data available to you.</p>
  </section>;
}
