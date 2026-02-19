import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, Boxes, ArrowLeftRight, Users, ShieldCheck, Coins, ArrowRightLeft, Fuel, LogIn, Shield, LogOut } from 'lucide-react';
import { useAuth } from '../lib/auth';
import { PrivadoLogin } from './PrivadoLogin';
import { usePrivacyEnabled } from '../hooks/usePrivacyEnabled';

const blockchainItems = [
  { to: '/blocks', label: 'Blocks', icon: Boxes },
  { to: '/transactions', label: 'Transactions', icon: ArrowLeftRight },
  { to: '/accounts', label: 'Top Accounts', icon: Users },
  { to: '/gas-tracker', label: 'Gas Tracker', icon: Fuel },
  { to: '/verify', label: 'Verify Contract', icon: ShieldCheck },
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
                className="flex items-center gap-3 px-4 py-3 hover:bg-primary-50 transition-colors"
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

function AuthButton() {
  const privacyEnabled = usePrivacyEnabled();
  const { isAuthenticated, ssoAuth, privadoLogout } = useAuth();
  const [showLogin, setShowLogin] = useState(false);
  const [showMenu, setShowMenu] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

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
          className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
        >
          <Shield className="w-4 h-4 text-success-500" />
          <span className="font-mono text-xs max-w-[100px] truncate">
            {ssoAuth.did ? `${ssoAuth.did.slice(0, 16)}...` : 'Signed in'}
          </span>
          <ChevronDown className={`w-4 h-4 transition-transform ${showMenu ? 'rotate-180' : ''}`} />
        </button>

        {showMenu && (
          <div className="absolute top-full right-0 mt-2 w-48 card overflow-hidden z-50 shadow-elevated">
            <Link
              to="/privacy"
              onClick={() => setShowMenu(false)}
              className="flex items-center gap-3 px-4 py-3 hover:bg-primary-50 transition-colors"
            >
              <Shield className="w-4 h-4 text-neutral-500" />
              <span className="text-sm text-neutral-700">Privacy</span>
            </Link>
            <button
              onClick={() => { privadoLogout(); setShowMenu(false); }}
              className="w-full flex items-center gap-3 px-4 py-3 hover:bg-primary-50 transition-colors"
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
    <>
      <button
        onClick={() => setShowLogin(true)}
        className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
      >
        <LogIn className="w-4 h-4" />
        Sign in
      </button>
      {showLogin && <PrivadoLogin onClose={() => setShowLogin(false)} />}
    </>
  );
}

function MetaMaskFox({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 35 33" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M32.96 1L19.7 10.86l2.45-5.81L32.96 1z" fill="#E2761B" stroke="#E2761B" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M2.02 1l13.14 9.96-2.33-5.91L2.02 1zM28.15 23.53l-3.53 5.4 7.55 2.08 2.17-7.35-6.19-.13zM.68 23.66l2.15 7.35 7.55-2.08-3.53-5.4-6.17.13z" fill="#E4761B" stroke="#E4761B" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M10.03 14.52l-2.1 3.18 7.49.34-.26-8.06-5.13 4.54zM24.95 14.52l-5.2-4.63-.17 8.15 7.48-.34-2.11-3.18zM10.38 28.93l4.52-2.2-3.89-3.04-.63 5.24zM20.08 26.73l4.53 2.2-.64-5.24-3.89 3.04z" fill="#E4761B" stroke="#E4761B" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M24.61 28.93l-4.53-2.2.36 2.96-.04 1.25 4.21-1.99zM10.38 28.93l4.21 2.01-.03-1.25.35-2.96-4.53 2.2z" fill="#D7C1B3" stroke="#D7C1B3" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M14.67 21.83l-3.76-1.1 2.65-1.22 1.11 2.32zM20.31 21.83l1.11-2.32 2.66 1.22-3.77 1.1z" fill="#233447" stroke="#233447" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M10.38 28.93l.65-5.4-4.18.13 3.53 5.27zM23.96 23.53l.65 5.4 3.54-5.27-4.19-.13zM27.06 17.7l-7.48.34.7 3.79 1.11-2.32 2.66 1.22 3.01-3.03zM10.91 20.73l2.66-1.22 1.1 2.32.7-3.79-7.48-.34 3.02 3.03z" fill="#CD6116" stroke="#CD6116" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M7.89 17.7l3.13 6.1-.1-3.07-3.03-3.03zM24.05 20.73l-.12 3.07 3.13-6.1-3.01 3.03zM15.37 18.04l-.7 3.79.88 4.54.2-5.98-.38-2.35zM19.58 18.04l-.37 2.34.18 6 .89-4.55-.7-3.79z" fill="#E4751F" stroke="#E4751F" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M20.28 21.83l-.89 4.55.64.44 3.89-3.04.12-3.07-3.76 1.12zM10.91 20.73l.1 3.05 3.89 3.04.64-.44-.88-4.54-3.75-1.11z" fill="#F6851B" stroke="#F6851B" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M20.32 30.94l.04-1.25-.34-.29h-5.06l-.32.29.03 1.25-4.21-2.01 1.47 1.2 2.98 2.07h5.15l2.99-2.07 1.47-1.2-4.2 2.01z" fill="#C0AD9E" stroke="#C0AD9E" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M20.08 26.73l-.64-.44h-3.9l-.64.44-.35 2.96.32-.29h5.06l.34.29-.19-2.96z" fill="#161616" stroke="#161616" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M33.52 11.35l1.13-5.44L32.96 1l-12.88 9.56 4.95 4.18 7 2.04 1.54-1.8-.67-.49 1.07-.97-.82-.64 1.07-.81-.7-.54zM.33 5.91l1.13 5.44-.72.54 1.07.8-.81.65 1.07.97-.68.49 1.54 1.8 7-2.04 4.95-4.18L2.02 1 .33 5.91z" fill="#763D16" stroke="#763D16" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M32.03 16.78l-7-2.04 2.11 3.18-3.13 6.1 4.14-.05h6.19l-2.31-7.19zM10.03 14.74l-7 2.04-2.33 7.19h6.17l4.14.05-3.13-6.1 2.15-3.18zM19.58 18.04l.44-7.72 2.03-5.47H12.83l2.02 5.47.45 7.72.17 2.36.01 5.97h3.9l.02-5.97.18-2.36z" fill="#F6851B" stroke="#F6851B" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

function addNetworkToMetaMask() {
  if (!window.ethereum) {
    alert('MetaMask is not installed. Please install MetaMask to add the network.');
    return;
  }
  window.ethereum.request({
    method: 'wallet_addEthereumChain',
    params: [{
      chainId: '0x' + Number(import.meta.env.VITE_CHAIN_ID || '1001').toString(16),
      chainName: import.meta.env.VITE_NETWORK_NAME || 'Gateway',
      nativeCurrency: { name: 'Ether', symbol: import.meta.env.VITE_NETWORK_CURRENCY || 'ETH', decimals: 18 },
      rpcUrls: [import.meta.env.VITE_RPC_URL || 'http://localhost:8545'],
    }],
  });
}

const TARGET_CHAIN_ID = '0x' + Number(import.meta.env.VITE_CHAIN_ID || '1001').toString(16);

export function NavDropdown() {
  const [networkAdded, setNetworkAdded] = useState(false);

  useEffect(() => {
    if (!window.ethereum) return;

    const checkChain = (chainId: string) => {
      setNetworkAdded(chainId.toLowerCase() === TARGET_CHAIN_ID.toLowerCase());
    };

    window.ethereum.request({ method: 'eth_chainId' }).then(checkChain).catch(() => {});

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
          className="flex items-center gap-1.5 px-3 py-2 text-amber-700 hover:text-amber-900 bg-amber-50 hover:bg-amber-100 border border-amber-200 rounded-lg transition-colors text-sm font-medium"
        >
          <MetaMaskFox className="w-4 h-4" />
          Add Network
        </button>
      )}
      <Dropdown label="Blockchain" items={blockchainItems} />
      <Dropdown label="Tokens" items={tokenItems} />
      <AuthButton />
    </div>
  );
}
