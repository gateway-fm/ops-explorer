import { afterEach, describe, expect, it, vi } from 'vitest';
import { connectNativeWallet, nativeHelperOrigin } from './nativeWalletHelper';
import type { AppOperation, OwnSelf } from './v2Session';
import type { WalletProvider } from './v2Wallet';

const org = '11111111-1111-4111-8111-111111111111',
  address = '0x' + '1'.repeat(40),
  hash = '0x' + '2'.repeat(64);
function fixture() {
  const now = Math.floor(Date.now() / 1000),
    actor = 'ops:p1:' + 'A'.repeat(43),
    abort = new AbortController();
  const self: OwnSelf = {
    actorId: actor,
    principalKind: 'human',
    sourceProfile: 'privado',
    sourceProfileDigest: 'sha256:' + 'a'.repeat(64),
    expiresAt: now + 120,
    organizations: [
      { id: org, slug: 'bank', name: 'Bank', isOrgAdmin: false, isOrgReadonlyAdmin: false },
    ],
  };
  const context = {
    protocol: 'ops.wallet-helper/v2',
    actorId: actor,
    sourceProfile: 'privado',
    sourceProfileDigest: self.sourceProfileDigest,
    expiresAt: now + 180,
    organizationId: org,
    chainId: '31337',
    genesisHash: '0x' + '3'.repeat(64),
    tokenContext: '4'.repeat(64),
  };
  const check = () => {
    if (abort.signal.aborted) throw Error('App context changed');
  };
  const scope = {
    identity: { actor, profile: 'privado', activation: org, expires: now + 120 },
    signal: abort.signal,
    check,
    self: vi.fn(async () => self),
    request: vi.fn(async () => [
      {
        address,
        verifiedAt: new Date(now * 1000).toISOString().replace('.000Z', 'Z'),
        proofVersion: 2,
      },
    ]),
    rpc: vi.fn(),
    dispose: () => abort.abort(),
  } as unknown as AppOperation;
  const listeners = new Map<string, () => void>();
  const request = vi.fn(async ({ method }: { method: string }) => {
    if (method === 'eth_chainId') return '0x7a69';
    if (method === 'eth_accounts' || method === 'eth_requestAccounts') return [address];
    if (method === 'ops_walletContext') return { ...context, notAfter: now + 120 };
    if (method === 'eth_sendTransaction') return hash;
    return null;
  });
  const provider: WalletProvider = {
    request,
    on: (event, listener) => listeners.set(event, listener),
    removeListener: (event) => listeners.delete(event),
  };
  const transport = vi.fn(
    async () =>
      new Response(JSON.stringify(context), { headers: { 'Content-Type': 'application/json' } }),
  ) as unknown as typeof fetch;
  const options = {
    operation: scope,
    organizationId: org,
    helperOrigin: 'http://127.0.0.1:9001',
    network: { chainId: '0x7a69', chainIdDecimal: '31337', genesisHash: context.genesisHash },
    chainName: 'Bank network',
    currency: 'ETH',
    provider,
    transport,
  };
  return { options, context, scope, provider, request, transport, abort, listeners, self };
}
afterEach(() => vi.restoreAllMocks());
describe('native ordinary-wallet helper', () => {
  it('rejects nonlocal/ambiguous helper origins', () => {
    for (const value of [
      'https://evil.test:9001',
      'http://127.0.0.1:9001/',
      'http://127.0.0.1:9001/x',
      'http://localhost:9001?x=1',
      'http://a@localhost:9001',
      'http://2130706433:9001',
      'http://127.0.0.1',
    ])
      expect(() => nativeHelperOrigin(value)).toThrow();
    expect(nativeHelperOrigin('http://127.0.0.1:9001')).toBe('http://127.0.0.1:9001');
  });
  it('attests wallet RPC identity and submits exactly once through the ordinary provider', async () => {
    const f = fixture(),
      onNetworkConfiguration = vi.fn();
    const wallet = await connectNativeWallet({ ...f.options, onNetworkConfiguration });
    expect(wallet.account).toBe(address);
    expect(wallet.expiresAt).toBe(f.scope.identity.expires);
    expect(onNetworkConfiguration).toHaveBeenCalledWith({
      rpcURL: `http://127.0.0.1:9001/native-v2/rpc/${f.context.tokenContext}/${f.scope.identity.expires}`,
      expiresAt: f.scope.identity.expires,
    });
    expect(await wallet.send({ to: address, data: '0x12345678', value: '0x0' })).toBe(hash);
    await expect(wallet.send({ to: address, data: '0x' })).rejects.toThrow('already handed');
    expect(
      f.request.mock.calls.filter(([arg]) => arg.method === 'eth_sendTransaction'),
    ).toHaveLength(1);
    expect(f.request.mock.calls.some(([arg]) => arg.method === 'eth_signTransaction')).toBe(false);
    expect((f.transport as ReturnType<typeof vi.fn>).mock.calls[0][1]).toMatchObject({
      credentials: 'omit',
      cache: 'no-store',
      redirect: 'error',
    });
    wallet.dispose();
    expect(f.listeners.size).toBe(0);
  });
  it.each([
    'actorId',
    'sourceProfile',
    'sourceProfileDigest',
    'organizationId',
    'genesisHash',
  ] as const)('refuses helper %s mismatch before wallet prompts', async (key) => {
    const f = fixture();
    f.context[key] = 'wrong';
    await expect(connectNativeWallet(f.options)).rejects.toThrow('helper must use');
    expect(f.request).not.toHaveBeenCalled();
  });
  it('refuses an existing wallet RPC URL bound to another helper token', async () => {
    const f = fixture();
    const original = f.provider.request;
    f.provider.request = async (args) =>
      args.method === 'ops_walletContext'
        ? { ...f.context, tokenContext: '5'.repeat(64), notAfter: f.scope.identity.expires }
        : original(args);
    await expect(connectNativeWallet(f.options)).rejects.toThrow('older RPC context');
    expect(f.request.mock.calls.some(([arg]) => arg.method === 'eth_sendTransaction')).toBe(false);
  });
  it('a context change while add-network is held prevents further prompts or submission', async () => {
    const f = fixture();
    let release!: () => void;
    const held = new Promise<void>((resolve) => (release = resolve)),
      original = f.provider.request;
    f.provider.request = async (args) => {
      if (args.method === 'wallet_addEthereumChain') {
        await held;
        return null;
      }
      return original(args);
    };
    const pending = connectNativeWallet(f.options);
    await vi.waitFor(() => expect(f.scope.self).toHaveBeenCalled());
    await new Promise((resolve) => setTimeout(resolve, 0));
    f.abort.abort();
    release();
    await expect(pending).rejects.toThrow('context changed');
    expect(f.request).not.toHaveBeenCalled();
  });
  it('account changes invalidate a connected context and never hand a write to the wallet', async () => {
    const f = fixture();
    const wallet = await connectNativeWallet(f.options);
    f.listeners.get('accountsChanged')?.();
    await expect(wallet.send({ to: address, data: '0x' })).rejects.toThrow('context changed');
    expect(f.request.mock.calls.some(([arg]) => arg.method === 'eth_sendTransaction')).toBe(false);
    wallet.dispose();
  });
  it('unlinked accounts cannot connect for transactions', async () => {
    const f = fixture();
    (f.scope.request as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    await expect(connectNativeWallet(f.options)).rejects.toThrow('Link this wallet');
  });
  it('does not retry an uncertain wallet request', async () => {
    const f = fixture(),
      original = f.provider.request;
    f.provider.request = async (args) => {
      if (args.method === 'eth_sendTransaction') throw Error('disconnected');
      return original(args);
    };
    const wallet = await connectNativeWallet(f.options);
    await expect(wallet.send({ data: '0x' })).rejects.toThrow('disconnected');
    await expect(wallet.send({ data: '0x' })).rejects.toThrow('already handed');
    wallet.dispose();
  });
  it('cancellation during the final attestation prevents dispatch and the actual wallet send', async () => {
    const f = fixture(),
      wallet = await connectNativeWallet(f.options),
      dispatch = vi.fn();
    let release!: (value: unknown) => void;
    const held = new Promise((resolve) => (release = resolve));
    (f.scope.request as ReturnType<typeof vi.fn>).mockReturnValueOnce(held);
    const sending = wallet.send({ data: '0x' }, dispatch);
    await vi.waitFor(() => expect(f.scope.request).toHaveBeenCalledTimes(2));
    wallet.dispose();
    release([
      { address, verifiedAt: new Date().toISOString().replace(/\.\d{3}Z$/, 'Z'), proofVersion: 2 },
    ]);
    await expect(sending).rejects.toThrow('context changed');
    expect(dispatch).not.toHaveBeenCalled();
    expect(f.request.mock.calls.some(([arg]) => arg.method === 'eth_sendTransaction')).toBe(false);
  });
  it('two concurrent final attestations can dispatch only one transaction', async () => {
    const f = fixture(),
      wallet = await connectNativeWallet(f.options),
      dispatch = vi.fn();
    const releases: ((value: unknown) => void)[] = [];
    (f.scope.request as ReturnType<typeof vi.fn>).mockImplementation(
      () => new Promise((resolve) => releases.push(resolve)),
    );
    const first = wallet.send({ data: '0x' }, dispatch),
      second = wallet.send({ data: '0x' }, dispatch);
    const outcomes = Promise.allSettled([first, second]);
    await vi.waitFor(() => expect(releases).toHaveLength(2));
    for (const release of releases)
      release([
        {
          address,
          verifiedAt: new Date().toISOString().replace(/\.\d{3}Z$/, 'Z'),
          proofVersion: 2,
        },
      ]);
    const settled = await outcomes;
    expect(settled.filter((result) => result.status === 'fulfilled')).toHaveLength(1);
    expect(settled.filter((result) => result.status === 'rejected')).toHaveLength(1);
    expect(dispatch).toHaveBeenCalledTimes(1);
    expect(
      f.request.mock.calls.filter(([arg]) => arg.method === 'eth_sendTransaction'),
    ).toHaveLength(1);
    wallet.dispose();
  });
});
