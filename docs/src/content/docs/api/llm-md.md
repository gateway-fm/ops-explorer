---
title: Markdown pages for AI agents
description: Every explorer page is also served as Markdown at its URL followed by /llm.md, with an index at /llms.txt, so AI agents can read the explorer without running JavaScript.
---

The explorer frontend is a JavaScript app, so an agent that fetches a page gets an
empty HTML shell. Every page therefore also has a Markdown version: take the page URL
and append `/llm.md`.

```
https://explorer.example/tx/0x5c50…e1a9            # HTML page
https://explorer.example/tx/0x5c50…e1a9/llm.md     # same page as Markdown
```

The index of pages and URL patterns is at `/llms.txt` (the [llms.txt](https://llmstxt.org/)
convention). A request for an HTML page with `Accept: text/markdown` gets the Markdown
version too.

## Pages

| Page | Markdown URL | Contents |
|------|--------------|----------|
| Home | `/llm.md` | Chain ID, totals, latest blocks and transactions |
| Blocks | `/blocks/llm.md` | Latest blocks; older ones with `?before=<block number>` |
| Block | `/block/<number or hash>/llm.md` | Header fields and the block's transactions |
| Transactions | `/transactions/llm.md` | Latest transactions; more with `?page=<n>` |
| Transaction | `/tx/<hash>/llm.md` | Status, value, fee, gas, token transfers, internal transactions, logs, input |
| Address | `/address/<address>/llm.md` | Balance, counts, token holdings, recent transactions; older ones with the page's `?cursor=` link |
| Contract | `/address/<address>/llm.md?tab=contract` | Verification details, ABI, source code, bytecode |
| Tokens | `/tokens/llm.md` | Tokens; more with `?page=<n>` |
| Token | `/token/<contract address>/llm.md` | Supply, holders, latest transfers |
| Statistics | `/stats/llm.md` | Totals, gas prices, daily activity for the last 30 days |

Links between pages point at their `/llm.md` versions, so an agent can follow them.
Unknown pages return `404` with a short Markdown body. Malformed hashes or addresses
return `400`.

The pages carry the same data as the HTML pages. They need no key and count against
the [public API's rate limit](../public-api/#rate-limiting).

Token names, symbols, revert reasons and contract source come from the chain and from
users. The pages escape them as Markdown text, but an agent should still treat them as
data, not instructions.

## How it is served

The [public API](../public-api/) service renders the pages. The edge forwards any path
ending in `/llm.md`, and `/llms.txt`, to it unchanged:

- **Docker Compose:** the frontend image's nginx does this, to the `public-api` service.
  It also rewrites requests that send `Accept: text/markdown` to the page's `/llm.md` URL.
- **Kubernetes / other ingress:** route those paths to the public API before the
  catch-all route to the frontend. With Istio, for example:

  ```yaml
  - match:
      - uri:
          regex: ^(/.*)?/llm\.md$
      - uri:
          exact: /llms.txt
    route:
      - destination:
          host: public-api
          port:
            number: 8082
  ```

Set `EXPLORER_NAME`, `EXPLORER_CURRENCY_SYMBOL` and, if links should not use the
request's host, `EXPLORER_PUBLIC_URL` on the public API (see
[Configuration](../public-api/#configuration)).
