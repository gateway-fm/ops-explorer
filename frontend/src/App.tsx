import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { TooltipProvider } from './components/ui/tooltip';
import { AuthProvider } from './lib/auth';
import { ImpersonationProvider, ImpersonationTokenMirror } from './hooks/useImpersonation';
import { ThemeProvider } from './hooks/useTheme';
import { getConfig } from './lib/runtimeConfig';
import { Layout } from './components/Layout';
import { Home } from './pages/Home';
import { Blocks } from './pages/Blocks';
import { BlockDetail } from './pages/BlockDetail';
import { Transactions } from './pages/Transactions';
import { TransactionDetail } from './pages/TransactionDetail';
import { Address } from './pages/Address';
import { Accounts } from './pages/Accounts';
import Tokens from './pages/Tokens';
import TokenDetail from './pages/TokenDetail';
import NftDetail from './pages/NftDetail';
import TokenTransfers from './pages/TokenTransfers';
import ContractVerification from './pages/ContractVerification';
import GasTracker from './pages/GasTracker';
import ChainInfo from './pages/ChainInfo';
import ApiDocs from './pages/ApiDocs';
import Stats from './pages/Stats';
import { PrivacyDashboard } from './pages/PrivacyDashboard';
import { ViewAsEntry } from './pages/ViewAsEntry';
import { GrantedAddressPage } from './pages/GrantedAddressPage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000,
      refetchOnWindowFocus: false,
    },
  },
});

function App() {
  return (
    <ThemeProvider>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <ImpersonationProvider>
          <ImpersonationTokenMirror />
          <TooltipProvider delayDuration={200}>
            <BrowserRouter basename={getConfig('VITE_BASE_PATH', '')}>
              <Routes>
                <Route path="/" element={<Layout />}>
                  <Route index element={<Home />} />
                  <Route path="blocks" element={<Blocks />} />
                  <Route path="block/:number" element={<BlockDetail />} />
                  <Route path="transactions" element={<Transactions />} />
                  <Route path="tx/:hash" element={<TransactionDetail />} />
                  <Route path="address/:address" element={<Address />} />
                  <Route path="accounts" element={<Accounts />} />
                  <Route path="tokens" element={<Tokens />} />
                  <Route path="token/:address" element={<TokenDetail />} />
                  <Route path="nft/:address/:tokenId" element={<NftDetail />} />
                  <Route path="token-transfers" element={<TokenTransfers />} />
                  <Route path="verify" element={<ContractVerification />} />
                  <Route path="gas-tracker" element={<GasTracker />} />
                  <Route path="chain-info" element={<ChainInfo />} />
                  <Route path="stats" element={<Stats />} />
                  <Route path="api-docs" element={<ApiDocs />} />
                  <Route path="privacy" element={<PrivacyDashboard />} />
                  <Route path="view-as" element={<ViewAsEntry />} />
                  <Route path="grant/:grantId/:addressId" element={<GrantedAddressPage />} />
                </Route>
              </Routes>
            </BrowserRouter>
          </TooltipProvider>
        </ImpersonationProvider>
      </AuthProvider>
    </QueryClientProvider>
    </ThemeProvider>
  );
}

export default App;
