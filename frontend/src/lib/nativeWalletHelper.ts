import type { AppOperation, OwnSelf } from './v2Session';
import { ownWallets, injectedWallet, type WalletProvider } from './v2Wallet';

export interface HelperNetwork {
  chainId: string;
  chainIdDecimal: string;
  genesisHash: string;
}
export interface HelperContext {
  protocol: 'ops.wallet-helper/v2';
  actorId: string;
  sourceProfile: string;
  sourceProfileDigest: string;
  expiresAt: number;
  organizationId: string;
  chainId: string;
  genesisHash: string;
  tokenContext: string;
}
export interface HelperTransaction {
  to?: string;
  data: string;
  value?: string;
  gas?: string;
  maxFeePerGas?: string;
  maxPriorityFeePerGas?: string;
}
export interface NativeWalletContext {
  readonly account: string;
  readonly expiresAt: number;
  send(transaction: HelperTransaction, onDispatch?: () => void): Promise<string>;
  dispose(): void;
}
const keys = [
  'protocol',
  'actorId',
  'sourceProfile',
  'sourceProfileDigest',
  'expiresAt',
  'organizationId',
  'chainId',
  'genesisHash',
  'tokenContext',
];
export function nativeHelperOrigin(raw: string): string {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw Error('Enter the local wallet helper origin.');
  }
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) ||
    !url.port ||
    url.username ||
    url.password ||
    url.pathname !== '/' ||
    url.search ||
    url.hash ||
    raw !== url.origin
  )
    throw Error('Use an exact localhost helper origin with its port.');
  return url.origin;
}
export function nativeHelperContext(
  value: unknown,
  operation: AppOperation,
  self: OwnSelf,
  organizationId: string,
  network: HelperNetwork,
): HelperContext {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw Error('Invalid wallet helper context.');
  const v = value as HelperContext;
  if (
    Object.keys(v).length !== keys.length ||
    Object.keys(v).some((key) => !keys.includes(key)) ||
    v.protocol !== 'ops.wallet-helper/v2' ||
    v.actorId !== operation.identity.actor ||
    v.actorId !== self.actorId ||
    v.sourceProfile !== operation.identity.profile ||
    v.sourceProfile !== self.sourceProfile ||
    v.sourceProfileDigest !== self.sourceProfileDigest ||
    v.organizationId !== organizationId ||
    !self.organizations.some((org) => org.id === organizationId) ||
    v.chainId !== network.chainIdDecimal ||
    v.genesisHash !== network.genesisHash ||
    network.chainId !== '0x' + BigInt(network.chainIdDecimal).toString(16) ||
    !Number.isSafeInteger(v.expiresAt) ||
    v.expiresAt <= Date.now() / 1000 ||
    v.expiresAt > Date.now() / 1000 + 330 ||
    !/^[0-9a-f]{64}$/.test(v.tokenContext)
  )
    throw Error(
      'The helper must use this account, identity profile, organization and network. Refresh its token file, then reconnect.',
    );
  return v;
}
async function readContext(
  origin: string,
  operation: AppOperation,
  transport: typeof fetch,
): Promise<unknown> {
  operation.check();
  const response = await transport(origin + '/native-v2/context', {
    method: 'GET',
    credentials: 'omit',
    cache: 'no-store',
    redirect: 'error',
    signal: operation.signal,
  });
  operation.check();
  if (
    !response.ok ||
    response.redirected ||
    !/^application\/json(?:;\s*charset=utf-8)?$/i.test(
      response.headers.get('Content-Type') ?? '',
    ) ||
    !response.body
  )
    throw Error(
      'Cannot verify the local helper. Check its token file, trusted origins and browser local-network permission.',
    );
  const reader = response.body.getReader();
  let raw = '',
    bytes = 0;
  const decoder = new TextDecoder('utf-8', { fatal: true });
  try {
    for (;;) {
      const next = await reader.read();
      operation.check();
      if (next.done) break;
      bytes += next.value.byteLength;
      if (bytes > 8192) throw Error('Invalid helper response.');
      raw += decoder.decode(next.value, { stream: true });
    }
    raw += decoder.decode();
    operation.check();
    return JSON.parse(raw);
  } finally {
    void reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}

// The caller owns/disposes the original AppOperation. The sensitive context URL
// is passed only to the user-selected wallet's network setup, never app storage.
export async function connectNativeWallet(options: {
  operation: AppOperation;
  helperOrigin: string;
  organizationId: string;
  network: HelperNetwork;
  chainName: string;
  currency: string;
  provider?: WalletProvider | null;
  transport?: typeof fetch;
  onNetworkConfiguration?: (configuration: { rpcURL: string; expiresAt: number }) => void;
}): Promise<NativeWalletContext> {
  const { operation, organizationId, network } = options;
  const provider = options.provider === undefined ? injectedWallet() : options.provider;
  if (!provider) throw Error('Install or unlock your browser wallet.');
  const origin = nativeHelperOrigin(options.helperOrigin),
    transport = options.transport ?? globalThis.fetch.bind(globalThis);
  operation.check();
  const self = await operation.self();
  operation.check();
  const context = nativeHelperContext(
    await readContext(origin, operation, transport),
    operation,
    self,
    organizationId,
    network,
  );
  const expiresAt = Math.min(operation.identity.expires, self.expiresAt, context.expiresAt);
  const rpcURL = `${origin}/native-v2/rpc/${context.tokenContext}/${expiresAt}`;
  operation.check();
  options.onNetworkConfiguration?.({ rpcURL, expiresAt });
  operation.check();
  await provider.request({
    method: 'wallet_addEthereumChain',
    params: [
      {
        chainId: network.chainId,
        chainName: options.chainName,
        nativeCurrency: { name: options.currency, symbol: options.currency, decimals: 18 },
        rpcUrls: [rpcURL],
      },
    ],
  });
  operation.check();
  await provider.request({
    method: 'wallet_switchEthereumChain',
    params: [{ chainId: network.chainId }],
  });
  operation.check();
  const accounts = await provider.request({ method: 'eth_requestAccounts' });
  operation.check();
  if (
    !Array.isArray(accounts) ||
    typeof accounts[0] !== 'string' ||
    !/^0x[0-9a-fA-F]{40}$/.test(accounts[0])
  )
    throw Error('Select a wallet account.');
  const account = accounts[0].toLowerCase();
  let invalid = false,
    disposed = false,
    sent = false;
  const changed = () => {
    invalid = true;
  };
  const events = ['accountsChanged', 'chainChanged', 'disconnect'];
  events.forEach((event) => provider.on?.(event, changed));
  const check = () => {
    operation.check();
    if (invalid || disposed || expiresAt <= Date.now() / 1000)
      throw Error('Wallet context changed or expired. Reconnect before submitting.');
  };
  const attest = async () => {
    check();
    const chain = await provider.request({ method: 'eth_chainId' });
    check();
    if (chain !== network.chainId) throw Error('Switch the wallet to the approved network.');
    const actual = await provider.request({ method: 'ops_walletContext', params: [] });
    check();
    if (!actual || typeof actual !== 'object' || Array.isArray(actual))
      throw Error(
        'The wallet is using a different RPC. Update its network RPC to the current helper context.',
      );
    const { notAfter, ...base } = actual as HelperContext & { notAfter: number };
    nativeHelperContext(base, operation, self, organizationId, network);
    if (notAfter !== expiresAt || (base as HelperContext).tokenContext !== context.tokenContext)
      throw Error('The wallet kept an older RPC context. Update its network RPC, then reconnect.');
    const current = await provider.request({ method: 'eth_accounts' });
    check();
    if (
      !Array.isArray(current) ||
      typeof current[0] !== 'string' ||
      current[0].toLowerCase() !== account
    )
      throw Error('Wallet account changed.');
    const links = ownWallets(await operation.request('/eth/addresses'));
    check();
    if (!links.some((link) => link.address === account))
      throw Error('Link this wallet to your OPS identity on the Account page first.');
  };
  const dispose = () => {
    disposed = true;
    events.forEach((event) => provider.removeListener?.(event, changed));
  };
  try {
    await attest();
  } catch (e) {
    dispose();
    throw e;
  }
  return Object.freeze({
    account,
    expiresAt,
    dispose,
    send: async (transaction: HelperTransaction, onDispatch?: () => void) => {
      check();
      if (sent)
        throw Error(
          'This submission was already handed to the wallet. Check its outcome before starting another.',
        );
      if (
        !transaction ||
        Object.keys(transaction).some(
          (key) =>
            !['to', 'data', 'value', 'gas', 'maxFeePerGas', 'maxPriorityFeePerGas'].includes(key),
        ) ||
        (transaction.to !== undefined && !/^0x[0-9a-f]{40}$/.test(transaction.to)) ||
        typeof transaction.data !== 'string' ||
        !/^0x(?:[0-9a-fA-F]{2})*$/.test(transaction.data) ||
        transaction.data.length > 131074
      )
        throw Error('Invalid contract transaction.');
      for (const name of ['value', 'gas', 'maxFeePerGas', 'maxPriorityFeePerGas'] as const)
        if (
          transaction[name] !== undefined &&
          !/^0x(?:0|[1-9a-f][0-9a-f]{0,63})$/.test(transaction[name]!)
        )
          throw Error('Invalid transaction quantity.');
      await attest();
      check();
      if (sent) {
        throw Error(
          'This submission was already handed to the wallet. Check its outcome before starting another.',
        );
      }
      onDispatch?.();
      sent = true;
      // From this call onward the wallet may sign/broadcast. Reject in the
      // wallet to cancel; page changes cannot undo a transaction already sent.
      const hash = await provider.request({
        method: 'eth_sendTransaction',
        params: [{ ...transaction, from: account, chainId: network.chainId }],
      });
      check();
      if (typeof hash !== 'string' || !/^0x[0-9a-f]{64}$/.test(hash))
        throw Error('The wallet outcome is uncertain. Check its activity before resubmitting.');
      return hash;
    },
  });
}
