//go:build !privacy

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"explorer/internal/privacy"

	"github.com/go-chi/chi/v5"
)

// newRoutedServer builds a Server with routes wired, in either privacy or
// standalone mode, without needing a real DB/rpc/proxy. Only the routing
// decision (gated by inPrivacyMode) is under test here.
func newRoutedServer(t *testing.T, privacyMode bool) *Server {
	t.Helper()
	s := &Server{router: chi.NewRouter()}
	if privacyMode {
		s.privacyClient = privacy.NewClient("https://proxy.invalid")
	}
	s.setupRoutes()
	return s
}

// probe sends a request through the server's router and returns the status code.
func probe(s *Server, method, target string, body string) int {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, r)
	return w.Code
}

// P-2: when the DEFAULT (non-privacy) binary is serving privacy mode
// (privacyClient enabled), the local-DB write surfaces — POST /api/verify,
// POST /api/verify/standard-json, the Etherscan RPC entry-point, and the
// Sourcify lookup routes — must NOT be mounted (404). Those persist
// attacker-controlled source/ABI into block-explorer's own Postgres, bypassing
// privacy-proxy redaction (PROD_READINESS_AUDIT §P-2).
func TestPrivacyMode_VerificationWriteSurfacesAre404(t *testing.T) {
	s := newRoutedServer(t, true /* privacyMode */)

	cases := []struct {
		name, method, path, body string
	}{
		{"native verify", http.MethodPost, "/api/verify", `{}`},
		{"native verify v1", http.MethodPost, "/api/v1/verify", `{}`},
		{"standard-json verify", http.MethodPost, "/api/verify/standard-json", `{}`},
		{"etherscan rpc root", http.MethodPost, "/api/", `{}`},
		{"sourcify lookup", http.MethodGet, "/api/addresses/0x52908400098527886E0F7030069857D2E4169EE7/sourcify", ""},
		{"sourcify check", http.MethodGet, "/api/addresses/0x52908400098527886E0F7030069857D2E4169EE7/sourcify/check", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := probe(s, c.method, c.path, c.body); got != http.StatusNotFound {
				t.Errorf("%s %s: expected 404 in privacy mode, got %d", c.method, c.path, got)
			}
		})
	}
}

// P-2: in standalone mode the same surfaces MUST be mounted (i.e. NOT 404 —
// the route exists and reaches its handler).
func TestStandaloneMode_VerificationWriteSurfacesAreMounted(t *testing.T) {
	s := newRoutedServer(t, false /* standalone */)

	cases := []struct {
		name, method, path, body string
	}{
		{"native verify", http.MethodPost, "/api/verify", `{}`},
		{"standard-json verify", http.MethodPost, "/api/verify/standard-json", `{}`},
		{"sourcify lookup", http.MethodGet, "/api/addresses/0x52908400098527886E0F7030069857D2E4169EE7/sourcify", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := probe(s, c.method, c.path, c.body); got == http.StatusNotFound {
				t.Errorf("%s %s: expected route to be MOUNTED in standalone mode, got 404", c.method, c.path)
			}
		})
	}
}
