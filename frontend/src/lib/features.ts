// Feature-flag gates derived from runtime configuration.
//
// Privacy-mode deployments hide capabilities that don't exist on the
// privacy-proxy side or whose access-control story isn't designed yet.
// The flag is read via getConfig() so the same compiled bundle can serve
// both privacy and standalone deployments — the entrypoint.sh writes
// VITE_PRIVACY_MODE into /config.js based on the container env.
//
// To add a new feature gate:
//   1. Add an entry to FeatureFlags below.
//   2. Read it via `useFeatures()` in components or `features` for non-React.
//   3. Set the corresponding VITE_* env var in privacy compose if the
//      feature should default off in privacy mode.

import { getConfig } from './runtimeConfig';

export interface FeatureFlags {
  /**
   * Contract verification UI + API call surface.
   *
   * Disabled in privacy mode (VITE_PRIVACY_MODE=true). The privacy proxy
   * doesn't implement the verification endpoint and the "who can verify
   * which contract" RBAC model is undecided — until that's designed, the
   * full surface (Verify Contract page, footer links, address-page CTAs,
   * navigation entries, Etherscan-compatible /api root, /verify routes,
   * Sourcify lookups) is compiled out / hidden.
   */
  contractVerification: boolean;

  /**
   * Charts / Stats UI + the chart-line API call surface.
   *
   * Disabled in privacy mode (VITE_PRIVACY_MODE=true). The /charts endpoints
   * forward to a chain-indexer surface the privacy proxy doesn't serve and
   * have no viewer-scoping/redaction story (RD-1063). When off, the Charts
   * nav entries, the /stats page, and the Home transaction-history chart are
   * hidden and fire no requests; the backend mounts no /charts routes.
   */
  charts: boolean;

  /**
   * Gas Tracker UI + the /gas API call surface (page + Layout header poller).
   *
   * Disabled in privacy mode (VITE_PRIVACY_MODE=true). /gas resolves to
   * ErrChainDataNotAvailable on the privacy side (RD-1063). When off, the Gas
   * Tracker nav entry and /gas-tracker page are hidden and fire no requests
   * (including the 15s Layout header poller); the backend mounts no /gas route.
   */
  gasTracker: boolean;
}

function isPrivacyMode(): boolean {
  return getConfig('VITE_PRIVACY_MODE') === 'true';
}

export function features(): FeatureFlags {
  const privacyMode = isPrivacyMode();
  return {
    contractVerification: !privacyMode,
    charts: !privacyMode,
    gasTracker: !privacyMode,
  };
}

/**
 * React hook variant — the underlying source (window.__runtimeConfig)
 * is set once at container start, so values are stable across renders.
 * Returning a fresh object per call is fine; no memoization needed.
 */
export function useFeatures(): FeatureFlags {
  return features();
}
