import { CopyButton } from './CopyButton';
import { useNativeName } from './NativeNames';
import { Link } from 'react-router-dom';
import { Lock } from 'lucide-react';
import { formatAddress } from '../lib/utils';
import { AddressLabel } from './AddressLabel';
import type { AddressMetadata } from '../lib/v2ReadTypes';

// Never look up a name for a masked address, or turn a privacy placeholder into
// a navigable account. Labels only use the metadata supplied for this row.
export function NativeAddress({ address, metadata, full = false, token = false, className = '' }: {
  address: string; metadata?: AddressMetadata; full?: boolean; token?: boolean; className?: string;
}) {
  const name = useNativeName(address);
  if (address === '0x' + '0'.repeat(40) || !/^0x[0-9a-f]{40}$/.test(address)) {
    return <span data-testid="private-address" data-redaction="private" className={`inline-flex items-center gap-1 text-neutral-400 italic ${className}`}><Lock className="w-3 h-3" />Private</span>;
  }
  const reason = metadata?.[address];
  const display = full ? address : formatAddress(address);
  if (reason === 'participant_override' || reason === 'no_access') return <span className={`font-mono text-neutral-500 ${className}`}>{display}</span>;
  return <span className="inline-flex items-center gap-1 flex-wrap"><Link className={`font-mono text-primary hover:text-primary-600 transition-colors ${full ? 'break-all' : ''} ${className}`} to={token ? `/token/${address}` : `/address/${address}`} title={address}>{name && <span className="font-sans font-medium mr-1">{name}</span>}{display}</Link><CopyButton text={address}/><AddressLabel reason={reason} /></span>;
}
