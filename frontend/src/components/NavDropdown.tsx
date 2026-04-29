import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, Boxes, ArrowLeftRight, Users, ShieldCheck, Coins, ArrowRightLeft, Fuel, Info, Settings, Sun, Moon, LogIn, Shield, LogOut, Copy, Check } from 'lucide-react';
import { useAuth } from '../lib/auth';
import { redirectToLogin } from '../lib/login';
import { usePrivacyEnabled } from '../hooks/usePrivacyEnabled';
import { MetaMaskFox } from './MetaMask';
import { addNetworkToMetaMask } from '../lib/metamask';
import { useTheme } from '../hooks/useTheme';
import { formatDID } from '../lib/utils';
import { getConfig } from '../lib/runtimeConfig';

const blockchainItems = [
  { to: '/blocks', label: 'Blocks', icon: Boxes },
  { to: '/transactions', label: 'Transactions', icon: ArrowLeftRight },
  { to: '/accounts', label: 'Top Accounts', icon: Users },
  { to: '/gas-tracker', label: 'Gas Tracker', icon: Fuel },
  { to: '/verify', label: 'Verify Contract', icon: ShieldCheck },
  { to: '/chain-info', label: 'Chain Info', icon: Info },
];

const tokenItems = [
  { to: '/tokens', label: 'Tokens', icon: Coins },
  { to: '/token-transfers', label: 'Token Transfers', icon: ArrowRightLeft },
];

interface DropdownProps {
  label: string;
  items: { to: string; label: string; icon: React.ComponentType<{ className?: string }> }[];
}

function Dropdown({ label, items }: DropdownProps) {
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


function AuthButton() {
  const privacyEnabled = usePrivacyEnabled();
  const { isAuthenticated, auth, logout } = useAuth();
  const [showMenu, setShowMenu] = useState(false);
  const [copied, setCopied] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  function copyDid() {
    if (!auth.did) return;
    navigator.clipboard.writeText(auth.did);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  if (!privacyEnabled) return null;

  if (isAuthenticated) {
    return (
      <div ref={menuRef} className="relative">
        <button
          onClick={() => setShowMenu(!showMenu)}
          title={auth.did || undefined}
          className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
        >
          <Shield className="w-4 h-4 text-success-500" />
          <span className="font-mono text-xs">
            {auth.did ? formatDID(auth.did) : 'Signed in'}
          </span>
          <ChevronDown className={`w-4 h-4 transition-transform ${showMenu ? 'rotate-180' : ''}`} />
        </button>

        {showMenu && (
          <div className="absolute top-full right-0 mt-2 w-64 card overflow-hidden z-50 shadow-elevated">
            {auth.did && (
              <div className="px-4 py-3 border-b border-neutral-100">
                <div className="text-xs text-neutral-400 mb-1">Your DID</div>
                <div className="flex items-center gap-1">
                  <span className="font-mono text-xs text-neutral-700 flex-1 truncate" title={auth.did}>
                    {auth.did}
                  </span>
                  <button
                    onClick={copyDid}
                    className="shrink-0 p-1 text-neutral-400 hover:text-neutral-700 transition-colors"
                    title="Copy DID"
                  >
                    {copied ? <Check className="w-3.5 h-3.5 text-success-500" /> : <Copy className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>
            )}
            <Link
              to="/privacy"
              onClick={() => setShowMenu(false)}
              className="flex items-center gap-3 px-4 py-3 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors"
            >
              <Shield className="w-4 h-4 text-neutral-500" />
              <span className="text-sm text-neutral-700">Auditor</span>
            </Link>
            <button
              onClick={() => { logout(); setShowMenu(false); }}
              className="w-full flex items-center gap-3 px-4 py-3 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors"
            >
              <LogOut className="w-4 h-4 text-neutral-500" />
              <span className="text-sm text-neutral-700">Sign out</span>
            </button>
          </div>
        )}
      </div>
    );
  }

  return (
    <button
      onClick={() => redirectToLogin()}
      className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
    >
      <LogIn className="w-4 h-4" />
      Sign In
    </button>
  );
}

const TARGET_CHAIN_ID = '0x' + Number(getConfig('VITE_CHAIN_ID', '1001')).toString(16);

export function NavDropdown() {
  const [networkAdded, setNetworkAdded] = useState(false);

  useEffect(() => {
    if (!window.ethereum) return;

    const checkChain = (chainId: string) => {
      setNetworkAdded(chainId.toLowerCase() === TARGET_CHAIN_ID.toLowerCase());
    };

    window.ethereum.request({ method: 'eth_chainId' }).then(checkChain).catch(() => {});

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const eth = window.ethereum as any;
    eth.on('chainChanged', checkChain);
    return () => {
      eth.removeListener('chainChanged', checkChain);
    };
  }, []);

  return (
    <div className="flex items-center gap-1">
      {!networkAdded && (
        <button
          onClick={addNetworkToMetaMask}
          className="flex items-center gap-1.5 px-3 py-2 text-amber-700 hover:text-amber-900 bg-amber-50 hover:bg-amber-100 border border-amber-200 rounded-lg transition-colors text-sm font-medium dark:text-amber-300 dark:bg-amber-900/30 dark:hover:bg-amber-900/50 dark:border-amber-700"
        >
          <MetaMaskFox className="w-4 h-4" />
          Add Network
        </button>
      )}
      <Dropdown label="Blockchain" items={blockchainItems} />
      <Dropdown label="Tokens" items={tokenItems} />
      <Link
        to="/stats"
        className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
      >
        Charts
      </Link>
      <Link
        to="/api-docs"
        className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
      >
        API
      </Link>
      <SettingsDropdown />
      <AuthButton />
    </div>
  );
}
