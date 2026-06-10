---
title: Wallet Access (MetaMask)
description: How a browser wallet connects to a privacy-mode chain — why it needs the jwt-injector helper, and how the in-app "Add Network" dialog sets it up.
---

Connecting a wallet like MetaMask works very differently in the two
[deployment modes](../../configuration/deployment-modes/), because privacy mode puts an
authenticated, redacting proxy between the wallet and the chain.

## Standalone: connect directly

In standalone mode the chain's JSON-RPC is public, so a wallet talks to it directly. The
explorer's **Add Network** button adds the network using the node's browser-facing RPC
(`VITE_RPC_URL`) and the authoritative `chainId` from `GET /api/v1/chain-info`. Nothing else
is needed.

## Privacy: connect through the jwt-injector

In privacy mode a wallet **cannot reach the chain directly**. Every chain request has to carry
an authenticated bearer JWT and an org-scoped path, and a browser wallet has no way to attach
them. (The anonymous proxy `/rpc` endpoint only serves claim-free metadata such as
`eth_chainId` and `eth_blockNumber`, so it is deliberately **not** a wallet target.)

The bridge is **[jwt-injector](https://github.com/gateway-fm/jwt-injector)**, a small helper
the user runs locally. It holds the user's token and forwards their wallet's requests to
privacy-proxy with the bearer token and org-scoped path attached. MetaMask is then pointed at
the helper's local port instead of at the chain.

So the flow is:

1. The user clicks **Add Network** in the explorer.
2. Because the deployment is in privacy mode, this opens an **in-app setup dialog** that walks
   the user through running jwt-injector locally.
3. The dialog pre-fills the injector's upstream target from
   `privacyProxyPublicUrl` (exposed by `GET /api/v1/chain-info`, the public proxy base URL
   with no `/rpc` suffix).
4. The user points MetaMask at the local jwt-injector port. From then on, the wallet's
   requests flow wallet → jwt-injector (adds JWT + org path) → privacy-proxy (authenticates,
   redacts) → chain.

:::note[Two different "injectors"]
This page is about the **jwt-injector helper** that gives a *wallet* access. That is separate
from the explorer API's own [JWT injection](../authentication/#jwt-injection-forwarding-your-identity),
which attaches the user's token to the explorer's *server-side* reads. Same idea (forward the
user's identity to privacy-proxy), two different places it happens.
:::

## Related settings

The wallet-facing values come from the frontend's `VITE_*` configuration, covered in
[Frontend & wallet settings](../../configuration/overview/#frontend--wallet-metamask). The
authoritative `chainId` always comes from `GET /api/v1/chain-info`; the `VITE_*` values are
deploy-time fallbacks.
