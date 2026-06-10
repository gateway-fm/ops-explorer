import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  resolveRpcUrl,
  resolveChainIdHex,
  buildAddEthereumChainParams,
  buildHelperAddEthereumChainParams,
  deriveNetworkButtonState,
} from './metamask';
import type { ChainInfo } from './api';

function chainInfo(overrides: Partial<ChainInfo> = {}): ChainInfo {
  return {
    chainId: '0x1092',
    chainIdDecimal: 4242,
    networkId: '4242',
    clientVersion: '',
    protocolVersion: '',
    latestBlock: 0,
    gasPrice: '0',
    peerCount: 0,
    isSyncing: false,
    genesisHash: '',
    updatedAt: '',
    ...overrides,
  };
}

describe('resolveRpcUrl (RD-1031, standalone)', () => {
  beforeEach(() => {
    window.__runtimeConfig = {};
  });
  afterEach(() => {
    window.__runtimeConfig = {};
  });

  it('prefers VITE_RPC_URL (the node RPC in standalone)', () => {
    window.__runtimeConfig = { VITE_RPC_URL: 'https://rpc.example.com' };
    expect(resolveRpcUrl()).toBe('https://rpc.example.com');
  });

  it('falls back to the node default localhost:8545 as a standalone last resort', () => {
    // This fallback equals the node default and is only reachable when there
    // is no privacy-proxy in front, so it is not a privacy bypass.
    expect(resolveRpcUrl()).toBe('http://localhost:8545');
  });

  it('never invents the explorer origin + /rpc (the removed RD-1031 fallback)', () => {
    const url = resolveRpcUrl();
    expect(url).not.toBe(`${window.location.origin}/rpc`);
  });
});

describe('resolveChainIdHex (RD-1031)', () => {
  beforeEach(() => {
    window.__runtimeConfig = {};
  });
  afterEach(() => {
    window.__runtimeConfig = {};
  });

  it('prefers the backend chainId (the real chain)', () => {
    window.__runtimeConfig = { VITE_CHAIN_ID: '1001' };
    expect(resolveChainIdHex({ chainId: '0x1092' })).toBe('0x1092');
  });

  it('converts VITE_CHAIN_ID (decimal) to hex when backend is absent', () => {
    window.__runtimeConfig = { VITE_CHAIN_ID: '4242' };
    expect(resolveChainIdHex(null)).toBe('0x1092');
  });

  it('never silently defaults to the wrong 1001 chain', () => {
    // No backend chainId, no config → empty, so callers skip the add rather
    // than pushing chainId 0x3e9 (1001), the original RD-1031 bug.
    expect(resolveChainIdHex(null)).toBe('');
  });
});

describe('buildAddEthereumChainParams (RD-1031, standalone)', () => {
  beforeEach(() => {
    window.__runtimeConfig = {};
  });
  afterEach(() => {
    window.__runtimeConfig = {};
  });

  it('builds params from the node RPC + authoritative backend chainId', () => {
    window.__runtimeConfig = { VITE_RPC_URL: 'https://rpc.example.com' };
    const params = buildAddEthereumChainParams(chainInfo({ chainId: '0x1092' }));
    expect(params).not.toBeNull();
    expect(params!.chainId).toBe('0x1092');
    expect(params!.rpcUrls).toEqual(['https://rpc.example.com']);
    expect(params!.nativeCurrency.decimals).toBe(18);
  });

  it('uses the standalone localhost fallback, never the explorer origin + /rpc', () => {
    window.__runtimeConfig = { VITE_CHAIN_ID: '4242' };
    const params = buildAddEthereumChainParams(null);
    expect(params).not.toBeNull();
    expect(params!.rpcUrls[0]).toBe('http://localhost:8545');
    expect(params!.rpcUrls[0]).not.toBe(`${window.location.origin}/rpc`);
  });

  it('returns null when no chain ID can be resolved (so we never push a broken network)', () => {
    expect(buildAddEthereumChainParams(null)).toBeNull();
    expect(buildAddEthereumChainParams(chainInfo({ chainId: '' }))).toBeNull();
  });
});

describe('buildHelperAddEthereumChainParams (RD-1031, privacy setup dialog)', () => {
  beforeEach(() => {
    window.__runtimeConfig = {};
  });
  afterEach(() => {
    window.__runtimeConfig = {};
  });

  it('uses the supplied jwt-injector helper URL as the wallet RPC', () => {
    const params = buildHelperAddEthereumChainParams(
      'http://127.0.0.1:9001',
      chainInfo({ chainId: '0x1092' }),
    );
    expect(params).not.toBeNull();
    expect(params!.chainId).toBe('0x1092');
    expect(params!.rpcUrls).toEqual(['http://127.0.0.1:9001']);
  });

  it('trims the helper URL', () => {
    const params = buildHelperAddEthereumChainParams(
      '  http://127.0.0.1:9001  ',
      chainInfo({ chainId: '0x1092' }),
    );
    expect(params!.rpcUrls).toEqual(['http://127.0.0.1:9001']);
  });

  it('falls back to VITE_CHAIN_ID for the chainId when the backend is absent', () => {
    window.__runtimeConfig = { VITE_CHAIN_ID: '4242' };
    const params = buildHelperAddEthereumChainParams('http://127.0.0.1:9001', null);
    expect(params!.chainId).toBe('0x1092');
  });

  it('returns null when no chain ID can be resolved', () => {
    expect(buildHelperAddEthereumChainParams('http://127.0.0.1:9001', null)).toBeNull();
  });

  it('returns null when no helper URL is supplied (never pushes a broken network)', () => {
    expect(buildHelperAddEthereumChainParams('', chainInfo({ chainId: '0x1092' }))).toBeNull();
    expect(buildHelperAddEthereumChainParams('   ', chainInfo({ chainId: '0x1092' }))).toBeNull();
  });

  it('never targets the proxy /rpc endpoint — only the explicit helper URL', () => {
    const params = buildHelperAddEthereumChainParams(
      'http://127.0.0.1:9001',
      chainInfo({ chainId: '0x1092', privacyProxyPublicUrl: 'https://proxy.example.com' }),
    );
    expect(params!.rpcUrls).toEqual(['http://127.0.0.1:9001']);
    expect(params!.rpcUrls[0]).not.toContain('proxy.example.com');
    expect(params!.rpcUrls[0]).not.toContain('/rpc');
  });
});

describe('deriveNetworkButtonState (RD-1030)', () => {
  it('reports active when the wallet is on the target chain', () => {
    expect(deriveNetworkButtonState('0x1092', '0x1092')).toBe('active');
    // case-insensitive compare
    expect(deriveNetworkButtonState('0x1092', '0X1092')).toBe('active');
  });

  it('reports not-added when the wallet is on a different chain', () => {
    expect(deriveNetworkButtonState('0x1092', '0x1')).toBe('not-added');
  });

  it('reports no-wallet when no provider is present', () => {
    expect(deriveNetworkButtonState('0x1092', null)).toBe('no-wallet');
  });

  it('reports not-added (never active) when target chain is unknown', () => {
    expect(deriveNetworkButtonState('', '0x1092')).toBe('not-added');
  });
});
