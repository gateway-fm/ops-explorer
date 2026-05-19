<p align="center">
  <img src="frontend/public/mascot.png" alt="Block Explorer" width="180" />
</p>

<h1 align="center">Block Explorer</h1>

<p align="center">
  A lightweight, self-hosted block explorer for EVM-compatible chains.<br/>
  Point it at any RPC endpoint and get a fully indexed, searchable explorer with real-time updates.
</p>

<p align="center">
  <a href="https://github.com/gateway-fm/block-explorer/actions/workflows/ci.yml"><img src="https://github.com/gateway-fm/block-explorer/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/gateway-fm/block-explorer/actions/workflows/docker-build.yml"><img src="https://github.com/gateway-fm/block-explorer/actions/workflows/docker-build.yml/badge.svg" alt="Docker Build" /></a>
  <a href="https://github.com/gateway-fm/block-explorer/actions/workflows/release.yml"><img src="https://github.com/gateway-fm/block-explorer/actions/workflows/release.yml/badge.svg" alt="Release" /></a>
</p>

---

## Architecture

| Service | Description |
|---------|-------------|
| **API** | Go HTTP server — blocks, transactions, addresses, gas prices |
| **Indexer** | Go service that polls an RPC node and indexes chain data into PostgreSQL |
| **Frontend** | React + TypeScript SPA built with Vite and TailwindCSS |

## Quickstart (Dev Mode)

Spins up a local [Anvil](https://book.getfoundry.sh/reference/anvil/) testnet, PostgreSQL, the backend services, the frontend, and the chain-indexer — all via Docker Compose.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- [Make](https://www.gnu.org/software/make/)
- A locally built `gatewayfm/chain-indexer:latest` image. The indexer lives in a separate repo ([gateway-fm/chain-indexer](https://github.com/gateway-fm/chain-indexer)) and is no longer bundled here. Clone it as a sibling of this checkout and build the image before running `make dev`:

  ```bash
  git clone https://github.com/gateway-fm/chain-indexer.git ../chain-indexer
  cd ../chain-indexer && make docker-build
  ```

  `make dev` checks for the image and exits with a hint if it isn't present. Rebuild the image whenever you pull new indexer changes.

### Start

```bash
make dev
```

Once ready:

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

### Stop / Destroy

```bash
make dev-stop      # stop containers (keep volumes)
make dev-destroy   # stop and remove containers, volumes, and images
```

### Other Dev Commands

```bash
make dev-logs              # tail logs from all services
make dev-rebuild-backend   # rebuild api + indexer with no cache
```

## External RPC

Point the explorer at any EVM-compatible RPC endpoint:

```bash
RPC_URL=https://rpc.example.com make run
```

```bash
make stop   # stop
make logs   # tail logs
```

## Custom Branding

The explorer supports full whitelabel branding via environment variables — change the name, logo, colors, and footer links without touching any code.

### Quick Example

```bash
# Dev mode with custom branding
make dev BRAND=examples/branding/docker-compose.full.yml

# Production with custom branding
RPC_URL=https://rpc.example.com make run BRAND=examples/branding/docker-compose.l2-chain.yml
```

### Example Override Files

| File | Description |
|------|-------------|
| `examples/branding/docker-compose.minimal.yml` | Just rename the explorer (2 variables) |
| `examples/branding/docker-compose.full.yml` | Every branding option customised |
| `examples/branding/docker-compose.no-socials.yml` | Internal deploy — no footer links |
| `examples/branding/docker-compose.l2-chain.yml` | L2 chain — branding + network config |

### Key Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_BRAND_NAME` | Explorer name | `Gateway Explorer` |
| `VITE_BRAND_COMPANY` | Footer copyright | `Gateway.fm AS` |
| `VITE_BRAND_COLOR_PRIMARY` | Primary color (hex) | `#8950FA` |
| `VITE_BRAND_LOGO` | Logo path or URL | `/logo.svg` |
| `VITE_BRAND_WEBSITE` | Company URL | `https://gateway.fm/` |

Set any social or company link to `""` to hide it from the footer.

See [docs/BRANDING.md](docs/BRANDING.md) for the full reference.

## Public API

The explorer includes a rate-limited public REST API for programmatic access to blockchain data. It runs as a separate service for isolation and independent scaling.

```bash
# Available at http://localhost:8082 by default
curl http://localhost:8082/api/v1/blocks/latest
curl http://localhost:8082/api/v1/transactions?page=1&pageSize=10
curl http://localhost:8082/api/v1/search/suggestions?q=0xf39
```

| Variable | Default | Description |
|----------|---------|-------------|
| `PUBLIC_API_PORT` | `8082` | Public API port |
| `RATE_LIMIT` | `100` | Requests per window |
| `RATE_LIMIT_WINDOW` | `1m` | Rate limit window |

Interactive API docs are available at `/api-docs` in the explorer UI. See [docs/PUBLIC_API.md](docs/PUBLIC_API.md) for the full reference.

## Docker Images

Production images are published to Docker Hub on each release:

- `gatewayfm/block-explorer-api`
- `gatewayfm/block-explorer-indexer`
- `gatewayfm/block-explorer-public-api`
- `gatewayfm/block-explorer-frontend`

### Build Locally

```bash
make docker-build           # build all images
make docker-build-dry-run   # build without pushing (CI verification)
```

## CI / Quality

```bash
make lint    # go vet + eslint + tsc
make test    # go test -race
make build   # compile backend + frontend
```

## Port Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | 8081 | API server |
| `FRONTEND_PORT` | 3001 | Frontend dev server |
| `POSTGRES_PORT` | 5433 | PostgreSQL |
| `ANVIL_PORT` | 8546 | Anvil RPC (dev only) |

## Endpoints

| Service | URL |
|---------|-----|
| Explorer | [localhost:3001](http://localhost:3001) |
| API | [localhost:8081](http://localhost:8081) |
| Anvil RPC | [localhost:8546](http://localhost:8546) |
