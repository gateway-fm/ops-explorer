import { useQuery } from '@tanstack/react-query';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import { disclosureQueryOptions } from '../lib/disclosureQuery';
import { useAuth } from '../lib/auth';
import { useImpersonation } from '../hooks/useImpersonation';
import { PageHeader } from '../components/PageHeader';
import { StateMessage } from '../components/StateMessage';
import { formatTimestamp, formatWei, getNetworkCurrency } from '../lib/utils';
import { redirectToLogin } from '../lib/login';

export function GrantedTransactionDetail() {
  const { grantId, addressId, hash } = useParams<{ grantId: string; addressId: string; hash: string }>();
  const { isAuthenticated, isLoading: authLoading, auth } = useAuth();
  const { isActive } = useImpersonation();
  const [searchParams] = useSearchParams();
  const viewingAs = isActive || searchParams.has('as');
  const grantPath = `/grant/${encodeURIComponent(grantId ?? '')}/${encodeURIComponent(addressId ?? '')}`;
  const enabled = !authLoading && isAuthenticated && !viewingAs && !!grantId && !!addressId && !!hash;
  const { data, fetchStatus, isPending, error } = useQuery({
    ...disclosureQueryOptions,
    queryKey: ['grantedTransaction', auth.did, grantId, addressId, hash, enabled],
    queryFn: ({ signal }) => api.getGrantedTransaction(grantId!, addressId!, hash!, signal),
    enabled,
  });

  // A background or offline-paused check must not expose the previous Full
  // response. View-as changes the key so leaving it cannot reuse an old request.
  if (authLoading || (enabled && (fetchStatus !== 'idle' || isPending))) return <StateMessage variant="loading" />;
  if (!isAuthenticated) return (
    <div className="space-y-4">
      <StateMessage variant="empty" title="Sign in to view disclosed transaction details" />
      <button className="btn-primary" onClick={() => redirectToLogin(`${grantPath}/tx/${encodeURIComponent(hash ?? '')}`)}>Sign In</button>
    </div>
  );
  if (viewingAs || error || !data) return (
    <div className="space-y-4">
      <StateMessage variant="empty" title="Transaction details not available" detail="This transaction may be outside your disclosure grant, or this service may not support disclosed transaction details yet." />
      <Link className="text-primary" to={grantPath}>Back to disclosed transactions</Link>
    </div>
  );

  const fields: [string, string | number | undefined][] = [
    ['Transaction hash', data.tx_hash], ['Status', data.status === 1 ? 'Success' : 'Failed'],
    ['Block', data.block_number], ['Transaction index', data.tx_index],
    ['Timestamp', data.block_timestamp ? formatTimestamp(data.block_timestamp) : undefined],
    ['From', data.from], ['To', data.to ?? 'Contract creation'], ['Contract address', data.contract_address],
    ['Value', `${formatWei(data.value)} ${getNetworkCurrency()}`], ['Gas used', data.gas_used.toLocaleString()],
    ['Gas price', String(data.gas_price)], ['Gas limit', data.gas_limit?.toLocaleString()],
    ['Max fee per gas', data.max_fee_per_gas], ['Max priority fee per gas', data.max_priority_fee_per_gas],
    ['Nonce', data.nonce], ['Type', data.tx_type], ['Input data', data.input_data],
    ['Error', data.error], ['Revert reason', data.revert_reason],
  ];
  return (
    <div className="space-y-6">
      <PageHeader title="Transaction"><span className="badge bg-amber-100 text-amber-800">Disclosed</span></PageHeader>
      <Link className="text-primary" to={grantPath}>Back to disclosed transactions</Link>
      <dl className="card divide-y divide-neutral-100">
        {fields.filter(([, value]) => value !== undefined).map(([label, value]) => (
          <div key={label} className="p-4 sm:flex gap-4"><dt className="sm:w-40 shrink-0 text-neutral-500">{label}</dt><dd className="font-mono text-sm break-all">{value}</dd></div>
        ))}
      </dl>
    </div>
  );
}
