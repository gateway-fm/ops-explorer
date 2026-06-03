import { getShortName } from './branding';
import { getConfig } from './runtimeConfig';
import { getNetworkCurrency } from './utils';
import { api, type ChainInfo } from './api';

// AddEthereumChainParameter is the shape MetaMask's wallet_addEthereumChain
// expects (EIP-3085). We keep it local rather than pulling a wallet SDK.
export interface AddEthereumChainParameter {
  chainId: string; // 0x-prefixed hex
  chainName: string;
  nativeCurrency: { name: string; symbol: string; decimals: number };
  rpcUrls: string[];
}

// Minimal EIP-1193 provider surface we rely on. window.ethereum is typed as
// Eip1193Provider elsewhere; that type lacks the event-emitter methods we use
// for live chain-state detection (RD-1030), so we narrow through this.
interface EthereumProvider {
  request: (args: { method: string; params?: unknown[] }) => Promise<unknown>;
  on?: (event: string, handler: (...args: unknown[]) => void) => void;
  removeListener?: (event: string, handler: (...args: unknown[]) => void) => void;
}

function getProvider(): EthereumProvider | undefined {
  return (window as unknown as { ethereum?: EthereumProvider }).ethereum;
}

/**
 * Resolve the canonical browser-facing JSON-RPC URL that wallets should
 * connect to. Priority (RD-1031):
 *   1. backend-provided rpcUrl from /chain-info (privacy-proxy public /rpc)
 *   2. VITE_RPC_URL build/runtime config
 *   3. the current page origin (same host the user already loaded the
 *      explorer from — reachable by definition)
 *
 * We deliberately NEVER fall back to http://localhost:8545: it is unreachable
 * for remote users and is a direct-node target that bypasses the privacy-proxy
 * redaction layer.
 */
export function resolveRpcUrl(chainInfo?: Pick<ChainInfo, 'rpcUrl'> | null): string {
  const fromBackend = chainInfo?.rpcUrl?.trim();
  if (fromBackend) return fromBackend;

  const fromConfig = getConfig('VITE_RPC_URL', '').trim();
  if (fromConfig) return fromConfig;

  // Same-origin fallback. In privacy deployments the explorer and the
  // proxy /rpc are typically served from the same public host, so the
  // page origin is a reachable default — unlike localhost.
  if (typeof window !== 'undefined' && window.location?.origin) {
    return `${window.location.origin}/rpc`;
  }
  return '';
}

/**
 * Resolve the target chain ID as a 0x-prefixed hex string. Priority:
 *   1. backend chainId from /chain-info (already 0x-hex, the real chain)
 *   2. VITE_CHAIN_ID config (decimal) converted to hex
 *
 * No silent "1001" default: a wrong chain ID is exactly the RD-1031 bug.
 * If neither source yields a value we return '' and callers skip the add.
 */
export function resolveChainIdHex(chainInfo?: Pick<ChainInfo, 'chainId'> | null): string {
  const fromBackend = chainInfo?.chainId?.trim();
  if (fromBackend) return fromBackend.toLowerCase();

  const fromConfig = getConfig('VITE_CHAIN_ID', '').trim();
  if (fromConfig) {
    const n = Number(fromConfig);
    if (Number.isFinite(n) && n > 0) {
      return '0x' + n.toString(16);
    }
  }
  return '';
}

/**
 * Build the wallet_addEthereumChain parameters from authoritative chain info.
 * Pure and testable — no network or wallet access. Returns null when no valid
 * chain ID or RPC URL can be resolved (so we never push a broken network to
 * MetaMask).
 */
export function buildAddEthereumChainParams(
  chainInfo?: ChainInfo | null,
): AddEthereumChainParameter | null {
  const chainId = resolveChainIdHex(chainInfo);
  const rpcUrl = resolveRpcUrl(chainInfo);
  if (!chainId || !rpcUrl) {
    return null;
  }
  return {
    chainId,
    chainName: getConfig('VITE_NETWORK_NAME') || getShortName(),
    nativeCurrency: { name: 'Ether', symbol: getNetworkCurrency(), decimals: 18 },
    rpcUrls: [rpcUrl],
  };
}

async function fetchChainInfo(): Promise<ChainInfo | null> {
  try {
    return await api.getChainInfo();
  } catch {
    // Backend unreachable — fall back to config/origin-derived values.
    return null;
  }
}

/**
 * Add the network to MetaMask using authoritative chain config from the
 * backend. Falls back to config/origin-derived values when the backend is
 * unreachable, but never to localhost.
 */
export async function addNetworkToMetaMask(): Promise<void> {
  const provider = getProvider();
  if (!provider) {
    alert('MetaMask is not installed. Please install MetaMask to add the network.');
    return;
  }

  const chainInfo = await fetchChainInfo();
  const params = buildAddEthereumChainParams(chainInfo);
  if (!params) {
    alert('Network configuration is unavailable. Please try again later.');
    return;
  }

  await provider.request({
    method: 'wallet_addEthereumChain',
    params: [params],
  });
}

/**
 * Ask the wallet to switch to the target network. Used by the "Switch network"
 * affordance (RD-1030) when the network is already added but not active.
 * If the chain is unknown to the wallet (4902), add it instead.
 */
export async function switchToNetwork(): Promise<void> {
  const provider = getProvider();
  if (!provider) {
    alert('MetaMask is not installed. Please install MetaMask to add the network.');
    return;
  }

  const chainInfo = await fetchChainInfo();
  const chainId = resolveChainIdHex(chainInfo);
  if (!chainId) {
    alert('Network configuration is unavailable. Please try again later.');
    return;
  }

  try {
    await provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId }],
    });
  } catch (err) {
    // 4902 = chain not added to the wallet yet → add it.
    if ((err as { code?: number })?.code === 4902) {
      await addNetworkToMetaMask();
      return;
    }
    throw err;
  }
}

/**
 * Read the wallet's currently-active chain ID (0x-hex, lowercased), or null if
 * no wallet is present / the call fails.
 */
export async function getWalletChainId(): Promise<string | null> {
  const provider = getProvider();
  if (!provider) return null;
  try {
    const chainId = (await provider.request({ method: 'eth_chainId' })) as string;
    return chainId?.toLowerCase() ?? null;
  } catch {
    return null;
  }
}

export type NetworkButtonState = 'no-wallet' | 'not-added' | 'active';

/**
 * Compare the wallet's active chain against the target chain. We can reliably
 * detect "active" (wallet currently on this chain). MetaMask does not expose a
 * way to enumerate added-but-inactive chains, so anything not active is
 * surfaced as "not-added" (the add flow is idempotent — re-adding an existing
 * chain is a no-op / switch). Returns 'no-wallet' when no provider is present
 * so the button still renders and prompts to install MetaMask.
 */
export function deriveNetworkButtonState(
  targetChainIdHex: string,
  walletChainIdHex: string | null,
): NetworkButtonState {
  if (walletChainIdHex === null) return 'no-wallet';
  if (!targetChainIdHex) return 'not-added';
  return walletChainIdHex.toLowerCase() === targetChainIdHex.toLowerCase() ? 'active' : 'not-added';
}

interface NetworkStatusListener {
  (state: NetworkButtonState): void;
}

/**
 * Subscribe to wallet network state for a target chain. Invokes the callback
 * immediately with the current state and again on every chainChanged event.
 * Returns an unsubscribe function. No-op (single 'no-wallet' callback) when no
 * wallet is present.
 */
export function watchNetworkState(
  targetChainIdHex: string,
  cb: NetworkStatusListener,
): () => void {
  const provider = getProvider();
  if (!provider) {
    cb('no-wallet');
    return () => {};
  }

  const emit = (walletChainId: string | null) =>
    cb(deriveNetworkButtonState(targetChainIdHex, walletChainId));

  getWalletChainId().then(emit).catch(() => emit(null));

  const handler = (...args: unknown[]) => {
    const chainId = typeof args[0] === 'string' ? (args[0] as string) : null;
    emit(chainId);
  };

  provider.on?.('chainChanged', handler);
  return () => provider.removeListener?.('chainChanged', handler);
}
