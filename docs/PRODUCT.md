# PRODUCT.md — Block Explorer Documentation

register: product

## Product purpose

Public documentation for the Gateway Block Explorer: a lightweight, self-hosted explorer
for EVM-compatible chains. The docs teach operators how to run, configure, brand, and
deploy the explorer, and document its public REST API. Hosted on GitHub Pages, built with
Astro Starlight.

## Users

- **Chain operators / DevOps** standing up an explorer for their own EVM chain (L1/L2,
  testnet, appchain). Technical, comfortable with Docker, env vars, and the terminal.
- **Integrators** consuming the public REST API.
- **Evaluators** deciding whether to adopt the explorer.

They read on desktop, often alongside a terminal. They scan for the exact env var, command,
or endpoint they need. Speed-to-answer and trustworthiness matter more than delight.

## Brand

The docs are an extension of the Block Explorer product and must feel like the same
software. The brand is **Gateway**: signature purple (`#8950FA`), Inter for UI/body and
JetBrains Mono for code, soft elevation, generously rounded corners (12–24px), a calm
blue-tinted dark surface and a clean light-slate light surface. The product mark is the
layered Gateway logo (`logo.svg`); the mascot illustration (`mascot.png`) is the friendly
hero image.

## Tone

Precise, calm, operator-to-operator. Short declarative sentences. Lead with the command or
value. No marketing fluff, no hand-holding beyond what's useful. Warnings are explicit and
unmissable (especially the fail-closed privacy-mode rules).

## Anti-references

- Generic SaaS docs with a stock blue accent and a default syntax theme.
- Loud, skeuomorphic code "terminals" with traffic-light chrome that distract from content.
- Any palette that drifts off-brand (no teal, no indigo, no default link blue).

## Strategic principles

1. **Match the app.** Same fonts, same purple, same radii, same surfaces. A reader should
   not feel a seam between the explorer and its docs.
2. **Answer fast.** Tables, copyable commands, and clear headings over prose.
3. **The brand is the system, not decoration.** Inherit `frontend/src/index.css` and
   `frontend/tailwind.config.js` rather than re-inventing tokens.
