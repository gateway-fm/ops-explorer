# DESIGN.md — Block Explorer Design System

Derived from the live product: `frontend/src/index.css`, `frontend/tailwind.config.js`,
and `docs/src/content/docs/design/style-guide.md`. The docs site inherits these tokens so
it is visually indistinguishable from the explorer.

## Color

Brand accent is **Gateway purple `#8950FA`** (`rgb(137, 80, 250)`). Neutrals are tinted,
never pure black/white, and flip between themes.

| Role | Light | Dark |
|------|-------|------|
| Accent | `#8950FA` | `#8950FA` |
| Accent (text / hover) | `#6B3DD4` | `#A478FC` |
| Accent tint | `#F5F3FF` | `#2A1A4D` |
| Page background | `#FFFFFF` / `#F1F5F9` | `#0F1117` |
| Surface / panel | `#FFFFFF` | `#1A1D27` |
| Border / hairline | `#E2E8F0` | `#2A2D3A` |
| Text primary | `#0F0F0F` | `#F1F5F9` |
| Text secondary | `#374151` | `#CBD5E1` |
| Text muted | `#6B7280` | `#94A3B8` |
| Code background | `#F5F6F9` | `#1A1D27` |

Status: success `#22C55E`, warning `#EAB308`, error `#EF4444`.

**Strategy:** Restrained. Tinted neutrals carry the surfaces; purple is the single accent
on links, active nav, focus rings, and primary actions. Syntax highlighting is the only
place multiple hues appear, and it stays muted (GitHub themes).

## Typography

- **Sans (UI + body):** `Inter`, system-ui fallback. Loaded from Google Fonts CDN.
- **Mono (code):** `JetBrains Mono`, Menlo/Monaco fallback. Loaded from the same CDN.
- Hierarchy by scale + weight. H1 700, H2 700, H3 600, body 400. Body measure capped
  ~70ch by Starlight's prose width.

## Elevation & shape

- Radii: `12px` (default / inputs / code), `16px` (cards), `24px` (large surfaces). Pills
  (`9999px`) for buttons and badges.
- Shadows: card `0 1px 3px rgba(0,0,0,.05), 0 1px 2px rgba(0,0,0,.1)`; hover lifts to
  `0 4px 6px -1px rgba(0,0,0,.1)`. Primary glow `0 0 20px rgba(137,80,250,.3)`.

## Components

- **Code blocks:** rounded 12px, surface background, hairline border, JetBrains Mono,
  soft shadow, muted frame chrome, purple active-tab indicator. No loud terminal traffic
  lights.
- **Callouts (asides):** rounded, full (not side-stripe) treatment, brand accent for
  note/tip, semantic color for caution/danger.
- **Tables:** uppercase muted header labels, hairline row dividers.
- **Links:** purple, underlined on hover with offset.
- **Logo:** `logo.svg` (layered purple mark) in the nav beside the "Block Explorer"
  wordmark; `mascot.png` as the splash hero.

## Motion

Ease-out, ~200ms. Card hover raises shadow only (no layout animation). No bounce/elastic.
