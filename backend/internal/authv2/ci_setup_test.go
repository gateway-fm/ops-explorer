package authv2_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"explorer/internal/authv2"
	"explorer/internal/authv2admin"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// This explicit offline setup only accepts a new, empty local CI database.
// Serving never owns this credential or provisions its own schema/role.
func TestPrepareOwnedV2Database(t *testing.T) {
	if os.Getenv("E2B_TEST_INITIALIZE") != "1" {
		t.Skip("explicit owned CI database initialization only")
	}
	dsn := os.Getenv("E2B_TEST_OWNER_DSN")
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "postgres" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") || !strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "ops_v2_ci_") || u.RawQuery != "sslmode=disable" || u.Fragment != "" {
		t.Fatal("requires an empty local ops_v2_ci_* test database")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil || (config.Host != "localhost" && config.Host != "127.0.0.1") || !strings.HasPrefix(config.Database, "ops_v2_ci_") || len(config.Fallbacks) != 0 || config.TLSConfig != nil {
		t.Fatal("effective connection must select only the owned local fixture")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("owned PostgreSQL unavailable", err)
	}
	defer conn.Close(ctx)
	var tables int
	err = conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%'`).Scan(&tables)
	if err != nil || tables != 0 {
		t.Fatal("fixture database must be empty before offline migration")
	}
	if _, err = conn.Exec(ctx, `CREATE ROLE explorer_sso_app LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD 'owned-app-fixture'`); err != nil {
		t.Fatal("requires an isolated fixture cluster with no preexisting app role", err)
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	spec := authv2.SourceSpec{APIVersion: "ops.explorer-sso/v2", Kind: "ExplorerSSO", AppOrigin: "https://app.test", OPSOrigin: "https://ops.test", CallbackURI: "https://app.test/api/auth/callback", ClientID: "explorer", ClientSecret: authv2.SecretRef{FromEnv: "TEST_CLIENT"}, Issuer: "ops", Audience: "core", Keys: []authv2.PublicKey{{ID: "access", X: base64.RawURLEncoding.EncodeToString(public)}}, Profiles: []authv2.Profile{{ID: "azure", Digest: "sha256:" + strings.Repeat("a", 64)}}, StateKey: authv2.KeyRef{ID: "state", FromEnv: "TEST_STATE"}, TokenKey: authv2.KeyRef{ID: "token", FromEnv: "TEST_TOKEN"}, RecoveryEpoch: "11111111-1111-4111-8111-111111111111", Limits: authv2.DefaultLimits(), Network: authv2.Network{ChainID: "100", GenesisHash: "0x" + strings.Repeat("1", 64)}, Retention: authv2.Retention{BatchSize: 100, IntervalMillis: 1000}}
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	source, err := authv2.ParseSource(raw, sha256.Sum256(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err = authv2admin.Run(ctx, dsn, source, "migrate", false); err != nil {
		t.Fatal("actual offline SSO migration", err)
	}
}
