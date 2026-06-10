---
title: Deploying in Privacy Mode
description: The fail-closed configuration, security hardening, and rollout steps for running the block explorer behind privacy-proxy.
---

Privacy mode is **fail-closed**: where standalone mode leans permissive for convenience,
privacy mode refuses to start unless the security-critical settings are explicit. This page
covers what changes and how to roll it out. For the full side-by-side environment-variable
table, see [Deployment Modes](../../configuration/deployment-modes/#environment-variables).

## Fail-closed rules

With `PRIVACY_PROXY_URL` set, the API enforces stricter defaults and requirements:

| Setting | Privacy-mode behaviour | Why |
|---------|------------------------|-----|
| `CORS_ALLOWED_ORIGINS` | **Required** (empty → startup error) | No credentialed request may come from an un-allowlisted origin. |
| `COOKIE_SECURE` | Defaults to `true` (always `Secure`) | Session cookies must never travel over plain HTTP. |
| `ENABLE_PRICE` | Defaults to `false` | No CoinGecko egress from a confidential deployment. |
| Provider cache | Must stay off | A per-caller redacted view must never be cached or shared; startup panics if it is. |
| Contract verification | Disabled at runtime (default build) | Avoids write surfaces in a locked-down deployment. |

:::caution[The CORS requirement is a breaking change]
Existing privacy operators upgrading to a fail-closed build must set `CORS_ALLOWED_ORIGINS`
explicitly. An empty value is a startup error, not a permissive default.
:::

## The `-tags privacy` build

The default image keeps contract-verification surfaces present but disabled at runtime, which
suits mixed deployments. For a hardened, confidential deployment, prefer the **`-tags privacy`
image**, which compiles those write surfaces out entirely so they cannot be reached at all.

## Rollout

```bash
export DATABASE_URL="postgres://user:pass@host:5432/explorer?sslmode=require"
export RPC_URL="http://privacy-proxy-backend:8080"
export PRIVACY_PROXY_URL="http://privacy-proxy-backend:8080"
export PRIVACY_PROXY_PUBLIC_URL="https://proxy.yourdomain.com"
export SSO_REDIRECT_URI="https://explorer.yourdomain.com/api/auth/callback"
export SSO_CLIENT_SECRET="$(aws secretsmanager get-secret-value ... )"  # never inline
# REQUIRED in privacy mode (fail-closed) — comma-separated trusted origins:
export CORS_ALLOWED_ORIGINS="https://explorer.yourdomain.com"
# Recommended — enables JWT verification + "View as user":
export SSO_JWKS_URL="http://privacy-proxy-backend:8080/.well-known/jwks.json"
# Do NOT set INDEXER_URL. Prefer the `-tags privacy` image.

# The external privacy-proxy network must exist before starting
docker network create privacy-proxy_proxy-network 2>/dev/null || true

docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

:::note
Docker Compose v2.x is required: the prod overlay uses the `!reset` tag to clear port and
volume mappings, which legacy `docker-compose` v1 does not support.
:::

## Operational notes

- **Privacy-proxy is a hard dependency.** If it is unreachable, authenticated endpoints
  return errors. Co-deploy it, or bring it up and confirm it is healthy before the explorer.
- **Client id alignment.** `SSO_CLIENT_ID` (default `explorer`) must match the client
  registered in privacy-proxy, or sign-in fails. See [Authentication](../authentication/).
- **Database role.** The explorer's own Postgres is used for contract-verification metadata
  only in mixed deployments; all chain data comes from privacy-proxy.

## Privacy-proxy documentation

The redaction, identity, and access-control rules live in privacy-proxy:

- [Getting started](https://gateway-fm.github.io/privacy-proxy/docs/getting-started/)
- [Architecture](https://gateway-fm.github.io/privacy-proxy/docs/architecture/)
- [Block explorer integration](https://gateway-fm.github.io/privacy-proxy/docs/explorer/)
- [Authentication](https://gateway-fm.github.io/privacy-proxy/docs/authentication/)
- [RBAC](https://gateway-fm.github.io/privacy-proxy/docs/rbac/)
- [Response filtering](https://gateway-fm.github.io/privacy-proxy/docs/security/response-filtering/)
- [View as user](https://gateway-fm.github.io/privacy-proxy/docs/security/view-as-user/)
