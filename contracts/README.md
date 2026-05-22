# Test Contracts

Test contracts for the block explorer. Used for deploying to local/dev chains and testing features like contract verification.

## Prerequisites

- [Foundry](https://book.getfoundry.sh/getting-started/installation) (`forge`, `cast`)
- A running local node (e.g. `anvil`)

## Contracts

| Contract | File | Constructor | Description |
|----------|------|-------------|-------------|
| Counter | `src/Counter.sol` | `uint256 _initialNumber` | Simple counter with get/set/increment |
| EventCounter | `src/EventCounter.sol` | `uint256 _initialNumber` | Counter that emits events on every state change |
| Forwarder | `src/Forwarder.sol` | _(none)_ | Forwards ETH to a recipient, creating internal transactions |
| MockUSDT | `src/MockUSDT.sol` | `uint256 _initialSupply` | ERC20 "Tether USD" (6 decimals) with unrestricted `mint` for test funding |

## Deploy

All contracts use a single deploy script:

```bash
./deploy.sh <source_file> <contract_name> [--constructor <types> <values...>] [--rpc-url <url>]
```

### Deploy Commands

```bash
# Counter (with initial value 42)
./deploy.sh src/Counter.sol Counter --constructor "uint256" 42

# EventCounter (with initial value 42)
./deploy.sh src/EventCounter.sol EventCounter --constructor "uint256" 42

# Forwarder (no constructor args)
./deploy.sh src/Forwarder.sol Forwarder
```

### Custom RPC / Private Key

```bash
# Deploy to a different RPC endpoint
./deploy.sh src/Counter.sol Counter --constructor "uint256" 42 --rpc-url http://localhost:9545

# Deploy with a custom private key
./deploy.sh src/Counter.sol Counter --constructor "uint256" 42 --private-key 0xYOUR_KEY
```

Default RPC is `http://localhost:8545`. Default private key is Anvil account #0.

## Verify (Standard JSON Input)

After deploying, you can verify contracts using the Standard JSON Input files in `standard-json/`.

### Via the UI

1. Go to the verification page in the block explorer
2. Click **"Standard JSON Input"** tab
3. Select the compiler version (check `pragma solidity` in the source)
4. Enter the contract name (e.g. `Counter`)
5. Upload the matching `.json` file from `standard-json/`
6. If the contract has constructor args, paste the hex from the deploy output
7. Click **Verify & Publish**

### Via curl

```bash
# Counter (replace ADDRESS with the deployed address, add constructorArgs from deploy output)
curl -X POST http://localhost:8080/api/verify/standard-json \
  -H 'Content-Type: application/json' \
  -d '{
    "address": "0xADDRESS",
    "compilerVersion": "0.8.13",
    "contractName": "Counter",
    "standardInput": '"$(cat standard-json/Counter.json)"',
    "constructorArgs": "0x000000000000000000000000000000000000000000000000000000000000002a"
  }'

# EventCounter
curl -X POST http://localhost:8080/api/verify/standard-json \
  -H 'Content-Type: application/json' \
  -d '{
    "address": "0xADDRESS",
    "compilerVersion": "0.8.13",
    "contractName": "EventCounter",
    "standardInput": '"$(cat standard-json/EventCounter.json)"',
    "constructorArgs": "0x000000000000000000000000000000000000000000000000000000000000002a"
  }'

# Forwarder (no constructor args)
curl -X POST http://localhost:8080/api/verify/standard-json \
  -H 'Content-Type: application/json' \
  -d '{
    "address": "0xADDRESS",
    "compilerVersion": "0.8.20",
    "contractName": "Forwarder",
    "standardInput": '"$(cat standard-json/Forwarder.json)"'
  }'
```

### Via source code verification

```bash
# Counter
curl -X POST http://localhost:8080/api/verify \
  -H 'Content-Type: application/json' \
  -d '{
    "address": "0xADDRESS",
    "compilerVersion": "0.8.13",
    "contractName": "Counter",
    "sourceCode": "'"$(cat src/Counter.sol)"'",
    "optimizationUsed": false,
    "optimizationRuns": 200,
    "constructorArgs": "0x000000000000000000000000000000000000000000000000000000000000002a"
  }'
```

## Other Scripts

| Script | Description | Usage |
|--------|-------------|-------|
| `fund.sh` | Fund a wallet from the Anvil deployer | `./fund.sh 0xADDRESS [amount] [rpc_url]` |
| `deploy-mock-usdt.sh` | Deploy MockUSDT and mint test tokens to an address | `./deploy-mock-usdt.sh [recipient] [usdt_amount] [rpc_url]` |
| `loadtest.sh` | Send many random transactions | `./loadtest.sh [rpc_url] [num_txs]` |

```bash
# Fund a wallet with 10 ETH
./fund.sh 0x70997970C51812dc3A010C7d01b50e0d17dc79C8

# Fund with 50 ETH on a custom RPC
./fund.sh 0x70997970C51812dc3A010C7d01b50e0d17dc79C8 50 http://localhost:9545

# Send 50 random transactions for load testing
./loadtest.sh http://localhost:8545 50

# Export the deployer key (well-known Anvil mnemonic, account #0) once:
export PRIVATE_KEY=$(cast wallet private-key \
  --mnemonic 'test test test test test test test test test test test junk')

# Deploy MockUSDT and mint 1,000,000 USDT to Anvil account #1 (defaults)
./deploy-mock-usdt.sh

# Deploy MockUSDT and mint 5,000 USDT to a specific address
./deploy-mock-usdt.sh 0x70997970C51812dc3A010C7d01b50e0d17dc79C8 5000
```

## Standard JSON Input Files

Pre-built `solc --standard-json` input files are in `standard-json/`:

| File | Contract | Compiler |
|------|----------|----------|
| `Counter.json` | Counter | 0.8.13 |
| `EventCounter.json` | EventCounter | 0.8.13 |
| `Forwarder.json` | Forwarder | 0.8.20 |

These match the source files in `src/` exactly (no optimizer, default settings). Use them for testing the Standard JSON Input verification method.
