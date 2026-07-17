---
title: Authentication & SSO
description: How sign-in works in privacy mode — the OAuth flow through the Open Privacy Suite proxy, session cookies, JWT verification, and the SSO settings.
---

In privacy mode every user signs in through the Open Privacy Suite proxy. The explorer acts as the OAuth
client: it runs the login exchange server-side and keeps the resulting tokens in secure,
short-lived cookies. Standalone mode has no authentication at all.

## The login flow

1. The user clicks sign in; the explorer redirects them to the Open Privacy Suite proxy's OAuth endpoint
   (using `PRIVACY_PROXY_PUBLIC_URL`, the browser-facing address).
2. After authenticating, the Open Privacy Suite proxy redirects back to the explorer's `SSO_REDIRECT_URI`
   with an authorization code, plus a `state` value that the explorer validates (CSRF
   protection).
3. The explorer exchanges the code for tokens **server-side**, calling the Open Privacy Suite proxy's
   `/oauth/token` with `SSO_CLIENT_ID` and `SSO_CLIENT_SECRET`. Tokens are never exposed to
   the browser.
4. The tokens are stored as `Secure`, HTTP-only cookies: a short-lived access cookie and a
   longer-lived refresh cookie. Active sessions are refreshed silently as the access token
   nears expiry.

This is the standard backend-for-frontend pattern: the browser only ever holds an opaque
session cookie, and the explorer attaches the real bearer token to its server-to-server calls
to the Open Privacy Suite proxy.

## JWT injection: forwarding your identity

Signing in is only half the job. On every authenticated request the explorer has to tell
the Open Privacy Suite proxy *who* is asking, so the Open Privacy Suite proxy can apply that user's redaction. It does this
by **injecting the session JWT as a bearer token** on each backend-to-backend call.

:::note[Not the same as the wallet jwt-injector]
This section is about the explorer **API** attaching the user's token to its own server-side
reads. Connecting a *wallet* (MetaMask) in privacy mode uses a separate, locally-run
[jwt-injector helper](../wallet-access/). Same goal, forward the user's identity to
the Open Privacy Suite proxy, but a different place it happens.
:::

What happens on every authenticated request:

1. The browser sends its `explorer_auth` session cookie to the explorer.
2. The explorer's API reads the JWT out of that cookie and carries it through the request
   context.
3. A transport layer wrapping every outbound call to the Open Privacy Suite proxy attaches the token as an
   `Authorization: Bearer <jwt>` header.
4. The Open Privacy Suite proxy reads that header, identifies the user, and redacts the response for them.

A few things worth being precise about:

- **It is not a separate service or sidecar.** The injection is plain HTTP middleware inside
  the explorer API (a request-context value plus a custom round-tripper), so there is no
  extra "injector" container to deploy or fail.
- **The browser never holds a bearer token.** The JWT lives only in a `Secure`, HTTP-only
  cookie; it becomes an `Authorization` header solely on the explorer's trusted server-side
  calls, never in client-side JavaScript.
- **The same token is injected everywhere the explorer talks to the Open Privacy Suite proxy**, including
  while [viewing as another user](../view-as-user/), so redaction is always applied for the
  correct identity.

The injected JWT is the same token the explorer optionally verifies in process (next), but
**the Open Privacy Suite proxy is always the authoritative verifier**: it re-validates the token on every
call before returning data, regardless of any local check.

## JWT verification

When `SSO_JWKS_URL` is set, the explorer fetches the Open Privacy Suite proxy's signing keys and verifies
the session JWT's signature in-process before trusting the identity inside it. The check is
deliberately strict:

- Signature algorithm is restricted to `RS256` / `ES256` (alg-confusion-safe).
- `exp` is mandatory, with a small (30s) clock-skew leeway.
- If `SSO_ISSUER` is set, the `iss` claim must match it.
- If `SSO_AUDIENCE` is set, the `aud` claim must include it.

Verifying the JWT is what lets the explorer trust the caller's identity for local decisions.
It is also **required to enable [View as user](../view-as-user/)**, which depends on a
verified operator identity.

## Settings

| Variable | Required | Role |
|----------|----------|------|
| `PRIVACY_PROXY_PUBLIC_URL` | Yes | Browser-facing Open Privacy Suite proxy URL used for OAuth redirects, e.g. `https://proxy.yourdomain.com`. |
| `SSO_REDIRECT_URI` | Yes | OAuth callback on the explorer, e.g. `https://explorer.yourdomain.com/api/auth/callback`. |
| `SSO_CLIENT_ID` | No (default `explorer`) | OAuth client id. **Must match the client registered in the Open Privacy Suite proxy.** |
| `SSO_CLIENT_SECRET` | Yes (OAuth) | Client secret for the token exchange. **Source from a secrets manager, never inline.** |
| `SSO_JWKS_URL` | Recommended | The Open Privacy Suite proxy JWKS endpoint. Enables in-process JWT verification and [View as user](../view-as-user/). |
| `SSO_ISSUER` | No | If set, the JWT `iss` must equal it (only when `SSO_JWKS_URL` is set). |
| `SSO_AUDIENCE` | No | If set, the JWT `aud` must include it (only when `SSO_JWKS_URL` is set). |

:::caution[Keep the client id in sync]
`SSO_CLIENT_ID` must match the client identifier registered on the Open Privacy Suite proxy side. A
mismatch fails the token exchange and blocks all sign-in.
:::

For the Open Privacy Suite proxy's side of the OAuth/SSO configuration, see its
[authentication docs](https://gateway-fm.github.io/open-privacy-suite/docs/authentication/).
