import { getShortName } from './branding';
import { getConfig } from './runtimeConfig';
import { getNetworkCurrency } from './utils';

export function addNetworkToMetaMask() {
  if (!window.ethereum) {
    alert('MetaMask is not installed. Please install MetaMask to add the network.');
    return;
  }
  window.ethereum.request({
    method: 'wallet_addEthereumChain',
    params: [{
      chainId: '0x' + Number(getConfig('VITE_CHAIN_ID', '1001')).toString(16),
      chainName: getConfig('VITE_NETWORK_NAME') || getShortName(),
      nativeCurrency: { name: 'Ether', symbol: getNetworkCurrency(), decimals: 18 },
      rpcUrls: [getConfig('VITE_RPC_URL', 'http://localhost:8545')],
    }],
  });
}
