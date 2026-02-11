import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useState, useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search, Menu, X, ArrowLeftRight, Users, Coins, ShieldCheck, TrendingUp, TrendingDown, Boxes, Fuel } from 'lucide-react';
import { api } from '../lib/api';
import type { SearchSuggestion } from '../lib/api';
import { NavDropdown } from './NavDropdown';

const mobileNavItems = [
  { to: '/blocks', label: 'Blocks', icon: Boxes },
  { to: '/transactions', label: 'Transactions', icon: ArrowLeftRight },
  { to: '/accounts', label: 'Top Accounts', icon: Users },
  { to: '/tokens', label: 'Tokens', icon: Coins },
  { to: '/token-transfers', label: 'Token Transfers', icon: ArrowLeftRight },
  { to: '/gas-tracker', label: 'Gas Tracker', icon: Fuel },
  { to: '/verify', label: 'Verify Contract', icon: ShieldCheck },
];

export function Layout() {
  const [search, setSearch] = useState('');
  const [suggestions, setSuggestions] = useState<SearchSuggestion[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const navigate = useNavigate();
  const searchRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const mobileInputRef = useRef<HTMLInputElement>(null);

  // Fetch ETH price
  const { data: priceData } = useQuery({
    queryKey: ['ethPrice'],
    queryFn: api.getPrice,
    refetchInterval: 60000,
    staleTime: 30000,
  });

  // Close mobile menu on route change
  useEffect(() => {
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

  // Global keyboard shortcut to focus search
  useEffect(() => {
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }
      if (e.key === '/') {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };

    document.addEventListener('keydown', handleGlobalKeyDown);
    return () => document.removeEventListener('keydown', handleGlobalKeyDown);
  }, []);

  // Debounced search suggestions
  useEffect(() => {
    if (!search.trim() || search.length < 1) {
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
      {/* Navigation Header */}
      <header className="nav">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-3 sm:py-4">
          <div className="flex items-center justify-between gap-2 sm:gap-4">
            {/* Logo and ETH Price */}
            <div className="flex items-center gap-2 sm:gap-3 shrink-0">
              <Link to="/" className="w-8 h-8 sm:w-10 sm:h-10 rounded-xl bg-primary flex items-center justify-center shadow-primary hover:opacity-80 transition-opacity">
                <Boxes className="w-4 h-4 sm:w-5 sm:h-5 text-white" />
              </Link>
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-neutral-100 border border-neutral-200">
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
            </div>

            {/* Desktop Search Bar */}
            <div ref={searchRef} className="hidden md:block flex-1 max-w-xl relative">
              <form onSubmit={handleSearch}>
                <div className="relative">
                  <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-neutral-400" />
                  <input
                    ref={inputRef}
                    type="text"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    onFocus={() => suggestions.length > 0 && setShowSuggestions(true)}
                    onKeyDown={handleKeyDown}
                    placeholder="Search by address, tx hash, or block..."
                    className="input pl-11 pr-12"
                  />
                  <kbd className="absolute right-3 top-1/2 -translate-y-1/2 px-1.5 py-0.5 text-xs font-mono text-neutral-400 bg-neutral-100 border border-neutral-200 rounded">
                    /
                  </kbd>
                </div>
              </form>

              {/* Desktop Suggestions dropdown */}
              {showSuggestions && suggestions.length > 0 && (
                <div className="absolute top-full left-0 right-0 mt-2 card overflow-hidden z-50 shadow-elevated max-h-80 overflow-y-auto">
                  {suggestions.map((suggestion, index) => (
                    <button
                      key={`${suggestion.type}-${suggestion.value}`}
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
        <div className="fixed inset-0 z-30 md:hidden">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/20 backdrop-blur-sm"
            onClick={() => setMobileMenuOpen(false)}
          />

          {/* Menu Panel */}
          <div className="absolute top-[57px] left-0 right-0 bottom-0 overflow-y-auto bg-white border-t border-neutral-200">
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
                    placeholder="Search..."
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

              {/* Mobile Navigation Links */}
              <div className="card overflow-hidden">
                <div className="px-4 py-2 border-b border-neutral-100 bg-neutral-50">
                  <span className="text-xs font-medium text-neutral-500 uppercase tracking-wider">Navigation</span>
                </div>
                {mobileNavItems.map((item) => {
                  const Icon = item.icon;
                  return (
                    <Link
                      key={item.to}
                      to={item.to}
                      onClick={() => setMobileMenuOpen(false)}
                      className="flex items-center gap-3 px-4 py-3 hover:bg-primary-50 transition-colors border-b border-neutral-100 last:border-b-0"
                    >
                      <Icon className="w-5 h-5 text-neutral-500" />
                      <span className="text-sm text-neutral-700">{item.label}</span>
                    </Link>
                  );
                })}
              </div>

              {/* Mobile ETH Price */}
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
      <footer className="mt-auto pt-8">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-6">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-2 text-sm text-neutral-600">
              <span>Built by</span>
              <a
                href="https://gateway.fm/"
                target="_blank"
                rel="noopener noreferrer"
                className="font-semibold text-neutral-900 hover:text-primary transition-colors"
              >
                Gateway FM
              </a>
            </div>
            <div className="flex items-center gap-4">
              <a
                href="https://x.com/intent/follow?screen_name=gateway_eth"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-sm text-neutral-500 hover:text-neutral-900 transition-colors"
              >
                <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                </svg>
                <span>@gateway_eth</span>
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
