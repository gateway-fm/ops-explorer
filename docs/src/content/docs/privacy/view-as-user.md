---
title: View as User
description: Let an authorised operator browse the explorer exactly as a specific user sees it — a read-only, cryptographically gated support and audit tool.
---

**View as user** lets an authorised operator see the explorer exactly as a chosen user sees
it, redaction and all. It is built for support and audit: reproducing what a user is looking
at without handing over their credentials.

## How it works

When an operator views as a user, the explorer rewrites its calls to the Open Privacy Suite proxy's
administrative impersonation path, naming the target user and organisation. The Open Privacy Suite proxy
then applies **that user's** visibility rules to every response, so the operator sees the
same redacted view the user would.

The feature is deliberately constrained:

- **Read-only.** Only `GET` requests are allowed; no state-changing calls can be made while
  impersonating.
- **Identity-bound.** The operator's own verified identity is checked against the
  impersonation session on every request, so a leaked token cannot be replayed by someone
  else.
- **Scoped server-side.** The Open Privacy Suite proxy enforces that the operator is allowed to impersonate
  within that organisation; the explorer cannot grant itself access.

## Enabling it

View as user is enabled **only when [`SSO_JWKS_URL`](../authentication/#jwt-verification) is
set**. That is not optional plumbing: the feature binds each impersonation session to the
operator's identity, and that identity is only trustworthy once the session JWT is
cryptographically verified. Without JWKS verification the feature stays disabled by design.

So, to turn it on:

1. Run in [privacy mode](../overview/) (`PRIVACY_PROXY_URL` set).
2. Set `SSO_JWKS_URL` to the Open Privacy Suite proxy's JWKS endpoint (see
   [Authentication](../authentication/)).
3. Configure the operator's impersonation permissions in the Open Privacy Suite proxy.

## The Open Privacy Suite side

The impersonation permissions, organisation scoping, and audit trail live in the Open Privacy Suite proxy.
See its [view as user documentation](https://gateway-fm.github.io/open-privacy-suite/docs/security/view-as-user/).
