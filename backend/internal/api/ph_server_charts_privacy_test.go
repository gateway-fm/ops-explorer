//go:build !privacy

package api

import (
	"net/http"
	"testing"
)

// RD-1063: in privacy mode the charts, gas, and price surfaces must NOT be
// mounted. They have no viewer-scoped/redaction-safe story on the privacy
// proxy: /charts forwards to a chain-indexer endpoint the proxy doesn't serve
// (404 upstream), /gas resolves to ErrChainDataNotAvailable (500), and /price
// triggers an outbound CoinGecko fetch on cache-miss (a no-egress hole). The
// frontend already early-returns / disables these in privacy mode; this gate
// is the authoritative backstop that turns any leaked call into a clean 404
// (mirrors the P-2 verification-write gating just below in this package).
//
// Reuses newRoutedServer/probe from ph_server_privacy_routes_test.go.
func TestPrivacyMode_ChartsGasPriceSurfacesAre404(t *testing.T) {
	s := newRoutedServer(t, true /* privacyMode */)

	cases := []struct {
		name, method, path string
	}{
		{"charts counters", http.MethodGet, "/api/charts/counters"},
		{"charts lines", http.MethodGet, "/api/charts/lines"},
		{"charts line by id", http.MethodGet, "/api/charts/lines/new_txns"},
		{"gas prices", http.MethodGet, "/api/gas"},
		{"price", http.MethodGet, "/api/price"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := probe(s, c.method, c.path, ""); got != http.StatusNotFound {
				t.Errorf("%s %s: expected 404 in privacy mode, got %d", c.method, c.path, got)
			}
		})
	}
}

// RD-1063: in standalone mode the same surfaces MUST stay mounted (i.e. NOT
// 404 — the route exists and reaches its handler). tx-history stays mounted in
// both modes and is intentionally not gated here (redaction-safe via proxy).
func TestStandaloneMode_ChartsGasPriceSurfacesAreMounted(t *testing.T) {
	s := newRoutedServer(t, false /* standalone */)

	cases := []struct {
		name, method, path string
	}{
		{"charts counters", http.MethodGet, "/api/charts/counters"},
		{"charts lines", http.MethodGet, "/api/charts/lines"},
		{"gas prices", http.MethodGet, "/api/gas"},
		{"price", http.MethodGet, "/api/price"},
		{"tx-history (ungated)", http.MethodGet, "/api/stats/tx-history"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := probe(s, c.method, c.path, ""); got == http.StatusNotFound {
				t.Errorf("%s %s: expected route to be MOUNTED in standalone mode, got 404", c.method, c.path)
			}
		})
	}
}
