---
title: How Privacy Mode Works
description: The end-to-end request, redaction, and auth flow when the block explorer runs behind privacy-proxy.
---

In privacy mode the explorer's API is a **backend-for-frontend (BFF)**: it holds no chain
data, and every read is proxied to privacy-proxy, which authenticates the caller and redacts
the response before it ever reaches the explorer.

## Architecture

```
                              ┌──────────────────────────┐
chain-indexer (gRPC) ───────► │       privacy-proxy      │
                              │  auth · RBAC · redaction │
                              └──────────────┬───────────┘
                                             │ REST (token attached)
                              ┌──────────────▼───────────┐
                              │   block-explorer API     │
                              │  backend-for-frontend    │ ──► postgres (verification only)
                              └──────────────┬───────────┘
                                             │ /api/*
                              ┌──────────────▼───────────┐
                              │   block-explorer frontend │
                              └───────────────────────────┘
```

Compare this with standalone mode, where the explorer talks to chain-indexer directly with no
service in between. See [Deployment Modes](../../configuration/deployment-modes/).

## A request, step by step

1. A signed-in user opens an address or transaction in the explorer UI.
2. The frontend calls the explorer API at `/api/v1/...`, sending the user's session cookie.
3. The explorer API (the BFF) forwards that read to privacy-proxy's REST explorer API,
   [injecting the user's token](../authentication/#jwt-injection-forwarding-your-identity) as
   an `Authorization: Bearer` header.
4. **Privacy-proxy authenticates the caller, resolves their role and grants, and redacts the
   response.** The explorer only ever receives already-filtered data.
5. The explorer renders the redacted result. Two different users can receive two different
   views of the same on-chain object.

Because each response is scoped to one caller, the explorer **never caches** privacy-proxy
data: a per-caller provider is structurally prevented from being cache-wrapped, so one user's
redacted view can never leak into another's.

## Access control and redaction

The rules that decide what each user sees are defined and enforced in privacy-proxy, not in
the explorer:

- **[RBAC](https://gateway-fm.github.io/privacy-proxy/docs/rbac/)** — roles and the data
  grants attached to them.
- **[Response filtering](https://gateway-fm.github.io/privacy-proxy/docs/security/response-filtering/)**
  — how fields and records are redacted before a response is returned.

The explorer simply displays the filtered data. To change what a user can see, change their
role or grants in privacy-proxy.

:::note[Who serves the frontend?]
In a typical privacy deployment, privacy-proxy serves the block-explorer frontend itself and
proxies `/api/*` straight to its own REST API, so the standalone block-explorer API is **not
deployed at all**. The API binary still supports privacy mode for mixed deployments where you
run the explorer's own API behind privacy-proxy.
:::

## Next

- [Authentication & SSO](../authentication/) — how users sign in.
- [View as user](../view-as-user/) — operator impersonation.
- [Deploying in privacy mode](../deployment/) — the fail-closed configuration.
