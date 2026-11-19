import { Link } from 'react-router-dom';
import { TransferMethodBadge } from './TransferMethodBadge';
import { TokenTypeBadge } from './TokenPresentation';
import { NativeAddress } from './NativeAddress';
import { EmptyRow } from './AddressPresentation';
import { useNativeMetadata } from '../hooks/useNativeMetadata';
import { decodeToken } from '../lib/nativeReadModels';
import { exactQuantity, exactUnits } from '../lib/nativePresentation';
import { formatHash } from '../lib/utils';
import type { Token, TokenTransfer } from '../lib/v2ReadTypes';

const zeroAddress = '0x' + '0'.repeat(40);
const tokenPath = (address: string) => `/v2/explorer/tokens/${address}`;
function tokenType(value: string) { return value.toUpperCase().replaceAll('-', ''); }
export function NativeTransferRows({items, token}: {items: TokenTransfer[]; token?: Token}) {
  const metadata = useNativeMetadata(items.filter(t => tokenType(t.tokenType) === 'ERC20' && t.tokenAddress !== zeroAddress && t.tokenAddress !== token?.address && !['no_access', 'participant_override'].includes(t.addressMetadata?.[t.tokenAddress] ?? '')).map(t => tokenPath(t.tokenAddress)), decodeToken);
  return <div className="space-y-3">{items.length ? items.map(transfer => {
    const type = tokenType(transfer.tokenType), nft = type === 'ERC721' || type === 'ERC1155';
    const candidate = token?.address === transfer.tokenAddress ? token : metadata[tokenPath(transfer.tokenAddress)];
    const known = candidate?.address === transfer.tokenAddress && tokenType(candidate.tokenType) === 'ERC20' && type === 'ERC20' ? candidate : undefined;
    const quantity = known ? `${exactUnits(transfer.value, known.decimals)}${transfer.value !== null ? ` ${known.symbol || 'token units'}` : ''}` : `${exactQuantity(transfer.value)}${transfer.value !== null && !nft ? ' base units' : ''}`;
    return <div key={transfer.id} data-testid="transfer-row" className="flex items-center gap-1.5 flex-wrap text-sm"><TransferMethodBadge method={transfer.transferType || 'transfer'}/><TokenTypeBadge type={transfer.tokenType}/><span className="text-neutral-500">From</span><NativeAddress address={transfer.from} metadata={transfer.addressMetadata}/><span className="text-neutral-500">To</span><NativeAddress address={transfer.to} metadata={transfer.addressMetadata}/><span className="text-neutral-500">{nft ? 'Quantity' : 'For'}</span><span className="font-mono text-success-600 font-medium">{quantity}</span>{(nft || transfer.tokenId !== undefined) && <span className="text-xs text-neutral-700">Token ID: <span className="font-mono break-all">{exactQuantity(transfer.tokenId)}</span></span>}<NativeAddress address={transfer.tokenAddress} metadata={transfer.addressMetadata} token/><Link title={transfer.txHash} className="font-mono text-primary" to={`/tx/${transfer.txHash}`}>{formatHash(transfer.txHash, 6)}</Link></div>;
  }) : <EmptyRow>No token transfers on this visible page.</EmptyRow>}</div>;
}
