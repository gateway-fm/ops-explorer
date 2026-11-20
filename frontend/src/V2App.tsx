import { NativeMetadataBoundary } from './hooks/useNativeMetadata';
import { useSyncExternalStore } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Outlet, Routes, Route, useParams, useLocation, Navigate } from 'react-router-dom';
import { AuthContext } from './lib/authContext';
import { V2AuthProvider } from './lib/authV2';
import { appSession } from './lib/v2Session';
import { TooltipProvider } from './components/ui/tooltip';
import { ThemeProvider } from './hooks/useTheme';
import { getConfig } from './lib/runtimeConfig';
import { PublicNetworkSetup } from './pages/NetworkSetupV2';
import { WalletsV2 } from './pages/WalletsV2';
import { BlocksNative, BlockNative } from './pages/BlocksNative';
import { TransactionsNative, AccountsNative, TransfersNative, LogsNative } from './pages/CollectionsNative';
import { TokensNative, TokenNative } from './pages/TokensNative';
import { SyncNative, SearchNative, ApiDocsNative } from './pages/NetworkNative';
import { NativeExplorerLayout } from './components/NativeExplorerControls';
import { HomeNative } from './pages/HomeNative';
import { TransactionNative } from './pages/TransactionNative';
import { AddressNative } from './pages/AddressNative';
import { DisclosuresV2, DisclosureCaseV2, DisclosureAddressV2, DisclosureTransactionV2 } from './pages/DisclosuresV2';

const queries = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false, staleTime: 1000 } } });
function Layout() {
  const location = useLocation();
  const state = useSyncExternalStore(appSession.subscribe, appSession.snapshot);
  // The whole private screen, including search and menus, belongs to this
  // activation. A batched account adoption must not reuse the prior query.
  return state.identity ? <NativeExplorerLayout key={state.identity.activation}><NativeMetadataBoundary ownerKey={location.key}><Outlet /></NativeMetadataBoundary></NativeExplorerLayout> : null;
}
function TokenAlias() { const {address} = useParams(), location = useLocation(); return <Navigate to={{pathname:`/tokens/${address}`,search:location.search,hash:location.hash}} state={location.state} replace/>; }

// Only this explicit approved-network page is public. Private routed pages
// remain unmounted until the original application identity is verified.
function PublicEntryLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  return <NativeExplorerLayout>{location.pathname === '/chain-info' ? <PublicNetworkSetup /> : children}</NativeExplorerLayout>;
}

// V2 never mounts the V1 impersonation, disclosure-consent, ABI-write, wallet
// provider or refresh-token components.
export function V2App() {
  if (window.self !== window.top) return <main><h1>Open Explorer to continue</h1><p>Private sessions require a top-level browser tab.</p><a href={window.location.href} target="_top">Open Explorer</a></main>;
  return <ThemeProvider><QueryClientProvider client={queries}><TooltipProvider><BrowserRouter basename={getConfig('VITE_BASE_PATH', '')}><V2AuthProvider context={AuthContext} gate={content => <PublicEntryLayout>{content}</PublicEntryLayout>}><Routes><Route path="/" element={<Layout />}><Route index element={<HomeNative />} />
<Route path="chain-info" element={<PublicNetworkSetup />} />
<Route path="wallets" element={<WalletsV2 />} />
<Route path="disclosures" element={<DisclosuresV2 />} />
<Route path="disclosures/cases/:caseId" element={<DisclosureCaseV2 />} />
<Route path="disclosures/cases/:caseId/addresses/:addressId" element={<DisclosureAddressV2 />} />
<Route path="disclosures/cases/:caseId/addresses/:addressId/transactions/:hash" element={<DisclosureTransactionV2 />} />
<Route path="blocks" element={<BlocksNative/>} />
<Route path="blocks/hash/:hash" element={<BlockNative/>} />
<Route path="blocks/:number" element={<BlockNative/>} />
<Route path="transactions" element={<TransactionsNative/>} />
<Route path="transactions/paginated" element={<TransactionsNative/>} />
<Route path="tx/:hash" element={<TransactionNative />} />
<Route path="transactions/:hash" element={<TransactionNative />} />
{['transfers','logs','internal'].map(suffix => <Route key={`tx-${suffix}`} path={`transactions/:hash/${suffix}`} element={<TransactionNative/>}/>)}
<Route path="address/:address" element={<AddressNative />} />
<Route path="addresses/:address" element={<AddressNative />} />
{['stats','transactions','balance','transfers','internal','logs','is-contract','code','balances','contract'].map(suffix => <Route key={`address-${suffix}`} path={`addresses/:address/${suffix}`} element={<AddressNative/>}/>)}
<Route path="block/:number" element={<BlockNative/>} />
<Route path="token/:address" element={<TokenAlias />} />
<Route path="accounts" element={<AccountsNative/>} />
<Route path="tokens" element={<TokensNative/>} />
<Route path="tokens/:address" element={<TokenNative/>}/>
{['holders','transfers','contract'].map(suffix=><Route key={`token-${suffix}`} path={`tokens/:address/${suffix}`} element={<TokenNative/>}/>)}
<Route path="token-transfers" element={<TransfersNative/>} />
<Route path="logs" element={<LogsNative/>} />
<Route path="search" element={<SearchNative/>} />
<Route path="sync" element={<SyncNative/>} />
<Route path="api-docs" element={<ApiDocsNative/>} />
<Route path="*" element={<p>Page not found.</p>} /></Route></Routes></V2AuthProvider></BrowserRouter></TooltipProvider></QueryClientProvider></ThemeProvider>;
}
