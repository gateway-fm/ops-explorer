//go:build !privacy

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"explorer/internal/privacy"

	"github.com/go-chi/chi/v5"
)

// csrfProbe runs a request through s.csrfProtect-wrapped trivial handler and
// returns the status code. 200 means "passed" (handler ran); 403 means blocked.
func csrfProbe(s *Server, method, origin, referer string) int {
	h := s.csrfProtect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(method, "/api/eth/link/verify", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if referer != "" {
		r.Header.Set("Referer", referer)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

// A-1: with the allowlist active, the CSRF middleware rejects state-changing
// requests whose Origin/Referer is foreign or absent, and passes allowed ones
// and all safe methods (PROD_READINESS_AUDIT §A-1).
func TestCSRFProtect_Allowlisted(t *testing.T) {
	s := &Server{cfg: &ServerConfig{CORSAllowedOrigins: []string{"https://app.example.com"}}}

	t.Run("foreign-Origin POST -> 403", func(t *testing.T) {
		if got := csrfProbe(s, http.MethodPost, "https://evil.example.com", ""); got != http.StatusForbidden {
			t.Errorf("foreign Origin POST: got %d, want 403", got)
		}
	})

	t.Run("allowed-Origin POST -> pass", func(t *testing.T) {
		if got := csrfProbe(s, http.MethodPost, "https://app.example.com", ""); got != http.StatusOK {
			t.Errorf("allowed Origin POST: got %d, want 200 (pass)", got)
		}
	})

	t.Run("both Origin and Referer absent -> 403 (fail-closed)", func(t *testing.T) {
		if got := csrfProbe(s, http.MethodPost, "", ""); got != http.StatusForbidden {
			t.Errorf("no Origin/Referer on POST: got %d, want 403 (fail-closed)", got)
		}
	})

	t.Run("allowed Referer (no Origin) -> pass", func(t *testing.T) {
		if got := csrfProbe(s, http.MethodPost, "", "https://app.example.com/some/path"); got != http.StatusOK {
			t.Errorf("allowed Referer POST: got %d, want 200 (pass)", got)
		}
	})

	t.Run("foreign Referer (no Origin) -> 403", func(t *testing.T) {
		if got := csrfProbe(s, http.MethodPost, "", "https://evil.example.com/x"); got != http.StatusForbidden {
			t.Errorf("foreign Referer POST: got %d, want 403", got)
		}
	})

	t.Run("safe GET passes even with foreign Origin", func(t *testing.T) {
		if got := csrfProbe(s, http.MethodGet, "https://evil.example.com", ""); got != http.StatusOK {
			t.Errorf("safe GET: got %d, want 200 (pass)", got)
		}
	})

	t.Run("DELETE is gated like POST (foreign -> 403)", func(t *testing.T) {
		if got := csrfProbe(s, http.MethodDelete, "https://evil.example.com", ""); got != http.StatusForbidden {
			t.Errorf("foreign Origin DELETE: got %d, want 403", got)
		}
	})
}

// A-1: with an empty allowlist (standalone), the CSRF middleware is a no-op —
// even a foreign-Origin POST passes (no breaking change for public explorers).
func TestCSRFProtect_EmptyAllowlistNoOp(t *testing.T) {
	s := &Server{cfg: &ServerConfig{CORSAllowedOrigins: nil}}
	if got := csrfProbe(s, http.MethodPost, "https://evil.example.com", ""); got != http.StatusOK {
		t.Errorf("empty allowlist must be a no-op, foreign POST got %d, want 200", got)
	}
}

// A-1: the impersonation restore endpoint GET /api/impersonation/{token} must
// NEVER be CSRF-blocked (it is a safe method and the frontend relies on it
// after a page refresh).
func TestCSRF_ImpersonationRestoreGetNeverBlocked(t *testing.T) {
	s := &Server{
		router:        chi.NewRouter(),
		privacyClient: privacy.NewClient("https://proxy.invalid"),
		cfg:           &ServerConfig{CORSAllowedOrigins: []string{"https://app.example.com"}},
	}
	// Enable the impersonation routes (normally gated on jwtVerifier via New;
	// set the store directly for this routing test).
	s.impersonations = NewMemoryImpersonationStoreNoGC()
	s.setupRoutes()

	// GET restore with a foreign Origin must not be 403 (route exists; handler
	// returns 401/404 for an unknown token, but never CSRF-403).
	r := httptest.NewRequest(http.MethodGet, "/api/impersonation/sometoken", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, r)
	if w.Code == http.StatusForbidden {
		t.Errorf("GET /api/impersonation/{token} restore must never be CSRF-blocked, got 403")
	}
}

// A-1: integration — a foreign-Origin POST to a wired state-changing route
// (/api/eth/link/verify) is rejected with 403 before reaching the handler.
func TestCSRF_EthPostForeignOriginBlocked(t *testing.T) {
	s := &Server{
		router:        chi.NewRouter(),
		privacyClient: privacy.NewClient("https://proxy.invalid"),
		cfg:           &ServerConfig{CORSAllowedOrigins: []string{"https://app.example.com"}},
	}
	s.setupRoutes()

	r := httptest.NewRequest(http.MethodPost, "/api/eth/link/verify", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	r.AddCookie(&http.Cookie{Name: AuthCookieName, Value: makeTestJWTForCSRF()})
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("foreign-Origin POST to /api/eth/link/verify: got %d, want 403", w.Code)
	}
}

func makeTestJWTForCSRF() string {
	return makeTestJWT("did:privado:csrf", time.Now().Add(time.Hour))
}
