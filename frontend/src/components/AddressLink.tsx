import { Link } from 'react-router-dom';
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip';
import { formatAddress } from '../lib/utils';
import { useContractName } from '../hooks/useContractName';

interface AddressLinkProps {
  address: string;
  chars?: number;
  full?: boolean;
  className?: string;
  label?: string; // Override label (won't fetch if provided)
  showLabel?: boolean; // Whether to auto-fetch and show contract name (default: true)
}

export function AddressLink({ address, chars = 6, full = false, className = '', label, showLabel = true }: AddressLinkProps) {
  // Auto-fetch contract name if showLabel is true and no label is provided
  const fetchedName = useContractName(showLabel && !label ? address : null);
  const displayLabel = label || fetchedName;

  const displayAddress = full ? address : formatAddress(address, chars);
  const baseClassName = "font-mono text-primary hover:text-primary-600 transition-colors";

  // If label is available, show: address (Label)
  const displayContent = displayLabel ? (
    <span className="inline-flex items-center gap-1 flex-wrap">
      <span>{displayAddress}</span>
      <span className="text-primary-600 font-sans text-xs">({displayLabel})</span>
    </span>
  ) : displayAddress;

  if (full) {
    return (
      <Link
        to={`/address/${address}`}
        className={`${baseClassName} ${className}`}
      >
        {displayContent}
      </Link>
    );
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          to={`/address/${address}`}
          className={`${baseClassName} ${className}`}
        >
          {displayContent}
        </Link>
      </TooltipTrigger>
      <TooltipContent>
        <span className="font-mono">{address}</span>
        {displayLabel && <span className="text-neutral-400 ml-2">({displayLabel})</span>}
      </TooltipContent>
    </Tooltip>
  );
}

interface TokenAddressLinkProps {
  address: string;
  chars?: number;
  className?: string;
  label?: string;
  showLabel?: boolean;
}

export function TokenAddressLink({ address, chars = 4, className = '', label, showLabel = true }: TokenAddressLinkProps) {
  const fetchedName = useContractName(showLabel && !label ? address : null);
  const displayLabel = label || fetchedName;
  const displayAddress = formatAddress(address, chars);
  const baseClassName = "font-mono text-warning-600 hover:text-warning-700 transition-colors";

  const displayContent = displayLabel ? (
    <span className="inline-flex items-center gap-1 flex-wrap">
      <span>{displayAddress}</span>
      <span className="text-primary-600 font-sans text-xs">({displayLabel})</span>
    </span>
  ) : displayAddress;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          to={`/address/${address}`}
          className={`${baseClassName} ${className}`}
        >
          {displayContent}
        </Link>
      </TooltipTrigger>
      <TooltipContent>
        <span className="font-mono">{address}</span>
        {displayLabel && <span className="text-neutral-400 ml-2">({displayLabel})</span>}
      </TooltipContent>
    </Tooltip>
  );
}
