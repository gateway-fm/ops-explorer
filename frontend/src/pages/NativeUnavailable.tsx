import { Link } from 'react-router-dom';
import { StateMessage } from '../components/StateMessage';
import { PageHeader } from '../components/PageHeader';

const explanations = {
  verify: {title: 'Contract verification unavailable', detail: 'Contract ABI and source metadata come from the approved registry on this network. Explorer cannot upload or verify arbitrary contract metadata.', to: '/tokens', label: 'Browse tokens'},
  gas: {title: 'Gas tracker unavailable', detail: 'This network does not expose a global gas tracker. Authorized transaction details show the returned gas and fee values.', to: '/transactions', label: 'Browse transactions'},
  stats: {title: 'Global charts unavailable', detail: 'Only counts observed within an authorized page are available. Complete network totals and historical charts are not supplied.', to: '/', label: 'View observed statistics'},
  nft: {title: 'NFT item metadata unavailable', detail: 'This network has no approved native source for NFT inventories, images or item metadata. Authorized token transfers retain their exact NFT IDs.', to: '/token-transfers', label: 'Browse token transfers'},
  privacy: {title: 'Use Auditor for disclosure cases', detail: 'Disclosures use approved cases with their own recipients, scope and expiry. Open Auditor to review the cases available to your current identity.', to: '/disclosures', label: 'Open Auditor'},
  grant: {title: 'Legacy disclosure link unavailable', detail: 'Consent-era grant links do not authorize a disclosure case. Open Auditor to review the approved cases available to your current identity.', to: '/disclosures', label: 'Open Auditor'},
  impersonation: {title: 'View as user unavailable', detail: 'Explorer uses your current application identity. To use a different account, choose Switch account from the account menu.', to: '/', label: 'Return to Explorer'},
};
export function NativeUnavailable({feature}: {feature: keyof typeof explanations}) {
  const entry = explanations[feature];
  return <section className="space-y-6"><PageHeader title={entry.title}/><StateMessage variant="restricted" title={entry.title} detail={entry.detail} className="rounded-2xl border border-neutral-200 bg-neutral-50 py-16" action={<Link className="btn-secondary" to={entry.to}>{entry.label}</Link>}/></section>;
}
