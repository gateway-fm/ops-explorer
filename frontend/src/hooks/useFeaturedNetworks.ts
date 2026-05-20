import { useEffect, useState } from 'react';
import { loadFeaturedNetworks, type FeaturedNetwork } from '../lib/featuredNetworks';

export function useFeaturedNetworks(): FeaturedNetwork[] {
  const [networks, setNetworks] = useState<FeaturedNetwork[]>([]);
  useEffect(() => {
    let cancelled = false;
    loadFeaturedNetworks().then((list) => {
      if (!cancelled) setNetworks(list);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  return networks;
}
