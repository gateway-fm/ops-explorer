//go:build !privacy

package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/internal/auth"
)

// A-3: cookieSecure(r) resolves the Secure flag from explicit config rather
// than a spoofable/absent inbound header (PROD_READINESS_AUDIT §A-3):
//   - "true"  -> always Secure
//   - "false" -> never Secure
//   - "auto"  -> Secure only when the request is actually HTTPS (r.TLS set) or
//                X-Forwarded-Proto: https is present (trusted-proxy case).
func TestCookieSecure(t *testing.T) {
	newReq := func(xfp string, tls bool) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
		if xfp != "" {
			r.Header.Set("X-Forwarded-Proto", xfp)
		}
		if tls {
			r.TLS = &dummyTLSState
		}
		return r
	}

	tests := []struct {
		name string
		mode string
		xfp  string
		tls  bool
		want bool
	}{
		{"true always secure (plain http)", "true", "", false, true},
		{"true always secure (even http header)", "true", "http", false, true},
		{"false never secure (even behind https)", "false", "https", false, false},
		{"false never secure (even real TLS)", "false", "", true, false},
		{"auto + no tls + no header -> false", "auto", "", false, false},
		{"auto + X-Forwarded-Proto https -> true", "auto", "https", false, true},
		{"auto + real TLS -> true", "auto", "", true, true},
		{"auto + X-Forwarded-Proto http -> false", "auto", "http", false, false},
		{"empty mode behaves as auto", "", "https", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{cfg: &ServerConfig{CookieSecure: tt.mode}}
			if got := s.cookieSecure(newReq(tt.xfp, tt.tls)); got != tt.want {
				t.Errorf("cookieSecure(mode=%q, xfp=%q, tls=%v) = %v, want %v", tt.mode, tt.xfp, tt.tls, got, tt.want)
			}
		})
	}
}

// A-3: the auth callback must emit cookies whose Secure flag follows
// cookieSecure — here, mode "true" forces Secure even over plain HTTP (the
// privacy-mode default), proving the inline spoofable-header logic was replaced.
func TestAuthCallback_ForcesSecureCookiesWhenConfigured(t *testing.T) {
	mockSrv := newMockTokenServer("access-tok", "refresh-tok")
	defer mockSrv.Close()

	ssoClient := auth.NewSSOClient(mockSrv.URL, mockSrv.URL, "client-id", "", "http://redirect")
	s := &Server{ssoClient: ssoClient, cfg: &ServerConfig{CookieSecure: "true"}}

	// Drive a successful callback (mirrors TestHandleAuthCallback_SetsBothCookies
	// but over plain HTTP with no X-Forwarded-Proto).
	state, err := ssoClient.GenerateState("/")
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=abc&state="+state, nil)
	w := httptest.NewRecorder()
	s.handleAuthCallback(w, r)

	resp := w.Result()
	for _, name := range []string{AuthCookieName, RefreshCookieName} {
		c := findCookie(resp, name)
		if c == nil {
			t.Fatalf("expected cookie %q to be set", name)
		}
		if !c.Secure {
			t.Errorf("cookie %q must be Secure when COOKIE_SECURE=true, even over plain HTTP", name)
		}
	}
}

var dummyTLSState = tls.ConnectionState{}
