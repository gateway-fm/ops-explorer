---
title: Quickstart
description: Spin up a local block explorer — testnet, database, indexer, API, and frontend — with a single command.
---

This guide gets a complete explorer running locally in **dev mode**: a local
[Anvil](https://book.getfoundry.sh/reference/anvil/) testnet, PostgreSQL, the backend
services, the frontend, and the chain-indexer — all via Docker Compose.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose (v2.x or newer)
- [Make](https://www.gnu.org/software/make/)
- A locally built `gatewayfm/chain-indexer:latest` image. The indexer lives in a
  [separate repo](https://github.com/gateway-fm/ops-indexer) — only the repo was
  renamed to `ops-indexer`; the Docker image keeps the `chain-indexer` name.
  Clone it as a sibling of this checkout and build the image before running `make dev`:

  ```bash
  git clone https://github.com/gateway-fm/ops-indexer.git ../chain-indexer
  cd ../chain-indexer && make docker-build
  ```

  `make dev` checks for the image and exits with a hint if it isn't present. Rebuild the
  image whenever you pull new indexer changes.

## Start

```bash
make dev
```

Once ready you'll see:

```
Block Explorer (dev) is ready!

  Explorer:  http://localhost:3001
  API:       http://localhost:8081
  Anvil RPC: http://localhost:8546
```

A pre-funded test account is available:

```
Address:     0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
Private Key: 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
```

:::caution[Test key]
This Anvil key is a well-known public test key. **Never** use it on a real network.
:::

## Stop / Destroy

```bash
make dev-stop      # stop containers (keep volumes)
make dev-destroy   # stop and remove containers, volumes, and images
```

## Other dev commands

```bash
make dev-logs              # tail logs from all services
make dev-rebuild-backend   # rebuild api + indexer with no cache
```

## Point at an external RPC

To run against any EVM-compatible RPC endpoint instead of the local testnet:

```bash
RPC_URL=https://rpc.example.com make run
```

```bash
make stop   # stop
make logs   # tail logs
```

## Next steps

- Customise the look with [Branding & Whitelabel](../../configuration/branding/).
- Review every available option in [Configuration](../../configuration/overview/).
- Going to production? Read [Deployment Modes](../../configuration/deployment-modes/).
