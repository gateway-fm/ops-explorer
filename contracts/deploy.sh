#!/bin/bash

# Universal contract deploy script
# Usage: ./deploy.sh <source_file> <contract_name> [constructor_args...] [--rpc-url URL] [--private-key KEY]
#
# Examples:
#   ./deploy.sh src/Counter.sol Counter --constructor "uint256" 42
#   ./deploy.sh src/EventCounter.sol EventCounter --constructor "uint256" 42
#   ./deploy.sh src/Forwarder.sol Forwarder
#   ./deploy.sh src/Counter.sol Counter --constructor "uint256" 100 --rpc-url http://localhost:9545

set -e

# --- Defaults ---
RPC_URL="http://localhost:8545"
PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"  # Anvil default key

# --- Parse args ---
SOURCE_FILE=""
CONTRACT_NAME=""
CONSTRUCTOR_TYPES=""
CONSTRUCTOR_VALS=()

while [[ $# -gt 0 ]]; do
  case $1 in
    --rpc-url)
      RPC_URL="$2"
      shift 2
      ;;
    --private-key)
      PRIVATE_KEY="$2"
      shift 2
      ;;
    --constructor)
      # Next arg is the type signature, remaining args until next flag are values
      CONSTRUCTOR_TYPES="$2"
      shift 2
      while [[ $# -gt 0 && ! "$1" =~ ^-- ]]; do
        CONSTRUCTOR_VALS+=("$1")
        shift
      done
      ;;
    *)
      if [ -z "$SOURCE_FILE" ]; then
        SOURCE_FILE="$1"
      elif [ -z "$CONTRACT_NAME" ]; then
        CONTRACT_NAME="$1"
      fi
      shift
      ;;
  esac
done

if [ -z "$SOURCE_FILE" ] || [ -z "$CONTRACT_NAME" ]; then
  echo "Usage: ./deploy.sh <source_file> <contract_name> [options]"
  echo ""
  echo "Arguments:"
  echo "  source_file      Path to .sol file (e.g. src/Counter.sol)"
  echo "  contract_name    Contract name (e.g. Counter)"
  echo ""
  echo "Options:"
  echo "  --constructor <types> <values...>   Constructor args (e.g. --constructor \"uint256\" 42)"
  echo "  --rpc-url <url>                     RPC endpoint (default: http://localhost:8545)"
  echo "  --private-key <key>                 Deployer private key (default: Anvil key #0)"
  echo ""
  echo "Examples:"
  echo "  ./deploy.sh src/Counter.sol Counter --constructor \"uint256\" 42"
  echo "  ./deploy.sh src/Forwarder.sol Forwarder"
  echo "  ./deploy.sh src/EventCounter.sol EventCounter --constructor \"uint256\" 0 --rpc-url http://localhost:9545"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FULL_SOURCE="$SCRIPT_DIR/$SOURCE_FILE"

if [ ! -f "$FULL_SOURCE" ]; then
  echo "Error: source file not found: $FULL_SOURCE"
  exit 1
fi

echo "=== Deploy $CONTRACT_NAME ==="
echo "  Source:   $SOURCE_FILE"
echo "  RPC:      $RPC_URL"

# Compile
echo ""
echo "Compiling..."
forge build "$FULL_SOURCE" --silent

# Get bytecode
BYTECODE=$(forge inspect "$FULL_SOURCE:$CONTRACT_NAME" bytecode)

if [ -z "$BYTECODE" ] || [ "$BYTECODE" = "0x" ]; then
  echo "Error: failed to get bytecode for $CONTRACT_NAME"
  exit 1
fi

# Append constructor args if provided
DEPLOY_DATA="$BYTECODE"
if [ -n "$CONSTRUCTOR_TYPES" ]; then
  CONSTRUCTOR_SIG="constructor($CONSTRUCTOR_TYPES)"
  ENCODED=$(cast abi-encode "$CONSTRUCTOR_SIG" "${CONSTRUCTOR_VALS[@]}" | cut -c3-)
  DEPLOY_DATA="${BYTECODE}${ENCODED}"
  echo "  Constructor: $CONSTRUCTOR_SIG ${CONSTRUCTOR_VALS[*]}"
fi

# Deploy
echo ""
echo "Deploying..."
DEPLOY_OUTPUT=$(cast send \
  --rpc-url "$RPC_URL" \
  --private-key "$PRIVATE_KEY" \
  --create "$DEPLOY_DATA" \
  --json)

CONTRACT_ADDRESS=$(echo "$DEPLOY_OUTPUT" | jq -r '.contractAddress')
TX_HASH=$(echo "$DEPLOY_OUTPUT" | jq -r '.transactionHash')
BLOCK=$(echo "$DEPLOY_OUTPUT" | jq -r '.blockNumber')

echo ""
echo "  Contract: $CONTRACT_ADDRESS"
echo "  Tx hash:  $TX_HASH"
echo "  Block:    $BLOCK"

# Print constructor args hex for verification
if [ -n "$CONSTRUCTOR_TYPES" ]; then
  echo "  Constructor args (hex): 0x${ENCODED}"
fi

echo ""
echo "Done!"
