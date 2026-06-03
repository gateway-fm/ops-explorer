import { Check } from 'lucide-react';
import { MetaMaskFox } from './MetaMask';
import { addNetworkToMetaMask } from '../lib/metamask';
import { useNetworkButton } from '../hooks/useNetworkButton';

interface AddNetworkButtonProps {
  /**
   * 'header' — compact pill used in the top nav.
   * 'footer' — wider button used in the page footer.
   */
  variant?: 'header' | 'footer';
}

const SIZE_CLASSES: Record<NonNullable<AddNetworkButtonProps['variant']>, string> = {
  header: 'gap-1.5 px-3 py-2 text-sm',
  footer: 'gap-2 px-4 py-2 text-sm',
};

const ACTIVE_CLASSES =
  'text-emerald-700 bg-emerald-50 border border-emerald-200 dark:text-emerald-300 dark:bg-emerald-900/30 dark:border-emerald-700 cursor-default';

const ADD_CLASSES =
  'text-amber-700 hover:text-amber-900 bg-amber-50 hover:bg-amber-100 border border-amber-200 dark:text-amber-300 dark:bg-amber-900/30 dark:hover:bg-amber-900/50 dark:border-amber-700';

/**
 * "Add Network to MetaMask" button that reflects the current wallet state
 * (RD-1030):
 *   - active  → disabled "Network added" with a check icon
 *   - else    → enabled "Add Network" / "Add Network to MetaMask"
 *
 * When no wallet is present the button still renders the add affordance; the
 * underlying handler prompts the user to install MetaMask. The add flow is
 * idempotent — re-adding an existing chain is a no-op in MetaMask — so we don't
 * need to detect added-but-inactive chains (which MetaMask doesn't expose).
 */
export function AddNetworkButton({ variant = 'footer' }: AddNetworkButtonProps) {
  const { state } = useNetworkButton();
  const isActive = state === 'active';
  const label = variant === 'header' ? 'Add Network' : 'Add Network to MetaMask';

  if (isActive) {
    return (
      <button
        type="button"
        disabled
        aria-disabled="true"
        title="This network is already added and active in MetaMask"
        className={`flex items-center rounded-lg font-medium transition-colors ${SIZE_CLASSES[variant]} ${ACTIVE_CLASSES}`}
      >
        <Check className="w-4 h-4" />
        Network added
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={() => {
        void addNetworkToMetaMask();
      }}
      className={`flex items-center rounded-lg font-medium transition-colors ${SIZE_CLASSES[variant]} ${ADD_CLASSES}`}
    >
      <MetaMaskFox className="w-4 h-4" />
      {label}
    </button>
  );
}
