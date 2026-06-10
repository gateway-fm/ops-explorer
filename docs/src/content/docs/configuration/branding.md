---
title: Branding & Whitelabel
description: Rebrand the entire explorer — name, logo, colours, and footer links — from environment variables alone, with no code changes.
---

The block explorer supports full whitelabel branding via environment variables. No code
changes are required — set the variables at build time and the entire UI updates
accordingly.

All branding is centralised in `frontend/src/lib/branding.ts`. Each variable has a sensible
Gateway default, so the explorer works out of the box without any configuration.

## Environment variables

### Identity

| Variable | Description | Default |
|---|---|---|
| `VITE_BRAND_NAME` | Explorer name shown in header, footer, and page title | `Gateway Explorer` |
| `VITE_BRAND_COMPANY` | Company name shown in footer copyright | `Gateway.fm AS` |
| `VITE_BRAND_DESCRIPTION` | HTML meta description for SEO | `Block Explorer - Browse blocks, transactions, and accounts on the blockchain` |

### Assets

| Variable | Description | Default |
|---|---|---|
| `VITE_BRAND_LOGO` | Logo image — path relative to `/public` or an absolute URL | `/logo.svg` |
| `VITE_BRAND_ICON` | Icon/mascot image — used for hover flip animation and as fallback favicon | `/mascot.png` |
| `VITE_BRAND_FAVICON` | Favicon — if empty, falls back to `VITE_BRAND_ICON` | _(empty — uses icon)_ |

### Colors

| Variable | Description | Default |
|---|---|---|
| `VITE_BRAND_COLOR_PRIMARY` | Primary brand color as a hex value. Applied to buttons, links, accents, and focus rings throughout the UI. A hover variant and light tint are auto-generated. | `#8950FA` |
| `VITE_BRAND_COLOR_HERO_BG` | Custom background for the hero/search bar section on the home page. Accepts any CSS background value (hex, gradient, etc.). If empty, uses the default primary color gradient. | _(empty — uses primary gradient)_ |

### Links

All link variables are optional. Set to an empty string to hide the corresponding footer link.

| Variable | Description | Default |
|---|---|---|
| `VITE_BRAND_WEBSITE` | Company website URL | `https://gateway.fm/` |
| `VITE_BRAND_DOCS` | Documentation URL | `https://docs.gateway.fm/` |
| `VITE_BRAND_LEGAL` | Legal/terms URL — also used for Terms of Service and Privacy Policy footer links | `https://gateway.fm/legal/` |
| `VITE_BRAND_SECURITY` | Security page URL | `https://gateway.fm/security/` |
| `VITE_BRAND_TRUST` | Trust center URL | `https://trust.gateway.fm` |

### Social media

All social variables are optional. Set to an empty string to hide the corresponding icon in the footer.

| Variable | Description | Default |
|---|---|---|
| `VITE_BRAND_TWITTER` | X (Twitter) profile URL | `https://x.com/gateway_eth` |
| `VITE_BRAND_LINKEDIN` | LinkedIn company URL | `https://www.linkedin.com/company/gatewayfm` |
| `VITE_BRAND_DISCORD` | Discord invite URL | `https://discord.gg/grPXnEbAyv` |
| `VITE_BRAND_TELEGRAM` | Telegram channel URL | `https://t.me/gateway_fm` |

## Usage with Docker Compose

The simplest way to apply branding is with a docker-compose override file. Create a file
like `docker-compose.brand.yml` and run:

```bash
docker compose -f docker-compose.dev.yml -f docker-compose.brand.yml up --build -d
```

The override file only needs to add environment variables to the `frontend` service:

```yaml
services:
  frontend:
    environment:
      VITE_BRAND_NAME: "My Chain Explorer"
      VITE_BRAND_COMPANY: "My Company"
      VITE_BRAND_COLOR_PRIMARY: "#FF6B00"
```

:::tip[Ready-made examples]
The `examples/branding/` directory contains complete override files:

| File | Description |
|------|-------------|
| `docker-compose.minimal.yml` | Just rename the explorer (2 variables) |
| `docker-compose.full.yml` | Every branding option customised |
| `docker-compose.no-socials.yml` | Internal deploy — no footer links |
| `docker-compose.l2-chain.yml` | L2 chain — branding + network config |
:::

## Usage with .env files

Create a `.env` file in the `frontend/` directory:

```ini
VITE_BRAND_NAME=My Chain Explorer
VITE_BRAND_COMPANY=My Company Inc.
VITE_BRAND_COLOR_PRIMARY=#FF6B00
VITE_BRAND_WEBSITE=https://mycompany.com
VITE_BRAND_TWITTER=https://x.com/mychain
```

Vite automatically loads `.env` files. See the [Vite docs](https://vite.dev/guide/env-and-mode)
for `.env.production`, `.env.local`, etc.

## Custom logos

The navbar logo has a flip animation — the **logo** is shown by default, and the
**icon/mascot** is revealed on hover. Both images also appear in the footer. The favicon
defaults to the icon if `VITE_BRAND_FAVICON` is not set.

**Local files** — place your images in `frontend/public/` and reference by path:

```ini
VITE_BRAND_LOGO=/my-logo.svg
VITE_BRAND_ICON=/my-mascot.png
```

**Remote URLs** — point to any hosted image:

```ini
VITE_BRAND_LOGO=https://cdn.example.com/logo.svg
VITE_BRAND_ICON=https://cdn.example.com/mascot.png
```

**Where logos appear:**

| Variable | Navbar (front) | Navbar (hover) | Footer | Favicon |
|----------|---------------|----------------|--------|---------|
| `VITE_BRAND_LOGO` | Yes | — | Yes | — |
| `VITE_BRAND_ICON` | — | Yes | — | Yes (fallback) |
| `VITE_BRAND_FAVICON` | — | — | — | Yes |

**Recommended formats:**

- **Logo**: SVG or PNG, displayed at 40×40px in the header and 32×32px in the footer
- **Icon/Mascot**: PNG, used for hover-flip and as fallback favicon
- **Favicon**: ICO, PNG, or SVG

## Hiding sections

To hide footer sections that don't apply to your brand, set the relevant variables to
empty strings:

```yaml
# Hide all social links
VITE_BRAND_TWITTER: ""
VITE_BRAND_LINKEDIN: ""
VITE_BRAND_DISCORD: ""
VITE_BRAND_TELEGRAM: ""

# Hide legal/terms footer links
VITE_BRAND_LEGAL: ""

# Hide company links
VITE_BRAND_WEBSITE: ""
VITE_BRAND_DOCS: ""
VITE_BRAND_SECURITY: ""
VITE_BRAND_TRUST: ""
```

## How it works

1. `frontend/src/lib/branding.ts` reads all `VITE_` environment variables at build time
   with fallback defaults.
2. `frontend/src/main.tsx` applies the page title, favicon, meta description, and CSS color
   variables on app load.
3. `frontend/src/components/Layout.tsx` reads from `branding` for all header/footer content.
4. Tailwind CSS references the `--primary-rgb` CSS variable, so the brand color flows
   through all utility classes and opacity modifiers.
