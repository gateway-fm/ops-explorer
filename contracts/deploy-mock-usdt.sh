#!/bin/bash

# Deploy MockUSDT and mint test tokens to a recipient.
# Usage: ./deploy-mock-usdt.sh [recipient_address] [amount_in_usdt] [rpc_url]
#
# Defaults:
#   recipient_address  Anvil account #1 (0x70997970C51812dc3A010C7d01b50e0d17dc79C8)
#   amount_in_usdt     1000000 (one million USDT, human units)
#   rpc_url            http://localhost:8545
#
# The deployer (Anvil account #0) also receives 1,000,000 USDT at construction
# time so you have a funded sender for transfer testing.

set -e

RECIPIENT="${1:-0x70997970C51812dc3A010C7d01b50e0d17dc79C8}"  # Anvil account #1
AMOUNT_HUMAN="${2:-1000000}"                                  # in whole USDT
RPC_URL="${3:-http://localhost:8545}"
PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"  # Anvil key #0
DEPLOYER="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

DECIMALS=6
# Raw token amount = AMOUNT_HUMAN * 10^DECIMALS
AMOUNT_RAW=$(cast --to-uint256 "$(echo "$AMOUNT_HUMAN * 10^$DECIMALS" | bc)")
INITIAL_SUPPLY_RAW="$AMOUNT_RAW"  # mint same amount to deployer

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SOURCE_FILE="$SCRIPT_DIR/src/MockUSDT.sol"

if [ ! -f "$SOURCE_FILE" ]; then
  echo "Error: source file not found: $SOURCE_FILE"
  exit 1
fi

echo "=== Deploy MockUSDT ==="
echo "  RPC:        $RPC_URL"
echo "  Deployer:   $DEPLOYER"
echo "  Recipient:  $RECIPIENT"
echo "  Amount:     ${AMOUNT_HUMAN} USDT (raw: ${AMOUNT_RAW})"
echo ""

echo "Compiling..."
forge build "$SOURCE_FILE" --silent

BYTECODE=$(forge inspect "$SOURCE_FILE:MockUSDT" bytecode)
if [ -z "$BYTECODE" ] || [ "$BYTECODE" = "0x" ]; then
  echo "Error: failed to get bytecode for MockUSDT"
  exit 1
fi

ENCODED=$(cast abi-encode "constructor(uint256)" "$INITIAL_SUPPLY_RAW" | cut -c3-)
DEPLOY_DATA="${BYTECODE}${ENCODED}"

echo "Deploying..."
DEPLOY_OUTPUT=$(cast send \
  --rpc-url "$RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  --create "$DEPLOY_DATA" \
  --json)

CONTRACT_ADDRESS=$(echo "$DEPLOY_OUTPUT" | jq -r '.contractAddress')
DEPLOY_TX=$(echo "$DEPLOY_OUTPUT" | jq -r '.transactionHash')
DEPLOY_BLOCK=$(echo "$DEPLOY_OUTPUT" | jq -r '.blockNumber')

echo "  Contract:   $CONTRACT_ADDRESS"
echo "  Tx hash:    $DEPLOY_TX"
echo "  Block:      $DEPLOY_BLOCK"
echo "  Args (hex): 0x${ENCODED}"
echo ""

echo "Minting ${AMOUNT_HUMAN} USDT to ${RECIPIENT}..."
MINT_TX=$(cast send \
  --rpc-url "$RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  "$CONTRACT_ADDRESS" \
  "mint(address,uint256)" "$RECIPIENT" "$AMOUNT_RAW" \
  --json | jq -r '.transactionHash')

RECIPIENT_BAL_RAW=$(cast call --rpc-url "$RPC_URL" "$CONTRACT_ADDRESS" "balanceOf(address)(uint256)" "$RECIPIENT" | awk '{print $1}')
DEPLOYER_BAL_RAW=$(cast call --rpc-url "$RPC_URL" "$CONTRACT_ADDRESS" "balanceOf(address)(uint256)" "$DEPLOYER" | awk '{print $1}')
TOTAL_SUPPLY_RAW=$(cast call --rpc-url "$RPC_URL" "$CONTRACT_ADDRESS" "totalSupply()(uint256)" | awk '{print $1}')

# Human-readable balances (raw / 10^6)
RECIPIENT_BAL=$(echo "scale=6; $RECIPIENT_BAL_RAW / 10^$DECIMALS" | bc)
DEPLOYER_BAL=$(echo "scale=6; $DEPLOYER_BAL_RAW / 10^$DECIMALS" | bc)
TOTAL_SUPPLY=$(echo "scale=6; $TOTAL_SUPPLY_RAW / 10^$DECIMALS" | bc)

echo "  Mint tx:    $MINT_TX"
echo ""
echo "Balances:"
echo "  Deployer (${DEPLOYER}): ${DEPLOYER_BAL} USDT"
echo "  Recipient (${RECIPIENT}): ${RECIPIENT_BAL} USDT"
echo "  Total supply: ${TOTAL_SUPPLY} USDT"
echo ""
echo "Done!"
