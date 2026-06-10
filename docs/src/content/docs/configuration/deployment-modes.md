---
title: Deployment Modes
description: Standalone vs. privacy mode — how chain data reaches the explorer, the environment variables for each, and the fail-closed security rules.
---

Block-explorer does not run its own indexer. Chain data comes from
[`gateway-fm/chain-indexer`](https://github.com/gateway-fm/chain-indexer) over gRPC, either
directly (**standalone mode**) or mediated by privacy-proxy (**privacy mode**).

:::danger[Pick exactly one source]
The API process **rejects having both `INDEXER_URL` and `PRIVACY_PROXY_URL` set** at
startup. The combination would silently route chain data around privacy-proxy's redaction
while still wiring SSO through it — a privacy footgun.
:::

## At a glance

|  | Standalone | Privacy |
|---|---|---|
| Chain data source | chain-indexer over gRPC (`INDEXER_URL`) | privacy-proxy REST (`PRIVACY_PROXY_URL`) |
| Data visibility | Raw and public; every visitor sees everything | RBAC-redacted, per authenticated user |
| Authentication | None | SSO / OAuth, via privacy-proxy |
| Who serves the frontend | The explorer's own frontend | Usually privacy-proxy itself |
| Use it for | Public explorers on open chains | Confidential or permissioned chains |

In **standalone mode** the explorer is self-contained: its API reads chain data straight
from chain-indexer and serves it to anyone. In **privacy mode** the explorer becomes a thin
layer in front of [privacy-proxy](https://gateway-fm.github.io/privacy-proxy/docs/getting-started/),
which owns identity, access control, and redaction. The explorer forwards every read to
privacy-proxy with the caller's auth token attached, and privacy-proxy decides what that
user is allowed to see.

:::tip[Privacy mode has a dedicated section]
Privacy mode depends on a separate privacy-proxy service and has its own setup. See the
**[Privacy](../../privacy/overview/)** section for how it works, sign-in, "View as user", and
fail-closed deployment.
:::

## Standalone mode — `INDEXER_URL`

Block-explorer reads chain data directly from chain-indexer. Raw chain data, no redaction,
no per-user visibility. Use for **public explorers on open chains**.

```
chain-indexer (gRPC :50051) ──► block-explorer api ──► frontend
                                    │
                                    ├─ postgres (verification only)
                                    └─ RPC node (contract verification)
```

## Privacy mode — `PRIVACY_PROXY_URL`

Block-explorer api is a thin frontend over privacy-proxy's REST explorer API. Privacy-proxy
applies RBAC-based redaction before data reaches block-explorer. Auth/SSO flows through
privacy-proxy too.

```
chain-indexer (gRPC) ──► privacy-proxy ──► block-explorer api ──► frontend
                          (redaction,              │
                           auth, RBAC)             └─ postgres (verification only)
```

Privacy mode has its own section covering the request and redaction flow, sign-in,
"View as user", and fail-closed deployment:

- **[Privacy Mode overview](../../privacy/overview/)**
- **[How privacy mode works](../../privacy/how-it-works/)**
- **[Authentication & SSO](../../privacy/authentication/)**
- **[View as user](../../privacy/view-as-user/)**
- **[Deploying in privacy mode](../../privacy/deployment/)**

## Environment variables

### API service

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
| `SSO_CLIENT_SECRET` | Privacy mode (OAuth) | Client secret sent to privacy-proxy `/oauth/token` via HTTP Basic. **Secret — source from a secrets manager, never plaintext.** |
| `SSO_JWKS_URL` | Privacy mode | privacy-proxy JWKS endpoint. When set, the auth-cookie JWT is signature-verified in-process (alg-confusion-safe; RS256/ES256 only; `exp` mandatory; 30s leeway) before its DID is trusted. **Required to enable "View as user" impersonation.** |
| `SSO_ISSUER` | No | If set, the JWT `iss` claim must equal it (only checked when `SSO_JWKS_URL` is set). |
| `SSO_AUDIENCE` | No | If set, the JWT `aud` claim must include it (only checked when `SSO_JWKS_URL` is set). |
| `CORS_ALLOWED_ORIGINS` | **Privacy: yes** / Standalone: no | Comma-separated allowlist of browser origins permitted to make credentialed cross-origin requests (also reused for CSRF Origin/Referer checks). **Required in privacy mode** (fail-closed). In standalone, empty = reflect any Origin (permissive, with a startup warning). |
| `COOKIE_SECURE` | No | `auto` \| `true` \| `false`. Controls the `Secure` flag on auth cookies. **Default: `true` in privacy mode**, `auto` in standalone (Secure only over real HTTPS). |
| `ENABLE_PRICE` | No | Gates the CoinGecko price refresher + `/price` data source. **Default: `false` in privacy mode** (no third-party egress), `true` in standalone. |
| `PRICE_COIN_ID` | No | Default: `ethereum`. CoinGecko coin id to price (set per chain). |
| `PRICE_CURRENCY` | No | Default: `usd`. Fiat currency for the price quote. |

### Validation rules

Enforced at startup (`internal/config` `Validate()` + `cmd/api/main.go`):

- Neither `INDEXER_URL` nor `PRIVACY_PROXY_URL` set → `log.Fatal`.
- Both set → `log.Fatal`. No mixed-mode is supported.
- Exactly one set → starts in the corresponding mode.

:::caution[Privacy mode is fail-closed]
With `PRIVACY_PROXY_URL` set:

- `CORS_ALLOWED_ORIGINS` is **required** (empty → startup error). This is an intentional
  **breaking change** for existing privacy operators.
- `COOKIE_SECURE` defaults to `true` (forced Secure cookies).
- `ENABLE_PRICE` defaults to `false` (no CoinGecko egress).
- "View as user" impersonation is enabled only when `SSO_JWKS_URL` is set (the caller-DID
  binding requires a verified DID).
- The default (non-`-tags privacy`) binary disables the contract-verification / Sourcify
  write surfaces at runtime; deploy the `-tags privacy` image to compile them out entirely.
  Keep `ENABLE_PROVIDER_CACHE` off in privacy mode (startup panics fail-closed if the
  per-caller proxy provider is cache-wrapped).
:::

There is no longer a block-explorer indexer service. Run chain-indexer as a sibling
deployment; see its repo for its own env vars.

## Deployment steps

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

:::note
Docker Compose v2.x is required — the prod overlay uses the `!reset` tag to clear port and
volume mappings, which legacy `docker-compose` v1 does not support.
:::

For the privacy-mode rollout, including the fail-closed settings and the `-tags privacy`
image, see [Deploying in privacy mode](../../privacy/deployment/).

## Notes for ops

**Health checks** — the API exposes `/health`, `/health/live`, and `/health/ready`. Use
`/health/ready` for Kubernetes readiness probes and `/health/live` for liveness.

**Database migrations** — run automatically on startup. If the database is unavailable at
first boot the service exits and restarts (standard Kubernetes behaviour). No separate
migration job is needed, but the database must be reachable before traffic is sent.

**Database connection string** — the base `docker-compose.yml` uses `sslmode=disable` for
local dev convenience. The prod overlay requires `DATABASE_URL` as an env var — set it with
`sslmode=require` for managed Postgres (RDS, Cloud SQL, etc.).

**Privacy proxy availability** — in privacy mode the API has a runtime dependency on the
privacy proxy. If the proxy is unreachable, authenticated endpoints return errors. Plan for
co-deployment or ensure the proxy is stable before the explorer is brought up.

**`SSO_CLIENT_ID` alignment** — the `SSO_CLIENT_ID` env var (default: `explorer`) must match
the client identifier registered on the privacy-proxy side. Keep these in sync.

## Next

Running in privacy mode? The **[Privacy](../../privacy/overview/)** section covers how it
works, authentication, "View as user", and the fail-closed deployment in depth, with links
into the privacy-proxy documentation.
