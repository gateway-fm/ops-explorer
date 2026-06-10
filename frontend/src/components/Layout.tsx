import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useState, useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search, Menu, X, ArrowLeftRight, Users, Coins, ShieldCheck, Boxes, Fuel, Eye, Info, BookOpen, BarChart3 } from 'lucide-react';
import { api } from '../lib/api';
import type { SearchSuggestion } from '../lib/api';
import { NavDropdown } from './NavDropdown';
import { MobileNetworkList } from './NetworkMenu';
import { AddNetworkButton } from './AddNetworkButton';
import { usePrivacyEnabled } from '../hooks/usePrivacyEnabled';
import { Tooltip, TooltipTrigger, TooltipContent } from './ui/tooltip';
import { ImpersonationBanner } from './ImpersonationBanner';
import { branding } from '../lib/branding';
import { features } from '../lib/features';

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

  // Fetch ETH price (used in commented-out UI sections).
  // RD-1063 (M1): gate the header pollers behind the gasTracker flag so privacy
  // mode doesn't storm /price (60s) and /gas (15s) on EVERY page — the backend
  // routes are 404 there and these run regardless of which page is mounted.
  useQuery({
    queryKey: ['ethPrice'],
    queryFn: api.getPrice,
    refetchInterval: 60000,
    staleTime: 30000,
    enabled: features().gasTracker,
  });

  const { data: gasPrices } = useQuery({
    queryKey: ['gasPrices-nav'],
    queryFn: api.getGasPrices,
    refetchInterval: 15000,
    staleTime: 10000,
    enabled: features().gasTracker,
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
        setShowSuggestions(true);
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

    if (selectedIndex >= 0 && suggestions[selectedIndex]) {
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

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'block':
        return '#';
      case 'transaction':
        return 'Tx';
      case 'address':
        return '@';
      default:
        return '?';
    }
  };

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'block':
        return 'bg-blue-50 text-blue-600 border border-blue-200';
      case 'transaction':
        return 'bg-green-50 text-green-600 border border-green-200';
      case 'address':
        return 'bg-primary-50 text-primary-600 border border-primary-200';
      default:
        return 'bg-neutral-100 text-neutral-600';
    }
  };

  return (
    <div className="min-h-screen bg-neutral-100 flex flex-col">
      {/* RD-928 "View as user" — sticky amber banner shown to admins
          who are currently impersonating a tier-2 user. Renders nothing
          when no impersonation session is active. */}
      <ImpersonationBanner />
      {/* Navigation Header */}
      <header className="nav">
        <div className="px-4 sm:px-6 py-3 sm:py-4">
          <div className="flex items-center justify-between gap-2 sm:gap-4">
            {/* Logo and ETH Price */}
            <div className="flex items-center gap-3 sm:gap-4 shrink-0">
              <Link to="/" className="shrink-0 w-8 h-8 sm:w-10 sm:h-10 [perspective:200px] group" data-testid="header-brand">
                <div className="relative w-full h-full transition-transform duration-500 [transform-style:preserve-3d] group-hover:[transform:rotateX(180deg)]">
                  <div className="absolute inset-0 [backface-visibility:hidden]">
                    <img src={branding.logo} alt={branding.name} className="w-full h-full rounded-xl object-contain" />
                  </div>
                  <div className="absolute inset-0 [backface-visibility:hidden] [transform:rotateX(180deg)]">
                    <img src={branding.icon} alt={branding.name} className="w-full h-full rounded-xl object-contain p-0.5" />
                  </div>
                </div>
              </Link>
              {gasPrices?.normal && (
                <>
                <div className="w-px h-6 sm:h-7 bg-neutral-200" />
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Link
                      to="/gas-tracker"
                      className="flex items-center gap-1.5 sm:gap-2 px-2.5 sm:px-3 h-8 sm:h-10 rounded-lg bg-neutral-100 border border-neutral-200 hover:bg-neutral-200 transition-colors"
                    >
                      <Fuel className="w-3.5 h-3.5 sm:w-4 sm:h-4 text-primary" />
                      <span className="text-xs sm:text-sm font-semibold text-neutral-700">
                        {gasPrices.normal.price < 0.01
                          ? gasPrices.normal.price.toFixed(4)
                          : gasPrices.normal.price < 1
                            ? gasPrices.normal.price.toFixed(3)
                            : gasPrices.normal.price.toFixed(1)
                        }
                      </span>
                      <span className="text-xs sm:text-sm text-neutral-400">Gwei</span>
                    </Link>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    <div className="space-y-1 py-0.5">
                      <div className="text-xs font-medium text-zinc-300 mb-1">Gas Price</div>
                      <div className="flex justify-between gap-4 text-xs">
                        <span className="text-green-400">Slow</span>
                        <span>{gasPrices.slow?.price.toFixed(4) ?? '-'} Gwei</span>
                      </div>
                      <div className="flex justify-between gap-4 text-xs">
                        <span className="text-yellow-400">Normal</span>
                        <span>{gasPrices.normal.price.toFixed(4)} Gwei</span>
                      </div>
                      <div className="flex justify-between gap-4 text-xs">
                        <span className="text-red-400">Fast</span>
                        <span>{gasPrices.fast?.price.toFixed(4) ?? '-'} Gwei</span>
                      </div>
                    </div>
                  </TooltipContent>
                </Tooltip>
                </>
              )}
              {/* ETH price tracker - hidden for now
              <div className="flex items-center gap-2 px-4 h-8 sm:h-10 rounded-lg bg-neutral-100 border border-neutral-200">
                <span className="text-sm font-medium text-neutral-600">ETH</span>
                {priceData ? (
                  <>
                    <span className="text-sm font-semibold text-neutral-900">
                      ${priceData.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </span>
                    {priceData.change24h !== undefined && (
                      <span className={`flex items-center gap-0.5 text-xs font-medium ${priceData.change24h >= 0 ? 'text-success-600' : 'text-error-600'}`}>
                        {priceData.change24h >= 0 ? (
                          <TrendingUp className="w-3 h-3" />
                        ) : (
                          <TrendingDown className="w-3 h-3" />
                        )}
                        {Math.abs(priceData.change24h).toFixed(2)}%
                      </span>
                    )}
                  </>
                ) : (
                  <span className="text-sm text-neutral-400">--</span>
                )}
              </div>
              */}
            </div>

            {/* Desktop Navigation */}
            <nav className="hidden md:flex items-center gap-3">
              <NavDropdown />
            </nav>

            {/* Mobile Menu Button */}
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="md:hidden p-2 rounded-lg bg-neutral-100 hover:bg-neutral-200 border border-neutral-200 transition-colors"
              aria-label="Toggle menu"
              data-testid="mobile-menu-trigger"
            >
              {mobileMenuOpen ? (
                <X className="w-5 h-5 text-neutral-700" />
              ) : (
                <Menu className="w-5 h-5 text-neutral-700" />
              )}
            </button>
          </div>
        </div>
      </header>

      {/* Mobile Menu Overlay */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-30 md:hidden" data-testid="mobile-menu-overlay">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/20 backdrop-blur-sm"
            onClick={() => setMobileMenuOpen(false)}
          />

          {/* Menu Panel */}
          <div className="absolute top-[72px] left-4 right-4 bottom-4 overflow-y-auto backdrop-blur-md border border-neutral-200 rounded-2xl" style={{ backgroundColor: 'color-mix(in srgb, var(--neutral-50) 80%, transparent)' }}>
            <div className="p-4 space-y-4">
              {/* Mobile Search */}
              <form onSubmit={handleSearch}>
                <div className="relative">
                  <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-neutral-400" />
                  <input
                    ref={mobileInputRef}
                    type="text"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={`Search ${branding.name}...`}
                    className="input pl-11"
                  />
                </div>
              </form>

              {/* Mobile Search Suggestions */}
              {search.trim() && suggestions.length > 0 && (
                <div className="card overflow-hidden max-h-64 overflow-y-auto">
                  {suggestions.map((suggestion, index) => (
                    <button
                      key={`mobile-${suggestion.type}-${suggestion.value}`}
                      onClick={() => navigateToSuggestion(suggestion)}
                      className={`w-full px-4 py-3 flex items-center gap-3 text-left transition-colors ${
                        index === selectedIndex ? 'bg-primary-50' : 'hover:bg-neutral-50'
                      }`}
                    >
                      <span className={`text-xs font-medium px-2 py-0.5 rounded ${getTypeColor(suggestion.type)}`}>
                        {getTypeIcon(suggestion.type)}
                      </span>
                      <span className="text-sm text-neutral-700 font-mono truncate">
                        {suggestion.label}
                      </span>
                    </button>
                  ))}
                </div>
              )}

              {/* Mobile Network Switcher (hidden when ≤1 network configured) */}
              <MobileNetworkList />

              {/* Mobile Navigation Links */}
              <div className="card overflow-hidden">
                <div className="px-4 py-2 border-b border-neutral-100 bg-neutral-50">
                  <span className="text-xs font-medium text-neutral-500 uppercase tracking-wider">Navigation</span>
                </div>
                {mobileNavItems.filter(item => item.to !== '/privacy' || privacyEnabled).map((item) => {
                  const Icon = item.icon;
                  return (
                    <Link
                      key={item.to}
                      to={item.to}
                      onClick={() => setMobileMenuOpen(false)}
                      className="flex items-center gap-3 px-4 py-3 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors border-b border-neutral-100 last:border-b-0"
                    >
                      <Icon className="w-5 h-5 text-neutral-500" />
                      <span className="text-sm text-neutral-700">{item.label}</span>
                    </Link>
                  );
                })}
              </div>

              {/* Mobile ETH Price - hidden for now
              {priceData && (
                <div className="card p-4">
                  <div className="mb-3">
                    <span className="text-xs font-medium text-neutral-500 uppercase tracking-wider">ETH Price</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-lg font-semibold text-neutral-900">
                      ${priceData.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </span>
                    {priceData.change24h !== undefined && (
                      <span className={`flex items-center gap-1 text-sm font-medium ${priceData.change24h >= 0 ? 'text-success-600' : 'text-error-600'}`}>
                        {priceData.change24h >= 0 ? (
                          <TrendingUp className="w-4 h-4" />
                        ) : (
                          <TrendingDown className="w-4 h-4" />
                        )}
                        {Math.abs(priceData.change24h).toFixed(2)}%
                      </span>
                    )}
                  </div>
                </div>
              )}
              */}

            </div>
          </div>
        </div>
      )}

      {/* Main Content */}
      <main className="flex-1 max-w-7xl mx-auto px-4 sm:px-6 py-4 sm:py-8 w-full">
        <div className="animate-fade-in">
          <Outlet />
        </div>
      </main>

      {/* Footer */}
      <footer className="mt-auto border-t border-neutral-200 bg-neutral-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-10">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            {/* About */}
            <div className="space-y-4">
              <Link to="/" className="flex items-center gap-2">
                <img src={branding.logo} alt={branding.name} className="w-8 h-8" />
                <span className="font-semibold text-neutral-900">{branding.name}</span>
              </Link>
              <p className="text-sm text-neutral-500 leading-relaxed">
                {branding.name} is a block explorer and analytics platform for EVM-based blockchains{branding.website && (<>, built by{' '}
                <a href={branding.website} target="_blank" rel="noopener noreferrer" className="text-primary hover:text-primary-600 transition-colors">{branding.company}</a></>)}.
                Search transactions, blocks, addresses, tokens, and more.
              </p>
              <AddNetworkButton variant="footer" />
            </div>

            {/* Quick Links */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-neutral-900">Explore</h3>
              <div className="grid grid-cols-2 gap-2">
                <Link to="/blocks" className="text-sm text-neutral-500 hover:text-primary transition-colors">Blocks</Link>
                <Link to="/tokens" className="text-sm text-neutral-500 hover:text-primary transition-colors">Tokens</Link>
                <Link to="/transactions" className="text-sm text-neutral-500 hover:text-primary transition-colors">Transactions</Link>
                <Link to="/token-transfers" className="text-sm text-neutral-500 hover:text-primary transition-colors">Token Transfers</Link>
                <Link to="/accounts" className="text-sm text-neutral-500 hover:text-primary transition-colors">Top Accounts</Link>
                {features().gasTracker && (
                  <Link to="/gas-tracker" className="text-sm text-neutral-500 hover:text-primary transition-colors">Gas Tracker</Link>
                )}
                {features().contractVerification && (
                  <Link to="/verify" className="text-sm text-neutral-500 hover:text-primary transition-colors">Verify Contract</Link>
                )}
              </div>
            </div>

            {/* Company & Socials */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-neutral-900">Company</h3>
              <div className="flex flex-col gap-2">
                {branding.website && <a href={branding.website} target="_blank" rel="noopener noreferrer" className="text-sm text-neutral-500 hover:text-primary transition-colors">About {branding.company}</a>}
                {branding.docs && <a href={branding.docs} target="_blank" rel="noopener noreferrer" className="text-sm text-neutral-500 hover:text-primary transition-colors">Documentation</a>}
                {branding.legal && <a href={branding.legal} target="_blank" rel="noopener noreferrer" className="text-sm text-neutral-500 hover:text-primary transition-colors">Legal Info</a>}
                {branding.security && <a href={branding.security} target="_blank" rel="noopener noreferrer" className="text-sm text-neutral-500 hover:text-primary transition-colors">Security</a>}
                {branding.trust && <a href={branding.trust} target="_blank" rel="noopener noreferrer" className="text-sm text-neutral-500 hover:text-primary transition-colors">Trust Center</a>}
              </div>
              <div className="flex items-center gap-3 pt-2">
                {branding.twitter && (
                <a href={branding.twitter} target="_blank" rel="noopener noreferrer" className="text-neutral-400 hover:text-neutral-700 transition-colors" aria-label="X (Twitter)">
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                  </svg>
                </a>
                )}
                {branding.linkedin && (
                <a href={branding.linkedin} target="_blank" rel="noopener noreferrer" className="text-neutral-400 hover:text-neutral-700 transition-colors" aria-label="LinkedIn">
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 01-2.063-2.065 2.064 2.064 0 112.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/>
                  </svg>
                </a>
                )}
                {branding.discord && (
                <a href={branding.discord} target="_blank" rel="noopener noreferrer" className="text-neutral-400 hover:text-neutral-700 transition-colors" aria-label="Discord">
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M20.317 4.37a19.791 19.791 0 00-4.885-1.515.074.074 0 00-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 00-5.487 0 12.64 12.64 0 00-.617-1.25.077.077 0 00-.079-.037A19.736 19.736 0 003.677 4.37a.07.07 0 00-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 00.031.057 19.9 19.9 0 005.993 3.03.078.078 0 00.084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 00-.041-.106 13.107 13.107 0 01-1.872-.892.077.077 0 01-.008-.128 10.2 10.2 0 00.372-.292.074.074 0 01.077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 01.078.01c.12.098.246.198.373.292a.077.077 0 01-.006.127 12.299 12.299 0 01-1.873.892.077.077 0 00-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 00.084.028 19.839 19.839 0 006.002-3.03.077.077 0 00.032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 00-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/>
                  </svg>
                </a>
                )}
                {branding.telegram && (
                <a href={branding.telegram} target="_blank" rel="noopener noreferrer" className="text-neutral-400 hover:text-neutral-700 transition-colors" aria-label="Telegram">
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M11.944 0A12 12 0 000 12a12 12 0 0012 12 12 12 0 0012-12A12 12 0 0012 0a12 12 0 00-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 01.171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.479.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z"/>
                  </svg>
                </a>
                )}
              </div>
            </div>
          </div>

          {/* Bottom bar */}
          <div className="mt-8 pt-6 border-t border-neutral-200 flex flex-col sm:flex-row items-center justify-between gap-3">
            <p className="text-xs text-neutral-400">
              &copy; {new Date().getFullYear()} {branding.company}. All rights reserved.
            </p>
            {branding.legal && (
            <div className="flex items-center gap-4 text-xs text-neutral-400">
              <a href={branding.legal} target="_blank" rel="noopener noreferrer" className="hover:text-neutral-600 transition-colors">Terms of Service</a>
              <a href={branding.legal} target="_blank" rel="noopener noreferrer" className="hover:text-neutral-600 transition-colors">Privacy Policy</a>
            </div>
            )}
          </div>
        </div>
      </footer>
    </div>
  );
}
