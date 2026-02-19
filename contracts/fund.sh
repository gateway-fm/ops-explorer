#!/bin/bash

# Fund a wallet address from the Anvil deployer account
# Usage: ./fund.sh <wallet_address> [amount_in_ether] [rpc_url]
# Example: ./fund.sh 0xYourAddress 10 http://localhost:8545

set -e

WALLET_ADDRESS="${1}"
AMOUNT="${2:-10}ether"
RPC_URL="${3:-http://localhost:8545}"
PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"  # Anvil default key
FROM_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

if [ -z "$WALLET_ADDRESS" ]; then
  echo "Usage: ./fund.sh <wallet_address> [amount_in_ether] [rpc_url]"
  echo ""
  echo "  wallet_address   The address to fund (required)"
  echo "  amount_in_ether  Amount to send, default: 10"
  echo "  rpc_url          RPC endpoint, default: http://localhost:8545"
  exit 1
fi

echo "Funding wallet from Anvil deployer account"
echo "  From:   $FROM_ADDRESS"
echo "  To:     $WALLET_ADDRESS"
echo "  Amount: $AMOUNT"
echo "  RPC:    $RPC_URL"
echo ""

BALANCE_BEFORE=$(cast balance --rpc-url "$RPC_URL" "$WALLET_ADDRESS" --ether 2>/dev/null || echo "0")
echo "Balance before: ${BALANCE_BEFORE} ETH"

TX_HASH=$(cast send \
  --rpc-url "$RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  "$WALLET_ADDRESS" \
  --value "$AMOUNT" \
  --json | jq -r '.transactionHash')

BALANCE_AFTER=$(cast balance --rpc-url "$RPC_URL" "$WALLET_ADDRESS" --ether)

echo "Tx hash:        $TX_HASH"
echo "Balance after:  ${BALANCE_AFTER} ETH"
echo ""
echo "Done!"
