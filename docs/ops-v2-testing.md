# Native V2 regression checks

The `Native V2 Explorer` workflow builds the privacy API and native browser
entry, runs the frontend tests, and requires six PostgreSQL/TLS session and BFF
controls to execute successfully. Missing or skipped required cases fail the
job. This complements the general backend and frontend test workflows.

For a local run, start an isolated PostgreSQL 16 cluster with an empty database
whose name begins with `ops_v2_ci_`, then supply its offline owner credential:

```sh
export E2B_TEST_OWNER_DSN='postgres://postgres:LOCAL_TEST_PASSWORD@127.0.0.1:5432/ops_v2_ci_session?sslmode=disable'
node scripts/test-native-v2.mjs prepare
node scripts/test-native-v2.mjs
```

Preparation uses the actual offline SSO migration and creates its restricted
application role. It refuses a nonlocal connection, another database name, a
nonempty database or an existing role. Use a fresh dedicated cluster rather
than an existing deployment. The application never receives the owner DSN.

The required checks cover original-token verification and one-use callback,
durable commit/output withholding, session changes, lossless ordinary reads,
disclosure forwarding and wallet actions. Their upstream OPS service is a
controlled TLS protocol fixture. The complete OPS/issuer/browser/chain
walkthrough is a separate cross-repository check; these tests do not claim
hosted Azure, physical wallet-extension or live-chain qualification.

The native browser build is selected explicitly:

```sh
cd frontend
npm ci
npx --no-install vitest run
npx --no-install tsc --noEmit --project tsconfig.app.json
npx --no-install vite build --config vite.v2.config.ts
```
