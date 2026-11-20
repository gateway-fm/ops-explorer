import { Link } from 'react-router-dom';
import { CopyButton } from './CopyButton';

export function LinkedWalletAddress({address}: {address: string}) {
  return <><Link to={`/address/${address}`} className="min-w-0 font-mono text-primary hover:text-primary-600 transition-colors" title={address}><span className="hidden break-all sm:inline">{address}</span><span className="sm:hidden">{address.slice(0,6)}...{address.slice(-4)}</span></Link><CopyButton text={address}/></>;
}
