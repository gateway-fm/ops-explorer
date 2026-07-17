---
title: System Architecture
description: How the block explorer's services fit together — indexer, API, public API, frontend, and the data path from chain to UI.
---

The explorer is a small set of independently deployable services. Chain data flows from an
RPC node, through the indexer, into PostgreSQL, and out to the UI and public API.

## Services

| Service | Language | Description |
|---------|----------|-------------|
| **chain-indexer** | Go | Polls an RPC node and indexes chain data into PostgreSQL. Lives in a [separate repo](https://github.com/gateway-fm/ops-indexer); exposes data over gRPC. |
| **API** | Go | HTTP server powering the explorer frontend — blocks, transactions, addresses, gas prices, contract verification. |
| **Public API** | Go | Standalone, rate-limited, read-only REST API for programmatic access. Crash-isolated from the explorer API. |
| **Frontend** | React + TypeScript | SPA built with Vite and TailwindCSS. Served by nginx in production. |
| **PostgreSQL** | — | Stores indexed chain data; the explorer API also uses it for contract-verification writes. |

## Data flow

```
        ┌──────────────┐
        │   RPC node   │
        └──────┬───────┘
               │ JSON-RPC (poll)
        ┌──────▼────────┐
        │ chain-indexer │  ──► PostgreSQL
        └──────┬────────┘
               │ gRPC
   ┌───────────┴───────────┐
   │                       │
┌──▼─────────────┐   ┌─────▼──────────┐
│ block-explorer │   │   public-api   │
│      API       │   │  (REST + rate  │
│                │   │    limiting)   │
└──────┬─────────┘   └─────┬──────────┘
       │ REST              │ REST /api/v1
┌──────▼─────────┐         │
│    Frontend    │         ▼
│  (React SPA)   │     API consumers
└────────────────┘
```

## How chain data reaches the explorer

The explorer API reads chain data from chain-indexer over gRPC in one of two modes:

- **Standalone mode** (`INDEXER_URL`) — the API talks to chain-indexer directly. Raw data,
  no redaction. Used for public explorers on open chains.
- **Privacy mode** (`PRIVACY_PROXY_URL`) — the API sits behind
  [Open Privacy Suite](https://gateway-fm.github.io/open-privacy-suite/docs/getting-started/), which
  applies RBAC-based redaction and SSO before data reaches the explorer.

Exactly one source must be configured. See
[Deployment Modes](../../configuration/deployment-modes/) for the full reasoning and
configuration, including how the explorer integrates with the Open Privacy Suite proxy.

## Why the public API is separate

The public API runs as its own Go binary (`cmd/public-api`) connecting to the same database
and RPC node as the explorer. Running it separately provides:

- **Crash isolation** — a panic in the public API doesn't affect the explorer.
- **Independent scaling** — scale the public API horizontally without touching the explorer.
- **Zero-downtime deploys** — update the public API without restarting the explorer.
- **Dedicated rate limiting** — public API limits don't affect the explorer UI.

## Docker images

Production images are published to Docker Hub on each release:

- `gatewayfm/block-explorer-api`
- `gatewayfm/block-explorer-indexer`
- `gatewayfm/block-explorer-public-api`
- `gatewayfm/block-explorer-frontend`
