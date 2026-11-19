import { useState, useRef, useEffect, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, Boxes, ArrowLeftRight, Users, ShieldCheck, Coins, ArrowRightLeft, Fuel, Info, Settings, Sun, Moon } from 'lucide-react';
import { useTheme } from '../hooks/useTheme';
import { features } from '../lib/features';

const blockchainItems = [
  { to: '/blocks', label: 'Blocks', icon: Boxes },
  { to: '/transactions', label: 'Transactions', icon: ArrowLeftRight },
  { to: '/accounts', label: 'Top Accounts', icon: Users },
  // "Gas Tracker" is hidden in privacy mode (RD-1063) — /gas is gated off on
  // the backend and the page early-returns FeatureUnavailable.
  ...(features().gasTracker
    ? [{ to: '/gas-tracker', label: 'Gas Tracker', icon: Fuel }]
    : []),
  // "Verify Contract" is hidden in privacy mode — the surface is
  // compiled out on the privacy build (see lib/features.ts).
  ...(features().contractVerification
    ? [{ to: '/verify', label: 'Verify Contract', icon: ShieldCheck }]
    : []),
  { to: '/chain-info', label: 'Chain Info', icon: Info },
];

const tokenItems = [
  { to: '/tokens', label: 'Tokens', icon: Coins },
  { to: '/token-transfers', label: 'Token Transfers', icon: ArrowRightLeft },
];

interface DropdownProps {
  label: string;
  items: { to: string; label: string; icon: React.ComponentType<{ className?: string }> }[];
  testid?: string;
}

function Dropdown({ label, items, testid }: DropdownProps) {
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

  return (
    <div ref={dropdownRef} className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        data-testid={testid}
        aria-expanded={isOpen}
        className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
      >
        {label}
        <ChevronDown className={`w-4 h-4 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute top-full right-0 mt-2 w-48 card overflow-hidden z-50 shadow-elevated">
          {items.map((item) => {
            const Icon = item.icon;
            return (
              <Link
                key={item.to}
                to={item.to}
                data-testid="nav-link"
                onClick={() => setIsOpen(false)}
                className="flex items-center gap-3 px-4 py-3 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors"
              >
                <Icon className="w-4 h-4 text-neutral-500" />
                <span className="text-sm text-neutral-700">{item.label}</span>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}

const themeOptions = [
  { value: 'light' as const, label: 'Light', icon: Sun },
  { value: 'dark' as const, label: 'Dark', icon: Moon },
];

function SettingsDropdown() {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const { theme, setTheme } = useTheme();

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <div ref={dropdownRef} className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        data-testid="theme-menu-trigger"
        aria-expanded={isOpen}
        className="flex items-center justify-center w-9 h-9 text-neutral-500 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors"
        title="Settings"
      >
        <Settings className="w-4 h-4" />
      </button>

      {isOpen && (
        <div className="absolute top-full right-0 mt-2 w-48 card overflow-hidden z-50 shadow-elevated">
          <div className="px-4 py-2 border-b border-neutral-100">
            <span className="text-xs font-medium text-neutral-400 uppercase tracking-wider">Theme</span>
          </div>
          {themeOptions.map((opt) => {
            const Icon = opt.icon;
            const active = theme === opt.value;
            return (
              <button
                key={opt.value}
                onClick={() => setTheme(opt.value)}
                data-testid={`theme-option-${opt.value}`}
                aria-selected={active}
                className={`w-full flex items-center gap-3 px-4 py-3 transition-colors ${
                  active
                    ? 'bg-primary-50 dark:bg-primary-900/20 text-primary dark:text-primary-400'
                    : 'hover:bg-neutral-50 text-neutral-700'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span className="text-sm">{opt.label}</span>
                {active && <span className="ml-auto w-1.5 h-1.5 rounded-full bg-primary" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// Slots leave authentication and network setup with the owning protocol.
export function ExplorerNavigation({ account, networkButton, networkMenu, accountLabel = 'Top Accounts', nativeReads = false }: {
  account: ReactNode; networkButton?: ReactNode; networkMenu?: ReactNode; accountLabel?: string; nativeReads?: boolean;
}) {
  return (
    <div className="flex items-center gap-1">
      {networkButton}
      <Dropdown label="Blockchain" items={[...blockchainItems.map(item => item.to === '/accounts' ? { ...item, label: accountLabel } : item), ...(nativeReads?[{to:'/logs',label:'Event Logs',icon:Info},{to:'/sync',label:'Network Sync',icon:Info}]:[])]} testid="nav-blockchain" />
      <Dropdown label="Tokens" items={tokenItems} testid="nav-tokens" />
      {features().charts && <Link to="/stats" className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium">Charts</Link>}
      <Link to="/api-docs" className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium">API</Link>
      <SettingsDropdown />
      {account}
      {networkMenu}
    </div>
  );
}
