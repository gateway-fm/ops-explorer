package db

import (
	"io/fs"
	"strings"
	"testing"
)

func TestSSOOwnerMigrationSeparateFromLegacyStartup(t *testing.T) {
	legacy, e := fs.Sub(migrations, "migrations")
	if e != nil {
		t.Fatal(e)
	}
	files, e := fs.Glob(legacy, "*.sql")
	if e != nil || len(files) == 0 {
		t.Fatal("missing legacy migration")
	}
	for _, file := range files {
		raw, e := fs.ReadFile(legacy, file)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(raw), "explorer_sso_v2") || strings.Contains(string(raw), "explorer_sso_app") {
			t.Fatal("legacy automatic migrations depend on operational SSO roles/schema")
		}
	}
	owner, e := SSOMigrationFiles()
	if e != nil {
		t.Fatal(e)
	}
	files, e = fs.Glob(owner, "*.sql")
	if e != nil || len(files) != 1 {
		t.Fatal("offline SSO migration set")
	}
	raw, e := fs.ReadFile(owner, files[0])
	if e != nil || !strings.Contains(string(raw), "CREATE SCHEMA explorer_sso_v2") || strings.Contains(string(raw), "CREATE ROLE") {
		t.Fatal("SSO migration must use externally provisioned fixed role")
	}
}
