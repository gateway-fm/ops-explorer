package authv2

import (
	"context"
	"testing"
)

func TestForeignColumnOnlyGrantAndInheritanceRejected(t *testing.T) {
	s, owner, _, _ := fixture(t)
	ctx := context.Background()
	if _, e := owner.Exec(ctx, `CREATE SCHEMA e2b_foreign; CREATE TABLE e2b_foreign.users(actor text); GRANT USAGE ON SCHEMA e2b_foreign TO explorer_sso_app; GRANT SELECT(actor) ON e2b_foreign.users TO explorer_sso_app`); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DROP SCHEMA IF EXISTS e2b_foreign CASCADE`) })
	var whole, column bool
	if e := owner.QueryRow(ctx, `SELECT has_table_privilege('explorer_sso_app','e2b_foreign.users','SELECT'),has_any_column_privilege('explorer_sso_app','e2b_foreign.users','SELECT')`).Scan(&whole, &column); e != nil || whole || !column {
		t.Fatal("negative control was not a genuine column-only capability")
	}
	if _, e := s.store.read(ctx, [32]byte{}); e == nil {
		t.Fatal("foreign column-only read capability accepted")
	}
	if _, e := owner.Exec(ctx, `REVOKE SELECT(actor) ON e2b_foreign.users FROM explorer_sso_app`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.store.read(ctx, [32]byte{}); e != nil {
		t.Fatal("repaired column capability denied", e)
	}
	if _, e := owner.Exec(ctx, `CREATE TABLE e2b_foreign.child() INHERITS(explorer_sso_v2.contexts)`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.store.read(ctx, [32]byte{}); e == nil {
		t.Fatal("foreign inherited child changed effective scope without rejection")
	}
	if _, e := owner.Exec(ctx, `DROP TABLE e2b_foreign.child`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.store.read(ctx, [32]byte{}); e != nil {
		t.Fatal("repaired inheritance denied", e)
	}
}

func TestOwnStateColumnInsertAndUnloggedRejected(t *testing.T) {
	s, owner, _, _ := fixture(t)
	ctx := context.Background()
	if _, e := owner.Exec(ctx, `GRANT INSERT(id) ON explorer_sso_v2.store_state TO explorer_sso_app`); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, `REVOKE INSERT(id) ON explorer_sso_v2.store_state FROM explorer_sso_app; ALTER TABLE explorer_sso_v2.contexts SET LOGGED`)
	})
	if _, e := s.store.read(ctx, [32]byte{}); e == nil {
		t.Fatal("extra state column INSERT accepted")
	}
	if _, e := owner.Exec(ctx, `REVOKE INSERT(id) ON explorer_sso_v2.store_state FROM explorer_sso_app`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.store.read(ctx, [32]byte{}); e != nil {
		t.Fatal("repaired own column capability denied", e)
	}
	if _, e := owner.Exec(ctx, `ALTER TABLE explorer_sso_v2.contexts SET UNLOGGED`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.store.read(ctx, [32]byte{}); e == nil {
		t.Fatal("unlogged operational token state accepted")
	}
	if _, e := owner.Exec(ctx, `ALTER TABLE explorer_sso_v2.contexts SET LOGGED`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.store.read(ctx, [32]byte{}); e != nil {
		t.Fatal("repaired durable table denied", e)
	}
}
