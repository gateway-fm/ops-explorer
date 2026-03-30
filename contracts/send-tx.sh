#!/usr/bin/env bash
# Send transactions between Anvil dev accounts #6-9.
# These accounts are NOT used by loadtest.sh (which uses #0-5).
#
# Usage:
#   ./send-tx.sh              # 5 transactions, default RPC
#   ./send-tx.sh 20           # 20 transactions
#   ./send-tx.sh 10 http://localhost:8546  # custom RPC

set -euo pipefail

COUNT="${1:-5}"
RPC_URL="${2:-http://localhost:8545}"

# Anvil dev accounts #6-9 (private keys)
KEYS=(
  "0x92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e"  # #6 0x976EA74026E726554dB657fA54763abd0C3a0aa9
  "0x4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356"  # #7 0x14dC79964da2C08b23698B3D3cc7Ca32193d9955
  "0xdbda1821b80551c9d65939329250298aa3472ba22feea921c0cf5d620ea67b97"  # #8 0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f
  "0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6"  # #9 0xa0Ee7A142d267C1f36714E4a8F75612F20a79720
)

ADDRS=(
  "0x976EA74026E726554dB657fA54763abd0C3a0aa9"
  "0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
  "0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f"
  "0xa0Ee7A142d267C1f36714E4a8F75612F20a79720"
)

NUM_ACCOUNTS=${#KEYS[@]}

echo "Sending $COUNT transactions between Anvil accounts #6-9"
echo "RPC: $RPC_URL"
echo ""

for i in $(seq 1 "$COUNT"); do
  # Pick a random sender
  SENDER_IDX=$(( RANDOM % NUM_ACCOUNTS ))
  # Pick a different receiver
  RECEIVER_IDX=$(( (SENDER_IDX + 1 + RANDOM % (NUM_ACCOUNTS - 1)) % NUM_ACCOUNTS ))

  SENDER_KEY="${KEYS[$SENDER_IDX]}"
  RECEIVER="${ADDRS[$RECEIVER_IDX]}"

  # Random value between 0.001 and 0.1 ETH
  VALUE="0.0$(( RANDOM % 100 + 1 ))ether"

  echo "[$i/$COUNT] ${ADDRS[$SENDER_IDX]:0:10}... -> ${RECEIVER:0:10}...  $VALUE"
  cast send \
    --private-key "$SENDER_KEY" \
    --rpc-url "$RPC_URL" \
    --value "$VALUE" \
    "$RECEIVER" \
    > /dev/null 2>&1

  if [ $? -eq 0 ]; then
    echo "         ✓ confirmed"
  else
    echo "         ✗ failed"
  fi
done

echo ""
echo "Done. Sent $COUNT transactions."
