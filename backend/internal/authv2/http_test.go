package authv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	uuidlib "github.com/google/uuid"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type logoutResponsePause struct {
	http.ResponseWriter
	beforeSend func()
	done       bool
}

func (w *logoutResponsePause) WriteHeader(status int) {
	if !w.done {
		w.done = true
		w.beforeSend()
	}
	w.ResponseWriter.WriteHeader(status)
}

func TestKnownLogoutRevokesWithoutLateCookieMutation(t *testing.T) {
	s, _, clock, key := fixture(t)
	clock.Store(time.Now().Unix())
	browser, oldHandle, oldSession := install(t, s, key, clock.Load())
	origin, _ := url.Parse("https://app.test")
	jar, _ := cookiejar.New(nil)
	initial := httptest.NewRecorder()
	setCookie(initial, browserCookie, browser, clock.Load()+7*86400)
	setCookie(initial, sessionCookie, oldHandle, oldSession.capture.expires)
	jar.SetCookies(origin, initial.Result().Cookies())
	result := httptest.NewRecorder()
	var newHandle string
	paused := &logoutResponsePause{ResponseWriter: result, beforeSend: func() {
		// Actual logout COMMIT completed, but its headers have not arrived.
		if _, e := s.session(context.Background(), browser, oldHandle); e != ErrUnauthenticated {
			t.Fatal("logout did not revoke old capture before response", e)
		}
		state, _ := randomToken()
		pending := pendingCapture{browser: oldSession.capture.browser, revision: uuidlib.NewString(), hash: handleHash("oauth-state", state), issued: clock.Load(), expires: clock.Load() + 300}
		if e := s.store.start(context.Background(), pending); e != nil {
			t.Fatal(e)
		}
		newHandle, _ = randomToken()
		claims := testClaims(s.source, clock.Load())
		claims.Subject = "ops:p1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
		active := activeCapture{browser: pending.browser, auth: handleHash("auth", newHandle), activation: uuidlib.NewString(), activated: clock.Load(), expires: claims.Expires, key: s.source.spec.TokenKey.ID}
		var e error
		active.cipher, e = s.codec.sealToken(active, sign(t, key, claims))
		if e != nil {
			t.Fatal(e)
		}
		if e = s.store.activate(context.Background(), pending, active); e != nil {
			t.Fatal(e)
		}
		newResponse := httptest.NewRecorder()
		setCookie(newResponse, sessionCookie, newHandle, active.expires)
		jar.SetCookies(origin, newResponse.Result().Cookies())
	}}
	r := req("POST", "/api/auth/logout", strings.NewReader("{}"), jar.Cookies(origin))
	r.Header.Set(SessionHeader, oldSession.capture.activation)
	s.Boundary(http.HandlerFunc(s.Auth), nil).ServeHTTP(paused, r)
	if !paused.done || result.Code != 200 || len(result.Result().Cookies()) != 0 {
		t.Fatal("known logout mutated a shared cookie", result.Code)
	}
	jar.SetCookies(origin, result.Result().Cookies())
	actual := ""
	for _, c := range jar.Cookies(origin) {
		if c.Name == sessionCookie {
			actual = c.Value
		}
	}
	if actual != newHandle {
		t.Fatal("late logout response overwrote newer cookie")
	}
	if _, e := s.session(context.Background(), browser, newHandle); e != nil {
		t.Fatal("new capture was not retained", e)
	}
	if _, e := s.session(context.Background(), browser, oldHandle); e != ErrUnauthenticated {
		t.Fatal("revoked original handle revived", e)
	}
}

func req(method, path string, body io.Reader, cookies []*http.Cookie) *http.Request {
	r := httptest.NewRequest(method, "https://app.test"+path, body)
	if method == "POST" {
		r.Header.Set("Origin", "https://app.test")
		r.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	return r
}
func authCall(s *Service, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.Boundary(http.HandlerFunc(s.Auth), func(string, string) bool { return true }).ServeHTTP(w, r)
	return w
}
func fixtureRebind(t *testing.T, s *Service, ops string) {
	t.Helper()
	v := s.source.spec
	v.OPSOrigin = ops
	raw, _ := json.Marshal(v)
	source, e := ParseSource(raw, sha256.Sum256(raw))
	if e != nil {
		t.Fatal(e)
	}
	s.source = source
	s.codec.source = source
	s.store.source = source
	tx, e := s.store.pool.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(context.Background(), `UPDATE explorer_sso_v2.store_state SET config_digest=$1`, source.digest[:]); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestTLSCallbackOriginalTokenAndOneUse(t *testing.T) {
	s, _, clock, key := fixture(t)
	calls := 0
	ops := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v2/sso/token" || r.Header.Get("Cookie") != "" || r.Header.Get("Origin") != "" {
			t.Error("unexpected backchannel surface")
			w.WriteHeader(400)
			return
		}
		id, secret, ok := r.BasicAuth()
		id, e := url.QueryUnescape(id)
		secret, e2 := url.QueryUnescape(secret)
		if !ok || e != nil || e2 != nil || id != s.source.spec.ClientID || secret != s.codec.clientSecret {
			t.Error("OAuth Basic was not decoded exactly once")
			w.WriteHeader(401)
			return
		}
		if r.ParseForm() != nil || r.Form.Get("client_id") != id || !token32(r.Form.Get("code_verifier")) || r.Form.Get("redirect_uri") != s.source.spec.CallbackURI {
			t.Error("wrong exact token form")
			w.WriteHeader(400)
			return
		}
		safeJSON(w, 200, map[string]any{"access_token": sign(t, key, testClaims(s.source, clock.Load())), "token_type": "Bearer", "expires_in": 300})
	}))
	defer ops.Close()
	fixtureRebind(t, s, ops.URL)
	s.client.Transport = ops.Client().Transport
	s.codec.clientSecret = "A+:%2B" + strings.Repeat("c", 26)
	login := authCall(s, req("POST", "/api/auth/login", strings.NewReader(`{"mode":"login","returnPath":"/transactions?x=%22%26"}`), nil))
	if login.Code != 200 {
		t.Fatal("start", login.Code)
	}
	var start struct {
		URL string `json:"authorizationUrl"`
	}
	_ = json.Unmarshal(login.Body.Bytes(), &start)
	u, e := url.Parse(start.URL)
	if e != nil || u.Path != "/api/v2/sso/authorize" || u.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("bad authorization URL")
	}
	cookies := login.Result().Cookies()
	browser, frame := "", ""
	for _, c := range cookies {
		if !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" || c.SameSite != http.SameSiteLaxMode {
			t.Fatal("weak cookie")
		}
		if c.Name == browserCookie {
			browser = c.Value
		}
		if c.Name == oauthCookie {
			frame = c.Value
			if len(c.String()) > 4096 {
				t.Fatal("OAuth cookie exceeds browser bound")
			}
		}
	}
	if browser == "" || frame == "" {
		t.Fatal("missing opaque cookies")
	}
	row, e := s.store.read(context.Background(), handleHash("browser", browser))
	if e != nil {
		t.Fatal(e)
	}
	p, _ := row.pending()
	payload, e := s.codec.openState(p, frame, clock.Load())
	if e != nil {
		t.Fatal(e)
	}
	challenge := sha256.Sum256([]byte(payload.Verifier))
	if u.Query().Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) {
		t.Fatal("PKCE mismatch")
	}
	code, _ := randomToken()
	callback := "/api/auth/callback?" + url.Values{"code": {code}, "state": {u.Query().Get("state")}}.Encode()
	done := authCall(s, req("GET", callback, nil, cookies))
	if done.Code != 303 || done.Header().Get("Location") != "/transactions?x=%22%26" {
		t.Fatal("callback", done.Code)
	}
	var auth string
	for _, c := range done.Result().Cookies() {
		if c.Name != sessionCookie {
			t.Fatal("callback cleared or replaced pending cookie")
		}
		auth = c.Value
	}
	if !token32(auth) {
		t.Fatal("missing opaque AUTH")
	}
	session, e := s.session(context.Background(), browser, auth)
	if e != nil || !strings.Contains(session.raw, ".") {
		t.Fatal("original verified token unavailable")
	}
	if bytes.Contains(done.Body.Bytes(), []byte(session.raw)) {
		t.Fatal("JWT sent to browser")
	}
	replay := authCall(s, req("GET", callback, nil, cookies))
	if replay.Code != 401 || calls != 1 || len(replay.Result().Cookies()) != 0 {
		t.Fatal("callback replay contacted token endpoint or emitted credentials", replay.Code, calls)
	}
}
func TestRenewStartRejectsCapturedAccountSwitch(t *testing.T) {
	s, _, clock, key := fixture(t)
	browser, _, original := install(t, s, key, clock.Load())
	state, _ := randomToken()
	first := pendingCapture{original.capture.browser, "11111111-1111-1111-1111-111111111111", handleHash("oauth-state", state), clock.Load(), clock.Load() + 300}
	if e := s.store.start(context.Background(), first); e != nil {
		t.Fatal(e)
	}
	// Original A read is complete. A different callback commits activation B
	// before renewal A obtains the state/context locks.
	c := testClaims(s.source, clock.Load())
	c.Subject = "ops:p1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
	auth, _ := randomToken()
	next := original.capture
	next.activation = "22222222-2222-2222-2222-222222222222"
	next.auth = handleHash("auth", auth)
	var e error
	next.cipher, e = s.codec.sealToken(next, sign(t, key, c))
	if e != nil {
		t.Fatal(e)
	}
	if e = s.store.activate(context.Background(), first, next); e != nil {
		t.Fatal(e)
	}
	pending := first
	pending.revision = "33333333-3333-3333-3333-333333333333"
	if e = s.store.startGuarded(context.Background(), pending, &original.capture, true); e != ErrChanged {
		t.Fatal("stale original active capture installed renewal", e)
	}
	if e = s.store.startGuarded(context.Background(), pending, nil, true); e != ErrChanged {
		t.Fatal("hint-only renewal replaced a live account")
	}
	actual, e := s.session(context.Background(), browser, auth)
	if e != nil || !sameActive(actual.capture, next) {
		t.Fatal("rejected renewal changed account B")
	}
}

func TestIngressExactCORSAndCachePolicy(t *testing.T) {
	s, _, _, _ := fixture(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public,max-age=100")
		w.Header().Set("ETag", "private-tag")
		w.Header().Set("Access-Control-Allow-Origin", "https://evil.test")
		w.Header().Set("Vary", "Accept-Encoding")
		w.WriteHeader(204)
	})
	for _, test := range []struct {
		name, origin, method string
		status               int
	}{{"same", "https://app.test", "GET", 204}, {"absent", "", "GET", 204}, {"foreign", "https://evil.test", "GET", 403}, {"null", "null", "GET", 403}, {"mutation-no-origin", "", "POST", 403}, {"valid-preflight", "https://app.test", "OPTIONS", 204}} {
		t.Run(test.name, func(t *testing.T) {
			r := req(test.method, "/api/private", nil, nil)
			r.Header.Del("Origin")
			if test.origin != "" {
				r.Header.Set("Origin", test.origin)
			}
			if test.method == "OPTIONS" {
				r.Header.Set("Access-Control-Request-Method", "GET")
				r.Header.Set("Access-Control-Request-Headers", "X-OPS-App-Session")
			}
			w := httptest.NewRecorder()
			s.Boundary(handler, func(path, method string) bool { return path == "/api/private" && method == "GET" }).ServeHTTP(w, r)
			if w.Code != test.status || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("ETag") != "" {
				t.Fatal("cache or status boundary", w.Code)
			}
			expected := ""
			if test.origin == "https://app.test" {
				expected = test.origin
			}
			if w.Header().Get("Access-Control-Allow-Origin") != expected {
				t.Fatal("origin reflected")
			}
		})
	}
}
