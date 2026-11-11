//go:build privacy

package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"explorer/internal/authv2"
	"explorer/internal/authv2admin"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type walletAppFixture struct {
	server     *Server
	cookies    []*http.Cookie
	activation string
	token      string
	effects    atomic.Int64
	onWallet   func(http.ResponseWriter, *http.Request)
}

type walletRoundTrip func(*http.Request) (*http.Response, error)

func (f walletRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// This fixture qualifies BFF/session/transport behavior, not native wallet SQL.
// The cross-repository built-browser fixture supplies the actual wallet service.
func newWalletAppFixture(t *testing.T) *walletAppFixture {
	t.Helper()
	dsn := os.Getenv("E2B_TEST_OWNER_DSN")
	if dsn == "" {
		t.Skip("E2B_TEST_OWNER_DSN must name an owned local PostgreSQL fixture")
	}
	f := &walletAppFixture{}
	public, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"iss": "ops", "aud": "core", "sub": "ops:p1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), "jti": uuid.NewString(), "iat": now, "nbf": now, "exp": now + 300, "principal_kind": "human", "source_profile": "azure", "source_profile_digest": "sha256:" + strings.Repeat("a", 64), "identity_authority": "https://identity.test", "source_subject": "external", "source_verified_at": now, "claims_as_of": now, "facts": []map[string]string{{"kind": "group", "value": "finance"}}})
	token.Header = map[string]any{"alg": "EdDSA", "typ": "ops-access+jwt", "kid": "access"}
	f.token, e = token.SignedString(key)
	if e != nil {
		t.Fatal(e)
	}
	ops := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v2/sso/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": f.token, "token_type": "Bearer", "expires_in": 300})
			return
		}
		f.effects.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+f.token || r.Header.Get("Cookie") != "" || r.Header.Get("X-OPS-App-Session") != "" || r.Header.Get("Origin") != "" || r.Header.Get("X-Impersonate-Token") != "" {
			t.Error("unexpected outbound credential/header")
		}
		if f.onWallet != nil {
			f.onWallet(w, r)
			return
		}
		_, _ = w.Write([]byte("[]\n"))
	}))
	t.Cleanup(ops.Close)
	original := http.DefaultTransport
	http.DefaultTransport = ops.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = original })
	spec := authv2.SourceSpec{APIVersion: "ops.explorer-sso/v2", Kind: "ExplorerSSO", AppOrigin: "https://app.test", OPSOrigin: ops.URL, CallbackURI: "https://app.test/api/auth/callback", ClientID: "explorer", ClientSecret: authv2.SecretRef{FromEnv: "TEST_CLIENT"}, Issuer: "ops", Audience: "core", Keys: []authv2.PublicKey{{ID: "access", X: base64.RawURLEncoding.EncodeToString(public)}}, Profiles: []authv2.Profile{{ID: "azure", Digest: "sha256:" + strings.Repeat("a", 64)}}, StateKey: authv2.KeyRef{ID: "state", FromEnv: "TEST_STATE"}, TokenKey: authv2.KeyRef{ID: "token", FromEnv: "TEST_TOKEN"}, RecoveryEpoch: uuid.NewString(), Limits: authv2.DefaultLimits(), Network: authv2.Network{ChainID: "100", GenesisHash: "0x" + strings.Repeat("1", 64)}, Retention: authv2.Retention{Enabled: false, BatchSize: 100, IntervalMillis: 1000}}
	raw, _ := json.Marshal(spec)
	source, e := authv2.ParseSource(raw, sha256.Sum256(raw))
	if e != nil {
		t.Fatal(e)
	}
	if e = authv2admin.Run(t.Context(), dsn, source, "drained-recover", true); e != nil {
		t.Fatal("owned drained fixture initialization", e)
	}
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	u.User = url.UserPassword(authv2.AppRole, "owned-app-fixture")
	service, e := authv2.New(t.Context(), source, []authv2.SecretBinding{{FromEnv: "TEST_CLIENT", Value: bytes.Repeat([]byte{'c'}, 32)}, {FromEnv: "TEST_STATE", Value: bytes.Repeat([]byte{2}, 32)}, {FromEnv: "TEST_TOKEN", Value: bytes.Repeat([]byte{3}, 32)}}, u.String(), func() int64 { return now })
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(service.Close)
	provider := NewV2ReadClient(service)
	base := provider.client.Transport
	provider.client.Transport = walletRoundTrip(func(r *http.Request) (*http.Response, error) {
		if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > 5*time.Second {
			t.Error("wallet transport lacks bounded request deadline")
		}
		return base.RoundTrip(r)
	})
	f.server, e = NewV2(service, provider, 8080)
	if e != nil {
		t.Fatal(e)
	}
	start := f.call("POST", "/api/auth/login", `{"mode":"login","returnPath":"/wallets"}`)
	if start.Code != 200 {
		t.Fatal("actual login", start.Code)
	}
	var destination struct {
		URL string `json:"authorizationUrl"`
	}
	if json.Unmarshal(start.Body.Bytes(), &destination) != nil {
		t.Fatal("start DTO")
	}
	authorize, _ := url.Parse(destination.URL)
	f.cookies = start.Result().Cookies()
	callback := f.call("GET", "/api/auth/callback?"+url.Values{"code": {strings.Repeat("A", 43)}, "state": {authorize.Query().Get("state")}}.Encode(), "")
	if callback.Code != 303 {
		t.Fatal("actual callback", callback.Code)
	}
	for _, next := range callback.Result().Cookies() {
		replaced := false
		for i, old := range f.cookies {
			if old.Name == next.Name {
				f.cookies[i] = next
				replaced = true
			}
		}
		if !replaced {
			f.cookies = append(f.cookies, next)
		}
	}
	status := f.call("GET", "/api/auth/status", "")
	var active struct {
		ID string `json:"session_id"`
	}
	if status.Code != 200 || json.Unmarshal(status.Body.Bytes(), &active) != nil || active.ID == "" {
		t.Fatal("actual current status", status.Code)
	}
	f.activation = active.ID
	return f
}
func (f *walletAppFixture) request(method, path string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, "https://app.test"+path, body)
	if method != "GET" {
		r.Header.Set("Origin", "https://app.test")
		r.Header.Set("Content-Type", "application/json")
	}
	if f.activation != "" {
		r.Header.Set(authv2.SessionHeader, f.activation)
	}
	for _, c := range f.cookies {
		r.AddCookie(c)
	}
	return r
}
func (f *walletAppFixture) call(method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	f.server.Handler().ServeHTTP(w, f.request(method, path, strings.NewReader(body)))
	return w
}

func TestV2WalletFixedWireAndIngress(t *testing.T) {
	f := newWalletAppFixture(t)
	address := "0x" + strings.Repeat("1", 40)
	f.onWallet = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "forbidden=upstream")
		switch r.URL.Path {
		case "/api/v2/eth/addresses":
			_, _ = w.Write([]byte("[]\n"))
		case "/api/v2/eth/link/challenge":
			_, _ = w.Write([]byte(`{"nonce":"fixture","message":"native","expiresAt":"fixture"}`))
		case "/api/v2/eth/link/verify":
			_, _ = w.Write([]byte(`{"address":"` + address + `","status":"linked"}`))
		case "/api/v2/eth/addresses/" + address:
			raw, _ := io.ReadAll(r.Body)
			if len(raw) != 0 {
				t.Error("DELETE acquired a body")
			}
			_, _ = w.Write([]byte(`{"address":"` + address + `","status":"revoked"}`))
		default:
			t.Error("unexpected upstream route")
		}
	}
	for _, v := range [][3]string{{"GET", "/api/eth/addresses", ""}, {"POST", "/api/eth/link/challenge", `{"address":"` + address + `"}`}, {"POST", "/api/eth/link/verify", `{"address":"` + address + `","nonce":"` + strings.Repeat("a", 64) + `","signature":"0x` + strings.Repeat("1", 130) + `"}`}, {"DELETE", "/api/eth/addresses/" + address, ""}} {
		r := f.call(v[0], v[1], v[2])
		if r.Code != 200 || r.Header().Get("Set-Cookie") != "" || r.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("finite native forwarding", v[0], r.Code)
		}
	}
	before := f.effects.Load()
	for _, v := range [][3]string{{"POST", "/api/eth/link/challenge", `{"address":"` + address + `","address":"` + address + `"}`}, {"POST", "/api/eth/link/challenge", `{"address":"` + address + `","user":"other"}`}, {"GET", "/api/eth/addresses?actor=other", ""}, {"GET", "/api/eth/addresses", "{}"}, {"DELETE", "/api/eth/addresses/" + address, "{}"}, {"POST", "/api/eth/link/challenge", strings.Repeat("x", 8193)}, {"POST", "/api/eth/addresses/" + address + "/refresh-ens", "{}"}, {"GET", "/api/v1/eth/addresses", ""}} {
		r := f.call(v[0], v[1], v[2])
		if r.Code < 400 {
			t.Fatal("invalid surface accepted", v[0], r.Code)
		}
	}
	if f.effects.Load() != before {
		t.Fatal("invalid request reached OPS")
	}
	preflight := f.request("OPTIONS", "/api/eth/addresses/"+address, nil)
	preflight.Header.Set("Access-Control-Request-Method", "DELETE")
	preflight.Header.Set("Access-Control-Request-Headers", authv2.SessionHeader)
	w := httptest.NewRecorder()
	f.server.Handler().ServeHTTP(w, preflight)
	if w.Code != 204 {
		t.Fatal("registered DELETE preflight", w.Code)
	}
}

type walletHeldBody struct {
	started, release chan struct{}
	once             sync.Once
	body             io.Reader
}

func (b *walletHeldBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.started); <-b.release })
	return b.body.Read(p)
}
func TestV2WalletCaptureChangesBeforeAndAfterForward(t *testing.T) {
	for _, phase := range []string{"before", "after"} {
		t.Run(phase, func(t *testing.T) {
			f := newWalletAppFixture(t)
			address := "0x" + strings.Repeat("1", 40)
			entered, release := make(chan struct{}), make(chan struct{})
			defer closeIfOpen(release)
			body := io.Reader(strings.NewReader(`{"address":"` + address + `"}`))
			if phase == "before" {
				body = &walletHeldBody{started: entered, release: release, body: body}
			} else {
				f.onWallet = func(w http.ResponseWriter, r *http.Request) {
					close(entered)
					<-release
					_, _ = w.Write([]byte(`{"private":"original Actor"}`))
				}
			}
			r := f.request("POST", "/api/eth/link/challenge", body)
			out := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { defer close(done); f.server.Handler().ServeHTTP(out, r) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("held checkpoint not reached")
			}
			logout := f.call("POST", "/api/auth/logout", "{}")
			if logout.Code != 200 {
				t.Fatal("actual durable logout", logout.Code)
			}
			close(release)
			<-done
			if out.Code != 409 || strings.Contains(out.Body.String(), "private") {
				t.Fatal("stale private reply released", out.Code)
			}
			expected := int64(0)
			if phase == "after" {
				expected = 1
			}
			if f.effects.Load() != expected {
				t.Fatal("wrong forward count", phase, f.effects.Load())
			}
		})
	}
}
func closeIfOpen(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}

func TestV2WalletMalformedOversizeAndRedirectFailClosed(t *testing.T) {
	f := newWalletAppFixture(t)
	for _, kind := range []string{"redirect", "oversize", "nonJSON", "unknownStatus", "duplicateError", "businessConflict"} {
		t.Run(kind, func(t *testing.T) {
			f.onWallet = func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "redirect":
					w.Header().Set("Location", "https://elsewhere.test/private")
					w.WriteHeader(302)
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat(" ", walletResponseLimit+1)))
				case "nonJSON":
					_, _ = w.Write([]byte("broken"))
				case "unknownStatus":
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"error":"private raw exception"}`))
				case "duplicateError":
					w.WriteHeader(409)
					_, _ = w.Write([]byte(`{"error":"private raw exception","error":"wallet state conflict"}`))
				case "businessConflict":
					w.WriteHeader(409)
					_, _ = w.Write([]byte(`{"error":"wallet state conflict"}`))
				}
			}
			before := f.effects.Load()
			r := f.call("GET", "/api/eth/addresses", "")
			expected := 503
			if kind == "businessConflict" {
				expected = 409
			}
			if r.Code != expected || f.effects.Load() != before+1 || r.Header().Get("Location") != "" || strings.Contains(r.Body.String(), "private raw") {
				t.Fatal("unqualified response or retry", kind, r.Code)
			}
		})
	}
}
