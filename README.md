# Block Explorer

A lightweight, self-hosted block explorer for EVM-compatible chains. Point it at any RPC endpoint and get a fully indexed, searchable explorer with real-time updates.

## TL;DR

```bash
# Set your RPC URL and run
RPC_URL=http://host.docker.internal:8545 make run
```

That's it. Open [http://localhost:3001](http://localhost:3001).

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)

## Architecture

```
Blockchain RPC
      |
  [Indexer]  -- fetches blocks, txs, logs, traces -->  [PostgreSQL]
      |                                                      |
  [API Server]  -- REST + WebSocket -------------------------+
      |
  [Frontend]  -- React SPA served via Nginx
      |
  Browser (http://localhost:3001)
```

| Component | Tech | Description |
|-----------|------|-------------|
| **Indexer** | Go | Syncs blocks, transactions, token transfers, and contract data from the chain into Postgres. Supports parallel catchup and real-time polling. |
| **API** | Go (Chi) | REST API and WebSocket server for querying indexed data. |
| **Frontend** | React, TypeScript, Tailwind | Search blocks, transactions, addresses, tokens. Real-time updates via WebSocket. |
| **Database** | PostgreSQL 16 | Stores all indexed chain data. |

## Usage

### Start with an external RPC

```bash
RPC_URL=http://host.docker.internal:8545 make run
```

> `host.docker.internal` lets Docker containers reach services on your host machine. If your RPC is remote, pass the URL directly (e.g. `RPC_URL=https://rpc.example.com make run`).

You can also set the starting block:

```bash
RPC_URL=https://rpc.example.com START_BLOCK=1000000 make run
```

### Start with a local testnet (Anvil)

Spins up a local Anvil node alongside the explorer:

```bash
make anvil
```

### Stop

```bash
make stop        # external RPC mode
make anvil-stop  # Anvil mode
```

### Logs

```bash
make logs        # external RPC mode
make anvil-logs  # Anvil mode
```

## Endpoints

| Service | Default URL |
|---------|-------------|
| Explorer | [http://localhost:3001](http://localhost:3001) |
| API | [http://localhost:8081](http://localhost:8081) |
| Anvil RPC (local mode only) | [http://localhost:8546](http://localhost:8546) |

## Port Configuration

All ports are configurable via environment variables:

```bash
# Override any port
FRONTEND_PORT=4000 API_PORT=4001 make run
```

| Variable | Default | Description |
|----------|---------|-------------|
| `FRONTEND_PORT` | `3001` | Explorer UI |
| `API_PORT` | `8081` | REST API + WebSocket |
| `POSTGRES_PORT` | `5433` | PostgreSQL (host-mapped) |
| `ANVIL_PORT` | `8546` | Anvil RPC (local mode only) |

## Features

- Block, transaction, and address pages
- Token tracking (ERC-20 / ERC-721)
- Token transfer history
- Smart contract verification (Solidity)
- Internal transaction tracing (optional)
- Gas tracker
- Full-text search
- Real-time updates via WebSocket
- Prometheus metrics
