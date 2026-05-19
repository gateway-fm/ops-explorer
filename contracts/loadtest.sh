#!/bin/bash

# Load test script for the explorer
# Usage: ./loadtest.sh [rpc_url] [num_transactions]
# Example: ./loadtest.sh http://localhost:8546 10000
#
# Submits each tx with --async (no wait for receipt) and pre-computes
# nonces so anvil's mempool can absorb them as fast as we can submit.

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
echo "Submitting $NUM_TXS transactions (async)..."
echo ""

START_NONCE=$(cast nonce --rpc-url "$RPC_URL" "$FROM_ADDRESS")
echo "Starting nonce: $START_NONCE"
echo ""

for i in $(seq 1 $NUM_TXS); do
  TO_ADDRESS=${ADDRESSES[$((RANDOM % ${#ADDRESSES[@]}))]}
  VALUE="0.00$((RANDOM % 9 + 1))ether"
  NONCE=$((START_NONCE + i - 1))

  cast send \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY" \
    --async \
    --nonce "$NONCE" \
    "$TO_ADDRESS" \
    --value "$VALUE" >/dev/null

  if (( i % 250 == 0 )); then
    echo "[$i/$NUM_TXS] submitted"
  fi
done

echo ""
echo "Load test complete! Submitted $NUM_TXS transactions to mempool."
echo "Anvil will mine them at its configured block time."
