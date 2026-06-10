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

/** The node's default local JSON-RPC port — used only as a standalone-mode
 * last resort (see resolveRpcUrl). It equals the geth/anvil default and is NOT
 * a privacy bypass: it is only reachable when no privacy-proxy is involved. */
const STANDALONE_NODE_RPC_FALLBACK = 'http://localhost:8545';

/**
 * Resolve the JSON-RPC URL for STANDALONE-mode wallet add and contract reads.
 * Priority (RD-1031):
 *   1. VITE_RPC_URL build/runtime config (the node's public RPC in standalone)
 *   2. http://localhost:8545 — the node default, as a standalone last resort
 *
 * This is a standalone/node concern. In privacy mode the wallet does NOT use
 * this: chain access goes through a locally-run jwt-injector configured in the
 * MetaMask setup dialog (see buildHelperAddEthereumChainParams). We deliberately
 * never invent a `window.location.origin + '/rpc'` target — the explorer origin
 * does not serve /rpc and would 404 — nor do we feed the proxy /rpc to a wallet.
 */
export function resolveRpcUrl(): string {
  const fromConfig = getConfig('VITE_RPC_URL', '').trim();
  if (fromConfig) return fromConfig;

  // Standalone last resort: the node's default local RPC. Reachable only when
  // there is no proxy in front, so it is not a privacy bypass.
  return STANDALONE_NODE_RPC_FALLBACK;
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
 * Build the wallet_addEthereumChain parameters for STANDALONE mode from
 * authoritative chain info, using the node RPC (resolveRpcUrl). Pure and
 * testable — no network or wallet access. Returns null when no valid chain ID
 * can be resolved (so we never push a broken network to MetaMask).
 *
 * Privacy mode does NOT use this; it builds params from the user-supplied
 * jwt-injector helper URL via buildHelperAddEthereumChainParams.
 */
export function buildAddEthereumChainParams(
  chainInfo?: ChainInfo | null,
): AddEthereumChainParameter | null {
  return buildHelperAddEthereumChainParams(resolveRpcUrl(), chainInfo);
}

/**
 * Build wallet_addEthereumChain parameters from an explicit RPC URL (e.g. the
 * jwt-injector helper URL entered in the privacy setup dialog) plus the
 * authoritative chain metadata. Pure and testable. Returns null when no valid
 * chain ID can be resolved or no rpcUrl is supplied, so callers never push a
 * broken network to MetaMask.
 */
export function buildHelperAddEthereumChainParams(
  rpcUrl: string,
  chainInfo?: ChainInfo | null,
): AddEthereumChainParameter | null {
  const chainId = resolveChainIdHex(chainInfo);
  const trimmedRpc = rpcUrl?.trim() ?? '';
  if (!chainId || !trimmedRpc) {
    return null;
  }
  return {
    chainId,
    chainName: getConfig('VITE_NETWORK_NAME') || getShortName(),
    nativeCurrency: { name: 'Ether', symbol: getNetworkCurrency(), decimals: 18 },
    rpcUrls: [trimmedRpc],
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
 * Add the network to MetaMask (STANDALONE mode) using authoritative chain
 * config from the backend, falling back to VITE_RPC_URL / the node's default
 * localhost RPC. Privacy mode uses the setup dialog (addNetworkViaHelper)
 * instead and never calls this directly.
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
 * Add the network to MetaMask pointing at an explicit RPC URL — the locally-run
 * jwt-injector helper URL entered in the privacy setup dialog. Throws on a
 * missing wallet (so the caller can surface the install prompt) or unresolved
 * chain config; rethrows wallet errors (e.g. the injector not being up) so the
 * dialog/MetaMask can surface them.
 */
export async function addNetworkViaHelper(
  helperUrl: string,
  chainInfo?: ChainInfo | null,
): Promise<void> {
  const provider = getProvider();
  if (!provider) {
    alert('MetaMask is not installed. Please install MetaMask to add the network.');
    return;
  }

  const info = chainInfo ?? (await fetchChainInfo());
  const params = buildHelperAddEthereumChainParams(helperUrl, info);
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
