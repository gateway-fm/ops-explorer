import { normalizeRouteIdentifier } from '../lib/nativeRoute';
import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { ArrowLeftRight, ArrowRightLeft, BookOpen, Boxes, ChevronDown, Coins, Eye, Info, LogOut, LogIn, Loader2, RefreshCw, Shield, Users, Wallet } from 'lucide-react';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeSearchSuggestions } from '../lib/nativeReadModels';
import { readTarget } from '../lib/v2Reads';
import { appSession } from '../lib/v2Session';
import { useAuth } from '../lib/authContext';
import { features } from '../lib/features';
import { ExplorerChrome } from './ExplorerChrome';
import { ExplorerNavigation } from './ExplorerNavigation';
import { SearchBarPresentation } from './SearchBarPresentation';
import { MetaMaskFox } from './MetaMask';
import { CopyButton } from './CopyButton';
import type { ReactNode } from 'react';

type Suggestion = ReturnType<typeof decodeSearchSuggestions>[number];
const suggestionPath = (row: Suggestion) => row.type === 'block' ? `/block/${row.value}` : row.type === 'transaction' ? `/tx/${row.value}` : row.type === 'token' ? `/token/${row.value}` : `/address/${row.value}${row.type === 'contract' ? '?tab=contract' : ''}`;
function directPath(raw: string): string | undefined {
  const hash = normalizeRouteIdentifier(raw, 'hash'); if (hash) return `/tx/${hash}`;
  const address = normalizeRouteIdentifier(raw, 'address'); if (address) return `/address/${address}`;
  const block = normalizeRouteIdentifier(raw, 'block'); if (block) return `/block/${block}`;
}

function useNativeSearch(keyboardShortcut = false, enabled = true) {
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState('');
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const clean = search.trim();
  const result = useNativeRead(enabled && clean && clean === query ? readTarget('/search/suggestions', new URLSearchParams({ q: query, limit: '20' })) : null, decodeSearchSuggestions);
  const suggestions = result.data ?? [];
  useEffect(() => { const timer = setTimeout(() => setQuery(clean), 150); return () => clearTimeout(timer); }, [clean]);
  useEffect(() => {
    const outside = (event: MouseEvent) => { if (!containerRef.current?.contains(event.target as Node)) setShowSuggestions(false); };
    const shortcut = (event: globalThis.KeyboardEvent) => {
      if (keyboardShortcut && event.key === '/' && !(event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) && !(event.target instanceof HTMLElement && event.target.isContentEditable)) { event.preventDefault(); inputRef.current?.focus(); }
    };
    document.addEventListener('mousedown', outside); document.addEventListener('keydown', shortcut);
    return () => { document.removeEventListener('mousedown', outside); document.removeEventListener('keydown', shortcut); };
  }, [keyboardShortcut]);
  const reset = () => { setSearch(''); setQuery(''); setSelectedIndex(-1); setShowSuggestions(false); };
  const navigateToSuggestion = (row: Suggestion) => { navigate(suggestionPath(row)); reset(); };
  const updateSearch = (value: string) => { setSearch(value.slice(0, 128)); setSelectedIndex(-1); setShowSuggestions(true); };
  const handleSearch = (event: FormEvent) => {
    event.preventDefault();
    if (showSuggestions && selectedIndex >= 0 && suggestions[selectedIndex]) { navigateToSuggestion(suggestions[selectedIndex]); return; }
    const target = directPath(clean);
    if (target) { navigate(target); reset(); }
    else { setShowSuggestions(true); inputRef.current?.focus(); }
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') { setShowSuggestions(false); setSelectedIndex(-1); return; }
    if (!showSuggestions || !suggestions.length) return;
    if (event.key === 'ArrowDown') { event.preventDefault(); setSelectedIndex(value => Math.min(value + 1, suggestions.length - 1)); }
    if (event.key === 'ArrowUp') { event.preventDefault(); setSelectedIndex(value => Math.max(value - 1, -1)); }
  };
  return { search, setSearch: updateSearch, suggestions, showSuggestions, setShowSuggestions, selectedIndex, inputRef, containerRef, handleSearch, handleKeyDown, navigateToSuggestion, error: result.error, loading: result.loading || !!clean && clean !== query };
}

export function NativeSearchBar({ variant = 'default' }: { variant?: 'hero' | 'default' }) {
  const search = useNativeSearch(true);
  return <><SearchBarPresentation variant={variant} {...search} />{search.error && <p role="alert" className="mt-2 text-sm text-error-600">{search.error}</p>}</>;
}

function NativeAccountMenu() {
  const { auth, logout, isLoading } = useAuth();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  const menu = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const outside = (event: MouseEvent) => { if (!menu.current?.contains(event.target as Node)) setOpen(false); };
    const escape = (event: globalThis.KeyboardEvent) => { if (event.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', outside); document.addEventListener('keydown', escape);
    return () => { document.removeEventListener('mousedown', outside); document.removeEventListener('keydown', escape); };
  }, []);
  const run = (operation: () => Promise<unknown>) => { if (pending) return; setPending(true); setError(undefined); void operation().catch(() => setError('The session action could not be completed. Try again.')).finally(() => setPending(false)); };
  if (!auth.authenticated) return <div className="relative">
    <button type="button" disabled={isLoading || pending} aria-busy={isLoading || pending}
      onClick={() => run(() => appSession.login())}
      className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium disabled:opacity-50 disabled:cursor-wait">
      {isLoading || pending ? <Loader2 className="w-4 h-4 animate-spin" aria-hidden="true" /> : <LogIn className="w-4 h-4" aria-hidden="true" />}
      {isLoading ? 'Checking session…' : pending ? 'Opening sign in…' : 'Sign in'}
    </button>
    {error && <p role="alert" className="absolute right-0 top-full mt-2 w-64 card p-3 text-sm text-error-600">{error}</p>}
  </div>;
  return <div ref={menu} className="relative">
    <button aria-expanded={open} aria-label="Account menu" onClick={() => setOpen(!open)} title={auth.did ?? undefined} className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"><Shield className="w-4 h-4 text-success-500" /><span className="font-mono text-xs max-w-[140px] truncate" data-field="app-actor">{auth.did ?? 'Signed in'}</span><ChevronDown className={`w-4 h-4 transition-transform ${open ? 'rotate-180' : ''}`} /></button>
    {open && <div className="absolute top-full right-0 mt-2 w-72 card overflow-hidden z-50 shadow-elevated">
      <div className="px-4 py-3 border-b border-neutral-100"><div className="text-xs text-neutral-400 mb-1">Your identity</div><div className="flex gap-1 items-center"><span className="font-mono text-xs text-neutral-700 flex-1 truncate" title={auth.did ?? undefined}>{auth.did}</span>{auth.did && <CopyButton text={auth.did} label="Copy identity"/>}</div></div>
      <Link to="/wallets" onClick={() => setOpen(false)} className="flex gap-3 px-4 py-3 hover:bg-primary-50"><Wallet className="w-4 h-4 text-neutral-500" /><span className="text-sm text-neutral-700">Wallets</span></Link>
      <Link to="/disclosures" onClick={() => setOpen(false)} className="flex gap-3 px-4 py-3 hover:bg-primary-50"><Eye className="w-4 h-4 text-neutral-500" /><span className="text-sm text-neutral-700">Auditor</span></Link>
      <button onClick={() => run(() => appSession.login(undefined, true))} className="w-full flex gap-3 px-4 py-3 hover:bg-primary-50"><RefreshCw className="w-4 h-4 text-neutral-500" /><span className="text-sm text-neutral-700">Refresh identity</span></button>
      <button onClick={() => run(() => appSession.login())} className="w-full flex gap-3 px-4 py-3 hover:bg-primary-50"><Users className="w-4 h-4 text-neutral-500" /><span className="text-sm text-neutral-700">Switch account</span></button>
      <button onClick={() => run(logout)} className="w-full flex gap-3 px-4 py-3 hover:bg-primary-50"><LogOut className="w-4 h-4 text-neutral-500" /><span className="text-sm text-neutral-700">Sign out</span></button>
      {error && <p role="alert" className="px-4 pb-3 text-sm text-error-600">{error}</p>}
    </div>}
  </div>;
}
function NetworkSetupButton() {
  return <Link to="/chain-info" className="flex items-center gap-1.5 px-3 py-2 rounded-lg font-medium transition-colors text-sm text-amber-700 hover:text-amber-900 bg-amber-50 hover:bg-amber-100 border border-amber-200 dark:text-amber-300 dark:bg-amber-900/30 dark:border-amber-700"><MetaMaskFox className="w-4 h-4" />Network setup</Link>;
}

const mobileItems = [
  { to: '/blocks', label: 'Blocks', icon: Boxes }, { to: '/transactions', label: 'Transactions', icon: ArrowLeftRight },
  { to: '/accounts', label: 'Accounts', icon: Users }, { to: '/tokens', label: 'Tokens', icon: Coins },
  { to: '/token-transfers', label: 'Token Transfers', icon: ArrowRightLeft }, { to: '/chain-info', label: 'Chain Info', icon: Info },
  { to: '/logs', label: 'Event Logs', icon: Info }, { to: '/sync', label: 'Network Sync', icon: RefreshCw }, { to: '/api-docs', label: 'API Docs', icon: BookOpen }, { to: '/disclosures', label: 'Auditor', icon: Eye }, { to: '/wallets', label: 'Wallets', icon: Wallet },
];
export function NativeExplorerLayout({ children }: { children: ReactNode }) {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const { isAuthenticated } = useAuth();
  const search = useNativeSearch(false, isAuthenticated);
  const location = useLocation();
  useEffect(() => {
    // A completed route transition closes the mobile overlay.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMobileMenuOpen(false);
  }, [location.pathname, location.search]);
  useEffect(() => { const prior = document.body.style.overflow; if (mobileMenuOpen) document.body.style.overflow = 'hidden'; return () => { document.body.style.overflow = prior; }; }, [mobileMenuOpen]);
  return <ExplorerChrome privacyEnabled searchEnabled={isAuthenticated} featureFlags={features()} accountLabel="Accounts" navigation={<ExplorerNavigation nativeReads account={<NativeAccountMenu />} networkButton={<NetworkSetupButton />} accountLabel="Accounts" />} networkButton={<NetworkSetupButton />} mobileAccount={<NativeAccountMenu />} mobileNavItems={mobileItems} mobileMenuOpen={mobileMenuOpen} setMobileMenuOpen={setMobileMenuOpen} search={search.search} setSearch={search.setSearch} suggestions={search.suggestions} showSuggestions={search.showSuggestions} setShowSuggestions={search.setShowSuggestions} searchContainerRef={search.containerRef} searchError={search.error} searchLoading={search.loading} selectedIndex={search.selectedIndex} mobileInputRef={search.inputRef} handleSearch={search.handleSearch} handleKeyDown={search.handleKeyDown} navigateToSuggestion={search.navigateToSuggestion}>{children}</ExplorerChrome>;
}
