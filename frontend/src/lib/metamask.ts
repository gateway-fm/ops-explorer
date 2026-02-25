export function addNetworkToMetaMask() {
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
