//go:build !privacy

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// corsServer builds a Server whose routes (and thus CORS middleware) are wired
// from the given allowlist + privacy flag, without a real DB/proxy.
func corsServer(allowed []string, privacyMode bool) *Server {
	s := &Server{
		router: chi.NewRouter(),
		cfg: &ServerConfig{
			CORSAllowedOrigins: allowed,
			PrivacyMode:        privacyMode,
		},
	}
	s.setupRoutes()
	return s
}

// preflight issues an OPTIONS preflight with the given Origin and returns the
// Access-Control-Allow-Origin / -Credentials response headers.
func preflight(s *Server, origin string) (acao, acac string) {
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/blocks", nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("Access-Control-Request-Method", http.MethodGet)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, r)
	return w.Header().Get("Access-Control-Allow-Origin"), w.Header().Get("Access-Control-Allow-Credentials")
}

// W-1: with a configured allowlist, a disallowed Origin must get NO
// Access-Control-Allow-Origin header; an allowed Origin must get its exact
// Origin reflected plus Access-Control-Allow-Credentials: true
// (PROD_READINESS_AUDIT §W-1).
func TestCORS_Allowlist(t *testing.T) {
	allowed := []string{"https://app.example.com"}
	s := corsServer(allowed, false)

	t.Run("disallowed origin gets no ACAO", func(t *testing.T) {
		acao, _ := preflight(s, "https://evil.example.com")
		if acao != "" {
			t.Errorf("disallowed Origin must not receive Access-Control-Allow-Origin, got %q", acao)
		}
	})

	t.Run("allowed origin is reflected with credentials", func(t *testing.T) {
		acao, acac := preflight(s, "https://app.example.com")
		if acao != "https://app.example.com" {
			t.Errorf("allowed Origin not reflected: ACAO=%q", acao)
		}
		if acac != "true" {
			t.Errorf("expected Access-Control-Allow-Credentials: true, got %q", acac)
		}
	})
}

// W-1: standalone with an empty allowlist preserves the current permissive
// behavior — any Origin is reflected (with credentials). This is the documented
// breaking-change boundary: only privacy mode is fail-closed.
func TestCORS_StandaloneEmptyAllowlistReflects(t *testing.T) {
	s := corsServer(nil, false)
	acao, acac := preflight(s, "https://anything.example.com")
	if acao != "https://anything.example.com" {
		t.Errorf("standalone empty allowlist should reflect any Origin, got ACAO=%q", acao)
	}
	if acac != "true" {
		t.Errorf("expected credentials allowed, got %q", acac)
	}
}
