#!/bin/bash

# Load test script for the explorer
# Usage: ./loadtest.sh [rpc_url] [num_transactions]
# Example: ./loadtest.sh http://localhost:8545 50

set -e

RPC_URL="${1:-http://localhost:8545}"
NUM_TXS="${2:-20}"
PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
FROM_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

# Some random addresses to send to
ADDRESSES=(
  "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
  "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
  "0x90F79bf6EB2c4f870365E785982E1f101E93b906"
  "0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"
  "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"
)

echo "Starting load test against $RPC_URL"
echo "Sending $NUM_TXS transactions..."
echo ""

for i in $(seq 1 $NUM_TXS); do
  # Pick a random address
  TO_ADDRESS=${ADDRESSES[$((RANDOM % ${#ADDRESSES[@]}))]}

  # Random value between 0.001 and 0.01 ETH
  VALUE="0.00$((RANDOM % 9 + 1))ether"

  echo -n "[$i/$NUM_TXS] Sending $VALUE to ${TO_ADDRESS:0:10}... "

  TX_HASH=$(cast send \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY" \
    "$TO_ADDRESS" \
    --value "$VALUE" \
    --json 2>/dev/null | jq -r '.transactionHash')

  echo "tx: ${TX_HASH:0:18}..."

  # Small delay to spread transactions
  sleep 0.2
done

echo ""
echo "Load test complete! Sent $NUM_TXS transactions."
