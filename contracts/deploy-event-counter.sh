#!/bin/bash

# Deploy EventCounter contract
# Usage: ./deploy-event-counter.sh [rpc_url]
# Example: ./deploy-event-counter.sh http://localhost:8545

set -e

RPC_URL="${1:-http://localhost:8545}"
PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"  # Anvil default key
INITIAL_VALUE=42

echo "Compiling..."
forge build contracts/src/EventCounter.sol --silent

echo "Deploying EventCounter to $RPC_URL..."

BYTECODE=$(forge inspect contracts/src/EventCounter.sol:EventCounter bytecode)
CONSTRUCTOR_ARGS=$(cast abi-encode "constructor(uint256)" $INITIAL_VALUE | cut -c3-)

cast send \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY" \
    --create "${BYTECODE}${CONSTRUCTOR_ARGS}"
