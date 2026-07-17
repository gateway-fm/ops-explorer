---
title: Configuration Overview
description: Every environment variable that configures the block explorer — ports, RPC, indexer tuning, and where to find the rest.
---

The explorer is configured entirely through **environment variables** — no config files to
edit, no code to change. This page covers the core variables; the deeper topics
(branding, network switching, and deployment/auth) each have their own page.

## Configuration by topic

| Topic | What it covers | Page |
|-------|----------------|------|
| **Ports** | Host ports for each service | [below](#ports) |
| **Chain data** | RPC URL, indexer connection, deployment mode | [Deployment Modes](../deployment-modes/) |
| **Branding** | Name, logo, colours, footer links | [Branding & Whitelabel](../branding/) |
| **Network switcher** | Single- vs multi-network deployments | [Network Modes](../network-modes/) |
| **Public API** | Rate limits and the standalone API service | [API Reference](../../api/public-api/) |

## Ports

These control the host ports exposed by Docker Compose. Internal container ports are
fixed; only the host-side mappings change.

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `8081` | API server |
| `FRONTEND_PORT` | `3001` | Frontend dev server |
| `POSTGRES_PORT` | `5433` | PostgreSQL |
| `ANVIL_PORT` | `8546` | Anvil RPC (dev only) |
| `PUBLIC_API_PORT` | `8082` | Public API server |

:::tip[Running parallel stacks]
For multiple stacks side by side (e.g. git worktrees), generate a `.env` with
non-conflicting ports automatically:

```bash
./scripts/stack-ports.sh auto > .env
```

`make dev-stack` does this for you when the directory name differs from `block-explorer`.
:::

## Core data sources

| Variable | Required | Description |
|----------|----------|-------------|
| `RPC_URL` | Yes | EVM JSON-RPC endpoint the explorer (and contract verifier) reads from. |
| `DATABASE_URL` | Yes | PostgreSQL connection string. Use `sslmode=require` for managed Postgres in production. |
| `INDEXER_URL` | One of these | Standalone mode — `host:port` of a chain-indexer gRPC endpoint. |
| `PRIVACY_PROXY_URL` | One of these | Privacy mode — internal URL of the Open Privacy Suite backend. |

:::caution
You must set **exactly one** of `INDEXER_URL` or `PRIVACY_PROXY_URL`. Setting both (or
neither) makes the API exit at startup. See [Deployment Modes](../deployment-modes/) for
the full reasoning and the auth/SSO variables that go with privacy mode.
:::

## Indexer tuning

These are read by the [chain-indexer](https://github.com/gateway-fm/ops-indexer) and are
commented out by default in `.env`:

| Variable | Description |
|----------|-------------|
| `CATCHUP_WORKERS` | Number of workers used when catching up on historical blocks. |
| `RPC_WORKERS` | Concurrent RPC request workers. |
| `RPC_RATE_LIMIT` | Maximum RPC requests per second. |
| `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`). |

## Frontend & wallet (MetaMask)

The frontend reads these `VITE_*` variables at container startup (injected into
`window.__runtimeConfig`). They configure the **Add Network** button that adds the chain to a
wallet like MetaMask. The authoritative `chainId` always comes from `GET /api/v1/chain-info`;
the values below are deploy-time fallbacks.

| Variable | Required | Description |
|----------|----------|-------------|
| `VITE_API_URL` | No | Path/URL to the internal API. Default: `/api` (proxied by nginx/ingress). |
| `VITE_RPC_URL` | Standalone | The node's **browser-facing public JSON-RPC URL** a wallet connects to directly in standalone mode. A node concern, **not** the Open Privacy Suite. Falls back to `http://localhost:8545`. |
| `VITE_CHAIN_ID` | Recommended | Chain ID in **decimal** (e.g. `4242`) for the Add Network button. Used only when the backend doesn't supply `chainId`. A wrong value here is what makes MetaMask add the network with the wrong chain. |
| `VITE_NETWORK_NAME` | No | Network name shown in MetaMask. |
| `VITE_NETWORK_CURRENCY` | No | Native currency symbol shown in MetaMask. Default: `ETH`. |

:::caution[Wallets in privacy mode]
In **standalone** mode a wallet uses `VITE_RPC_URL` to talk to the node directly. In
**privacy** mode the wallet cannot reach the chain directly; it connects through a locally-run
[jwt-injector](../../privacy/wallet-access/) helper instead, so `VITE_RPC_URL` is not the
wallet's target there. See [Wallet Access](../../privacy/wallet-access/).
:::

## Optional integrations

| Variable | Default | Description |
|----------|---------|-------------|
| `ENABLE_PRICE` | `true` (standalone) | Enables the CoinGecko price refresher and `/price` data source. |
| `PRICE_COIN_ID` | `ethereum` | CoinGecko coin id to price (set per chain). |
| `PRICE_CURRENCY` | `usd` | Fiat currency for the price quote. |
| `SSO_CLIENT_ID` | `explorer` | OAuth client id (privacy mode — must match the Open Privacy Suite). |
