# OPS v2 confidential app sessions

The privacy build provides a separate OPS v2 browser entry and confidential BFF
session. Select `EXPLORER_AUTH_PROTOCOL=ops-v2`. The selected startup reads an
approved source file and its exact SHA-256 before opening a listener. It does not
load legacy `.env` configuration, migrate a database, construct a raw RPC client,
start a chain-info poller or websocket hub, or use V1 SSO, refresh or JWKS lookup.
The standalone build rejects this protocol selection.

Core identity remains an original signed OPS token. The browser receives three
Secure, HttpOnly, host-only, SameSite=Lax cookies: a fixed browser binding, an
encrypted five-minute OAuth context, and an opaque app authentication handle.
The token stays encrypted in this app's restricted operational SQL store. There
are no user or membership rows, directory endpoints, browser access tokens,
provider refresh credentials or BFF-issued identity tokens.

The selected protected read families return `503 private_data_unavailable`.
Legacy numeric/offset read handlers and their React pages are not mounted by
this V2 entry. Unavailable reads do not become fabricated zero counts.

## Approved source and deployment

The source JSON has `apiVersion: "ops.explorer-sso/v2"`, `kind: "ExplorerSSO"`
and the following fields. Unknown fields, duplicate JSON keys and noncanonical
origins are rejected.

| Field | Meaning |
| --- | --- |
| `appOrigin`, `opsOrigin`, `callbackURI` | Canonical HTTPS origins and registered callback exactly `appOrigin + "/api/auth/callback"`. Preserve the configured Host through the reverse proxy. Forwarded-host headers and arbitrary callback prefixes are not supported. |
| `clientId`, `clientSecret.fromEnv` | Registered confidential OPS client and its dedicated 32–256 byte printable secret reference. |
| `issuer`, `audience`, `keys: [{id,x}]` | Exact original-token issuer/audience and 1–8 approved Ed25519 public keys; `x` is canonical base64url of the 32 public bytes. There is no network key discovery. |
| `profiles: [{id,digest}]` | 1–64 exact approved source-profile IDs and their `sha256:` identity-profile digests. |
| `stateKey`, `tokenKey` | Distinct `{id,fromEnv}` references to distinct nonzero 32-byte AES keys, supplied as canonical unpadded base64url environment values. Neither key may reuse the client secret or any supplied access public key. Never share these keys with another security role. |
| `recoveryEpoch` | Nonzero canonical UUID chosen for the drained deployment. |
| `limits` | Lower-only charged limits: at most 200,000 contexts, 1 GiB ciphertext and 512 MiB metadata charged at 2,048 bytes/context. These are logical charges, not physical PostgreSQL/WAL size estimates. |
| `network` | Positive decimal-string uint256 `chainId` and nonzero canonical `genesisHash`. These are approved static setup, not an observation of live node health. |
| `retention` | `{enabled,batchSize,intervalMillis}`; batch 1–1,000, interval 1–60 seconds. Reads never extend retention or token authority. |

Serving configuration uses `EXPLORER_SSO_SOURCE`,
`EXPLORER_SSO_SOURCE_SHA256`, `EXPLORER_SSO_APP_DSN` and optionally `API_PORT`.
Only the three source-selected secret references are resolved. Do not expose any
secret through `VITE_*` frontend configuration. Serving V2 rejects
`EXPLORER_SSO_OWNER_DSN`, legacy `DATABASE_URL` and `INDEXER_URL`.

Provision `explorer_sso_app LOGIN NOINHERIT` separately. It must have no role
memberships, elevated attributes, database/schema CREATE, foreign-object access
or ownership. It has USAGE on `explorer_sso_v2`, SELECT/UPDATE on `store_state`,
and SELECT/INSERT/UPDATE/DELETE on `contexts`. PUBLIC must not provide additional
capabilities. Runtime checks the exact structural catalog and effective role
powers at startup and on every operation, including literal temp-lookalike
schema names. Actual temporary tables remain compatible.

The offline owner command is `go run ./cmd/sso-v2-admin -source FILE -sha256 SHA
-action migrate`, with the owner DSN supplied only to that process through
`EXPLORER_SSO_OWNER_DSN`. Run `-action initialize` once on an empty deployment.
The SSO migration set is separate from the legacy verification migration set.
No new role or SSO table is required by ordinary V1 startup.

For a source change or recovery, drain all app instances first. Run
`-action drained-recover -traffic-drained` using the new approved source, then
start serving instances with its exact hash and restricted credential. Recovery
clears every old context, even if the epoch was reused. It never reconstructs a
token or keeps an old handle alive. An uncertain owner command must be
investigated while traffic remains drained; runtime does not repair state.

Build the browser entry with `vite build --config vite.v2.config.ts` and serve
`index-v2.html` for its document paths. Supply public `/config.js` when runtime
API/base-path configuration is needed. The V2 entry selects the V2 provider even
without a runtime protocol override and makes no external font request. Open it
in a top-level tab; unsupported embedded cookie flows show an explicit top-level
link rather than transporting credentials through an iframe.

## HTTP and renewal

| Route | Behavior |
| --- | --- |
| `POST /api/auth/login` | Closed JSON `{mode,returnPath}`; renewal additionally binds `expectedActorId` and `expectedProfile` and the current `X-OPS-App-Session` when live. Returns only `authorizationUrl` after known commit. |
| `GET /api/auth/callback` | Exact one-use `code`/`state`, encrypted current pending context and S256 verifier; one confidential Basic exchange, with OAuth form encoding exactly once. Original signed token verification precedes conditional activation. |
| `GET /api/auth/status` | `{protocol:"ops-v2",authenticated:false}` or verified own `did`, `source_profile`, `session_id`, `expires_at`. The activation header is optional here; a mismatch returns 409 without another account's identity. |
| `POST /api/auth/logout` | Empty JSON `{}` and exact activation header; conditionally revokes only this app context after known commit. It sends no auth-cookie deletion and does not revoke the OPS family. |

Every authenticated data request carries the captured app activation in
`X-OPS-App-Session`. Source/epoch, the actual encrypted token bytes, cookie hashes,
activation, original times and key ID are rechecked before private headers or
body bytes are released. Pending OAuth changes do not invalidate an unchanged
active capture. Renewal start repeats its original active comparison under the
existing state/context lock, so an old tab cannot install renewal over a newer
live account. Callback consumes its exact original pending revision. Logout also
compares the exact original active capture. All mutations lock state first.

Successful logout leaves the opaque browser handle until its original expiry or
the next callback replaces it. The revoked handle no longer authenticates. This
avoids a delayed logout response deleting a newer sign-in cookie. Durable capture
and pending-revision checks do not control the order in which completed HTTP
responses reach the browser: callback A can commit first, callback B can commit
and set its cookie, then A's delayed response can overwrite that cookie with A's
now-invalid handle. B stays current in durable storage and A fails closed, but
the browser may need to restart sign-in. Newest concurrent browser sign-in
continuity is not guaranteed; this is an availability limit, not permission to
use a stale capture or retry a consumed callback automatically.

Unknown COMMIT outcomes release no handle, authorization URL or callback redirect;
there is no automatic SQL/action retry. A durably consumed but withheld callback
stays consumed. An interrupted login must be started afresh. No provider request
or external token exchange runs while SQL locks are held.

Tokens retain original `iat`, `claims_as_of`, source expiry and signed `exp`.
Signed identity age remains at most 300 seconds, with zero expiry leeway. Core's
existing 30-second positive clock skew means app metadata can cover at most 330
seconds from local activation. Only that metadata/GC ceiling is wider. Pending
OAuth remains at most 300 seconds. Retention is the maximum of actual pending
expiry, actual signed expiry and last change plus 300 seconds; it is not an
authentication lease. Browser binding expiry is fixed at issue time and clamped
at the HTTP year-9999 limit.

The browser clears private children, pending requests, query data and feed cursors
when activation changes, even if the actor and groups are unchanged. A running
tab never silently adopts another activation. After a mismatch it requires
explicit acceptance. Owned renewal is constrained to the original actor/profile.
Suspension clears private state synchronously; a persisted return loads a fresh
document and verifies again.

One canonical, closed local-storage record of at most 256 bytes contains only
opaque activation/revision UUIDs, with a null activation for a known successful
logout. Native same-origin storage notifications reread its actual current value;
queued event payloads are not trusted. Every relevant foreign native event is an
invalidation hint, even if a competing write was overwritten and the current key
already equals this document's own value. Reordered events may conservatively
clear the display again; clearing emits no notice. No locking or atomic
cross-document read/write is assumed. A notice only invalidates the old
document. It never authenticates, supplies an Actor/profile, extends expiry or
adopts an account. Every awaited response/body and subsequent local publication
is fenced by its original document owner and shared revision, including cleanup.
Same-activation status emits no notice. Pagehide, local expiry and failed or
uncertain logout emit none. Malformed, unavailable or quota-failed storage clears
local private state and blocks private use until storage works and the session
is verified again; it is not treated as successful notification. The existing
server capture/commit gates remain authoritative.

The BFF cookie cannot renew Core identity. Renewal visits the real OPS browser
flow. Azure may perform genuine provider refresh and, after the first explicit
handoff, automatically return for the exact remembered client/actor/profile
digest/OPS family. That noncredential UX choice has its original 24-hour expiry,
at most 32 clients, and an explicit Forget action. It grants no authority and
does not slide its expiry. Privado requires a genuine new proof and explicit
holder confirmation; no JWT or cached proof is reconstructed from a cookie.
Login/Continue is an identity/app handoff, never customer disclosure consent.

All selected responses force no-store and safe cache headers. Credentialed CORS
allows only the exact configured app Origin and actual mounted route/method,
without reflection. Mutations require that Origin. Session mismatch, denial and
unavailability release no staged private Location/Set-Cookie/body. The private
writer gates header-only responses, Write, string/copy paths and Flush, denies
upgrade/hijack/raw-writer access, and clamps real socket deadlines. It does not
promise atomic network delivery or revocation of bytes already sent.

Public `/api/chain-info`, `/api/v1/chain-info` and `/api/v2/chain-info` return only
the six approved setup fields: `protocol`, `source`, `chainId`, decimal-string
`chainIdDecimal`, `genesisHash`, `privacyProxyPublicUrl`. Actual encoded setup is
checked against 2 KiB at startup. Missing live fields never become fabricated
zero counts or a "fully synced" claim.
