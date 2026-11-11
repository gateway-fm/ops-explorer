package authv2

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	uuidlib "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func testSource(t *testing.T, app, ops string) (*Source, ed25519.PrivateKey, []SecretBinding) {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	v := SourceSpec{APIVersion: "ops.explorer-sso/v2", Kind: "ExplorerSSO", AppOrigin: app, OPSOrigin: ops, CallbackURI: app + "/api/auth/callback", ClientID: "explorer", ClientSecret: SecretRef{"TEST_CLIENT"}, Issuer: "ops", Audience: "core", Keys: []PublicKey{{"access", base64.RawURLEncoding.EncodeToString(pub)}}, Profiles: []Profile{{"azure", "sha256:" + strings.Repeat("a", 64)}}, StateKey: KeyRef{"state", "TEST_STATE"}, TokenKey: KeyRef{"token", "TEST_TOKEN"}, RecoveryEpoch: uuidlib.NewString(), Limits: DefaultLimits(), Network: Network{"100", "0x" + strings.Repeat("1", 64)}, Retention: Retention{false, 100, 1000}}
	raw, _ := json.Marshal(v)
	s, e := ParseSource(raw, sha256.Sum256(raw))
	if e != nil {
		t.Fatal(e)
	}
	return s, key, []SecretBinding{{"TEST_CLIENT", []byte(strings.Repeat("c", 32))}, {"TEST_STATE", bytes.Repeat([]byte{2}, 32)}, {"TEST_TOKEN", bytes.Repeat([]byte{3}, 32)}}
}
func testClaims(s *Source, now int64) claims {
	return claims{Issuer: s.spec.Issuer, Audience: s.spec.Audience, Subject: "ops:p1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), ID: uuidlib.NewString(), Issued: now, NotBefore: now, Expires: now + 300, Kind: "human", Profile: "azure", ProfileDigest: s.spec.Profiles[0].Digest, Authority: "https://identity.test", SourceSubject: "external-subject", Verified: now, AsOf: now, Facts: []fact{{"group", "finance"}}}
}
func sign(t *testing.T, key ed25519.PrivateKey, c claims) string {
	t.Helper()
	raw, _ := json.Marshal(c)
	var m jwt.MapClaims
	_ = json.Unmarshal(raw, &m)
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, m)
	token.Header = map[string]any{"typ": "ops-access+jwt", "alg": "EdDSA", "kid": "access"}
	out, e := token.SignedString(key)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func fixture(t *testing.T) (*Service, *pgx.Conn, *atomic.Int64, ed25519.PrivateKey) {
	t.Helper()
	dsn := os.Getenv("E2B_TEST_OWNER_DSN")
	if dsn == "" {
		t.Skip("E2B_TEST_OWNER_DSN must name an owned local PostgreSQL fixture")
	}
	ctx := context.Background()
	owner, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal("owned PostgreSQL unavailable")
	}
	t.Cleanup(func() { owner.Close(ctx) })
	s, key, bindings := testSource(t, "https://app.test", "https://ops.test")
	m := s.StorageMetadata()
	_, e = owner.Exec(ctx, `DELETE FROM explorer_sso_v2.contexts; DELETE FROM explorer_sso_v2.store_state`)
	if e != nil {
		t.Fatal(e)
	}
	_, e = owner.Exec(ctx, `INSERT INTO explorer_sso_v2.store_state(id,config_digest,recovery_epoch,ready,max_contexts,max_cipher_bytes,max_metadata_bytes)VALUES(1,$1,$2,true,$3,$4,$5)`, m.ConfigDigest[:], m.RecoveryEpoch, m.Limits.MaxContexts, m.Limits.MaxCipherBytes, m.Limits.MaxMetadataBytes)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.User = url.UserPassword(AppRole, "owned-app-fixture")
	var clock atomic.Int64
	clock.Store(1700000000)
	service, e := New(ctx, s, bindings, cfg.String(), clock.Load)
	if e != nil {
		t.Fatal("restricted service initialization", e)
	}
	t.Cleanup(service.Close)
	return service, owner, &clock, key
}
func install(t *testing.T, s *Service, key ed25519.PrivateKey, now int64) (string, string, *requestSession) {
	t.Helper()
	browser, _ := randomToken()
	state, _ := randomToken()
	p := pendingCapture{handleHash("browser", browser), uuidlib.NewString(), handleHash("oauth-state", state), now, now + 300}
	if e := s.store.start(context.Background(), p); e != nil {
		t.Fatal("start", e)
	}
	auth, _ := randomToken()
	c := testClaims(s.source, now)
	a := activeCapture{browser: p.browser, auth: handleHash("auth", auth), activation: uuidlib.NewString(), activated: now, expires: c.Expires, key: s.source.spec.TokenKey.ID}
	raw := sign(t, key, c)
	var e error
	a.cipher, e = s.codec.sealToken(a, raw)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.store.activate(context.Background(), p, a); e != nil {
		t.Fatal("activate", e)
	}
	session, e := s.session(context.Background(), browser, auth)
	if e != nil {
		t.Fatal("session", e)
	}
	return browser, auth, session
}

func TestOriginalJWTBoundsAndAAD(t *testing.T) {
	s, key, b := testSource(t, "https://app.test", "https://ops.test")
	c, e := newCodec(s, b)
	if e != nil {
		t.Fatal(e)
	}
	now := int64(1700000000)
	for _, test := range []struct {
		name  string
		delta int64
		valid bool
	}{{"current", 0, true}, {"allowed-skew", 30, true}, {"beyond-skew", 31, false}, {"expired", -300, false}} {
		t.Run(test.name, func(t *testing.T) {
			claims := testClaims(s, now+test.delta)
			raw := sign(t, key, claims)
			_, e := s.verify(raw, now)
			if (e == nil) != test.valid {
				t.Fatal("unexpected original JWT decision", e)
			}
			if !test.valid {
				return
			}
			a := activeCapture{browser: sha256.Sum256([]byte("browser")), auth: sha256.Sum256([]byte("auth")), activation: uuidlib.NewString(), activated: now, expires: claims.Expires, key: s.spec.TokenKey.ID}
			a.cipher, e = c.sealToken(a, raw)
			if e != nil {
				t.Fatal(e)
			}
			if got, _, e := c.openToken(a, now); e != nil || got != raw {
				t.Fatal("original token altered or denied")
			}
			for i := range a.cipher {
				changed := a
				changed.cipher = append([]byte(nil), a.cipher...)
				changed.cipher[i] ^= 1
				if _, _, e = c.openToken(changed, now); e == nil {
					t.Fatal("cipher tamper accepted")
				}
			}
			changed := a
			changed.activation = uuidlib.NewString()
			if _, _, e = c.openToken(changed, now); e == nil {
				t.Fatal("activation AAD transplant accepted")
			}
		})
	}
}
func TestPendingPreservesActiveAndCaptureCAS(t *testing.T) {
	s, _, clock, key := fixture(t)
	browser, auth, original := install(t, s, key, clock.Load())
	state, _ := randomToken()
	p := pendingCapture{original.capture.browser, uuidlib.NewString(), handleHash("oauth-state", state), clock.Load(), clock.Load() + 300}
	if e := s.store.start(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	again, e := s.session(context.Background(), browser, auth)
	if e != nil || !sameActive(original.capture, again.capture) {
		t.Fatal("pending update changed immutable active capture", e)
	}
	if e = s.recheck(context.Background(), original); e != nil {
		t.Fatal(e)
	}
	if e = s.store.logout(context.Background(), original.capture); e != nil {
		t.Fatal(e)
	}
	if e = s.recheck(context.Background(), original); e != ErrChanged {
		t.Fatal("logout did not invalidate original capture")
	}
	if e = s.store.activate(context.Background(), p, original.capture); e != ErrChanged {
		t.Fatal("stale pending replay accepted")
	}
}

func TestPrivateFirstByteAllPaths(t *testing.T) {
	s, _, clock, key := fixture(t)
	browser, auth, session := install(t, s, key, clock.Load())
	modes := []string{"write", "write-string", "read-from", "flush", "response-controller", "header-only", "head"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			r := httptest.NewRequest("GET", "https://app.test/api/private", nil)
			if mode == "head" {
				r.Method = "HEAD"
			}
			r.Header.Set("Origin", "https://app.test")
			r.Header.Set(SessionHeader, session.capture.activation)
			r.AddCookie(&http.Cookie{Name: browserCookie, Value: browser})
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: auth})
			w := httptest.NewRecorder()
			handler := s.Protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "/private-location")
				w.Header().Set("Set-Cookie", "private=value")
				w.Header().Set("Cache-Control", "public,max-age=900")
				clock.Store(session.capture.expires)
				switch mode {
				case "write":
					_, _ = w.Write([]byte("PRIVATE"))
				case "write-string":
					_, _ = io.WriteString(w, "PRIVATE")
				case "read-from":
					_, _ = w.(io.ReaderFrom).ReadFrom(strings.NewReader("PRIVATE"))
				case "flush":
					w.(http.Flusher).Flush()
				case "response-controller":
					_ = http.NewResponseController(w).Flush()
				case "header-only", "head":
					w.WriteHeader(204)
				}
			}))
			s.Boundary(handler, func(string, string) bool { return true }).ServeHTTP(w, r)
			if w.Code != 401 || strings.Contains(w.Body.String(), "PRIVATE") || w.Header().Get("Location") != "" || w.Header().Get("Set-Cookie") != "" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("private release via %s status=%d", mode, w.Code)
			}
			clock.Store(session.capture.activated)
		})
	}
}
func TestLiveRoleLookalikeAndTemp(t *testing.T) {
	s, owner, _, _ := fixture(t)
	ctx := context.Background()
	if _, e := owner.Exec(ctx, `CREATE SCHEMA pgxtempyuntrusted; GRANT CREATE ON SCHEMA pgxtempyuntrusted TO explorer_sso_app`); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DROP SCHEMA IF EXISTS pgxtempyuntrusted CASCADE`) })
	if _, e := s.store.read(ctx, [32]byte{}); e == nil {
		t.Fatal("persistent temp-lookalike schema bypassed live role guard")
	}
	if _, e := owner.Exec(ctx, `REVOKE CREATE ON SCHEMA pgxtempyuntrusted FROM explorer_sso_app`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.store.read(ctx, [32]byte{}); e != nil {
		t.Fatal("repaired role denied", e)
	}
	conn, e := s.store.pool.Acquire(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Release()
	if _, e = conn.Exec(ctx, `CREATE TEMP TABLE pg_namespace(dummy text)`); e != nil {
		t.Fatal(e)
	}
	tx, e := conn.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = checkCatalog(ctx, tx); e != nil {
		t.Fatal("legitimate TEMP table should not spoil qualified role/catalog check", e)
	}
}
