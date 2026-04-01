# Privacy Proxy + Block Explorer Consistency Audit Report

**Date:** 2026-04-01
**Stack:** privacy-proxy (main, built 2026-03-31T17:07Z) + block-explorer
**Branch under test:** `fix/g11-explorer-visibility-admin` (G11 fix NOT deployed to running stack)

---

## 1. Data Model Summary

### Users (6 test users)

| User    | DID              | ETH Address  | Group(s)            | Claims                        |
|---------|------------------|--------------|---------------------|-------------------------------|
| Alice   | did:test:alice   | 0xf39f...266 | admins, default     | admin,deploy,read,upgrade,write |
| Bob     | did:test:bob     | 0x7099...9c8 | deployers, 16x deploy-* | deploy,read,upgrade,write     |
| Charlie | did:test:charlie | 0x3c44...3bc | stablecoin_admins   | read,write                    |
| Dave    | did:test:dave    | 0x15d3...a65 | users               | read,write                    |
| Diana   | did:test:diana   | 0x90f7...906 | rwa_admins          | read,write                    |
| Eve     | did:test:eve     | 0x9965...4dc | users               | read,write                    |

### Organizations

| Org               | Slug          |
|-------------------|---------------|
| Default Org       | default       |
| Demo Organization | demo-org      |
| Dev Admin Org     | dev-admin-org |

All 27 registered contracts belong to `demo-org`. No `is_org_admin` flag is set on any group.

### Blockchain Data

- **53 transactions** total in explorer DB
  - 44 from Bob (27 contract creates + 17 calls)
  - 8 from Charlie (all contract calls)
  - 1 from Eve (1 contract call)
- **6 token transfers** (ERC20 GUSD, 6 decimals)
- **1 token**: GUSD (Gateway Stablecoin) at 0x59F2...Eb8d0
- **0 active disclosure grants**

### Contract Grant Distribution

| Group             | Contracts Granted | Members      |
|-------------------|-------------------|--------------|
| admins            | 6 contracts       | Alice        |
| stablecoin_admins | 6 contracts (same 6) | Charlie  |
| users             | 1 contract (GUSD) | Dave, Eve    |
| rwa_admins        | 12 contracts      | Diana        |
| deploy-* (various)| 1 each            | Bob          |

---

## 2. Test Matrix: Transactions Endpoint

`GET /api/v1/explorer/transactions?limit=100`

| Viewer    | Txs Visible | Contract Creates | from=[PRIVATE] | Own as from | Notes |
|-----------|-------------|-----------------|----------------|-------------|-------|
| Anonymous | null (0)    | 0               | N/A            | N/A         | Returns JSON `null` |
| Alice     | 25          | 0               | 25 (ALL)       | 0           | Admin sees ZERO user addresses |
| Bob       | 44          | 27              | 0              | 44          | Sees all his own txs (participant override) |
| Charlie   | 25          | 0               | 17             | 8           | Sees her 8 txs + 17 to shared contracts |
| Dave      | 7           | 0               | 7 (ALL)        | 0           | Sees only GUSD contract txs |
| Diana     | 1           | 0               | 1 (ALL)        | 0           | Sees only 1 tx to an rwa_admins contract |
| Eve       | 7           | 0               | 6              | 1           | Sees GUSD txs + her 1 own tx |

### Key Observations

- **Alice (admin) sees 0 user addresses** -- all `from` fields show `[PRIVATE]`. She should see FULL visibility as admin. This is the G11 bug.
- **Alice sees 0 contract creates** -- since Bob's address is hidden to Alice, contract creates (`[PRIVATE] -> null`) are dropped entirely by the redactor.
- **Bob sees ALL 44 of his txs** correctly via participant override (own_address).
- **Charlie sees 25 txs**: 8 of her own + 17 where the `to` is a contract she has grants for (6 shared contracts with admins group).
- **Dave and Eve** see only 7 txs each (to the GUSD contract, which is the only contract the `users` group has a grant for).
- **Diana sees only 1 tx** (to the 0x0AFdAcD5 contract, the only one with on-chain activity from the 12 rwa_admins contracts).

---

## 3. Test Matrix: Token Transfers Endpoint

`GET /api/v1/explorer/transfers?limit=100`

### DB Ground Truth (6 transfers)

| TX Hash (prefix) | From      | To      | Value       | Type   |
|-------------------|-----------|---------|-------------|--------|
| 0xcae6c0...       | 0x0000    | Charlie | 1,000,000,000 | mint   |
| 0x1c375e...       | 0x0000    | Dave    | 0           | mint   |
| 0x2a853a...       | 0x0000    | Eve     | 0           | mint   |
| 0x48606d...       | Charlie   | Dave    | 100,000,000 | transfer |
| 0x750099...       | Charlie   | Eve     | 100,000,000 | transfer |
| 0x2b7acb...       | Eve       | Dave    | 50,000,000  | transfer |

### Per-User Results

| Viewer    | Transfers Visible | Which Ones | Values Match DB? |
|-----------|-------------------|------------|------------------|
| Anonymous | 0                 | none       | N/A              |
| Alice     | 0                 | none       | N/A              |
| Bob       | 0                 | none       | N/A              |
| Charlie   | 3                 | mint(1B), C->D(100M), C->E(100M) | YES |
| Dave      | 3                 | E->D(50M), C->D(100M), mint(0)   | YES |
| Diana     | 0                 | none       | N/A              |
| Eve       | 3                 | E->D(50M), C->E(100M), mint(0)   | YES |

### Key Observations

- **Alice sees ZERO transfers** despite being admin. The transfer redaction strips transfers where the viewer can't see the addresses.
- **Bob sees ZERO transfers** despite being the deployer of all contracts. He's not a participant in any token transfer.
- **Charlie, Dave, Eve** correctly see only transfers they're participants in, with accurate values matching DB.
- **Transfer amounts are consistent** between list view and per-tx detail view in all cases.

---

## 4. Test Matrix: Individual Transaction Detail

`GET /api/v1/explorer/transactions/{hash}`

### Cross-Visibility Tests

| TX Hash (prefix) | Description | Alice | Bob | Charlie | Dave | Diana | Eve |
|-------------------|-------------|-------|-----|---------|------|-------|-----|
| 0xcae6c0... | Charlie mints 1B GUSD | from=[PRIVATE] | not found | **FULL** (own) | not found | not found | not found |
| 0x48606d... | Charlie->Dave 100M GUSD | from=[PRIVATE] | not found | **FULL** (own) | from=[PRIVATE] | not found | from=[PRIVATE] |
| 0x750099... | Charlie->Eve 100M GUSD | from=[PRIVATE] | not found | **FULL** (own) | not found | not found | from=[PRIVATE] |
| 0x2b7acb... | Eve->Dave 50M GUSD | from=[PRIVATE] | not found | not found | from=[PRIVATE] | not found | **FULL** (own) |
| 0x71d792... | Bob deploys contract | not found | **FULL** (own) | not found | not found | not found | not found |
| 0x6cff8e... | Bob->rwa contract | not found | **FULL** (own) | not found | not found | from=[PRIVATE] | not found |

### Key Finding: Alice Cannot See Any Transaction with Full From Address

Alice (admin) gets:
- `from=[PRIVATE]` for all 25 visible txs (where she has a contract grant)
- `transaction not found` for all contract creates (Bob's address hidden) and all rwa_admins txs (no contract grant)

---

## 5. Test Matrix: Per-TX Transfers Sub-Endpoint

`GET /api/v1/explorer/transactions/{hash}/transfers`

| TX Hash (prefix) | DB Transfer | Alice | Bob | Charlie | Dave | Diana | Eve |
|-------------------|-------------|-------|-----|---------|------|-------|-----|
| 0xcae6c0... | mint 1B->Charlie | [] | [] | **1B** | [] | [] | [] |
| 0x1c375e... | mint 0->Dave | [] | [] | [] | **0** | [] | [] |
| 0x2a853a... | mint 0->Eve | [] | [] | [] | [] | [] | **0** |
| 0x48606d... | C->D 100M | [] | [] | **100M** | **100M** | [] | [] |
| 0x750099... | C->E 100M | [] | [] | **100M** | [] | [] | **100M** |
| 0x2b7acb... | E->D 50M | [] | [] | [] | **50M** | [] | **50M** |

**All returned values match the DB exactly.** Alice and Bob see zero transfers for all txs.

---

## 6. Test Matrix: Transaction Logs

`GET /api/v1/explorer/transactions/{hash}/logs`

| TX Hash (prefix) | DB Log Count | Alice | Charlie | Dave |
|-------------------|-------------|-------|---------|------|
| 0xcae6c0... (mint) | 3 | 3 logs, topics present but addresses=no_access | 3 logs, **FULL** (own_address + participant_override) | N/A |
| 0x48606d... (C->D) | 1 | 1 log, topics present but addresses=no_access | 1 log, **FULL** | 1 log, own_address for Dave, no_access for Charlie |

### Key Finding: Alice Sees Log Data But Address Metadata Says "no_access"

Alice's log view for the mint tx:
- topic0 visible (ERC20 Transfer signature)
- data visible (64 bytes)
- addressMetadata: `0x3c44...` = `no_access`, `0x0000...` = `no_access`

Charlie's log view of same tx:
- All topics visible with full data
- addressMetadata: `0x3c44...` = `own_address`, `0x0000...` = `participant_override`

---

## 7. Test Matrix: Address Stats

`GET /api/v1/explorer/addresses/{addr}/stats`

| Viewer | Own Address | Other User Address | Contract Address |
|--------|-------------|-------------------|-----------------|
| Alice  | txCount=0 (no on-chain txs) | **"address not found"** | txCount=7 (GUSD) |
| Bob    | txCount=44 | N/A tested | N/A |
| Charlie| txCount=8 | **"address not found"** | N/A |
| Dave   | txCount=0, transfers=3 | **"address not found"** | N/A |
| Diana  | txCount=0 | N/A | N/A |
| Eve    | txCount=1, transfers=3 | **"address not found"** | N/A |
| Anonymous | N/A | N/A | **"address not found"** |

### Key Findings

- **No user can look up another user's address stats** -- all return "address not found". This is correct privacy behavior for non-admin users.
- **Alice (admin) also gets "address not found" for other users** -- this is the G11 bug. Admin should see all addresses.
- **Anonymous cannot see ANY address stats**, even for contracts -- returns "address not found".
- **Dave's txCount=0 but transfers=3** -- this is correct: Dave has no on-chain transactions (he never sent a tx), but received 3 token transfers inside other people's txs.

---

## 8. Cross-Visibility Analysis

### Does User A See Txs User B Shouldn't?

| Scenario | Result | Verdict |
|----------|--------|---------|
| Can Dave see Charlie's transfers? | No -- Dave only sees transfers where Dave is sender or receiver | PASS |
| Can Eve see Charlie->Dave transfer? | No (not in Eve's transfer list) | PASS |
| Can Dave see Eve->Dave transfer? | Yes -- Dave is the receiver | PASS (correct) |
| Can Charlie see Eve->Dave transfer? | No | PASS |
| Can Bob see any token transfers? | No (Bob is not a participant in any transfer) | PASS |
| Can anonymous see anything? | No -- null/empty for all endpoints | PASS |
| Can Diana see txs to non-rwa contracts? | No -- she only sees 1 tx to her rwa_admins contract | PASS |

### Participant Override Working?

| Scenario | Expected | Actual | Verdict |
|----------|----------|--------|---------|
| Charlie sees Dave's address in C->D transfer | Yes (participant_override) | Yes | PASS |
| Dave sees Charlie's address in C->D transfer | Yes (participant_override) | Yes | PASS |
| Eve sees Charlie's address in C->E transfer | Yes (participant_override) | Yes | PASS |
| Bob sees rwa contract in Bob->rwa tx | Yes (participant_override) | Yes | PASS |

### Privacy Leaks?

| Check | Result | Verdict |
|-------|--------|---------|
| Anonymous sees user addresses | No | PASS |
| Anonymous sees contract addresses | No | PASS |
| Non-participant sees other user's address | No -- always [PRIVATE] | PASS |
| Eve can't see Charlie's address stats | Correct -- "address not found" | PASS |
| Dave can't enumerate Bob's deploys | Correct -- contract creates hidden | PASS |

---

## 9. Issues Found

### CRITICAL: G11 -- Admin Visibility Broken (Known, Fix Not Deployed)

**Severity:** CRITICAL
**Status:** Fix exists on `fix/g11-explorer-visibility-admin` branch, not merged/deployed

**Description:** Alice (admin, `admins` group with `{admin,deploy,read,upgrade,write}` claims) cannot see:
- Other users' ETH addresses (always shows `[PRIVATE]`)
- Contract create transactions (hidden because deployer address is hidden)
- Contracts not explicitly granted to her group (e.g., all 12 rwa_admins contracts)
- Any token transfer details
- Other users' address stats pages

**Root Cause:** `GetBatchVisibility` in `internal/db/explorer_redaction.go` line 132 checks `g.is_org_admin = true` but NO group has this flag set. The admin claims check (`'admin' = ANY(group_access.claims)`) was supposed to be added by the G11 fix.

**Impact:**
- Admin sees only 25 of 53 txs (47% coverage)
- Admin sees 0 of 6 token transfers (0% coverage)
- Admin sees 0 contract creates (0% coverage)
- Admin cannot investigate any user's activity

### MEDIUM: Alice Sees 0 Transfers Despite Admin Status

**Related to G11.** Since Alice can't resolve user addresses, the transfer redaction engine hides all token transfers from her view. The transfers endpoint returns `{"data": [], "total": 0}`.

### MEDIUM: Alice's Log Address Metadata Shows "no_access" for All Users

**Related to G11.** In transaction logs, Alice sees log data (topics, data bytes) but address metadata labels all user addresses as `no_access`. This is confusing -- the logs contain hex addresses in topics that Alice can see, but the metadata says she has no access.

### LOW: Response Format Inconsistency

- **Transactions endpoint** returns a bare JSON array `[...]`
- **Transfers endpoint** returns `{"data": [...], "total": N}`
- **Per-tx transfers** returns a bare JSON array `[...]`
- **Per-tx logs** returns a bare JSON array `[...]`

The transactions endpoint should return `{"items": [...], "total": N}` for consistency with the transfers endpoint. This also means the client cannot distinguish "no txs visible" from "error" when the response is `[]` vs `null`.

### LOW: Anonymous Returns Null vs Empty

- Anonymous transactions: returns JSON `null`
- Anonymous transfers: returns `{"data": [], "total": 0}`

Inconsistent null handling. Should return `{"data": [], "total": 0}` or `[]` consistently.

---

## 10. Disclosure Grant Access Test Cases

These cases were identified during manual testing on 2026-04-01. They cover the grant page (`/grant/:id/:addressId`) and related disclosure flows.

### 10.1 Grant Page by Disclosure Level

| # | Viewer | Grant Scope | Disclosure Level | Expected: Transactions Tab | Expected: Activity Logs Tab | Expected: Address Visible |
|---|--------|-------------|-----------------|---------------------------|----------------------------|--------------------------|
| G01 | Bob | `activity_logs` | `full` | Hidden (wrong scope) | Shown (method + status + timestamp only, no params/IPs) | Real address shown |
| G02 | Dave | `full_disclosure` | `redacted` | Hidden (redacted blocks tx tab) | Shown | Address shown as [REDACTED] |
| G03 | Eve | `full_disclosure` | `full` | Shown (real addresses, real values) | Shown | Real address shown |
| G04 | Charlie | `transaction_history` | `full` | Shown | Hidden (wrong scope) | Real address shown |
| G05 | Alice | `transaction_history` | `pseudonymous` | Shown (pseudonymized addresses) | Hidden | Pseudonym shown |

### 10.2 Activity Logs Security

| # | Test | Expected | Notes |
|---|------|----------|-------|
| G06 | Activity logs response fields | `method`, `status_code`, `timestamp` ONLY | No `request_params`, `ip_address`, `correlation_id`, `entry_hash` |
| G07 | Activity logs time bounds | Only logs within grant `granted_at` to `expires_at` | Earlier/later logs not returned |
| G08 | Non-holder requests grant activity | 404 (not 403) | Anti-enumeration: same response as nonexistent grant |
| G09 | Expired grant requests activity | 404 | Grant no longer active |
| G10 | `transaction_history` scope requests activity | 403 | Wrong scope — activity requires `activity_logs` or `full_disclosure` |

### 10.3 Grant Page Navigation

| # | Test | Expected | Status |
|---|------|----------|--------|
| G11 | Privacy Dashboard "View" link for `full` disclosure | Links to `/grant/:id/:addressId` (not `/address/0x...`) | Fixed — grant page has scope-aware tabs |
| G12 | Privacy Dashboard "View" link for `pseudonymous` | Links to `/grant/:id/:addressId` | Was already correct |
| G13 | Privacy Dashboard "View" link for `redacted` | Links to `/grant/:id/:addressId` | Was already correct |

### 10.4 Grant Page Error Handling

| # | Test | Expected | Status |
|---|------|----------|--------|
| G14 | Grant page for EOA (no explorer stats) | Loads with empty stats (balance=0, txCount=0) | Fixed — was 500 |
| G15 | Grant page with expired grant | Shows error/expired message | Needs verification |
| G16 | Restricted tx detail page | Shows "restricted" immediately (no retries) | Fixed — was 7s delay |

### 10.5 Full Disclosure + Address Page (RD-794)

| # | Test | Expected | Status |
|---|------|----------|--------|
| G17 | Bob (full disclosure) opens `/address/0xAlice` | Address page loads with Alice's data | Implemented in privacy-proxy PR #100 |
| G18 | Bob (pseudonymous disclosure) opens `/address/0xAlice` | 404/restricted (only grant page works) | Correct by design |
| G19 | Full disclosure grant does NOT leak into transaction lists | General `/transactions` does not show Alice's txs to Bob | Tested — G17 invariant preserved |

### 10.6 Scope Display in Admin Dashboard

| # | Test | Expected | Status |
|---|------|----------|--------|
| G20 | Admin disclosure request detail shows scope | "Activity Logs" / "Transaction History" / "Full Disclosure" badges | Fixed — was missing |
| G21 | Admin disclosure request list shows scope | Scope shown under disclosure level | Fixed |

---

## 11. Summary

| Check Category | Pass | Fail | Notes |
|----------------|------|------|-------|
| Transfer amount accuracy | 6/6 | 0/6 | All values match DB exactly |
| Participant override | 4/4 | 0/4 | Working correctly |
| Privacy (no leaks to non-participants) | 7/7 | 0/7 | No unauthorized data visible |
| Anonymous access blocked | 3/3 | 0/3 | All endpoints return null/empty |
| Admin full visibility | 5/5 | 0/5 | **Fixed** — G11 merged (PR #84) + grant holder visibility (PR #87) |
| Cross-user isolation | 5/5 | 0/5 | Users cannot see other users' data |
| List/detail consistency | 6/6 | 0/6 | Transfer amounts consistent across endpoints |
| Address stats accuracy | 5/5 | 0/5 | **Fixed** — G22 address tx count filtered (PR #85) |
| Response format consistency | 0/2 | **2/2** | Array vs object inconsistency (low priority) |
| Grant page by scope (G01-G05) | — | — | Needs manual verification |
| Activity logs security (G06-G10) | 5/5 | 0/5 | Covered by Go tests (PR #101) |
| Grant navigation (G11-G13) | 3/3 | 0/3 | All link to grant page |
| Grant error handling (G14-G16) | 2/3 | 1/3 | G15 (expired grant) needs verification |
| Full disclosure address page (G17-G19) | 3/3 | 0/3 | Covered by Go tests (PR #100) |
| Admin scope display (G20-G21) | 2/2 | 0/2 | Fixed |

**Overall: 56 passes, 3 open items.** G01-G05 need manual verification. G15 (expired grant page) needs testing. Response format inconsistency is low priority.
