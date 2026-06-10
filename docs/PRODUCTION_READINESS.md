# Production Readiness

Last updated: 2026-04-24

## Deployment Modes

Block-explorer no longer runs its own indexer. Chain data comes from
`gateway-fm/chain-indexer` over gRPC, either directly (standalone mode)
or mediated by privacy-proxy (privacy mode).

Pick **exactly one** chain-data source. The api process rejects having
both `INDEXER_URL` and `PRIVACY_PROXY_URL` set at startup, because the
combination would silently route chain data around privacy-proxy's
redaction while still wiring SSO through it — a privacy footgun.

### Standalone mode — `INDEXER_URL`

Block-explorer reads chain data direct from chain-indexer. Raw chain
data, no redaction, no per-user visibility. Use for public explorers on
open chains.

```
chain-indexer (gRPC :50051) ──► block-explorer api ──► frontend
                                    │
                                    ├─ postgres (verification only)
                                    └─ RPC node (contract verification)
```

### Privacy mode — `PRIVACY_PROXY_URL`

Block-explorer api is a thin frontend over privacy-proxy's REST
explorer API. Privacy-proxy applies RBAC-based redaction before data
reaches block-explorer. Auth/SSO flows through privacy-proxy too.

```
chain-indexer (gRPC) ──► privacy-proxy ──► block-explorer api ──► frontend
                          (redaction,              │
                           auth, RBAC)             └─ postgres (verification only)
```

> In practice, in privacy mode the block-explorer api is typically
> **not deployed at all** — `privacy-proxy`'s own `docker-compose.privacy.yml`
> ships only the block-explorer frontend, which proxies `/api/*` direct
> to privacy-proxy. This mode exists in the api binary for mixed
> deployments.

---

## Environment Variables

### API Service

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | `postgres://user:pass@host:5432/explorer?sslmode=require`. Used for contract-verification writes only. |
| `RPC_URL` | Yes | RPC endpoint used by the contract verifier. |
| `API_PORT` | No | Default: `8080`. |
| **Chain data — pick one:** | | |
| `INDEXER_URL` | One required | Standalone mode. `host:port` of a chain-indexer gRPC endpoint, e.g. `chain-indexer:50051`. |
| `PRIVACY_PROXY_URL` | One required | Privacy mode. Internal URL of privacy-proxy backend, e.g. `http://privacy-proxy-backend:8080`. |
| `PRIVACY_PROXY_PUBLIC_URL` | Privacy mode | Browser-facing URL of privacy-proxy, e.g. `https://proxy.yourdomain.com`. |
| `SSO_REDIRECT_URI` | Privacy mode | OAuth callback URI, e.g. `https://explorer.yourdomain.com/api/auth/callback`. |
| `SSO_CLIENT_ID` | No | Default: `explorer` — must match the client ID registered in privacy-proxy. |

**Validation rules (enforced in `cmd/api/main.go` at startup):**

- Neither `INDEXER_URL` nor `PRIVACY_PROXY_URL` set → `log.Fatal`.
- Both set → `log.Fatal`. No mixed-mode is supported.
- Exactly one set → starts in the corresponding mode.

**MetaMask "Add Network" (RD-1031).** How a wallet connects depends on the
mode:

- **Standalone:** the wallet connects to the node's public JSON-RPC directly.
  The "Add Network" button adds the network using `VITE_RPC_URL` (the node's
  browser-facing RPC; see Frontend Service below) and the authoritative
  `chainId` from `GET /api/v1/chain-info`.
- **Privacy:** the wallet cannot reach the network directly — every chain
  request needs an authenticated bearer JWT and an org-scoped path, which a
  browser wallet cannot attach. The anonymous proxy `/rpc` endpoint only serves
  claim-free metadata (`eth_chainId`, `eth_blockNumber`, …), so it is **not a
  wallet target**. Instead, the "Add Network" button opens an in-app setup
  dialog that walks the user through running a local
  [jwt-injector](https://github.com/gateway-fm/jwt-injector) helper (which holds
  their token and forwards requests with the bearer + org path attached) and
  points MetaMask at the helper's local port.

`PRIVACY_PROXY_PUBLIC_URL` is surfaced by `GET /api/v1/chain-info` as
`privacyProxyPublicUrl` (public base URL, **no** `/rpc` suffix) purely to
pre-fill the jwt-injector `--upstream` hint in that dialog. It is omitted when
unset, and it is never handed to a wallet as an RPC endpoint.

There is no longer a block-explorer indexer service — the separate
`Indexer Service` section has been removed. Run chain-indexer as a
sibling deployment; see its repo for its own env vars.

### Frontend Service

The frontend reads `VITE_*` variables at container startup (injected into
`window.__runtimeConfig`). The authoritative `chainId` comes from
`GET /api/v1/chain-info`; the `VITE_*` values below are deploy-time fallbacks.

| Variable | Required | Description |
|----------|----------|-------------|
| `VITE_API_URL` | No | Path/URL to the internal API. Default: `/api` (proxied by nginx/ingress). |
| `VITE_RPC_URL` | Standalone | The node's **browser-facing public JSON-RPC URL** that a wallet (and contract reads) connect to directly in **standalone** mode (e.g. `https://rpc.yourdomain.com`). A standalone/node concern — **not** the privacy-proxy. In **privacy** mode the wallet connects via a locally-run [jwt-injector](https://github.com/gateway-fm/jwt-injector) configured in the in-app setup dialog, so this is not used for the wallet there. Falls back to `http://localhost:8545` (the node default) when unset. |
| `VITE_CHAIN_ID` | Recommended | Chain ID in **decimal** (e.g. `4242`) for the MetaMask "Add Network" button. Used only when the backend does not supply `chainId`. A wrong value here is what causes MetaMask to add the network with the wrong chain. |
| `VITE_NETWORK_NAME` | No | Network name shown in MetaMask. |
| `VITE_NETWORK_CURRENCY` | No | Native currency symbol shown in MetaMask. Default: `ETH`. |

> **Standalone vs privacy.** In standalone the wallet talks to the node
> directly, so set `VITE_RPC_URL` to the node's public RPC. In privacy mode the
> wallet connects through a locally-run jwt-injector (configured per-user in the
> in-app MetaMask setup dialog), so `VITE_RPC_URL` is not the wallet's target —
> the dialog uses the proxy public base URL (from `/chain-info`) only as a hint
> for the injector's `--upstream`. The base `docker-compose.yml` defaults
> `VITE_RPC_URL` to `http://localhost:8545`; `docker-compose.prod.yml` leaves it
> to the operator.

---

## Deployment Steps

### Standalone

```bash
export DATABASE_URL="postgres://user:pass@host:5432/explorer?sslmode=require"
export RPC_URL="https://rpc.yourdomain.com"
export INDEXER_URL="chain-indexer:50051"     # or your chain-indexer host:port
# Do NOT set PRIVACY_PROXY_URL — the api rejects both being set.

docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build

# Verify (exec into container — API port is not exposed to host in prod)
docker compose exec api wget -qO- http://localhost:8080/api/v1/blocks?limit=1
```

### Privacy Mode

```bash
export DATABASE_URL="postgres://user:pass@host:5432/explorer?sslmode=require"
export RPC_URL="http://privacy-proxy-backend:8080"
export PRIVACY_PROXY_URL="http://privacy-proxy-backend:8080"
export PRIVACY_PROXY_PUBLIC_URL="https://proxy.yourdomain.com"
export SSO_REDIRECT_URI="https://explorer.yourdomain.com/api/auth/callback"
# Do NOT set INDEXER_URL — the api rejects both being set.

# The external privacy-proxy network must exist before starting
docker network create privacy-proxy_proxy-network 2>/dev/null || true

docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

> **Note:** Docker Compose v2.x is required — the prod overlay uses the `!reset` tag to clear port and volume mappings, which is not supported by legacy `docker-compose` v1.

---

## Notes for Ops

### Test Coverage
Backend test coverage is ~6.7% (tracked in `requirements.md`). Core indexing and API logic relies on integration-level validation rather than unit tests. Recent security patches (SSRF, integer overflows) have been applied.

### Health Checks
The API exposes `/health`, `/health/live`, and `/health/ready`. Use `/health/ready` for Kubernetes readiness probes and `/health/live` for liveness.

### Database Migrations
Migrations run automatically on startup. If the database is unavailable at first boot the service will exit and restart — standard Kubernetes behaviour. No separate migration job is needed, but the database must be reachable before traffic is sent.

### Database Connection String
The base `docker-compose.yml` uses `sslmode=disable` for local dev convenience. The prod overlay replaces `DATABASE_URL` with a required env var — set it with `sslmode=require` for managed Postgres (RDS, Cloud SQL, etc.).

### Privacy Proxy Availability
In privacy mode the API has a runtime dependency on the privacy proxy. If the proxy is unreachable, authenticated endpoints will return errors. Plan for co-deployment or ensure the proxy is stable before the explorer is brought up.

### `SSO_CLIENT_ID` Alignment
The `SSO_CLIENT_ID` env var (default: `explorer`) must match the client identifier registered on the Privacy Proxy side. Keep these in sync when configuring both services.