---
title: Introduction
description: What the block explorer is, what it indexes, and how its services fit together.
---

The Block Explorer is a **lightweight, self-hosted explorer for EVM-compatible chains**.
Point it at any JSON-RPC endpoint and it indexes the chain into PostgreSQL, then serves a
fully searchable, real-time UI and a public REST API.

## What it indexes

- **Blocks** — with full transaction lists and timing
- **Transactions** — including event logs, token transfers, and internal calls
- **Addresses** — balances, transaction history, token holdings, and contract details
- **Tokens** — ERC-20, ERC-721, and ERC-1155, with holders and transfer history
- **Contracts** — source verification (via Sourcify) and ABI display
- **Chain stats** — gas prices, transaction history, and sync status

## The services

The explorer is composed of a few small services, each independently deployable:

| Service | Description |
|---------|-------------|
| **API** | Go HTTP server that powers the explorer frontend — blocks, transactions, addresses, gas prices. |
| **Public API** | A separate Go binary exposing a rate-limited, read-only REST API for programmatic access. |
| **Frontend** | React + TypeScript SPA built with Vite and TailwindCSS. |
| **chain-indexer** | A Go service (in a [separate repo](https://github.com/gateway-fm/ops-indexer)) that polls an RPC node and indexes chain data. |

Chain data reaches the explorer over gRPC from `chain-indexer`, either directly
(**standalone mode**) or mediated by the Open Privacy Suite proxy (**privacy mode**). See
[Deployment Modes](../../configuration/deployment-modes/) for the difference.

## Where to go next

- **[Quickstart](../quickstart/)** — get a local explorer running with one command.
- **[Configuration](../../configuration/overview/)** — every environment variable and option.
- **[Architecture](../../architecture/overview/)** — how the pieces fit together.
