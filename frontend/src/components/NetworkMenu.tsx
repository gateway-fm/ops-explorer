import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, Globe, Check } from 'lucide-react';
import { isActiveNetwork, type FeaturedNetwork } from '../lib/featuredNetworks';
import { useFeaturedNetworks } from '../hooks/useFeaturedNetworks';

function groupNetworks(networks: FeaturedNetwork[]): Array<[string, FeaturedNetwork[]]> {
  const groups = new Map<string, FeaturedNetwork[]>();
  for (const n of networks) {
    const key = n.group ?? '';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(n);
  }
  return Array.from(groups.entries());
}

export function MobileNetworkList() {
  const networks = useFeaturedNetworks();
  if (networks.length < 2) return null;
  const grouped = groupNetworks(networks);
  return (
    <div className="card overflow-hidden">
      <div className="px-4 py-2 border-b border-neutral-100 bg-neutral-50">
        <span className="text-xs font-medium text-neutral-500 uppercase tracking-wider">Networks</span>
      </div>
      {grouped.map(([groupName, items], gi) =>
        items.map((n) => {
          const active = isActiveNetwork(n.url);
          return (
            <a
              key={`${groupName || gi}-${n.url}`}
              href={n.url}
              className={`flex items-center gap-3 px-4 py-3 transition-colors border-b border-neutral-100 last:border-b-0 ${
                active
                  ? 'bg-primary-50 dark:bg-primary-900/20 text-primary dark:text-primary-400'
                  : 'hover:bg-primary-50 dark:hover:bg-primary-900/20 text-neutral-700'
              }`}
            >
              {n.icon ? (
                <img src={n.icon} alt="" className="w-5 h-5 rounded" />
              ) : (
                <Globe className="w-5 h-5 text-neutral-500" />
              )}
              <span className="text-sm flex-1 truncate">{n.title}</span>
              {active && <Check className="w-4 h-4 text-primary" />}
            </a>
          );
        }),
      )}
    </div>
  );
}

export function NetworkMenu() {
  const networks = useFeaturedNetworks();
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const active = useMemo(() => networks.find((n) => isActiveNetwork(n.url)), [networks]);
  const grouped = useMemo(() => groupNetworks(networks), [networks]);

  // Hide silently when there's nothing meaningful to switch between.
  if (networks.length < 2) return null;

  const label = active?.title ?? 'Networks';

  return (
    <div ref={dropdownRef} className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="ml-1 flex items-center gap-1.5 px-3 py-2 text-neutral-700 hover:text-neutral-900 bg-neutral-50 hover:bg-neutral-100 border border-neutral-200 rounded-lg transition-colors text-sm font-medium dark:bg-neutral-800/40 dark:hover:bg-neutral-800 dark:border-neutral-700 dark:text-neutral-300"
      >
        <Globe className="w-4 h-4 text-primary" />
        <span className="hidden lg:inline">{label}</span>
        <ChevronDown className={`w-4 h-4 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute top-full right-0 mt-2 w-64 card overflow-hidden z-50 shadow-elevated">
          {grouped.map(([groupName, items], gi) => (
            <div key={groupName || `g-${gi}`}>
              {groupName && (
                <div className="px-4 py-2 border-b border-neutral-100 bg-neutral-50">
                  <span className="text-xs font-medium text-neutral-500 uppercase tracking-wider">
                    {groupName}
                  </span>
                </div>
              )}
              {items.map((n) => {
                const isActive = isActiveNetwork(n.url);
                return (
                  <a
                    key={n.url}
                    href={n.url}
                    className={`flex items-center gap-3 px-4 py-3 transition-colors ${
                      isActive
                        ? 'bg-primary-50 dark:bg-primary-900/20 text-primary dark:text-primary-400'
                        : 'hover:bg-primary-50 dark:hover:bg-primary-900/20 text-neutral-700'
                    }`}
                  >
                    {n.icon ? (
                      <img src={n.icon} alt="" className="w-5 h-5 rounded" />
                    ) : (
                      <Globe className="w-4 h-4 text-neutral-500" />
                    )}
                    <span className="text-sm flex-1 truncate">{n.title}</span>
                    {isActive && <Check className="w-4 h-4 text-primary" />}
                  </a>
                );
              })}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
