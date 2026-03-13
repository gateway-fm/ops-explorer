# Security Vulnerability & Coverage Report

## 1. Test Coverage Estimation

### Backend (Go)
The overall test coverage for the backend is **6.7%**. This highlights a severe lack of testing across critical internal components, particularly in the `indexer`, `rpc`, and `api` modules.

### Frontend (Node/React/Vite)
No automated test scripts (e.g., Jest, Vitest) were found mapped in the `package.json`. The estimated automated test coverage for the frontend is therefore **0%**. 

---

## 2. Security Vulnerability Analysis (White Hat Expert Review)

### A. Backend Issues (Identified via `gosec` & Manual Trace)
The backend analysis exposed several high and medium severity issues:

#### **High Severity:**
1. **SSRF (Server-Side Request Forgery) [G704]**
   - **Location:** `backend/internal/api/handlers.go` lines `1123`, `1450`
   - **Details:** User-supplied inputs (`address`, `chainIDStr`) are directly concatenated into a URL for upstream HTTP requests (e.g., to the Sourcify API). An attacker could manipulate these inputs to force the backend server to make unauthorized requests to internal infrastructure or external sensitive locations.
2. **Integer Overflows (`uint64` -> `int64` / `int`) [G115]**
   - **Location:** Pervasive across `backend/internal/rpc/rpc.go`, `backend/internal/indexer/indexer.go`, and `realtime.go`
   - **Details:** Direct casting of potentially large unsigned numbers (like block numbers, gas limits, and statuses) to smaller bounded or signed integers. An exceptionally large block number or gas limit value returned from a malicious RPC could cause wrap-around/overflow, leading to an index out of bounds or persistent DoS in the indexer routines.
3. **Goroutine Leaks / Context Misuse [G118]**
   - **Location:** `backend/internal/indexer/catchup.go` and `backend/internal/api/server.go` 
   - **Details:** Spawning goroutines without tying their lifecycles to request-scoped contexts can lead to resource exhaustion attacks (Slowloris/DoS). 

#### **Medium/Low Severity:**
1. **Subprocess Arbitrary Code Execution / Command Injection Risk [G204]**
   - **Location:** `backend/internal/verifier/compiler.go`
   - **Details:** The backend executes local compiler binaries (`exec.CommandContext`). While currently supplied through static configs, any user influence over the `c.path` parameter would lead to full Remote Code Execution (RCE).
2. **Weak Directory Permissions [G306/G301]**
   - **Location:** `backend/internal/verifier/solc_manager.go`
   - **Details:** Repos directories are being written with `0755`. This could allow local privilege escalation or tampering of the cached Solc compilers if multiple services share the filesystem.
3. **Potential HTTP Request made with variable URL [G107]**
   - **Location:** Multiple points making unvalidated calls on the network boundary.

---

### B. Frontend Vulnerabilities (Identified via `npm audit`)

The frontend contains **3 known vulnerabilities** associated with its dependencies:

1. **High Severity: Arbitrary File Write via Path Traversal (`rollup`)**
   - **Details:** Rollup versions `<4.59.0` suffer from a vulnerability where maliciously crafted inputs can result in a file write outside the designated compilation directory.
   - **Remediation:** Update `rollup` to `4.59.0` or higher.
   
2. **High Severity: ReDoS - Regular Expression Denial of Service (`minimatch`)**
   - **Details:** Extglobs and nested wildcard patterns can trigger catastrophic backtracking timeouts. An attacker supplying a crafted file path query/glob could DoS the frontend compiling or path routing routines.
   - **Remediation:** Update `minimatch` dependency.

3. **Moderate Severity: ReDoS (`ajv`)**
   - **Details:** Another ReDoS issue via the `$data` option in the JSON schema validator.

---

## 3. Recommended Immediate Actions

1. **Patch SSRF:** Validate and strictly sanitize network inputs before injecting them into `$sourcifyAPIBase` fetch calls. Implement an allowlist of permitted hosts.
2. **Fix Overflows:** Ensure safe integer bounds checking before casting `uint64` values from blockchain RPC endpoints into `int/int64`.
3. **Frontend Remediation:** Run `npm audit fix` and upgrade `vite` / `rollup` to clear the high-severity path traversal and ReDoS findings.
4. **Boost Coverage:** Begin implementing table-driven tests for core handlers and indexer logic to secure confidence above a 60-70% threshold.

