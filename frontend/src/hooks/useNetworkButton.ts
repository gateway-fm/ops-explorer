import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import {
  resolveChainIdHex,
  watchNetworkState,
  type NetworkButtonState,
} from '../lib/metamask';

export interface NetworkButtonInfo {
  /** Live wallet/network state used to render the "Add Network" button. */
  state: NetworkButtonState;
  /** True once the wallet is confirmed to be on the target network. */
  isActive: boolean;
}

/**
 * Shared state for the "Add Network to MetaMask" button (RD-1030). Resolves the
 * authoritative target chain ID from /chain-info (the real chain, RD-1031) and
 * tracks the wallet's current chain so callers can disable / relabel the button
 * when the network is already active. Falls back to config-derived chain ID
 * when the backend is unreachable.
 */
export function useNetworkButton(): NetworkButtonInfo {
  const { data: chainInfo } = useQuery({
    queryKey: ['chainInfo'],
    queryFn: api.getChainInfo,
    staleTime: 60000,
  });

  const targetChainId = resolveChainIdHex(chainInfo);
  const [state, setState] = useState<NetworkButtonState>('no-wallet');

  useEffect(() => {
    // watchNetworkState fires immediately with the current state and again on
    // every chainChanged event; returns an unsubscribe.
    return watchNetworkState(targetChainId, setState);
  }, [targetChainId]);

  return { state, isActive: state === 'active' };
}
