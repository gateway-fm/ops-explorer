# Block Explorer Requirements & Status

This document outlines the core functional requirements of the Block Explorer and their current implementation status.

## 1. Core Explorer Data
- **REQ-1.1: Block Indexing**
  - Ability to poll RPC nodes for new blocks.
  - Index block number, hash, timestamp, parent hash, and gas details.
  - Status: [x] Implemented
- **REQ-1.2: Transaction Indexing**
  - Index transactions within blocks: from, to, value, gas, hash, and status.
  - Status: [x] Implemented
- **REQ-1.3: Internal Transactions**
  - Use `debug_traceBlock` or similar to index internal calls.
  - Status: [x] Implemented (requires tracing support)
- **REQ-1.4: Event Logs**
  - Index Ethereum logs for Transfers and other critical events.
  - Status: [x] Implemented

## 2. Token & Balance Support
- **REQ-2.1: ERC20/ERC721 Metadata**
  - Fetch and cache token name, symbol, and decimals.
  - Status: [x] Implemented
- **REQ-2.2: Token Balances**
  - Index and track account balances for native and ERC20 tokens.
  - Status: [x] Implemented (including async worker pool)

## 3. Contract Verification
- **REQ-3.1: External Verification (Sourcify)**
  - Integrate with Sourcify for automatic verification.
  - Status: [x] Implemented
- **REQ-3.2: Local Verification (Solc)**
  - Local verification using specific solc versions.
  - Status: [x] Implemented

## 4. Search & UI
- **REQ-4.1: Search Functionality**
  - Search by block, tx hash, or address.
  - Provide search suggestions.
  - Status: [x] Implemented
- **REQ-4.2: Real-time Updates**
  - WebSocket hub for broadcasting new blocks and transactions to the frontend.
  - Status: [x] Implemented

## 5. Security & Privacy
- **REQ-5.1: Open Privacy Suite (privacy proxy)**
  - Access control and address visibility logic (Pseudonymization, Redaction).
  - Status: [x] Implemented
- **REQ-5.2: Auth/SSO**
  - JWT-based authentication and OIDC/SSO integration.
  - Status: [x] Implemented
- **REQ-5.3: Session Token Refresh**
  - Access tokens are intentionally short-lived (5 minutes, set on the Open Privacy Suite proxy). This
    creates a bounded enforcement window: when a user is banned in the admin panel, they
    remain active at most until their current access token expires, at which point the
    refresh attempt is rejected server-side and the session is terminated.
  - The block explorer performs a silent token refresh via `POST /api/v1/refresh` when
    the access token has ≤ 5 minutes remaining. The Open Privacy Suite proxy rotates the refresh token
    on every call (old token is revoked, new token is issued). Both the access and refresh
    tokens are stored as `HttpOnly` cookies.
  - Because rotation is single-use, concurrent refreshes for one session (a page load
    fires several API requests at once, all carrying the same refresh cookie) are collapsed
    into a single `/refresh` call. Exactly one rotation happens and every in-flight request
    receives the same new token pair — otherwise the losing requests would present the
    just-rotated token, be rejected as "revoked", and tear the session down (the cause of
    spurious "logged out after a page refresh" reports).
  - If the refresh token is revoked (banned user, logout from another tab, admin revocation)
    **and the current access token has also expired**, both cookies are cleared and the
    user is redirected to the login screen. A revocation seen while the access token is
    still valid is treated as a lost rotation race (a sibling request already rotated the
    pair) and does not force an immediate logout; that session ends when its access token
    expires — within the same ≤ AccessTokenTTL window, so the enforcement bound is unchanged.
  - **Do not increase AccessTokenTTL without reconsidering ban enforcement.** A longer
    access token TTL widens the window during which a banned user can still act.
  - Status: [x] Implemented

- **REQ-5.4: Participant Visibility**
  - Transaction participants (sender or receiver) must see the counterparty's address
    in their own transactions, even if the counterparty is otherwise private.
  - The visibility override is per-transaction only — the counterparty address remains
    hidden in transactions where the viewer is not a participant.
  - Implemented in the Open Privacy Suite proxy's RedactionEngine (not the block-explorer).
  - Status: [x] Implemented

- **REQ-5.5: Address Visibility Labels**
  - Transaction lists and detail pages show labels next to addresses indicating
    the viewer's relationship: Mine, My Org, Public, Disclosed, Private.
  - "Private" label shown for counterparty addresses visible via participant visibility,
    indicating the address is private but visible in this transaction context.
  - Status: [x] Implemented

- **REQ-5.6: Privacy-Restricted Pages**
  - When an unauthenticated user accesses a private address, transaction, or token page,
    show a clean "Restricted" message with sign-in prompt instead of a raw error.
  - Public addresses must be viewable without authentication.
  - Status: [x] Implemented

## 6. Advanced Chain Support
- **REQ-6.1: OP Stack Support**
  - Handle L1->L2 deposit transactions (Type 0x7E).
  - Status: [x] Implemented

---

## Technical Debt & Issues
- **Coverage**: Core logic has low automated test coverage (~6.7% for backend).
- **Security**: Critical patches applied for SSRF and Integer Overflows.
- **Verification**: Local verification depends on `solc-static-linux` being available.
