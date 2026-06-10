---
title: Public REST API
description: A read-only, rate-limited REST API for programmatic access to indexed blockchain data — blocks, transactions, addresses, tokens, search, and stats.
---

The block explorer exposes a read-only public REST API for programmatic access to indexed
blockchain data. It runs as a separate service with its own rate limiting, isolated from
the explorer frontend API.

## Architecture

The public API is a standalone Go binary (`cmd/public-api`) that connects directly to the
same PostgreSQL database and RPC node as the explorer. Running it as a separate service
provides crash isolation, independent scaling, zero-downtime deploys, and dedicated rate
limiting. See [System Architecture](../../architecture/overview/) for the full picture.

## Quick start

```bash
make dev    # dev mode (includes public API automatically)
make run    # production
```

The public API is available at `http://localhost:8082` by default.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PUBLIC_API_PORT` | `8082` | Port for the public API server |
| `RATE_LIMIT` | `100` | Maximum requests per time window |
| `RATE_LIMIT_WINDOW` | `1m` | Rate limit time window (e.g. `1m`, `30s`, `5m`) |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `RPC_URL` | — | Ethereum JSON-RPC endpoint |

## Rate limiting

All requests are rate-limited by client IP address using a sliding window algorithm.
**Default: 100 requests per minute.**

| Response header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests allowed per window |
| `X-RateLimit-Remaining` | Requests remaining in current window |
| `X-RateLimit-Reset` | Unix timestamp when the window resets |

When the limit is exceeded, the API returns `429 Too Many Requests`:

```json
{ "error": "rate limit exceeded" }
```

## Base URL

All endpoints are prefixed with `/api/v1`.

```
http://localhost:8082/api/v1/blocks
```

## Endpoints

### Blocks

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/blocks` | List blocks |
| GET | `/api/v1/blocks/latest` | Get latest block |
| GET | `/api/v1/blocks/{number}` | Get block by number (includes transactions) |

**Query parameters for list:** `limit` (int, default 25, max 100), `before` (int — cursor:
return blocks before this number).

### Transactions

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/transactions` | List transactions |
| GET | `/api/v1/transactions/{hash}` | Get transaction by hash |
| GET | `/api/v1/transactions/{hash}/logs` | Get transaction event logs |
| GET | `/api/v1/transactions/{hash}/transfers` | Get token transfers in transaction |
| GET | `/api/v1/transactions/{hash}/internal` | Get internal transactions |

**Query parameters for list:** `page` (int, default 1), `pageSize` (int, default 25, max 100).

### Addresses

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/addresses/{address}` | Get address info (balance, tx count, contract flag) |
| GET | `/api/v1/addresses/{address}/transactions` | Get address transactions |
| GET | `/api/v1/addresses/{address}/transfers` | Get address token transfers |
| GET | `/api/v1/addresses/{address}/balances` | Get address token balances |
| GET | `/api/v1/addresses/{address}/internal` | Get address internal transactions |
| GET | `/api/v1/addresses/{address}/logs` | Get address event logs |
| GET | `/api/v1/addresses/{address}/contract` | Get contract details + ABI |

**Address transactions** use cursor pagination (`limit`, `before`). Other sub-endpoints use
offset pagination (`page`, `pageSize`).

### Tokens

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/tokens` | List tokens |
| GET | `/api/v1/tokens/{address}` | Get token detail |
| GET | `/api/v1/tokens/{address}/holders` | Get token holders |
| GET | `/api/v1/tokens/{address}/transfers` | Get token transfers |

**Query parameters for list:** `page` (int, default 1), `pageSize` (int, default 25, max
100), `type` (string — filter by token type: `erc20`, `erc721`).

### Token transfers

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/token-transfers` | List all token transfers |

### Accounts

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/accounts` | List top accounts by balance |

### Search

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/search` | Search blocks, transactions, addresses |
| GET | `/api/v1/search/suggestions` | Get autocomplete suggestions |

**Query parameter:** `q` (string, required) — the search query.

### Stats

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/stats` | Chain statistics (total blocks, txs, addresses) |
| GET | `/api/v1/stats/tx-history` | Transaction history chart data |
| GET | `/api/v1/gas` | Current gas prices |
| GET | `/api/v1/chain-info` | Chain info (chain ID, client version, peers) |
| GET | `/api/v1/sync` | Indexer sync status |

### Meta

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/openapi.json` | OpenAPI 3.0 specification |
| GET | `/health` | Health check |

## Pagination

The API uses two pagination styles depending on the endpoint.

**Cursor pagination** (blocks, address transactions):

```json
{
  "data": [],
  "nextCursor": "12345",
  "hasMore": true
}
```

Pass the `nextCursor` value as the `before` parameter to get the next page.

**Offset pagination** (transactions, tokens, accounts, etc.):

```json
{
  "data": [],
  "total": 1000,
  "page": 1,
  "pageSize": 25,
  "totalPages": 40
}
```

## Error responses

All errors return a JSON object with an `error` field:

```json
{ "error": "block not found" }
```

| Status | Description |
|--------|-------------|
| 400 | Bad request (invalid parameters) |
| 404 | Resource not found |
| 429 | Rate limit exceeded |
| 500 | Internal server error |

## Examples

```bash
# Get latest block
curl http://localhost:8082/api/v1/blocks/latest

# List recent transactions
curl "http://localhost:8082/api/v1/transactions?page=1&pageSize=10"

# Get address info
curl http://localhost:8082/api/v1/addresses/0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266

# Search suggestions
curl "http://localhost:8082/api/v1/search/suggestions?q=0xf39"

# Chain info and gas
curl http://localhost:8082/api/v1/chain-info
curl http://localhost:8082/api/v1/gas
```

## Interactive docs

The explorer includes an interactive API documentation page at **`/api-docs`** in the
frontend. It documents all endpoints with example requests and responses, and includes a
"Try it" feature for making live requests.

## Docker

The public API runs as the `public-api` service in Docker Compose:

```yaml
services:
  public-api:
    build:
      context: ./backend
      dockerfile: Dockerfile.public-api
    ports:
      - "8082:8082"
    environment:
      DATABASE_URL: postgres://postgres:postgres@postgres:5432/explorer?sslmode=disable
      RPC_URL: http://your-rpc-node:8545
      RATE_LIMIT: "100"
      RATE_LIMIT_WINDOW: "1m"
```
