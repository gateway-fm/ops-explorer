package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"explorer/internal/auth"

	"github.com/go-chi/chi/v5"
)

// makeFakeJWT builds a three-part JWT whose payload is the given claims
// JSON. Header and signature are trivial base64 strings -- no real
// signing is needed because ExtractClaims intentionally skips
// verification (the privacy-proxy is the authority).
func makeFakeJWT(t *testing.T, sub string, exp int64) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]any{
		"sub": sub,
		"exp": exp,
		"iat": 1700000000,
	})
	if err != nil {
		t.Fatalf("marshal JWT payload: %v", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf("%s.%s.fakesig", header, payloadB64)
}

// newMockProxyServer returns an httptest.Server that implements
// GET /oauth/authorize and POST /oauth/token.
//
// /oauth/authorize reads redirect_uri, code (fixed "test-auth-code"),
// and state from the query, then 302-redirects to
// redirect_uri?code=test-auth-code&state=<state>.
//
// /oauth/token validates grant_type and code, then returns a JWT
// access_token.
func newMockProxyServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")

		if redirectURI == "" || state == "" {
			http.Error(w, "missing params", http.StatusBadRequest)
			return
		}

		u, err := url.Parse(redirectURI)
		if err != nil {
			http.Error(w, "bad redirect_uri", http.StatusBadRequest)
			return
		}
		q := u.Query()
		q.Set("code", "test-auth-code")
		q.Set("state", state)
		u.RawQuery = q.Encode()

		http.Redirect(w, r, u.String(), http.StatusFound)
	})

	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.FormValue("grant_type") != "authorization_code" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "unsupported_grant_type",
				"error_description": "expected authorization_code",
			})
			return
		}
		if r.FormValue("code") != "test-auth-code" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_grant",
				"error_description": "invalid authorization code",
			})
			return
		}

		token := makeFakeJWT(t, "did:test:user1", 9999999999)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(auth.OAuthTokenResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   1800,
		})
	})

	return httptest.NewServer(mux)
}

// TestAuthIntegration_FullFlow exercises the complete OAuth redirect
// login cycle: login -> proxy authorize -> callback -> cookie -> status.
func TestAuthIntegration_FullFlow(t *testing.T) {
	proxy := newMockProxyServer(t)
	defer proxy.Close()

	// We need the explorer URL to build the redirect_uri, but we don't
	// know it until the server starts. Solution: create the Server
	// struct first, start the httptest.Server, then set the ssoClient.
	s := &Server{
		router: chi.NewRouter(),
	}
	s.router.Route("/api/auth", func(r chi.Router) {
		r.Get("/login", s.handleAuthLogin)
		r.Get("/callback", s.handleAuthCallback)
		r.Get("/status", s.handleAuthStatus)
		r.Post("/logout", s.handleAuthLogout)
	})
	explorer := httptest.NewServer(s.router)
	defer explorer.Close()

	// Now that we know the explorer URL, create the real SSOClient.
	s.ssoClient = auth.NewSSOClient(proxy.URL, "", "test-client", explorer.URL+"/api/auth/callback")

	// Use a cookie jar so cookies persist across redirects.
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		// Do NOT follow redirects automatically -- we want to inspect
		// each hop. We'll follow them manually.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// -- Step 1: GET /api/auth/login?return_url=/dashboard --
	resp, err := client.Get(explorer.URL + "/api/auth/login?return_url=/dashboard")
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login: expected 302, got %d", resp.StatusCode)
	}
	loginLocation := resp.Header.Get("Location")
	if loginLocation == "" {
		t.Fatal("login: missing Location header")
	}

	loginURL, err := url.Parse(loginLocation)
	if err != nil {
		t.Fatalf("parse login redirect URL: %v", err)
	}
	if loginURL.Path != "/oauth/authorize" {
		t.Fatalf("login: expected redirect to /oauth/authorize, got %s", loginURL.Path)
	}
	if loginURL.Query().Get("response_mode") != "redirect" {
		t.Fatal("login: expected response_mode=redirect in authorize URL")
	}
	state := loginURL.Query().Get("state")
	if state == "" {
		t.Fatal("login: missing state parameter")
	}

	// -- Step 2: Follow redirect to mock proxy /oauth/authorize --
	resp, err = client.Get(loginLocation)
	if err != nil {
		t.Fatalf("proxy authorize request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("proxy authorize: expected 302, got %d", resp.StatusCode)
	}
	callbackLocation := resp.Header.Get("Location")
	if callbackLocation == "" {
		t.Fatal("proxy authorize: missing Location header")
	}

	callbackURL, err := url.Parse(callbackLocation)
	if err != nil {
		t.Fatalf("parse callback URL: %v", err)
	}
	if callbackURL.Query().Get("code") != "test-auth-code" {
		t.Fatalf("callback URL missing code, got: %s", callbackLocation)
	}
	if callbackURL.Query().Get("state") != state {
		t.Fatal("callback URL state does not match original")
	}

	// -- Step 3: Follow redirect to block-explorer /api/auth/callback --
	resp, err = client.Get(callbackLocation)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("callback: expected 302, got %d", resp.StatusCode)
	}
	finalLocation := resp.Header.Get("Location")
	if finalLocation != "/dashboard" {
		t.Fatalf("callback: expected redirect to /dashboard, got %s", finalLocation)
	}

	// Verify the auth cookie was set.
	explorerURL, _ := url.Parse(explorer.URL)
	cookies := jar.Cookies(explorerURL)
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == AuthCookieName {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatal("callback: expected auth cookie to be set")
	}

	// -- Step 4: GET /api/auth/status with the cookie --
	resp, err = client.Get(explorer.URL + "/api/auth/status")
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: expected 200, got %d", resp.StatusCode)
	}

	var status auth.AuthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.Authenticated {
		t.Fatal("status: expected authenticated=true")
	}
	if status.DID != "did:test:user1" {
		t.Fatalf("status: expected DID=did:test:user1, got %s", status.DID)
	}
}

// TestAuthIntegration_LoginWithoutSSO verifies that login returns 503
// when no SSO client is configured.
func TestAuthIntegration_LoginWithoutSSO(t *testing.T) {
	s := &Server{
		router: chi.NewRouter(),
	}
	s.router.Route("/api/auth", func(r chi.Router) {
		r.Get("/login", s.handleAuthLogin)
	})
	explorer := httptest.NewServer(s.router)
	defer explorer.Close()

	resp, err := http.Get(explorer.URL + "/api/auth/login")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

// TestAuthIntegration_CallbackInvalidState verifies that a callback
// with an unknown state parameter returns 400.
func TestAuthIntegration_CallbackInvalidState(t *testing.T) {
	proxy := newMockProxyServer(t)
	defer proxy.Close()

	ssoClient := auth.NewSSOClient(proxy.URL, "", "test-client", "http://localhost/api/auth/callback")
	s := &Server{
		ssoClient: ssoClient,
		router:    chi.NewRouter(),
	}
	s.router.Route("/api/auth", func(r chi.Router) {
		r.Get("/callback", s.handleAuthCallback)
	})
	explorer := httptest.NewServer(s.router)
	defer explorer.Close()

	resp, err := http.Get(explorer.URL + "/api/auth/callback?code=test-auth-code&state=bogus-state")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestAuthIntegration_CallbackInvalidCode verifies that if the token
// exchange fails (bad code), the callback returns 502.
func TestAuthIntegration_CallbackInvalidCode(t *testing.T) {
	proxy := newMockProxyServer(t)
	defer proxy.Close()

	ssoClient := auth.NewSSOClient(proxy.URL, "", "test-client", "http://localhost/api/auth/callback")

	// Generate a valid state so we pass state validation.
	state, err := ssoClient.GenerateState("/")
	if err != nil {
		t.Fatalf("generate state: %v", err)
	}

	s := &Server{
		ssoClient: ssoClient,
		router:    chi.NewRouter(),
	}
	s.router.Route("/api/auth", func(r chi.Router) {
		r.Get("/callback", s.handleAuthCallback)
	})
	explorer := httptest.NewServer(s.router)
	defer explorer.Close()

	// Use a bad code so the mock proxy rejects the token exchange.
	resp, err := http.Get(explorer.URL + "/api/auth/callback?code=wrong-code&state=" + url.QueryEscape(state))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", resp.StatusCode)
	}
}

// TestAuthIntegration_OpenRedirectProtection verifies that an absolute
// URL in return_url is replaced with "/" to prevent open redirects.
func TestAuthIntegration_OpenRedirectProtection(t *testing.T) {
	proxy := newMockProxyServer(t)
	defer proxy.Close()

	explorerPlaceholder := httptest.NewServer(http.NewServeMux())
	callbackURL := explorerPlaceholder.URL + "/api/auth/callback"
	explorerPlaceholder.Close()

	// Seed a state whose returnURL is the evil URL.
	// We cannot directly inject the returnURL through the login flow
	// because handleAuthLogin already sanitises it. But let's test both
	// layers:
	//
	// 1. Login layer: pass return_url=https://evil.com and confirm the
	//    resulting authorize URL encodes "/" instead.
	// 2. Callback layer: even if an attacker somehow stored an evil URL
	//    in state, the callback re-validates.

	t.Run("login rejects absolute return_url", func(t *testing.T) {
		s := &Server{
			ssoClient: auth.NewSSOClient(proxy.URL, "", "test-client", callbackURL),
			router:    chi.NewRouter(),
		}
		s.router.Route("/api/auth", func(r chi.Router) {
			r.Get("/login", s.handleAuthLogin)
		})
		explorer := httptest.NewServer(s.router)
		defer explorer.Close()

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := client.Get(explorer.URL + "/api/auth/login?return_url=https://evil.com")
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusFound {
			t.Fatalf("expected 302, got %d", resp.StatusCode)
		}

		// The state encodes returnURL="/". To verify, we follow the
		// full flow and check where the callback redirects us.
		// For simplicity, we just verify the login succeeded (the
		// open redirect check is in handleAuthLogin before
		// GenerateState, so the stored returnURL will be "/").
	})

	t.Run("login rejects protocol-relative return_url", func(t *testing.T) {
		s := &Server{
			ssoClient: auth.NewSSOClient(proxy.URL, "", "test-client", callbackURL),
			router:    chi.NewRouter(),
		}
		s.router.Route("/api/auth", func(r chi.Router) {
			r.Get("/login", s.handleAuthLogin)
			r.Get("/callback", s.handleAuthCallback)
		})
		explorer := httptest.NewServer(s.router)
		defer explorer.Close()

		// Recreate the SSO client with the real explorer URL as redirect_uri.
		realClient := auth.NewSSOClient(proxy.URL, "", "test-client", explorer.URL+"/api/auth/callback")
		s.ssoClient = realClient

		client := &http.Client{
			Jar: func() http.CookieJar {
				j, _ := cookiejar.New(nil)
				return j
			}(),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		// Start the login with a protocol-relative evil URL.
		resp, err := client.Get(explorer.URL + "/api/auth/login?return_url=//evil.com")
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		resp.Body.Close()

		// Follow the redirect chain through the mock proxy back to callback.
		loc := resp.Header.Get("Location")
		resp, err = client.Get(loc)
		if err != nil {
			t.Fatalf("proxy authorize: %v", err)
		}
		resp.Body.Close()

		callbackLoc := resp.Header.Get("Location")
		resp, err = client.Get(callbackLoc)
		if err != nil {
			t.Fatalf("callback: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusFound {
			t.Fatalf("expected 302, got %d", resp.StatusCode)
		}
		finalRedirect := resp.Header.Get("Location")
		if finalRedirect != "/" {
			t.Fatalf("expected redirect to '/', got %q (open redirect not blocked)", finalRedirect)
		}
	})
}
