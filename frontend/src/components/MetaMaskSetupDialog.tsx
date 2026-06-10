import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, ChevronDown, Copy, ExternalLink } from 'lucide-react';
import { api } from '../lib/api';
import { getConfig } from '../lib/runtimeConfig';
import { getShortName } from '../lib/branding';
import { getNetworkCurrency } from '../lib/utils';
import { addNetworkViaHelper, resolveChainIdHex } from '../lib/metamask';
import { Dialog } from './ui/dialog';
import { MetaMaskFox } from './MetaMask';

const INJECTOR_REPO_URL = 'https://github.com/gateway-fm/jwt-injector';
const DEFAULT_HELPER_URL = 'http://127.0.0.1:9001';

interface MetaMaskSetupDialogProps {
  open: boolean;
  onClose: () => void;
}

function CopyButton({ value, label = 'Copy' }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      onClick={() => {
        navigator.clipboard?.writeText(value);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }}
      className="inline-flex shrink-0 items-center gap-1 rounded-md border border-neutral-200 bg-white px-2 py-1 text-xs font-medium text-neutral-600 transition-colors hover:bg-neutral-100 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 dark:hover:bg-neutral-700"
      aria-label={label}
    >
      {copied ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
      {copied ? 'Copied' : label}
    </button>
  );
}

/**
 * Privacy-mode "Connect MetaMask" setup dialog (Option B for RD-1031).
 *
 * A privacy-enabled network cannot be reached directly by a browser wallet:
 * every chain request needs an authenticated bearer token + the caller's
 * organization path, which MetaMask cannot attach. So instead of silently
 * pushing the anonymous proxy /rpc endpoint to the wallet (which clears the
 * "Unable to connect" banner but yields a non-functional wallet), we walk the
 * user through running the local jwt-injector helper and point MetaMask at it.
 */
export function MetaMaskSetupDialog({ open, onClose }: MetaMaskSetupDialogProps) {
  const { data: chainInfo } = useQuery({
    queryKey: ['chainInfo'],
    queryFn: api.getChainInfo,
    staleTime: 60000,
  });

  const [helperUrl, setHelperUrl] = useState(DEFAULT_HELPER_URL);
  const [showManual, setShowManual] = useState(false);
  const [adding, setAdding] = useState(false);

  const networkName = getConfig('VITE_NETWORK_NAME') || getShortName();
  const currency = getNetworkCurrency();
  const chainIdHex = resolveChainIdHex(chainInfo);
  const chainIdDecimal = chainInfo?.chainIdDecimal
    ? String(chainInfo.chainIdDecimal)
    : chainIdHex
      ? String(parseInt(chainIdHex, 16))
      : getConfig('VITE_CHAIN_ID', '');

  // Pre-fill the injector --upstream hint from the proxy public base URL when
  // the backend surfaces it; otherwise use a placeholder. We never embed org
  // ids or tokens — those stay user-supplied placeholders.
  const upstream = chainInfo?.privacyProxyPublicUrl?.trim() || '<PROXY_URL>';

  const dockerCommand = [
    'docker build -t jwt-injector .',
    '',
    '# Put your JWT in a file (keep it out of shell history / argv):',
    'mkdir -p secrets && printf \'%s\' "$YOUR_JWT" > secrets/token.jwt',
    '',
    'docker run -d --name jwt-injector \\',
    '  -p 127.0.0.1:9001:9001 \\',
    '  -v "$PWD/secrets:/tokens:ro" \\',
    '  -e JWT_INJECTOR_BIND=0.0.0.0 \\',
    '  -e JWT_INJECTOR_JWT_FILE=/tokens/token.jwt \\',
    `  -e JWT_INJECTOR_UPSTREAM=${upstream} \\`,
    '  -e JWT_INJECTOR_ORG_ID=<YOUR_ORG_ID> \\',
    '  -e JWT_INJECTOR_CORS_ALLOW_ORIGIN=<EXPLORER_ORIGIN> \\',
    '  jwt-injector',
  ].join('\n');

  async function handleAdd() {
    setAdding(true);
    try {
      await addNetworkViaHelper(helperUrl, chainInfo);
      onClose();
    } catch {
      // MetaMask surfaces its own error (e.g. injector not running / rejected
      // request); nothing actionable to add here.
    } finally {
      setAdding(false);
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title={`Connect MetaMask to ${networkName}`}>
      <div className="space-y-5 text-sm text-neutral-700 dark:text-neutral-300">
        <p className="leading-relaxed">
          {networkName} is a privacy-enabled network. MetaMask can&apos;t connect to it
          directly because every request needs your authenticated session and
          organization, which a browser wallet can&apos;t attach. To use your wallet,
          run a small local helper — <span className="font-medium">jwt-injector</span> —
          that holds your token and forwards requests on your behalf.
        </p>

        <a
          href={INJECTOR_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 rounded-lg border border-primary-200 bg-primary-50 px-3 py-2 text-sm font-medium text-primary transition-colors hover:bg-primary-100 dark:border-primary-700 dark:bg-primary-900/30 dark:text-primary-300 dark:hover:bg-primary-900/50"
        >
          <ExternalLink className="h-4 w-4" />
          gateway-fm/jwt-injector
        </a>

        {/* Step 1 — run the helper */}
        <div className="space-y-2">
          <h3 className="font-semibold text-neutral-900 dark:text-neutral-100">
            1. Run the helper locally
          </h3>
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            Build and run jwt-injector. Replace the placeholders with your own
            values; the token stays on your machine.
          </p>
          <div className="relative">
            <pre className="overflow-x-auto rounded-lg border border-neutral-200 bg-neutral-50 p-3 text-xs leading-relaxed text-neutral-800 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-200">
              <code>{dockerCommand}</code>
            </pre>
            <div className="absolute right-2 top-2">
              <CopyButton value={dockerCommand} />
            </div>
          </div>
        </div>

        {/* Step 2 — helper URL */}
        <div className="space-y-2">
          <h3 className="font-semibold text-neutral-900 dark:text-neutral-100">
            2. Helper URL
          </h3>
          <p className="text-xs text-neutral-500 dark:text-neutral-400">
            The local address MetaMask will talk to. Change this only if you ran
            the helper on a custom port.
          </p>
          <input
            type="text"
            value={helperUrl}
            onChange={(e) => setHelperUrl(e.target.value)}
            spellCheck={false}
            className="w-full rounded-md border border-neutral-200 px-3 py-2 text-sm focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/50 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
          />
        </div>

        {/* Primary action */}
        <button
          type="button"
          onClick={handleAdd}
          disabled={adding || !chainIdHex}
          className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <MetaMaskFox className="h-4 w-4" />
          {adding ? 'Opening MetaMask…' : 'Add to MetaMask'}
        </button>
        <p className="text-xs text-neutral-400 dark:text-neutral-500">
          If the helper isn&apos;t running yet, MetaMask will show a connection error
          — start it (step 1) and try again.
        </p>

        {/* Manual entry */}
        <div className="border-t border-neutral-100 pt-3 dark:border-neutral-800">
          <button
            type="button"
            onClick={() => setShowManual((v) => !v)}
            className="flex items-center gap-1.5 text-sm font-medium text-neutral-600 hover:text-neutral-900 dark:text-neutral-300 dark:hover:text-neutral-100"
          >
            <ChevronDown
              className={`h-4 w-4 transition-transform ${showManual ? 'rotate-180' : ''}`}
            />
            Prefer manual entry?
          </button>
          {showManual && (
            <div className="mt-3 space-y-2 rounded-lg border border-neutral-200 bg-neutral-50 p-3 text-xs dark:border-neutral-700 dark:bg-neutral-800">
              <p className="text-neutral-500 dark:text-neutral-400">
                In MetaMask, open <span className="font-medium">Add a network manually</span>{' '}
                and enter:
              </p>
              <ManualRow label="Network name" value={networkName} />
              <ManualRow label="New RPC URL" value={helperUrl} />
              <ManualRow label="Chain ID" value={chainIdDecimal || '—'} />
              <ManualRow label="Currency symbol" value={currency} />
            </div>
          )}
        </div>
      </div>
    </Dialog>
  );
}

function ManualRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-neutral-500 dark:text-neutral-400">{label}</span>
      <span className="flex items-center gap-2">
        <span className="font-mono text-neutral-800 dark:text-neutral-200">{value}</span>
        <CopyButton value={value} label="" />
      </span>
    </div>
  );
}
