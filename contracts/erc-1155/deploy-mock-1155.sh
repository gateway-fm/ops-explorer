#!/bin/bash

# Deploy MockERC1155 and mint a mixed multi-token collection: two fungible
# editions and a 1-of-1. Then batch-transfer some units to a second holder so
# the explorer's holder counts and the indexer's TransferBatch path are both
# exercised.
#
# Usage: ./deploy-mock-1155.sh [recipient_address] [rpc_url]
#
# Defaults:
#   recipient_address  Anvil account #1 (0x70997970C51812dc3A010C7d01b50e0d17dc79C8)
#   rpc_url            http://localhost:8545
#
# Token layout (all minted to the DEPLOYER, then partially distributed):
#   id 1  "Gold Coin"        fungible, supply 1000
#   id 2  "Health Potion"    fungible, supply 50
#   id 3  "Legendary Sword"  1-of-1,   supply 1
# A safeBatchTransferFrom moves 100 of id 1 and 10 of id 2 to the recipient,
# emitting a single TransferBatch and giving ids 1 & 2 two holders each.
#
# Each id carries a self-contained metadata URI: a base64 data:application/json
# payload with an inline base64 SVG image — no metadata server required.
#
# Auth: reads PRIVATE_KEY from the environment. For a local Anvil dev chain:
#   export PRIVATE_KEY=$(cast wallet private-key --mnemonic 'test test test test test test test test test test test junk')

set -e

RECIPIENT="${1:-0x70997970C51812dc3A010C7d01b50e0d17dc79C8}"  # Anvil account #1
RPC_URL="${2:-http://localhost:8545}"

COLLECTION_NAME="Mock Game Items"
COLLECTION_SYMBOL="MGI"

# id:name:supply:hexcolor — drives both the mint and the metadata.
ITEMS=(
  "1:Gold Coin:1000:#f5a623"
  "2:Health Potion:50:#d0021b"
  "3:Legendary Sword:1:#4a90e2"
)

if [ -z "${PRIVATE_KEY:-}" ]; then
  echo "Error: PRIVATE_KEY env var is not set."
  echo "For local Anvil, export the deployer key first (see header comment)."
  exit 1
fi

DEPLOYER=$(cast wallet address --private-key "$PRIVATE_KEY")

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SOURCE_FILE="$SCRIPT_DIR/MockERC1155.sol"

if [ ! -f "$SOURCE_FILE" ]; then
  echo "Error: source file not found: $SOURCE_FILE"
  exit 1
fi

echo "=== Deploy MockERC1155 ==="
echo "  RPC:        $RPC_URL"
echo "  Deployer:   $DEPLOYER"
echo "  Recipient:  $RECIPIENT"
echo "  Collection: $COLLECTION_NAME ($COLLECTION_SYMBOL)"
echo ""

echo "Compiling..."
forge build "$SOURCE_FILE" --silent

BYTECODE=$(forge inspect "$SOURCE_FILE:MockERC1155" bytecode)
if [ -z "$BYTECODE" ] || [ "$BYTECODE" = "0x" ]; then
  echo "Error: failed to get bytecode for MockERC1155"
  exit 1
fi

ENCODED=$(cast abi-encode "constructor(string,string)" "$COLLECTION_NAME" "$COLLECTION_SYMBOL" | cut -c3-)
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
echo ""

# Build a base64 data: URI of token metadata (with an inline SVG image).
# Usage: make_token_uri <id> <name> <supply> <hex_color>
make_token_uri() {
  local id="$1" item_name="$2" supply="$3" hex="$4"

  local svg
  svg=$(printf '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="320"><rect width="320" height="320" fill="%s"/><text x="160" y="140" font-family="sans-serif" font-size="32" fill="#ffffff" text-anchor="middle">%s</text><text x="160" y="190" font-family="sans-serif" font-size="22" fill="#ffffff" text-anchor="middle">id #%s</text></svg>' \
    "$hex" "$item_name" "$id")
  local svg_b64
  svg_b64=$(printf '%s' "$svg" | base64 | tr -d '\n')

  local json
  json=$(printf '{"name":"%s","description":"Test ERC-1155 item (id %s, supply %s) from the %s, used for exercising the block explorer multi-token features.","image":"data:image/svg+xml;base64,%s","attributes":[{"trait_type":"Item","value":"%s"},{"trait_type":"Supply","value":%s}]}' \
    "$item_name" "$id" "$supply" "$COLLECTION_NAME" "$svg_b64" "$item_name" "$supply")
  local json_b64
  json_b64=$(printf '%s' "$json" | base64 | tr -d '\n')

  printf 'data:application/json;base64,%s' "$json_b64"
}

echo "Minting ${#ITEMS[@]} token id(s) to deployer..."
for entry in "${ITEMS[@]}"; do
  IFS=':' read -r id item_name supply hex <<< "$entry"
  uri=$(make_token_uri "$id" "$item_name" "$supply" "$hex")

  MINT_TX=$(cast send \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY" \
    "$CONTRACT_ADDRESS" \
    "mint(address,uint256,uint256,string)" "$DEPLOYER" "$id" "$supply" "$uri" \
    --json | jq -r '.transactionHash')

  echo "  id #$id ($item_name x$supply)   tx: $MINT_TX"
done
echo ""

echo "Batch-transferring 100 of id 1 and 10 of id 2 to $RECIPIENT (emits TransferBatch)..."
BATCH_TX=$(cast send \
  --rpc-url "$RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  "$CONTRACT_ADDRESS" \
  "safeBatchTransferFrom(address,address,uint256[],uint256[],bytes)" \
  "$DEPLOYER" "$RECIPIENT" "[1,2]" "[100,10]" "0x" \
  --json | jq -r '.transactionHash')
echo "  tx: $BATCH_TX"
echo ""

echo "Collection state:"
echo "  Contract:           $CONTRACT_ADDRESS"
echo "  Deployer id 1 bal:  $(cast call --rpc-url "$RPC_URL" "$CONTRACT_ADDRESS" "balanceOf(address,uint256)(uint256)" "$DEPLOYER" 1 | awk '{print $1}')"
echo "  Recipient id 1 bal: $(cast call --rpc-url "$RPC_URL" "$CONTRACT_ADDRESS" "balanceOf(address,uint256)(uint256)" "$RECIPIENT" 1 | awk '{print $1}')"
echo "  Recipient id 2 bal: $(cast call --rpc-url "$RPC_URL" "$CONTRACT_ADDRESS" "balanceOf(address,uint256)(uint256)" "$RECIPIENT" 2 | awk '{print $1}')"
echo "  uri(1):             $(cast call --rpc-url "$RPC_URL" "$CONTRACT_ADDRESS" "uri(uint256)(string)" 1 | head -c 80)..."
echo ""
echo "Done!"
