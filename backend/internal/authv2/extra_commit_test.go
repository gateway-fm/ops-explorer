package authv2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func relayStore(t *testing.T, s *Service) *commitRelay {
	t.Helper()
	u, e := url.Parse(os.Getenv("E2B_TEST_OWNER_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	relay := newCommitRelay(t, u.Host)
	u.Host = relay.listener.Addr().String()
	u.User = url.UserPassword(AppRole, "owned-app-fixture")
	st, e := openStore(context.Background(), s.source, u.String(), s.clock)
	if e != nil {
		t.Fatal(e)
	}
	s.store.close()
	s.store = st
	return relay
}

func TestUnknownStartCommitEmitsNoHandle(t *testing.T) {
	s, owner, _, _ := fixture(t)
	relay := relayStore(t, s)
	relay.remaining.Store(1)
	result := authCall(s, req("POST", "/api/auth/login", strings.NewReader(`{"mode":"login","returnPath":"/"}`), nil))
	if result.Code != 503 || len(result.Result().Cookies()) != 0 || strings.Contains(result.Body.String(), "authorizationUrl") || relay.withheld.Load() != 1 {
		t.Fatal("unknown start COMMIT emitted a handle or URL")
	}
	var n int
	if owner.QueryRow(context.Background(), `SELECT context_count FROM explorer_sso_v2.store_state`).Scan(&n) != nil || n != 1 {
		t.Fatal("owned start was not actually durable")
	}
}

func TestUnknownPendingReadCommitDoesNotExchangeCode(t *testing.T) {
	s, _, clock, key := fixture(t)
	var calls atomic.Int64
	ops := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		safeJSON(w, 200, map[string]any{"access_token": sign(t, key, testClaims(s.source, clock.Load())), "token_type": "Bearer", "expires_in": 300})
	}))
	defer ops.Close()
	fixtureRebind(t, s, ops.URL)
	s.client.Transport = ops.Client().Transport
	relay := relayStore(t, s)
	start := authCall(s, req("POST", "/api/auth/login", strings.NewReader(`{"mode":"login","returnPath":"/"}`), nil))
	if start.Code != 200 {
		t.Fatal("start")
	}
	var payload struct {
		URL string `json:"authorizationUrl"`
	}
	_ = json.Unmarshal(start.Body.Bytes(), &payload)
	u, _ := url.Parse(payload.URL)
	code, _ := randomToken()
	path := "/api/auth/callback?" + url.Values{"code": {code}, "state": {u.Query().Get("state")}}.Encode()
	relay.remaining.Store(1)
	result := authCall(s, req("GET", path, nil, start.Result().Cookies()))
	if result.Code != 503 || calls.Load() != 0 || len(result.Result().Cookies()) != 0 || relay.withheld.Load() != 1 {
		t.Fatal("unknown pending-read COMMIT performed token action or output")
	}
}

func TestUnknownFinalReadCommitDiscardsPrivateHeaders(t *testing.T) {
	s, _, clock, key := fixture(t)
	browser, auth, selected := install(t, s, key, clock.Load())
	relay := relayStore(t, s)
	relay.remaining.Store(2)
	request := req("GET", "/api/private", nil, []*http.Cookie{{Name: browserCookie, Value: browser}, {Name: sessionCookie, Value: auth}})
	request.Header.Set(SessionHeader, selected.capture.activation)
	response := httptest.NewRecorder()
	handler := s.Protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/private")
		w.Header().Set("Set-Cookie", "private=value")
		_, _ = w.Write([]byte("PRIVATE"))
	}))
	s.Boundary(handler, nil).ServeHTTP(response, request)
	if response.Code != 503 || strings.Contains(response.Body.String(), "PRIVATE") || response.Header().Get("Location") != "" || response.Header().Get("Set-Cookie") != "" || relay.withheld.Load() != 1 {
		t.Fatal("unknown final read released staged private data")
	}
}
