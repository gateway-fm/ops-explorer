---
title: Feature Overview
description: The core capabilities of the block explorer — browsing, search, tokens, contracts, charts, and the public API.
---

The explorer covers the everyday needs of browsing an EVM chain, plus a few extras for
operators. This page is a tour of what's built in.

## Browsing & search

- **Blocks** — paginated block lists with timing, gas usage, and full transaction lists.
- **Transactions** — status, value, gas, event logs, token transfers, and internal calls,
  with a Blockscout-style call-trace tree on the transaction page.
- **Addresses** — balances, transaction history, token holdings, internal transactions, and
  emitted logs.
- **Unified search** — search blocks, transactions, and addresses from one bar, with
  autocomplete suggestions as you type.

## Tokens

- First-class support for **ERC-20**, **ERC-721**, and **ERC-1155** tokens.
- Token detail pages with **holders** and **transfer history**.
- Per-address **token balances** and holdings.

## Contracts

- **Source verification** via Sourcify, with verified source and ABI display.
- **Contract read** surfaces the ABI for verified contracts.
- **UML diagrams** — generate a [sol2uml](https://github.com/naddison36/sol2uml) class
  diagram for a verified contract directly from its page.

## Charts & stats

- **Chain statistics** — total blocks, transactions, and addresses.
- **Transaction history** chart and **gas price** tracking.
- **Sync status** showing how far the indexer has progressed.

:::note
Charts, gas, and price pages can be **gated** per deployment (e.g. hidden in privacy
deployments). See [Deployment Modes](../../configuration/deployment-modes/).
:::

## Public REST API

A separate, rate-limited, read-only API exposes blocks, transactions, addresses, tokens,
search, and stats for programmatic access — with an interactive "Try it" docs page at
`/api-docs`. See the [API Reference](../../api/public-api/).

## Whitelabel & multi-network

- **Full whitelabel branding** — name, logo, colours, and footer links from environment
  variables alone. See [Branding](../../configuration/branding/).
- **Network switcher** — let users hop between several explorers you operate. See
  [Network Modes](../../configuration/network-modes/).

## Privacy & access control

When deployed behind privacy-proxy, the explorer supports **RBAC-based redaction**,
**SSO/OAuth login**, and a **"View as user"** impersonation mode for operators. See
[Deployment Modes](../../configuration/deployment-modes/).
