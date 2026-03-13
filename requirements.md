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
- **REQ-5.1: Privacy Proxy**
  - Access control and address visibility logic (Pseudonymization, Redaction).
  - Status: [x] Implemented
- **REQ-5.2: Auth/SSO**
  - JWT-based authentication and OIDC/SSO integration.
  - Status: [x] Implemented
- **REQ-5.3: Session Token Refresh**
  - Access tokens are intentionally short-lived (5 minutes, set on privacy-proxy). This
    creates a bounded enforcement window: when a user is banned in the admin panel, they
    remain active at most until their current access token expires, at which point the
    refresh attempt is rejected server-side and the session is terminated.
  - The block explorer performs a silent token refresh via `POST /api/v1/refresh` when
    the access token has ≤ 5 minutes remaining. Privacy-proxy rotates the refresh token
    on every call (old token is revoked, new token is issued). Both the access and refresh
    tokens are stored as `HttpOnly` cookies.
  - If the refresh token is revoked (banned user, logout from another tab, admin revocation),
    both cookies are cleared and the user is redirected to the login screen.
  - **Do not increase AccessTokenTTL without reconsidering ban enforcement.** A longer
    access token TTL widens the window during which a banned user can still act.
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
