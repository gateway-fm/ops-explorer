# OPS V2 wallet links

The selected `ops-v2` privacy build has a **Wallets** page. It uses the existing
opaque app session; the original OPS bearer stays on the server. Linking is a
user action and does not change Git configuration, groups or transaction rights.
The page displays only this OPS identity's active, signature-verified links.

Choose an address from an injected Ethereum wallet, inspect the ownership
message, sign it, then select **Confirm link**. The wallet signs the exact UTF-8
message with `personal_sign`; this does not submit a transaction. The message
binds the address, OPS Actor, nonce, service and deployment namespace. The
service name is an OPS-owned binding and can differ from the browser sign-in
origin. ENS lookup and ENS address input are not offered here.

Changing account, refreshing identity, signing out, suspending the page or
reaching the original expiry discards the operation. A late wallet prompt cannot
submit its signature under the next account. No signature or nonce is saved to
browser storage, URLs or logs.

**Unlink is permanent under the current deployment rules.** A revoked address
cannot be linked to the same OPS identity again through these endpoints. There
is no Undo. A lost or uncertain response does not establish whether the operation
committed: use **Refresh links** to read the current state before starting another
action. The app never automatically repeats a wallet mutation.

The BFF exposes exactly these routes:

| Method | App route | Native OPS route |
| --- | --- | --- |
| GET | `/api/eth/addresses` | `/api/v2/eth/addresses` |
| POST | `/api/eth/link/challenge` | `/api/v2/eth/link/challenge` |
| POST | `/api/eth/link/verify` | `/api/v2/eth/link/verify` |
| DELETE | `/api/eth/addresses/{address}` | `/api/v2/eth/addresses/{address}` |

List and delete have empty bodies and no query. Challenge requires `{address}`;
verification requires `{nonce,address,signature}`. Addresses are canonical
lowercase hex. The native own-list response is an array of
`{address,verifiedAt,proofVersion:2}`; it has no inferred system-link records.
Requests are limited to 8 KiB, responses to 1 MiB and the route deadline to five
seconds or an earlier parent deadline. The optional native ENS route is not
forwarded, nor are V1 wallet aliases.

Each call checks the captured app session before contacting the fixed approved
OPS origin and again before releasing private response bytes. No lock spans the
network: an operation already forwarded for the original Actor may commit after
an app-session change while its response is withheld. It never uses the next
Actor's token. App logout does not itself revoke the signed Core bearer.

Wallet mutations use the existing restricted operational wallet SQL store. Own
lookup completes a bounded ready-state SELECT and checks the live identity before
output; it is not a separate read-COMMIT protocol. The BFF app-capture check and
wallet mutation success each retain their own known-COMMIT requirements.

The SSO callback response-order availability limit described in
[app sessions](ops-v2-app-sessions.md) still applies: concurrent sign-ins can
require a fresh sign-in when a delayed response installs an older invalid handle.
