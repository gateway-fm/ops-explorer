import { useSyncExternalStore } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Link, Outlet, Routes, Route, Navigate } from 'react-router-dom';
import { AuthContext, useAuth } from './lib/authContext';
import { V2AuthProvider } from './lib/authV2';
import { appSession } from './lib/v2Session';
import { TooltipProvider } from './components/ui/tooltip';
import { ThemeProvider } from './hooks/useTheme';
import { getConfig } from './lib/runtimeConfig';
import { PublicNetworkSetup } from './pages/NetworkSetupV2';
import { WalletsV2 } from './pages/WalletsV2';
import { OverviewV2, ReadPanel, BlockV2, TransactionV2, TransactionReadV2, AddressV2, AddressReadV2, TokenV2, TokenReadV2, SyncV2, SearchV2 } from './pages/ExplorerDataV2';
import { DisclosuresV2, DisclosureCaseV2, DisclosureAddressV2, DisclosureTransactionV2 } from './pages/DisclosuresV2';

const queries = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false, staleTime: 1000 } } });
function Layout() {
  const { auth, logout } = useAuth();
  return <div className="max-w-7xl mx-auto p-5 space-y-5"><header className="space-y-3"><h1>Explorer</h1><p className="break-all" data-field="app-actor">{auth.did}</p><nav className="flex flex-wrap gap-4"><Link to="/">Overview</Link><Link to="/blocks">Blocks</Link><Link to="/transactions">Transactions</Link><Link to="/accounts">Accounts</Link><Link to="/tokens">Tokens</Link><Link to="/token-transfers">Transfers</Link><Link to="/logs">Logs</Link><Link to="/search">Search</Link><Link to="/sync">Index status</Link><Link to="/wallets">Wallets</Link><Link to="/disclosures">Disclosures</Link><Link to="/chain-info">Network setup</Link><button onClick={() => { void appSession.login(undefined, true).catch(() => {}); }}>Refresh identity</button><button onClick={() => { void appSession.login().catch(() => {}); }}>Switch account</button><button onClick={() => { void logout().catch(() => {}); }}>Sign out</button></nav></header><PrivateOutlet /></div>;
}

function PrivateOutlet() {
  const state = useSyncExternalStore(appSession.subscribe, appSession.snapshot);
  // A renewed original token invalidates source cursors even for the same actor.
  return state.identity ? <Outlet key={state.identity.activation} /> : null;
}

// V2 never mounts the V1 impersonation, disclosure-consent, ABI-write, wallet
// provider or refresh-token components.
export function V2App() {
  if (window.self !== window.top) return <main><h1>Open Explorer to continue</h1><p>Private sessions require a top-level browser tab.</p><a href={window.location.href} target="_top">Open Explorer</a></main>;
  return <ThemeProvider><QueryClientProvider client={queries}><V2AuthProvider context={AuthContext}><TooltipProvider><BrowserRouter basename={getConfig('VITE_BASE_PATH', '')}><Routes><Route path="/" element={<Layout />}><Route index element={<OverviewV2 />} />
<Route path="chain-info" element={<PublicNetworkSetup />} />
<Route path="wallets" element={<WalletsV2 />} />
<Route path="disclosures" element={<DisclosuresV2 />} />
<Route path="disclosures/cases/:caseId" element={<DisclosureCaseV2 />} />
<Route path="disclosures/cases/:caseId/addresses/:addressId" element={<DisclosureAddressV2 />} />
<Route path="disclosures/cases/:caseId/addresses/:addressId/transactions/:hash" element={<DisclosureTransactionV2 />} />
<Route path="blocks" element={<ReadPanel path="/blocks" title="Blocks" collection />} />
<Route path="blocks/hash/:hash" element={<BlockV2 />} />
<Route path="blocks/:number" element={<BlockV2 />} />
<Route path="transactions" element={<ReadPanel path="/transactions" title="Transactions" collection filters="transactions" />} />
<Route path="transactions/paginated" element={<ReadPanel path="/transactions/paginated" title="Transactions" collection filters="transactions" />} />
<Route path="transactions/:hash" element={<TransactionV2 />}>
<Route index element={<TransactionReadV2 />} />
<Route path="transfers" element={<TransactionReadV2 suffix="/transfers" collection />} />
<Route path="logs" element={<TransactionReadV2 suffix="/logs" collection filters="logs" />} />
<Route path="internal" element={<TransactionReadV2 suffix="/internal" collection />} />
</Route>
<Route path="addresses/:address" element={<AddressV2 />}>
<Route index element={<Navigate to="stats" replace />} />
{['stats','transactions','balance','code','balances','transfers','internal','logs','contract','is-contract'].map(suffix=><Route key={suffix} path={suffix} element={<AddressReadV2 suffix={suffix} collection={['transactions','balances','transfers','internal','logs'].includes(suffix)} filters={suffix==='transactions'?'transactions':suffix==='logs'?'logs':''} />} />)}
</Route>
<Route path="accounts" element={<ReadPanel path="/accounts" title="Accounts" collection />} />
<Route path="tokens" element={<ReadPanel path="/tokens" title="Tokens" collection filters="tokens" />} />
<Route path="tokens/:address" element={<TokenV2 />}>
<Route index element={<TokenReadV2 />} />
<Route path="holders" element={<TokenReadV2 suffix="/holders" collection />} />
<Route path="transfers" element={<TokenReadV2 suffix="/transfers" collection />} />
</Route>
<Route path="token-transfers" element={<ReadPanel path="/transfers" title="Transfers" collection />} />
<Route path="logs" element={<ReadPanel path="/logs" title="Logs" collection filters="logs" />} />
<Route path="search" element={<SearchV2 />} />
<Route path="sync" element={<SyncV2 />} />
<Route path="*" element={<p>Page not found.</p>} /></Route></Routes></BrowserRouter></TooltipProvider></V2AuthProvider></QueryClientProvider></ThemeProvider>;
}
