import { Link } from 'react-router-dom';
import { Lock } from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip';
import { formatAddress } from '../lib/utils';
import { useContractName } from '../hooks/useContractName';
import type { AddressVisibility } from '../lib/api';

interface AddressLinkProps {
  address: string;
  chars?: number;
  full?: boolean;
  className?: string;
  label?: string; // Override label (won't fetch if provided)
  showLabel?: boolean; // Whether to auto-fetch and show contract name (default: true)
  visibility?: AddressVisibility; // Privacy visibility info
  reason?: import('../lib/api').VisibilityReason;
}

export function AddressLink({ address, chars = 6, full = false, className = '', label, showLabel = true, visibility, reason }: AddressLinkProps) {
  // Auto-fetch contract name if showLabel is true and no label is provided
  // Hook must be called unconditionally before any early returns
  const isPrivatePlaceholder = address === '[PRIVATE]' || address.startsWith('[PRIVATE');
  const shouldFetch = showLabel && !label && !isPrivatePlaceholder && !(visibility && !visibility.visible) && !(visibility?.level === 'pseudonymous' && visibility.pseudonym);
  const fetchedName = useContractName(shouldFetch ? address : null);
  const displayLabel = label || fetchedName;

  // Handle [PRIVATE] placeholder from backend redaction
  if (isPrivatePlaceholder) {
    return (
      <span className={`inline-flex items-center gap-1 text-neutral-400 italic ${className}`}>
        <Lock className="w-3 h-3" />
        <span>Private</span>
      </span>
    );
  }

  // Handle participant override from backend redactor (render 'Private Counterparty' with a disabled link instead of 'Public')
  if (reason === 'participant_override') {
    return (
      <span className={`inline-flex items-center gap-1 text-neutral-500 font-mono ${className}`}>
        {formatAddress(address, chars)}
      </span>
    );
  }

  // If visibility is provided and address is not fully visible, render privacy-aware display
  if (visibility && !visibility.visible) {
    if (visibility.level === 'redacted') {
      return <span className={`text-neutral-400 italic ${className}`}>[REDACTED]</span>;
    }
    return <span className={`text-neutral-400 italic ${className}`}>[PRIVATE]</span>;
  }

  if (visibility?.level === 'pseudonymous' && visibility.pseudonym) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={`font-mono text-amber-600 cursor-help ${className}`}>
            {visibility.pseudonym}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          <span className="text-xs">Pseudonymous address</span>
        </TooltipContent>
      </Tooltip>
    );
  }

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
  visibility?: AddressVisibility; // Privacy visibility info
  reason?: import('../lib/api').VisibilityReason;
}

export function TokenAddressLink({ address, chars = 4, className = '', label, showLabel = true, visibility, reason }: TokenAddressLinkProps) {
  // Hook must be called unconditionally before any early returns
  const isPrivatePlaceholder = address === '[PRIVATE]' || address.startsWith('[PRIVATE');
  const shouldFetch = showLabel && !label && !isPrivatePlaceholder && !(visibility && !visibility.visible) && !(visibility?.level === 'pseudonymous' && visibility.pseudonym);
  const fetchedName = useContractName(shouldFetch ? address : null);
  const displayLabel = label || fetchedName;

  // Handle [PRIVATE] placeholder from backend redaction
  if (isPrivatePlaceholder) {
    return (
      <span className={`inline-flex items-center gap-1 text-neutral-400 italic ${className}`}>
        <Lock className="w-3 h-3" />
        <span>Private</span>
      </span>
    );
  }

  // Handle participant override from backend redactor (render 'Private Counterparty' with a disabled link instead of 'Public')
  if (reason === 'participant_override') {
    return (
      <span className={`inline-flex items-center gap-1 text-neutral-500 font-mono ${className}`}>
        {formatAddress(address, chars)}
      </span>
    );
  }

  // If visibility is provided and address is not fully visible, render privacy-aware display
  if (visibility && !visibility.visible) {
    if (visibility.level === 'redacted') {
      return <span className={`text-neutral-400 italic ${className}`}>[REDACTED]</span>;
    }
    return <span className={`text-neutral-400 italic ${className}`}>[PRIVATE]</span>;
  }

  if (visibility?.level === 'pseudonymous' && visibility.pseudonym) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={`font-mono text-amber-600 cursor-help ${className}`}>
            {visibility.pseudonym}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          <span className="text-xs">Pseudonymous address</span>
        </TooltipContent>
      </Tooltip>
    );
  }
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
