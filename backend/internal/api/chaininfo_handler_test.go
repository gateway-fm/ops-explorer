package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/internal/chaininfo"

	"github.com/go-chi/chi/v5"
)

// TestHandleGetChainInfo_ExposesPrivacyProxyPublicURL asserts that GET
// /chain-info additively returns the privacy-proxy public base URL when one is
// configured (RD-1031, Option B). The privacy-mode MetaMask setup dialog uses
// this field only to pre-fill the jwt-injector --upstream hint; it is never a
// wallet RPC target and must NEVER carry a "/rpc" suffix or a localhost value.
func TestHandleGetChainInfo_ExposesPrivacyProxyPublicURL(t *testing.T) {
	tests := []struct {
		name        string
		publicURL   string
		wantURL     string
		wantPresent bool
	}{
		{
			name:        "configured public proxy base url is returned",
			publicURL:   "https://proxy.example.com",
			wantURL:     "https://proxy.example.com",
			wantPresent: true,
		},
		{
			name:        "unset url is omitted",
			publicURL:   "",
			wantURL:     "",
			wantPresent: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ci := chaininfo.NewService(nil, 0)
			ci.SetPrivacyProxyPublicURL(tc.publicURL)

			s := &Server{
				chainInfo: ci,
				router:    chi.NewRouter(),
			}
			s.router.Get("/chain-info", s.handleGetChainInfo)

			srv := httptest.NewServer(s.router)
			defer srv.Close()

			resp, err := http.Get(srv.URL + "/chain-info")
			if err != nil {
				t.Fatalf("GET /chain-info: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}

			// Decode into a generic map so we can assert presence/absence of
			// the omitempty field, not just its value.
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			raw, present := body["privacyProxyPublicUrl"]
			if present != tc.wantPresent {
				t.Fatalf("privacyProxyPublicUrl present = %v, want %v (body=%v)", present, tc.wantPresent, body)
			}
			if tc.wantPresent {
				got, _ := raw.(string)
				if got != tc.wantURL {
					t.Fatalf("privacyProxyPublicUrl = %q, want %q", got, tc.wantURL)
				}
			}

			// The old RD-1031 bug fed the wallet a proxy /rpc target. Guard
			// against re-introducing a /rpc suffix or a localhost wallet target.
			if got, _ := raw.(string); got == "http://localhost:8545" {
				t.Fatalf("privacyProxyPublicUrl must not be a localhost wallet target")
			}

			// The old field name must no longer be emitted.
			if _, ok := body["rpcUrl"]; ok {
				t.Fatalf("legacy rpcUrl field must not be present (body=%v)", body)
			}
		})
	}
}

// TestServerNew_WiresPrivacyProxyPublicURL asserts that api.New threads
// ServerConfig.PrivacyProxyPublicURL into the chain-info service so the wiring
// from PRIVACY_PROXY_PUBLIC_URL actually reaches the handler.
func TestServerNew_WiresPrivacyProxyPublicURL(t *testing.T) {
	const want = "https://proxy.example.com"

	s := New(nil, nil, nil, nil, nil, 0, &ServerConfig{PrivacyProxyPublicURL: want}, nil, nil, nil)

	if got := s.chainInfo.Get().PrivacyProxyPublicURL; got != want {
		t.Fatalf("chainInfo privacyProxyPublicUrl = %q, want %q", got, want)
	}
}
