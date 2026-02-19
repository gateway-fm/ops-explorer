import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { TooltipProvider } from './components/ui/tooltip';
import { AuthProvider } from './lib/auth';
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
import TokenTransfers from './pages/TokenTransfers';
import ContractVerification from './pages/ContractVerification';
import GasTracker from './pages/GasTracker';
import { PrivacyDashboard } from './pages/PrivacyDashboard';
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
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <TooltipProvider delayDuration={200}>
          <BrowserRouter>
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
                <Route path="token-transfers" element={<TokenTransfers />} />
                <Route path="verify" element={<ContractVerification />} />
                <Route path="gas-tracker" element={<GasTracker />} />
                <Route path="privacy" element={<PrivacyDashboard />} />
                <Route path="grant/:grantId/:addressId" element={<GrantedAddressPage />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </TooltipProvider>
      </AuthProvider>
    </QueryClientProvider>
  );
}

export default App;
