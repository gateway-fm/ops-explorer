import { Outlet, useNavigate } from 'react-router-dom';
import { useState, useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeftRight, Users, Coins, ShieldCheck, Boxes, Fuel, Eye, Info, BookOpen, BarChart3 } from 'lucide-react';
import { api } from '../lib/api';
import type { SearchSuggestion } from '../lib/api';
import { NavDropdown } from './NavDropdown';
import { MobileNetworkList } from './NetworkMenu';
import { AddNetworkButton } from './AddNetworkButton';
import { usePrivacyEnabled } from '../hooks/usePrivacyEnabled';
import { ImpersonationBanner } from './ImpersonationBanner';
import { features } from '../lib/features';
import { ExplorerChrome } from './ExplorerChrome';

const mobileNavItems = [
  { to: '/blocks', label: 'Blocks', icon: Boxes },
  { to: '/transactions', label: 'Transactions', icon: ArrowLeftRight },
  { to: '/accounts', label: 'Top Accounts', icon: Users },
  { to: '/tokens', label: 'Tokens', icon: Coins },
  { to: '/token-transfers', label: 'Token Transfers', icon: ArrowLeftRight },
  // "Gas Tracker" hidden in privacy mode (RD-1063, see lib/features.ts).
  ...(features().gasTracker
    ? [{ to: '/gas-tracker', label: 'Gas Tracker', icon: Fuel }]
    : []),
  // "Verify Contract" hidden in privacy mode (see lib/features.ts).
  ...(features().contractVerification
    ? [{ to: '/verify', label: 'Verify Contract', icon: ShieldCheck }]
    : []),
  { to: '/chain-info', label: 'Chain Info', icon: Info },
  // "Charts" hidden in privacy mode (RD-1063, see lib/features.ts).
  ...(features().charts
    ? [{ to: '/stats', label: 'Charts', icon: BarChart3 }]
    : []),
  { to: '/api-docs', label: 'API Docs', icon: BookOpen },
  { to: '/privacy', label: 'Auditor', icon: Eye },
];

export function Layout() {
  const privacyEnabled = usePrivacyEnabled();
  const [search, setSearch] = useState('');
  const [suggestions, setSuggestions] = useState<SearchSuggestion[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const navigate = useNavigate();
  const searchRef = useRef<HTMLDivElement>(null);
  const mobileInputRef = useRef<HTMLInputElement>(null);

  // Header pollers — ETH price (60s) and gas prices (15s, shown in the nav).
  // They run on EVERY page, so they must not fire in privacy mode where the
  // backend 404s /price and /gas.
  // RD-1063 (M1): gate behind the gasTracker feature flag (VITE_PRIVACY_MODE).
  // RD-1082: ALSO gate on the backend-derived privacy signal (`privacyEnabled`,
  // from /api/stats). The build-time flag lives in deploy env, not in code, so
  // a privacy deployment that forgot to set VITE_PRIVACY_MODE would otherwise
  // storm both endpoints with 404s. The backend always reports privacyEnabled,
  // so this gate holds regardless of the flag. (404s aren't retried — App.tsx
  // shouldRetryQuery — and the query is disabled once /stats resolves, so the
  // worst case is a single request during the brief pre-stats window.)
  useQuery({
    queryKey: ['ethPrice'],
    queryFn: api.getPrice,
    refetchInterval: 60000,
    staleTime: 30000,
    enabled: features().gasTracker && !privacyEnabled,
  });

  const { data: gasPrices } = useQuery({
    queryKey: ['gasPrices-nav'],
    queryFn: api.getGasPrices,
    refetchInterval: 15000,
    staleTime: 10000,
    enabled: features().gasTracker && !privacyEnabled,
  });

  // Build identity of the running backend (stamped via -ldflags), shown in the
  // footer. Static for the life of the page, so fetch once and never retry — a
  // failure just hides the version rather than logging noise.
  const { data: versionInfo } = useQuery({
    queryKey: ['appVersion'],
    queryFn: api.getVersion,
    staleTime: Infinity,
    gcTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  });

  // Close mobile menu on route change
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- intentional reset on navigation
    setMobileMenuOpen(false);
  }, [navigate]);

  // Prevent body scroll when mobile menu is open
  useEffect(() => {
    if (mobileMenuOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [mobileMenuOpen]);

  // Debounced search suggestions
  useEffect(() => {
    if (!search.trim() || search.length < 1) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- clearing suggestions when search is empty
      setSuggestions([]);
      return;
    }

    const timer = setTimeout(async () => {
      try {
        const response = await api.searchSuggestions(search.trim());
        setSuggestions(response.suggestions || []);
        setSelectedIndex(-1);
      } catch (err) {
        console.error('Search suggestions error:', err);
        setSuggestions([]);
      }
    }, 150);

    return () => clearTimeout(timer);
  }, [search]);

  // Close suggestions when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        setShowSuggestions(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const navigateToSuggestion = (suggestion: SearchSuggestion) => {
    switch (suggestion.type) {
      case 'block':
        navigate(`/block/${suggestion.value}`);
        break;
      case 'transaction':
        navigate(`/tx/${suggestion.value}`);
        break;
      case 'address':
        navigate(`/address/${suggestion.value}`);
        break;
    }
    setSearch('');
    setSuggestions([]);
    setShowSuggestions(false);
    setMobileMenuOpen(false);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!search.trim()) return;

    if (showSuggestions && selectedIndex >= 0 && suggestions[selectedIndex]) {
      navigateToSuggestion(suggestions[selectedIndex]);
      return;
    }

    const q = search.trim();
    if (/^\d+$/.test(q)) {
      navigate(`/block/${q}`);
    } else if (q.startsWith('0x') && q.length === 66) {
      navigate(`/tx/${q}`);
    } else if (q.startsWith('0x') && q.length === 42) {
      navigate(`/address/${q}`);
    }
    setSearch('');
    setSuggestions([]);
    setShowSuggestions(false);
    setMobileMenuOpen(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showSuggestions || suggestions.length === 0) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((prev) =>
          prev < suggestions.length - 1 ? prev + 1 : prev
        );
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : -1));
        break;
      case 'Escape':
        setShowSuggestions(false);
        setSelectedIndex(-1);
        break;
    }
  };

  return <ExplorerChrome
    navigation={<NavDropdown />}
    banner={<ImpersonationBanner />}
    mobileNetworks={<MobileNetworkList />}
    networkButton={<AddNetworkButton variant="footer" />}
    privacyEnabled={privacyEnabled}
    featureFlags={features()}
    gasPrices={gasPrices}
    versionInfo={versionInfo}
    mobileNavItems={mobileNavItems}
    mobileMenuOpen={mobileMenuOpen}
    setMobileMenuOpen={setMobileMenuOpen}
    search={search}
    setSearch={value => { setSearch(value); setShowSuggestions(true); }}
    suggestions={suggestions}
    showSuggestions={showSuggestions}
    setShowSuggestions={setShowSuggestions}
    searchContainerRef={searchRef}
    selectedIndex={selectedIndex}
    mobileInputRef={mobileInputRef}
    handleSearch={handleSearch}
    handleKeyDown={handleKeyDown}
    navigateToSuggestion={navigateToSuggestion}
  ><Outlet /></ExplorerChrome>;
}
