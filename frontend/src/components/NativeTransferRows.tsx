import { Link } from 'react-router-dom';
import { TransferMethodBadge } from './TransferMethodBadge';
import { TokenTypeBadge } from './TokenPresentation';
import { NativeAddress } from './NativeAddress';
import { EmptyRow } from './AddressPresentation';
import { ExplorerTable } from './ExplorerReadPresentation';
import { NativeNames, mayEnrich, nativeTokenPath } from './NativeNames';
import { useNativeMetadata } from '../hooks/useNativeMetadata';
import { decodeToken } from '../lib/nativeReadModels';
import { exactQuantity, exactUnits, exactTimestamp } from '../lib/nativePresentation';
import { formatHash } from '../lib/utils';
import type { Token, TokenTransfer } from '../lib/v2ReadTypes';
const tokenType = (value: string) => value.toUpperCase().replaceAll('-', '');
export function NativeTransferRows({items, token, currentAddress}: {items: TokenTransfer[]; token?: Token; currentAddress?: string}) {
  const metadata = useNativeMetadata(items.filter(t => mayEnrich(t.tokenAddress, t.addressMetadata) && t.tokenAddress !== token?.address).map(t => nativeTokenPath(t.tokenAddress)), decodeToken, 1);
  return <NativeNames items={items.flatMap(t => [t.from,t.to].map(address => ({address,metadata:t.addressMetadata})))}><ExplorerTable headings={['Transaction / date','Method / type','Block','From → To','Quantity / token ID','Token']}>{items.map(transfer => {
    const type = tokenType(transfer.tokenType), nft = type === 'ERC721' || type === 'ERC1155';
    const candidate = token?.address === transfer.tokenAddress ? token : metadata[nativeTokenPath(transfer.tokenAddress)];
    const permitted = mayEnrich(transfer.tokenAddress, transfer.addressMetadata);
    const known = permitted && candidate?.address === transfer.tokenAddress && tokenType(candidate.tokenType) === type ? candidate : undefined;
    const quantity = known && type === 'ERC20' ? `${exactUnits(transfer.value, known.decimals)}${transfer.value !== null ? ` ${known.symbol || 'token units'}` : ''}` : `${exactQuantity(transfer.value)}${transfer.value !== null && !nft ? ' base units' : ''}`;
    const direction = !currentAddress ? undefined : transfer.from === currentAddress && transfer.to === currentAddress ? 'SELF' : transfer.from === currentAddress ? 'OUT' : transfer.to === currentAddress ? 'IN' : undefined;
    return <tr key={transfer.id} data-testid="transfer-row"><td><Link title={transfer.txHash} className="font-mono text-primary" to={`/tx/${transfer.txHash}`}>{formatHash(transfer.txHash, 6)}</Link><div className="text-xs text-neutral-500 mt-1">{exactTimestamp(transfer.timestamp)}</div></td><td><div className="flex flex-wrap gap-1"><TransferMethodBadge method={transfer.transferType || 'transfer'}/><TokenTypeBadge type={transfer.tokenType}/>{direction && <span className="badge badge-neutral">{direction}</span>}</div></td><td><Link className="text-primary font-mono" to={`/block/${transfer.blockNumber}`}>{exactQuantity(transfer.blockNumber)}</Link></td><td><div className="flex items-center gap-2 flex-wrap"><NativeAddress address={transfer.from} metadata={transfer.addressMetadata}/><span className="text-neutral-400">→</span><NativeAddress address={transfer.to} metadata={transfer.addressMetadata}/></div></td><td><span className="font-mono text-success-600">{quantity}</span>{(nft || transfer.tokenId !== undefined) && <div className="text-xs text-neutral-500">Token ID: <span className="font-mono break-all">{exactQuantity(transfer.tokenId)}</span></div>}</td><td>{known && <div className="font-medium">{known.name || known.symbol}{known.name && known.symbol ? ` (${known.symbol})` : ''}</div>}<NativeAddress address={transfer.tokenAddress} metadata={transfer.addressMetadata} token/></td></tr>;
  })}</ExplorerTable>{!items.length && <EmptyRow>No token transfers on this visible page.</EmptyRow>}</NativeNames>;
}
