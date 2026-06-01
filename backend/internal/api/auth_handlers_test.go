package api

import (
	"encoding/base64"
	"encoding/json"
	"explorer/internal/auth"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleAuthLogin_RedirectsToSSO(t *testing.T) {
	ssoClient := auth.NewSSOClient("https://proxy.example.com", "", "test-client", "", "https://explorer.example.com/api/auth/callback")

	s := &Server{
		ssoClient: ssoClient,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login?return_url=/dashboard", nil)
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	// Should redirect to privacy-proxy
	if !strings.Contains(location, "proxy.example.com/oauth/authorize") {
		t.Fatalf("expected redirect to proxy, got %s", location)
	}
	if !strings.Contains(location, "response_mode=redirect") {
		t.Fatalf("expected response_mode=redirect in URL, got %s", location)
	}
}

func TestHandleAuthLogin_NoSSO(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleAuthLogin_DefaultReturnURL(t *testing.T) {
	ssoClient := auth.NewSSOClient("https://proxy.example.com", "", "test-client", "", "https://explorer.example.com/api/auth/callback")

	s := &Server{
		ssoClient: ssoClient,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}

// makeTestJWT builds a JWT with the given subject and expiry. ExtractClaims
// only base64-decodes the payload — no signature verification — so the fake
// signature is fine.
func makeTestJWT(sub string, exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"sub":%q,"exp":%d,"iat":%d}`, sub, exp.Unix(), time.Now().Unix())
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	return header + "." + payload + ".fakesig"
}

// findCookie searches response cookies by name.
func findCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// newMockTokenServer returns an httptest.Server that serves both /oauth/token
// and /api/v1/refresh. The accessToken and refreshToken fields control what
// tokens are returned.
func newMockTokenServer(accessToken, refreshToken string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			json.NewEncoder(w).Encode(auth.OAuthTokenResponse{
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
				TokenType:    "Bearer",
				ExpiresIn:    300,
			})
		case "/api/v1/refresh":
			json.NewEncoder(w).Encode(auth.RefreshResponse{
				AccessToken:  "refreshed-access",
				RefreshToken: "refreshed-refresh",
				TokenType:    "Bearer",
				ExpiresIn:    300,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestHandleAuthCallback_SetsBothCookies(t *testing.T) {
	mockSrv := newMockTokenServer("access-tok", "refresh-tok")
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	// Pre-generate a valid state so the callback can validate it.
	state, err := ssoClient.GenerateState("/dashboard")
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=test-code&state="+state, nil)
	w := httptest.NewRecorder()
	s.handleAuthCallback(w, req)

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)

	if authCookie == nil {
		t.Fatal("expected auth cookie to be set")
	}
	if authCookie.Value != "access-tok" {
		t.Fatalf("auth cookie value = %q, want %q", authCookie.Value, "access-tok")
	}
	if refreshCookie == nil {
		t.Fatal("expected refresh cookie to be set")
	}
	if refreshCookie.Value != "refresh-tok" {
		t.Fatalf("refresh cookie value = %q, want %q", refreshCookie.Value, "refresh-tok")
	}
	if refreshCookie.MaxAge != RefreshCookieMaxAge {
		t.Fatalf("refresh cookie MaxAge = %d, want %d", refreshCookie.MaxAge, RefreshCookieMaxAge)
	}
}

func TestHandleAuthCallback_NoRefreshToken(t *testing.T) {
	// Server returns empty refresh token.
	mockSrv := newMockTokenServer("access-tok", "")
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	state, err := ssoClient.GenerateState("/")
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=test-code&state="+state, nil)
	w := httptest.NewRecorder()
	s.handleAuthCallback(w, req)

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)

	if authCookie == nil {
		t.Fatal("expected auth cookie to be set")
	}
	if refreshCookie != nil {
		t.Fatalf("expected no refresh cookie, but got value %q", refreshCookie.Value)
	}
}

func TestHandleAuthLogout_ClearsBothCookies(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	s.handleAuthLogout(w, req)

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)

	if authCookie == nil {
		t.Fatal("expected auth cookie to be cleared")
	}
	if authCookie.MaxAge != -1 {
		t.Fatalf("auth cookie MaxAge = %d, want -1", authCookie.MaxAge)
	}
	if refreshCookie == nil {
		t.Fatal("expected refresh cookie to be cleared")
	}
	if refreshCookie.MaxAge != -1 {
		t.Fatalf("refresh cookie MaxAge = %d, want -1", refreshCookie.MaxAge)
	}
}

func TestRefreshAuthMiddleware_FreshToken(t *testing.T) {
	// Token expires in 10 minutes — well above the 5-minute threshold.
	jwt := makeTestJWT("did:example:user", time.Now().Add(10*time.Minute))

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("refresh endpoint should not be called for a fresh token")
	}))
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	called := false
	handler := s.refreshAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "some-refresh"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	// No new cookies should be set.
	resp := w.Result()
	if findCookie(resp, AuthCookieName) != nil {
		t.Fatal("expected no auth cookie to be set for a fresh token")
	}
}

func TestRefreshAuthMiddleware_ExpiredToken(t *testing.T) {
	// Token already expired 5 minutes ago.
	jwt := makeTestJWT("did:example:user", time.Now().Add(-5*time.Minute))

	s := &Server{}

	called := false
	handler := s.refreshAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "some-refresh"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)

	if authCookie == nil || authCookie.MaxAge != -1 {
		t.Fatal("expected auth cookie to be cleared (MaxAge=-1)")
	}
	if refreshCookie == nil || refreshCookie.MaxAge != -1 {
		t.Fatal("expected refresh cookie to be cleared (MaxAge=-1)")
	}
}

func TestRefreshAuthMiddleware_NearExpiry_Success(t *testing.T) {
	// Token expires in 3 minutes — within the 5-minute threshold.
	jwt := makeTestJWT("did:example:user", time.Now().Add(3*time.Minute))

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(auth.RefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			TokenType:    "Bearer",
			ExpiresIn:    300,
		})
	}))
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	called := false
	handler := s.refreshAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "old-refresh"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)

	if authCookie == nil {
		t.Fatal("expected auth cookie to be updated")
	}
	if authCookie.Value != "new-access" {
		t.Fatalf("auth cookie = %q, want %q", authCookie.Value, "new-access")
	}
	if authCookie.MaxAge != CookieMaxAge {
		t.Fatalf("auth cookie MaxAge = %d, want %d", authCookie.MaxAge, CookieMaxAge)
	}
	if refreshCookie == nil {
		t.Fatal("expected refresh cookie to be updated")
	}
	if refreshCookie.Value != "new-refresh" {
		t.Fatalf("refresh cookie = %q, want %q", refreshCookie.Value, "new-refresh")
	}
	if refreshCookie.MaxAge != RefreshCookieMaxAge {
		t.Fatalf("refresh cookie MaxAge = %d, want %d", refreshCookie.MaxAge, RefreshCookieMaxAge)
	}
}

func TestRefreshAuthMiddleware_NearExpiry_Revoked(t *testing.T) {
	// Token expires in 3 minutes — within the threshold.
	jwt := makeTestJWT("did:example:user", time.Now().Add(3*time.Minute))

	// Mock server returns 401 to simulate revocation.
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"token revoked"}`))
	}))
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	called := false
	handler := s.refreshAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "revoked-refresh"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)

	if authCookie == nil || authCookie.MaxAge != -1 {
		t.Fatal("expected auth cookie to be cleared (MaxAge=-1)")
	}
	if refreshCookie == nil || refreshCookie.MaxAge != -1 {
		t.Fatal("expected refresh cookie to be cleared (MaxAge=-1)")
	}
}

func TestRefreshAuthMiddleware_NoRefreshCookie(t *testing.T) {
	// Token near expiry but no refresh cookie — should pass through without refresh.
	jwt := makeTestJWT("did:example:user", time.Now().Add(3*time.Minute))

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("refresh endpoint should not be called when no refresh cookie")
	}))
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	called := false
	handler := s.refreshAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	// No RefreshCookieName cookie.
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	// No cookie changes expected.
	resp := w.Result()
	if findCookie(resp, AuthCookieName) != nil {
		t.Fatal("expected no auth cookie to be set")
	}
}

func TestRefreshAuthMiddleware_NoSSO(t *testing.T) {
	// Token near expiry, ssoClient is nil — should pass through.
	jwt := makeTestJWT("did:example:user", time.Now().Add(3*time.Minute))

	s := &Server{ssoClient: nil}

	called := false
	handler := s.refreshAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "some-refresh"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	// No cookie changes expected.
	resp := w.Result()
	if findCookie(resp, AuthCookieName) != nil {
		t.Fatal("expected no auth cookie to be set")
	}
}

func TestRefreshAuthMiddleware_NetworkError_DoesNotClearCookies(t *testing.T) {
	// Network error (not revocation) must NOT clear cookies — the existing
	// near-expiry access token should still carry the request through.
	jwt := makeTestJWT("did:example:user", time.Now().Add(3*time.Minute))

	// Point at a closed server to force a network error.
	closedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedSrv.Close()

	ssoClient := auth.NewSSOClient(closedSrv.URL, closedSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	called := false
	handler := s.refreshAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "some-refresh"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}

	resp := w.Result()
	// Cookies must NOT be cleared on a transient network error.
	authCookie := findCookie(resp, AuthCookieName)
	if authCookie != nil && authCookie.MaxAge == -1 {
		t.Fatal("auth cookie must not be cleared on network error")
	}
	refreshCookie := findCookie(resp, RefreshCookieName)
	if refreshCookie != nil && refreshCookie.MaxAge == -1 {
		t.Fatal("refresh cookie must not be cleared on network error")
	}
}

func TestHandleAuthLogout_RevokesServerSide(t *testing.T) {
	revokeCalled := false
	var revokedToken string

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/revoke" {
			revokeCalled = true
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			revokedToken = body["refresh_token"]
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "my-refresh-token"})
	w := httptest.NewRecorder()

	s.handleAuthLogout(w, req)

	if !revokeCalled {
		t.Fatal("expected /api/v1/revoke to be called on logout")
	}
	if revokedToken != "my-refresh-token" {
		t.Fatalf("expected refresh token to be revoked, got %q", revokedToken)
	}

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)
	if authCookie == nil || authCookie.MaxAge != -1 {
		t.Fatal("expected auth cookie to be cleared")
	}
	if refreshCookie == nil || refreshCookie.MaxAge != -1 {
		t.Fatal("expected refresh cookie to be cleared")
	}
}

func TestHandleAuthStatus_ExpiredToken_ClearsBothCookies(t *testing.T) {
	jwt := makeTestJWT("did:example:user", time.Now().Add(-1*time.Minute))

	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "some-refresh"})
	w := httptest.NewRecorder()

	s.handleAuthStatus(w, req)

	resp := w.Result()
	authCookie := findCookie(resp, AuthCookieName)
	refreshCookie := findCookie(resp, RefreshCookieName)

	if authCookie == nil || authCookie.MaxAge != -1 {
		t.Fatal("expected auth cookie to be cleared on expired token")
	}
	if refreshCookie == nil || refreshCookie.MaxAge != -1 {
		t.Fatal("expected refresh cookie to be cleared on expired token")
	}
}
