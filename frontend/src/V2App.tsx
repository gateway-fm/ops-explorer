import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Link, Outlet, Routes, Route } from 'react-router-dom';
import { AuthContext, useAuth } from './lib/authContext';
import { V2AuthProvider } from './lib/authV2';
import { appSession } from './lib/v2Session';
import { TooltipProvider } from './components/ui/tooltip';
import { ThemeProvider } from './hooks/useTheme';
import { getConfig } from './lib/runtimeConfig';
import { PublicNetworkSetup } from './pages/NetworkSetupV2';
import { WalletsV2 } from './pages/WalletsV2';

const queries = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false, staleTime: 1000 } } });
function Layout() {
  const { auth, logout } = useAuth();
  return <div className="max-w-7xl mx-auto p-5 space-y-5"><header className="space-y-3"><h1>Explorer</h1><p className="break-all" data-field="app-actor">{auth.did}</p><nav className="flex flex-wrap gap-4"><Link to="/">Overview</Link><Link to="/blocks">Blocks</Link><Link to="/transactions">Transactions</Link><Link to="/accounts">Accounts</Link><Link to="/tokens">Tokens</Link><Link to="/token-transfers">Transfers</Link><Link to="/wallets">Wallets</Link><Link to="/chain-info">Network setup</Link><button onClick={() => { void appSession.login(undefined, true).catch(() => {}); }}>Refresh identity</button><button onClick={() => { void appSession.login().catch(() => {}); }}>Switch account</button><button onClick={() => { void logout().catch(() => {}); }}>Sign out</button></nav></header><Outlet /></div>;
}

// V2 never mounts the V1 impersonation, disclosure-consent, ABI-write, wallet
// provider or refresh-token components.
export function V2App() {
  if (window.self !== window.top) return <main><h1>Open Explorer to continue</h1><p>Private sessions require a top-level browser tab.</p><a href={window.location.href} target="_top">Open Explorer</a></main>;
  return <ThemeProvider><QueryClientProvider client={queries}><V2AuthProvider context={AuthContext}><TooltipProvider><BrowserRouter basename={getConfig('VITE_BASE_PATH', '')}><Routes><Route path="/" element={<Layout />}><Route index element={<UnavailableData />} /><Route path="chain-info" element={<PublicNetworkSetup />} /><Route path="wallets" element={<WalletsV2 />} /><Route path="*" element={<UnavailableData />} /></Route></Routes></BrowserRouter></TooltipProvider></V2AuthProvider></QueryClientProvider></ThemeProvider>;
}

function UnavailableData() { return <section role="status"><h2>Private data is unavailable</h2><p>No chain data is shown while the current privacy read service is unavailable.</p></section>; }
