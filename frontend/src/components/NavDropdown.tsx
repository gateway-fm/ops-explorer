import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, LogIn, Shield, LogOut, Copy, Check } from 'lucide-react';
import { useAuth } from '../lib/auth';
import { redirectToLogin } from '../lib/login';
import { usePrivacyEnabled } from '../hooks/usePrivacyEnabled';
import { AddNetworkButton } from './AddNetworkButton';
import { formatDID } from '../lib/utils';
import { NetworkMenu } from './NetworkMenu';

import { ExplorerNavigation } from './ExplorerNavigation';


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
      data-testid="sign-in-button"
      className="flex items-center gap-1.5 px-3 py-2 text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-lg transition-colors text-sm font-medium"
    >
      <LogIn className="w-4 h-4" />
      Sign In
    </button>
  );
}

export function NavDropdown() {
  return <ExplorerNavigation account={<AuthButton />} networkButton={<AddNetworkButton variant="header" />} networkMenu={<NetworkMenu />} />;
}
