import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import {
  resolveRpcUrl,
  resolveChainIdHex,
  buildAddEthereumChainParams,
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

describe('resolveRpcUrl (RD-1031)', () => {
  beforeEach(() => {
    window.__runtimeConfig = {};
  });
  afterEach(() => {
    window.__runtimeConfig = {};
  });

  it('prefers the backend-provided rpcUrl', () => {
    window.__runtimeConfig = { VITE_RPC_URL: 'https://config.example.com/rpc' };
    expect(resolveRpcUrl({ rpcUrl: 'https://proxy.example.com/rpc' })).toBe(
      'https://proxy.example.com/rpc',
    );
  });

  it('falls back to VITE_RPC_URL when backend rpcUrl is absent', () => {
    window.__runtimeConfig = { VITE_RPC_URL: 'https://config.example.com/rpc' };
    expect(resolveRpcUrl(null)).toBe('https://config.example.com/rpc');
    expect(resolveRpcUrl({ rpcUrl: '' })).toBe('https://config.example.com/rpc');
  });

  it('falls back to the page origin + /rpc, never localhost:8545', () => {
    const url = resolveRpcUrl(null);
    expect(url).toBe(`${window.location.origin}/rpc`);
    expect(url).not.toBe('http://localhost:8545');
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

describe('buildAddEthereumChainParams (RD-1031)', () => {
  beforeEach(() => {
    window.__runtimeConfig = {};
  });
  afterEach(() => {
    window.__runtimeConfig = {};
  });

  it('builds params from authoritative backend chain info', () => {
    const params = buildAddEthereumChainParams(
      chainInfo({ chainId: '0x1092', rpcUrl: 'https://proxy.example.com/rpc' }),
    );
    expect(params).not.toBeNull();
    expect(params!.chainId).toBe('0x1092');
    expect(params!.rpcUrls).toEqual(['https://proxy.example.com/rpc']);
    expect(params!.nativeCurrency.decimals).toBe(18);
  });

  it('never emits the unreachable localhost RPC fallback', () => {
    window.__runtimeConfig = { VITE_CHAIN_ID: '4242' };
    const params = buildAddEthereumChainParams(null);
    expect(params).not.toBeNull();
    expect(params!.rpcUrls[0]).not.toBe('http://localhost:8545');
    expect(params!.rpcUrls[0]).toBe(`${window.location.origin}/rpc`);
  });

  it('returns null when no chain ID can be resolved (so we never push a broken network)', () => {
    expect(buildAddEthereumChainParams(null)).toBeNull();
    expect(buildAddEthereumChainParams(chainInfo({ chainId: '' }))).toBeNull();
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
