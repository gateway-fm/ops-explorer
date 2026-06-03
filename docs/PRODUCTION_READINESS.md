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

`PRIVACY_PROXY_PUBLIC_URL` also drives the **MetaMask "Add Network"**
integration: the api derives the canonical browser-facing JSON-RPC URL as
`PRIVACY_PROXY_PUBLIC_URL` + `/rpc` and returns it from
`GET /api/v1/chain-info` as `rpcUrl`. The frontend uses that value (and the
authoritative `chainId`) when adding the network to a wallet. If
`PRIVACY_PROXY_PUBLIC_URL` is unset, `rpcUrl` is omitted and the frontend
falls back to `VITE_RPC_URL` (below) or the explorer's own page origin + `/rpc`
— never a direct node or `localhost`.

There is no longer a block-explorer indexer service — the separate
`Indexer Service` section has been removed. Run chain-indexer as a
sibling deployment; see its repo for its own env vars.

### Frontend Service

The frontend reads `VITE_*` variables at container startup (injected into
`window.__runtimeConfig`). These are **fallbacks** for the MetaMask "Add
Network" button and contract read calls — the authoritative `chainId` and
`rpcUrl` come from `GET /api/v1/chain-info`.

| Variable | Required | Description |
|----------|----------|-------------|
| `VITE_API_URL` | No | Path/URL to the internal API. Default: `/api` (proxied by nginx/ingress). |
| `VITE_RPC_URL` | Recommended | Browser-facing JSON-RPC URL for the MetaMask "Add Network" button and contract reads. **Must be the privacy-proxy `/rpc` endpoint** (e.g. `https://proxy.yourdomain.com/rpc`), never a direct node or `localhost` — all chain access must go through the proxy. Used only when the backend does not supply `rpcUrl`. |
| `VITE_CHAIN_ID` | Recommended | Chain ID in **decimal** (e.g. `4242`) for the MetaMask "Add Network" button. Used only when the backend does not supply `chainId`. A wrong value here is what causes MetaMask to add the network with the wrong chain. |
| `VITE_NETWORK_NAME` | No | Network name shown in MetaMask. |
| `VITE_NETWORK_CURRENCY` | No | Native currency symbol shown in MetaMask. Default: `ETH`. |

> **Why both backend and frontend config?** The backend (`/chain-info`) is the
> source of truth, so a correctly-configured `PRIVACY_PROXY_PUBLIC_URL` is
> enough on its own. `VITE_RPC_URL` / `VITE_CHAIN_ID` exist as deploy-time
> fallbacks (and for standalone deployments where the proxy public URL is not
> set). The base `docker-compose.yml` and `docker-compose.prod.yml` default
> `VITE_RPC_URL` from `PRIVACY_PROXY_PUBLIC_URL` + `/rpc` so a stock privacy
> deploy works without extra wiring.

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