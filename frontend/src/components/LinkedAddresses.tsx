import { useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import {
  ShieldCheck,
  Info,
  Wallet,
  Trash2,
  Loader2,
  AlertTriangle,
  Copy,
  Check,
  LinkIcon,
} from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { api, type LinkedAddress } from '../lib/api';
import { useLinkedAddresses, LINKED_ADDRESSES_QUERY_KEY } from '../hooks/useLinkedAddresses';
import { Tooltip, TooltipContent, TooltipTrigger } from './ui/tooltip';

// MetaMask provider interface for wallet interactions
interface MetaMaskProvider {
  request: (args: { method: string; params?: unknown[] }) => Promise<unknown>;
  isMetaMask?: boolean;
}

function getMetaMask(): MetaMaskProvider | null {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const eth = (window as any).ethereum;
  return eth && typeof eth.request === 'function' ? eth : null;
}

function truncateAddress(address: string): string {
  if (!address) return '';
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          onClick={handleCopy}
          className="p-1 rounded hover:bg-neutral-100 transition-colors"
        >
          {copied ? (
            <Check className="w-3.5 h-3.5 text-success-600" />
          ) : (
            <Copy className="w-3.5 h-3.5 text-neutral-400" />
          )}
        </button>
      </TooltipTrigger>
      <TooltipContent>{copied ? 'Copied!' : 'Copy address'}</TooltipContent>
    </Tooltip>
  );
}

function AddressRow({
  addr,
  onUnlink,
  unlinking,
}: {
  addr: LinkedAddress;
  onUnlink: (address: string) => void;
  unlinking: string | null;
}) {
  const isUser = addr.link_type === 'user';
  const isUnlinking = unlinking === addr.address.toLowerCase();

  return (
    <tr>
      <td>
        <div className="flex items-center gap-2">
          {isUser ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <ShieldCheck className="w-4 h-4 text-success-600 shrink-0" />
              </TooltipTrigger>
              <TooltipContent>Verified by wallet signature</TooltipContent>
            </Tooltip>
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                <Info className="w-4 h-4 text-neutral-400 shrink-0" />
              </TooltipTrigger>
              <TooltipContent>
                This address was automatically linked when you sent a transaction
                through the network. If you don&apos;t recognise it, contact your
                administrator.
              </TooltipContent>
            </Tooltip>
          )}
          <Link
            to={`/address/${addr.address}`}
            className="font-mono text-primary hover:text-primary-600 transition-colors"
          >
            {/* SECURITY: Address is rendered as text content, not dangerouslySetInnerHTML.
                React auto-escapes text content, preventing XSS. */}
            <span className="hidden sm:inline">{addr.address}</span>
            <span className="sm:hidden">{truncateAddress(addr.address)}</span>
          </Link>
          <CopyButton text={addr.address} />
          {addr.ens_name && (
            // SECURITY: ENS name rendered as text content (auto-escaped by React)
            <span className="text-xs text-neutral-500 bg-neutral-100 px-2 py-0.5 rounded">
              {addr.ens_name}
            </span>
          )}
        </div>
      </td>
      <td className="hidden md:table-cell">
        <span
          className={`badge ${
            isUser
              ? 'badge-primary'
              : 'bg-neutral-100 text-neutral-600 border-neutral-200'
          }`}
        >
          {isUser ? 'Verified' : 'From transactions'}
        </span>
      </td>
      <td className="hidden sm:table-cell">
        <span className="text-sm text-neutral-500">
          {new Date(addr.verified_at).toLocaleDateString()}
        </span>
      </td>
      <td className="text-right">
        {/* Only allow unlinking user-verified addresses. System-linked addresses
            are managed by the proxy and should not be user-removable. */}
        {isUser && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                onClick={() => onUnlink(addr.address)}
                disabled={isUnlinking}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-error-600 hover:bg-error-50 rounded-lg transition-colors disabled:opacity-50"
              >
                {isUnlinking ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Trash2 className="w-3.5 h-3.5" />
                )}
                Unlink
              </button>
            </TooltipTrigger>
            <TooltipContent>Remove this address from your account</TooltipContent>
          </Tooltip>
        )}
      </td>
    </tr>
  );
}

export function LinkedAddresses() {
  const { addresses, userAddresses, systemAddresses, isLoading, error, enabled } =
    useLinkedAddresses();
  const queryClient = useQueryClient();

  const [linking, setLinking] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);
  const [unlinking, setUnlinking] = useState<string | null>(null);
  const [confirmUnlink, setConfirmUnlink] = useState<string | null>(null);

  const handleLink = useCallback(async () => {
    setLinkError(null);
    setLinking(true);

    try {
      const ethereum = getMetaMask();
      if (!ethereum) {
        setLinkError('MetaMask is not installed. Please install MetaMask to link your wallet.');
        return;
      }

      // Step 1: Request accounts from MetaMask
      let accounts: string[];
      try {
        accounts = (await ethereum.request({
          method: 'eth_requestAccounts',
        })) as string[];
      } catch {
        // User rejected the connection request or MetaMask error
        setLinkError('Wallet connection was rejected. Please try again.');
        return;
      }

      if (!accounts || accounts.length === 0) {
        setLinkError('No accounts available in MetaMask.');
        return;
      }

      const signerAddress = accounts[0];

      // Step 2: Get challenge from server
      const challenge = await api.eth.createLinkChallenge();

      // Step 3: Sign the challenge message with MetaMask
      let signature: string;
      try {
        signature = (await ethereum.request({
          method: 'personal_sign',
          params: [challenge.message, signerAddress],
        })) as string;
      } catch {
        // User rejected the signing request
        setLinkError('Signature was rejected. Please try again.');
        return;
      }

      // SECURITY: Verify the address we send to the server matches what MetaMask
      // actually signed from. The signerAddress comes from eth_requestAccounts
      // (controlled by MetaMask), and it's the same address passed to personal_sign.
      // The server will independently recover the signer from the signature and verify
      // it matches the address in the request body. This prevents a malicious script
      // from substituting a different address after signing.
      await api.eth.verifyLink(challenge.nonce, signerAddress, signature);

      // Step 4: Invalidate cache to refetch
      await queryClient.invalidateQueries({ queryKey: LINKED_ADDRESSES_QUERY_KEY });
    } catch (err) {
      // Generic error from server communication
      setLinkError(
        err instanceof Error ? err.message : 'Failed to link wallet. Please try again.'
      );
    } finally {
      setLinking(false);
    }
  }, [queryClient]);

  const handleUnlink = useCallback(
    async (address: string) => {
      // Require confirmation
      if (confirmUnlink !== address) {
        setConfirmUnlink(address);
        return;
      }

      setConfirmUnlink(null);
      setUnlinking(address.toLowerCase());

      try {
        await api.eth.unlinkAddress(address);
        await queryClient.invalidateQueries({ queryKey: LINKED_ADDRESSES_QUERY_KEY });
      } catch (err) {
        setLinkError(
          err instanceof Error ? err.message : 'Failed to unlink address. Please try again.'
        );
      } finally {
        setUnlinking(null);
      }
    },
    [confirmUnlink, queryClient]
  );

  if (!enabled) return null;

  if (isLoading) {
    return (
      <div className="card p-6 flex items-center justify-center gap-2 text-neutral-400">
        <Loader2 className="w-5 h-5 animate-spin" />
        <span>Loading linked addresses...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="card p-6">
        <div className="flex items-center gap-3 text-error-600">
          <AlertTriangle className="w-5 h-5" />
          <span className="text-sm">
            {error instanceof Error ? error.message : 'Failed to load linked addresses.'}
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="card">
      <div className="px-4 py-3 border-b border-neutral-200 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Wallet className="w-4 h-4 text-neutral-500" />
          <h3 className="text-sm font-semibold text-neutral-900">
            Linked Addresses ({addresses.length})
          </h3>
        </div>
        <button
          onClick={handleLink}
          disabled={linking}
          className="btn-primary text-xs px-3 py-1.5 flex items-center gap-1.5"
        >
          {linking ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
          ) : (
            <LinkIcon className="w-3.5 h-3.5" />
          )}
          Link wallet
        </button>
      </div>

      {linkError && (
        <div className="px-4 py-2 bg-error-50 border-b border-error-200 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-error-600 shrink-0" />
          <span className="text-xs text-error-700">{linkError}</span>
          <button
            onClick={() => setLinkError(null)}
            className="ml-auto text-xs text-error-600 hover:text-error-800 font-medium"
          >
            Dismiss
          </button>
        </div>
      )}

      {confirmUnlink && (
        <div className="px-4 py-2 bg-warning-50 border-b border-warning-200 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-warning-600 shrink-0" />
          <span className="text-xs text-warning-700">
            Are you sure you want to unlink {truncateAddress(confirmUnlink)}?
          </span>
          <button
            onClick={() => handleUnlink(confirmUnlink)}
            className="ml-auto text-xs text-error-600 hover:text-error-800 font-medium"
          >
            Confirm
          </button>
          <button
            onClick={() => setConfirmUnlink(null)}
            className="text-xs text-neutral-600 hover:text-neutral-800 font-medium"
          >
            Cancel
          </button>
        </div>
      )}

      {addresses.length === 0 ? (
        <div className="empty-state py-8">
          <Wallet className="w-12 h-12 mx-auto mb-4 text-neutral-300" />
          <p className="text-neutral-500 text-sm">No linked addresses.</p>
          <p className="text-neutral-400 text-xs mt-1">
            Link your wallet to see your transactions filtered to your activity.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto">
          {userAddresses.length > 0 && (
            <>
              <div className="px-4 py-2 bg-neutral-50 border-b border-neutral-100">
                <span className="text-xs font-medium text-neutral-500 uppercase tracking-wider">
                  Verified addresses
                </span>
              </div>
              <table className="table">
                <thead>
                  <tr>
                    <th>Address</th>
                    <th className="hidden md:table-cell">Type</th>
                    <th className="hidden sm:table-cell">Linked</th>
                    <th className="text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {userAddresses.map((addr) => (
                    <AddressRow
                      key={addr.address}
                      addr={addr}
                      onUnlink={handleUnlink}
                      unlinking={unlinking}
                    />
                  ))}
                </tbody>
              </table>
            </>
          )}

          {systemAddresses.length > 0 && (
            <>
              <div className="px-4 py-2 bg-neutral-50 border-b border-neutral-100 flex items-center gap-2">
                <span className="text-xs font-medium text-neutral-500 uppercase tracking-wider">
                  Inferred from transactions
                </span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Info className="w-3.5 h-3.5 text-neutral-400" />
                  </TooltipTrigger>
                  <TooltipContent>
                    These addresses were automatically linked when you sent
                    transactions through the network.
                  </TooltipContent>
                </Tooltip>
              </div>
              <table className="table">
                <thead>
                  <tr>
                    <th>Address</th>
                    <th className="hidden md:table-cell">Type</th>
                    <th className="hidden sm:table-cell">Linked</th>
                    <th className="text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {systemAddresses.map((addr) => (
                    <AddressRow
                      key={addr.address}
                      addr={addr}
                      onUnlink={handleUnlink}
                      unlinking={unlinking}
                    />
                  ))}
                </tbody>
              </table>
            </>
          )}
        </div>
      )}
    </div>
  );
}
