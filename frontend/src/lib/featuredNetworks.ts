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

export function loadFeaturedNetworks(): Promise<FeaturedNetwork[]> {
  if (cache) return cache;
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
