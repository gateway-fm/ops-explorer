import { getConfig } from './runtimeConfig';

export interface FeaturedNetwork {
  title: string;
  url: string;
  group?: string;
  icon?: string;
}

let cache: Promise<FeaturedNetwork[]> | null = null;

export function getFeaturedNetworksUrl(): string {
  return getConfig('VITE_FEATURED_NETWORKS_URL', '/featured-networks.json');
}

/**
 * Whether multi-network mode is enabled. The network switcher is OFF by default;
 * an operator must explicitly opt in by setting VITE_MULTI_NETWORK_ENABLED=true
 * AND providing the network list (featured-networks.json). This keeps
 * single-network deployments from ever showing the switcher — e.g. stale
 * localhost defaults baked into the image must not surface in production.
 */
export function isMultiNetworkEnabled(): boolean {
  return getConfig('VITE_MULTI_NETWORK_ENABLED', 'false').toLowerCase() === 'true';
}

export function loadFeaturedNetworks(): Promise<FeaturedNetwork[]> {
  if (cache) return cache;
  // Single-network mode (the default): never fetch the list, so the switcher
  // stays hidden regardless of what featured-networks.json happens to contain.
  if (!isMultiNetworkEnabled()) {
    cache = Promise.resolve([]);
    return cache;
  }
  cache = (async () => {
    try {
      const res = await fetch(getFeaturedNetworksUrl(), { cache: 'no-store' });
      if (!res.ok) return [];
      const data = (await res.json()) as unknown;
      if (!Array.isArray(data)) return [];
      return data.filter(
        (n): n is FeaturedNetwork =>
          typeof n === 'object' &&
          n !== null &&
          typeof (n as FeaturedNetwork).title === 'string' &&
          typeof (n as FeaturedNetwork).url === 'string',
      );
    } catch {
      return [];
    }
  })();
  return cache;
}

export function isActiveNetwork(url: string): boolean {
  try {
    return new URL(url).origin === window.location.origin;
  } catch {
    return false;
  }
}
