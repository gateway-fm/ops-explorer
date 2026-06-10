# Network Modes: Single vs. Multi-Network

The block explorer can run in one of two modes:

| Mode | Network switcher in the navbar? | When to use |
| --- | --- | --- |
| **Single-network** (default) | **Hidden** | You are running an explorer for **one** chain. This is almost everyone. |
| **Multi-network** | **Shown** — a dropdown that links to your other explorers | You run **several** explorers (e.g. a mainnet and a testnet) and want users to hop between them. |

> **The switcher is OFF by default.** It only appears when you explicitly turn it
> on *and* give it the list of networks. A single-network deployment will never
> show it — so it can never accidentally show stale "localhost" links in
> production.

---

## How it works (the two settings)

Multi-network mode is controlled by exactly two things:

1. **`VITE_MULTI_NETWORK_ENABLED`** — an environment variable.
   - `false` (or unset) → single-network mode, switcher hidden. **This is the default.**
   - `true` → multi-network mode, switcher shown (provided step 2 lists ≥ 2 networks).

2. **`featured-networks.json`** — a small JSON file listing the networks to show
   in the dropdown. Served by the explorer at `/featured-networks.json`.
   - In single-network mode this file is **ignored** (and ships empty: `[]`).
   - In multi-network mode it must list **2 or more** networks.

Both must be set for the switcher to appear. If the flag is `true` but the file
has fewer than 2 networks, the switcher still stays hidden (there is nothing to
switch to).

### `featured-networks.json` format

A JSON array. Each entry describes one explorer:

```json
[
  {
    "title": "Ethereum Mainnet",
    "url": "https://explorer.example.com",
    "group": "Production",
    "icon": "https://example.com/eth.svg"
  },
  {
    "title": "Sepolia Testnet",
    "url": "https://sepolia-explorer.example.com",
    "group": "Testnets"
  }
]
```

| Field | Required | Meaning |
| --- | --- | --- |
| `title` | ✅ | Name shown in the dropdown (e.g. "Ethereum Mainnet"). |
| `url`   | ✅ | Full URL of that network's explorer. The entry whose `url` matches the page you're on is marked as the active one. |
| `group` | optional | A heading the entry is listed under (e.g. "Production", "Testnets"). Entries with the same `group` are shown together. |
| `icon`  | optional | URL of a small logo shown next to the title. Falls back to a generic globe. |

---

## Single-network mode (the default)

**You do not need to do anything.** Leave `VITE_MULTI_NETWORK_ENABLED` unset (or
`false`) and the switcher will not appear. The shipped `featured-networks.json`
is empty (`[]`), so there is nothing to clean up.

Local dev example:

```bash
make dev
```

→ explorer at <http://localhost:3001>, **no** network switcher.

---

## Multi-network mode

You need to do two things: **(1)** turn the flag on, and **(2)** provide the
network list.

### Option A — Local development

This is already wired up for you:

```bash
make dev-multi
```

This starts two chains (Network A on :3001, Network B on :3002) with the switcher
enabled. The network list it uses lives in **`deploy/featured-networks.multi.json`** —
edit that file to rename or add networks. (Behind the scenes, `make dev-multi`
sets `VITE_MULTI_NETWORK_ENABLED=true` and mounts that file over the explorer's
`featured-networks.json`.)

### Option B — Production / Docker

In production the frontend is an nginx container. `VITE_*` environment variables
are injected into the page at container startup, and static files (including
`featured-networks.json`) are served from the web root `/usr/share/nginx/html`.

To enable the switcher:

1. **Set the flag** on the frontend container:

   ```yaml
   # docker-compose.yml (frontend service)
   environment:
     VITE_MULTI_NETWORK_ENABLED: "true"
   ```

2. **Provide the network list.** Either:

   **(a) Mount your file** over the served path (simplest):

   ```yaml
   # docker-compose.yml (frontend service)
   volumes:
     - ./featured-networks.json:/usr/share/nginx/html/featured-networks.json:ro
   ```

   **(b) Or host it elsewhere** and point the explorer at it:

   ```yaml
   environment:
     VITE_MULTI_NETWORK_ENABLED: "true"
     VITE_FEATURED_NETWORKS_URL: "https://cdn.example.com/featured-networks.json"
   ```

3. Restart the frontend container. The dropdown appears once it serves a list of
   ≥ 2 networks.

> **Tip:** Use the **same** `featured-networks.json` (with the full list of all
> your networks) on **every** explorer in the group. Each explorer highlights
> whichever entry matches its own URL as the active one.

---

## Troubleshooting

**The switcher shows "Local Mainnet / Local Testnet" pointing at `localhost` in production.**
Your deployment is serving an old `featured-networks.json` that contains those
entries, and `VITE_MULTI_NETWORK_ENABLED` is `true`. For a single-network deploy,
set `VITE_MULTI_NETWORK_ENABLED=false` (or unset it) — the switcher will disappear
regardless of the file. For a multi-network deploy, replace the file with your
real network URLs.

**I set `VITE_MULTI_NETWORK_ENABLED=true` but the switcher doesn't show.**
Check that `/featured-networks.json` returns an array of **2 or more** valid
entries (each with a `title` and `url`):

```bash
curl https://your-explorer.example.com/featured-networks.json
```

With fewer than 2 networks the switcher stays hidden by design.

**The switcher shows but the active network isn't highlighted.**
The active entry is matched by comparing its `url`'s origin to the page's origin.
Make sure each entry's `url` exactly matches the address users actually visit
(scheme + host + port).

---

## Quick reference

| I want… | Do this |
| --- | --- |
| One explorer, no switcher | Nothing — it's the default. |
| Local two-chain demo | `make dev-multi`; edit `deploy/featured-networks.multi.json`. |
| Switcher in production | Set `VITE_MULTI_NETWORK_ENABLED=true` **and** serve a `featured-networks.json` with ≥ 2 networks. |
| Turn the switcher off | Set `VITE_MULTI_NETWORK_ENABLED=false` (or unset it). |
