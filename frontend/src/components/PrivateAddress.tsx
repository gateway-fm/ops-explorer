import { Link } from 'react-router-dom';
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip';
import type { AddressVisibility, VisibilityReason } from '../lib/api';

interface PrivateAddressProps {
  /** The actual address */
  address: string;
  /** Optional visibility info if already fetched */
  visibility?: AddressVisibility | null;
  /** Show compact version for tables/lists */
  compact?: boolean;
  /** Make it clickable (link to address page) */
  clickable?: boolean;
  /** Custom class name */
  className?: string;
}

/**
 * PrivateAddress component displays a placeholder for addresses
 * that the current user doesn't have permission to view.
 */
export function PrivateAddress({
  address,
  visibility,
  compact = true,
  clickable = true,
  className = ''
}: PrivateAddressProps) {
  const isPrivate = !visibility || !visibility.visible;

  if (!isPrivate) {
    // Address is visible - show normal display
    const displayName = visibility?.level === 'pseudonymous' && visibility?.pseudonym
      ? visibility.pseudonym
      : address;

    if (clickable) {
      return (
        <Link
          to={`/address/${address}`}
          className={`font-mono text-primary hover:text-primary-600 transition-colors ${className}`}
        >
          {compact ? `${address.slice(0, 6)}...${address.slice(-4)}` : displayName}
        </Link>
      );
    }

    return (
      <span className={`font-mono ${className}`}>
        {compact ? `${address.slice(0, 6)}...${address.slice(-4)}` : displayName}
      </span>
    );
  }

  // Address is private
  if (compact) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span
            data-testid="private-address"
            data-redaction="private"
            className={`text-neutral-400 italic cursor-help ${className}`}
          >
            [PRIVATE]
          </span>
        </TooltipTrigger>
        <TooltipContent>
          <p className="text-sm">This address is private.</p>
          <p className="text-xs text-neutral-400">You need a disclosure grant to view it.</p>
        </TooltipContent>
      </Tooltip>
    );
  }

  // Full private address card
  return (
    <div className={`text-center p-8 bg-neutral-50 rounded-lg ${className}`}>
      <div className="text-neutral-400 mb-4">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="48"
          height="48"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="mx-auto"
        >
          <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
          <path d="M7 11V7a5 5 0 0 1 10 0v4" />
        </svg>
      </div>

      <h2 className="text-xl font-semibold mb-2">Private Address</h2>

      <p className="text-neutral-600 mb-6">
        This address belongs to a user who has not granted you access to view their information.
      </p>

      <div className="bg-white border border-neutral-200 rounded-lg p-4 text-left">
        <h3 className="font-medium mb-2">Why is this address private?</h3>
        <p className="text-sm text-neutral-600 mb-3">
          Privacy-preserving blockchain explorers allow users to control who can view
          their on-chain activity.
        </p>
        <h4 className="font-medium text-sm mb-1">How to get access:</h4>
        <ul className="text-sm text-neutral-600 list-disc list-inside space-y-1">
          <li>Request disclosure access from the address owner</li>
          <li>Submit a formal disclosure request for compliance needs</li>
          <li>If this is your address, sign in with Privado ID</li>
        </ul>
      </div>
    </div>
  );
}

/**
 * PrivateAddressInline is a minimal inline version for use in text
 */
export function PrivateAddressInline({ className = '' }: { className?: string }) {
  return <span data-testid="private-address" data-redaction="private" className={`text-neutral-400 italic ${className}`}>[PRIVATE]</span>;
}

/**
 * Helper to get a display label for visibility reason
 */
// eslint-disable-next-line react-refresh/only-export-components
export function getVisibilityReasonLabel(reason: VisibilityReason): string {
  switch (reason) {
    case 'own_address':
      return 'Your address';
    case 'disclosure_grant':
      return 'Disclosed to you';
    case 'public_address':
      return 'Public';
    case 'no_access':
      return 'Private';
    default:
      return 'Unknown';
  }
}

/**
 * VisibilityBadge shows the visibility status of an address
 */
export function VisibilityBadge({ visibility, className = '' }: { visibility: AddressVisibility; className?: string }) {
  const baseClasses = 'text-xs px-2 py-0.5 rounded-full font-medium';

  if (!visibility.visible) {
    return (
      <span className={`${baseClasses} bg-neutral-100 text-neutral-600 ${className}`}>
        Private
      </span>
    );
  }

  switch (visibility.reason) {
    case 'own_address':
      return (
        <span className={`${baseClasses} bg-primary-100 text-primary-700 ${className}`}>
          Your Address
        </span>
      );
    case 'disclosure_grant':
      return (
        <span className={`${baseClasses} bg-success-100 text-success-700 ${className}`}>
          Disclosed
        </span>
      );
    case 'public_address':
      return (
        <span className={`${baseClasses} bg-neutral-100 text-neutral-600 ${className}`}>
          Public
        </span>
      );
    default:
      return null;
  }
}
