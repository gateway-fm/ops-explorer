# Production Readiness

Last updated: 2026-03-17

## Deployment Modes

### Standalone
Explorer connects directly to an RPC node. No authentication, all data public.

```
RPC Node → Indexer → PostgreSQL → API → Frontend
```

### Privacy Mode
Explorer connects via Privacy Proxy. Address visibility is enforced per-user based on ZK-proof authentication and RBAC grants. Set `PRIVACY_PROXY_URL` to enable.

```
RPC Node → Privacy Proxy → Indexer → PostgreSQL → API (proxy mode) → Frontend
```

In privacy mode the indexer's `RPC_URL` should point at the Privacy Proxy RPC endpoint (not the raw node) so indexed data flows through the same access control layer.

---

## Environment Variables

### API Service

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | `postgres://user:pass@host:5432/explorer?sslmode=require` |
| `RPC_URL` | Yes | RPC endpoint (raw node or privacy proxy) |
| `API_PORT` | No | Default: `8080` |
| `PRIVACY_PROXY_URL` | Privacy mode | Internal URL of privacy proxy backend, e.g. `http://privacy-proxy-backend:8080` |
| `PRIVACY_PROXY_PUBLIC_URL` | Privacy mode | Browser-facing URL of privacy proxy, e.g. `https://proxy.yourdomain.com` |
| `SSO_REDIRECT_URI` | Privacy mode | OAuth callback URI, e.g. `https://explorer.yourdomain.com/api/auth/callback` |
| `SSO_CLIENT_ID` | No | Default: `explorer` — must match the client ID registered in Privacy Proxy |
| `CATCHUP_WORKERS` | No | Parallel workers for historical sync (default: `10`) |
| `RPC_WORKERS` | No | Parallel RPC fetch workers (default: `50`) |
| `RPC_RATE_LIMIT` | No | Max RPC requests/sec (default: `500`) |

### Indexer Service

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | Same database as API |
| `RPC_URL` | Yes | RPC endpoint to index from |
| `POLL_INTERVAL` | No | Default: `2s` |
| `START_BLOCK` | No | Block to start indexing from (default: `0`) |

---

## Deployment Steps

### Standalone

```bash
export DATABASE_URL="postgres://user:pass@host:5432/explorer?sslmode=require"
export RPC_URL="https://rpc.yourdomain.com"

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
Neither the API nor the indexer exposes a dedicated health endpoint. For Kubernetes readiness probes, use a basic HTTP check against `/api/v1/blocks`.

### Database Migrations
Migrations run automatically on startup. If the database is unavailable at first boot the service will exit and restart — standard Kubernetes behaviour. No separate migration job is needed, but the database must be reachable before traffic is sent.

### Database Connection String
The base `docker-compose.yml` uses `sslmode=disable` for local dev convenience. The prod overlay replaces `DATABASE_URL` with a required env var — set it with `sslmode=require` for managed Postgres (RDS, Cloud SQL, etc.).

### Privacy Proxy Availability
In privacy mode the API has a runtime dependency on the privacy proxy. If the proxy is unreachable, authenticated endpoints will return errors. Plan for co-deployment or ensure the proxy is stable before the explorer is brought up.

### `SSO_CLIENT_ID` Alignment
The `SSO_CLIENT_ID` env var (default: `explorer`) must match the client identifier registered on the Privacy Proxy side. Keep these in sync when configuring both services.