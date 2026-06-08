//go:build dbtest

package db

// Integration tests for block-explorer's own postgres layer, gated behind the
// `dbtest` build tag (NOT a plain env-skip) so they are invisible to the normal
// `go test ./...` run and CANNOT silently green-by-skip there.
//
// Contract under test: the contract_verifications schema
// (migrations/001_contract_verifications.sql) and the round-trip behavior its
// methods promise:
//   - Migrate() applies cleanly from an empty database and is idempotent.
//   - VerifyContract inserts a full verification row; GetContractVerification
//     reads it back with IsVerified=true and the ABI preserved.
//   - VerifyContract ON CONFLICT (address) overwrites the prior row.
//   - SetContractABI inserts an ABI-only row, and on conflict replaces the ABI
//     while leaving the address PK stable.
//   - GetContractVerification returns (nil, nil) for an unknown address.
// Expected values are derived from the schema + method doc comments, not from
// observing runtime output.
//
// IMPORTANT (must-not-skip-silently): when built with -tags dbtest, a missing
// EXPLORER_TEST_DATABASE_URL is a FATAL test failure, not a skip — running the
// db job without a database must go RED. The CI db job (A9) sets the URL against
// a postgres:16 service and additionally asserts a non-zero count of db tests
// ran. Helpers use the `tc` prefix per the coexistence contract.

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// tcDBURL returns the test database URL or fails the test. Under -tags dbtest
// the database is REQUIRED; an unset URL is a hard failure so the suite can
// never green-by-skip.
func tcDBURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("EXPLORER_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("EXPLORER_TEST_DATABASE_URL must be set when running -tags dbtest " +
			"(point it at an ephemeral postgres:16). Refusing to skip — that would " +
			"hide a non-running db integration suite.")
	}
	return url
}

// tcOpenMigrated opens a pool, migrates from clean, and registers cleanup that
// drops the schema so each test starts fresh and leaves nothing behind.
func tcOpenMigrated(t *testing.T) *DB {
	t.Helper()
	database, err := New(tcDBURL(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(database.Close)

	// Start from a clean slate: drop the table if a previous run left it.
	tcDropSchema(t, database)
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate from clean: %v", err)
	}
	t.Cleanup(func() { tcDropSchema(t, database) })
	return database
}

func tcDropSchema(t *testing.T, d *DB) {
	t.Helper()
	ctx := context.Background()
	// Drop both the app table and tern's bookkeeping so Migrate re-runs clean.
	if _, err := d.pool.Exec(ctx, `DROP TABLE IF EXISTS contract_verifications`); err != nil {
		t.Fatalf("drop contract_verifications: %v", err)
	}
	if _, err := d.pool.Exec(ctx, `DROP TABLE IF EXISTS schema_version`); err != nil {
		t.Fatalf("drop schema_version: %v", err)
	}
}

func TestMigrateFromClean(t *testing.T) {
	database := tcOpenMigrated(t)

	// Contract: the contract_verifications table exists after Migrate.
	var exists bool
	err := database.pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'contract_verifications')`,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("query table existence: %v", err)
	}
	if !exists {
		t.Fatalf("contract_verifications table missing after Migrate")
	}

	// Idempotent: running Migrate again must not error.
	if err := database.Migrate(); err != nil {
		t.Fatalf("second Migrate (idempotency): %v", err)
	}
}

func TestVerifyContractRoundTrip(t *testing.T) {
	database := tcOpenMigrated(t)
	ctx := context.Background()

	const addr = "0x407d73d8a49eeb85d32cf465507dd71d507100c1"
	abi := json.RawMessage(`[{"type":"function","name":"transfer"}]`)

	err := database.VerifyContract(ctx, addr, "MyToken", "v0.8.26+commit.8a97fa7a",
		true, "contract MyToken {}", abi, "paris", "MIT", "0x", 200)
	if err != nil {
		t.Fatalf("VerifyContract: %v", err)
	}

	got, err := database.GetContractVerification(ctx, addr)
	if err != nil {
		t.Fatalf("GetContractVerification: %v", err)
	}
	if got == nil {
		t.Fatalf("GetContractVerification returned nil for a verified contract")
	}
	if got.Address != addr {
		t.Errorf("address = %q, want %q", got.Address, addr)
	}
	if tcDerefStr(got.ContractName) != "MyToken" {
		t.Errorf("contract_name = %q, want MyToken", tcDerefStr(got.ContractName))
	}
	if tcDerefStr(got.CompilerVersion) != "v0.8.26+commit.8a97fa7a" {
		t.Errorf("compiler_version = %q, want v0.8.26+commit.8a97fa7a", tcDerefStr(got.CompilerVersion))
	}
	if !tcDerefBool(got.OptimizationUsed) {
		t.Errorf("optimization_used = false, want true")
	}
	if tcDerefInt(got.OptimizationRuns) != 200 {
		t.Errorf("optimization_runs = %d, want 200", tcDerefInt(got.OptimizationRuns))
	}
	if !got.IsVerified {
		t.Errorf("IsVerified = false, want true (GetContractVerification sets it)")
	}
	// The abi column is JSONB; postgres normalizes key order/whitespace, so the
	// contract is that the JSON VALUE round-trips, not the byte representation.
	tcAssertJSONEqual(t, got.ABI, abi)
}

func TestVerifyContractOverwrite(t *testing.T) {
	// Contract: ON CONFLICT (address) DO UPDATE — a second VerifyContract for
	// the same address replaces the row rather than erroring or duplicating.
	database := tcOpenMigrated(t)
	ctx := context.Background()

	const addr = "0xabc0000000000000000000000000000000000001"
	if err := database.VerifyContract(ctx, addr, "First", "v1", false, "src1",
		json.RawMessage(`{"a":1}`), "paris", "MIT", "", 0); err != nil {
		t.Fatalf("first VerifyContract: %v", err)
	}
	if err := database.VerifyContract(ctx, addr, "Second", "v2", true, "src2",
		json.RawMessage(`{"b":2}`), "shanghai", "GPL", "0x01", 999); err != nil {
		t.Fatalf("second VerifyContract: %v", err)
	}

	got, err := database.GetContractVerification(ctx, addr)
	if err != nil || got == nil {
		t.Fatalf("GetContractVerification: got=%v err=%v", got, err)
	}
	if tcDerefStr(got.ContractName) != "Second" || tcDerefStr(got.CompilerVersion) != "v2" || tcDerefInt(got.OptimizationRuns) != 999 {
		t.Errorf("overwrite not applied: name=%q version=%q runs=%d, want Second/v2/999",
			tcDerefStr(got.ContractName), tcDerefStr(got.CompilerVersion), tcDerefInt(got.OptimizationRuns))
	}

	// Exactly one row for this address (overwrite, not duplicate).
	var count int
	if err := database.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contract_verifications WHERE address = $1`, addr).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count for %s = %d, want exactly 1 (ON CONFLICT overwrite)", addr, count)
	}
}

func TestSetContractABIRoundTripAndOverwrite(t *testing.T) {
	database := tcOpenMigrated(t)
	ctx := context.Background()

	const addr = "0xdef0000000000000000000000000000000000002"
	abi1 := json.RawMessage(`[{"type":"event","name":"Transfer"}]`)
	if err := database.SetContractABI(ctx, addr, abi1); err != nil {
		t.Fatalf("SetContractABI (insert): %v", err)
	}

	got, err := database.GetContractVerification(ctx, addr)
	if err != nil || got == nil {
		t.Fatalf("GetContractVerification after SetContractABI: got=%v err=%v", got, err)
	}
	tcAssertJSONEqual(t, got.ABI, abi1)

	// Overwrite the ABI; the address PK stays stable and the ABI is replaced.
	abi2 := json.RawMessage(`[{"type":"function","name":"approve"}]`)
	if err := database.SetContractABI(ctx, addr, abi2); err != nil {
		t.Fatalf("SetContractABI (overwrite): %v", err)
	}
	got, err = database.GetContractVerification(ctx, addr)
	if err != nil || got == nil {
		t.Fatalf("GetContractVerification after overwrite: got=%v err=%v", got, err)
	}
	tcAssertJSONEqual(t, got.ABI, abi2)
}

func TestGetContractVerificationNotFound(t *testing.T) {
	// Contract: an unknown address returns (nil, nil), not an error.
	database := tcOpenMigrated(t)
	got, err := database.GetContractVerification(context.Background(),
		"0x0000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("GetContractVerification(unknown) error = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("GetContractVerification(unknown) = %+v, want nil", got)
	}
}

// TestVerifiedAtIsSet sanity-checks the verified_at default/contract: a freshly
// verified row has a non-zero, recent timestamp.
func TestVerifiedAtIsSet(t *testing.T) {
	database := tcOpenMigrated(t)
	ctx := context.Background()
	const addr = "0x1110000000000000000000000000000000000003"
	before := time.Now().Add(-time.Minute)
	if err := database.VerifyContract(ctx, addr, "T", "v", false, "", json.RawMessage(`{}`), "", "", "", 0); err != nil {
		t.Fatalf("VerifyContract: %v", err)
	}
	got, err := database.GetContractVerification(ctx, addr)
	if err != nil || got == nil {
		t.Fatalf("GetContractVerification: %v", err)
	}
	if got.CreatedAt.Before(before) {
		t.Errorf("verified_at = %v, want a recent timestamp (>= %v)", got.CreatedAt, before)
	}
}

// --- pointer deref helpers (types.Contract verification fields are *T) -------

func tcDerefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func tcDerefBool(p *bool) bool {
	return p != nil && *p
}

func tcDerefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// tcAssertJSONEqual compares two JSON documents by VALUE (the abi column is
// JSONB, which normalizes key order and whitespace), not by raw bytes.
func tcAssertJSONEqual(t *testing.T, got, want json.RawMessage) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got ABI not valid JSON: %v (%s)", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("want ABI not valid JSON: %v (%s)", err, want)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if string(gb) != string(wb) {
		t.Errorf("ABI JSON value = %s, want %s", got, want)
	}
}
