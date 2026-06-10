---
title: Privacy Mode
description: Run the block explorer as a privacy-preserving, permissioned explorer behind privacy-proxy — RBAC-redacted data, SSO login, and per-user visibility.
---

The explorer can run in one of two [deployment modes](../../configuration/deployment-modes/).
This section covers **privacy mode**: the explorer runs in front of
[privacy-proxy](https://gateway-fm.github.io/privacy-proxy/docs/getting-started/), which
authenticates every user and returns only the data they are permitted to see.

In **standalone mode** the explorer is a public window onto an open chain. Privacy mode turns
it into a **permissioned, per-user explorer** for confidential chains: two users hitting the
same address can see two different, individually redacted views.

## At a glance

|  | Standalone | Privacy |
|---|---|---|
| Chain data source | chain-indexer over gRPC (`INDEXER_URL`) | privacy-proxy REST (`PRIVACY_PROXY_URL`) |
| Data visibility | Raw and public; every visitor sees everything | RBAC-redacted, per authenticated user |
| Authentication | None | SSO / OAuth, via privacy-proxy |
| Who serves the frontend | The explorer's own frontend | Usually privacy-proxy itself |
| Use it for | Public explorers on open chains | Confidential or permissioned chains |

## The explorer and privacy-proxy

Privacy mode is a partnership between two services:

- **privacy-proxy** owns identity, access control, and redaction. It sits between clients and
  the chain, authenticates users, and filters every response according to their role and
  grants. This is where the privacy rules live.
- **the block explorer** provides the UI and a thin API in front of privacy-proxy. It holds
  no chain data of its own; it forwards each read to privacy-proxy with the user's session
  attached, and renders whatever privacy-proxy returns.

Because privacy-proxy is the source of truth for visibility, you configure roles, grants, and
filtering in **its** documentation, not here. This section explains how the explorer plugs
into it.

:::tip[Set up privacy-proxy first]
Privacy mode does nothing without a running privacy-proxy. Stand one up using its own docs,
then point the explorer at it.

- [Getting started](https://gateway-fm.github.io/privacy-proxy/docs/getting-started/)
- [Architecture](https://gateway-fm.github.io/privacy-proxy/docs/architecture/)
- [Block explorer integration](https://gateway-fm.github.io/privacy-proxy/docs/explorer/)
:::

## In this section

- **[How privacy mode works](../how-it-works/)** — the request and redaction flow end to end.
- **[Authentication & SSO](../authentication/)** — how login works and the `SSO_*` settings.
- **[Wallet access (MetaMask)](../wallet-access/)** — connecting a wallet via the jwt-injector.
- **[View as user](../view-as-user/)** — let an operator browse exactly as a chosen user.
- **[Deploying in privacy mode](../deployment/)** — the fail-closed rules and rollout steps.

For the side-by-side configuration of both modes, see
[Deployment Modes](../../configuration/deployment-modes/).
