package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/internal/chaininfo"

	"github.com/go-chi/chi/v5"
)

// TestHandleGetChainInfo_ExposesBrowserRPCURL asserts that GET /chain-info
// additively returns the canonical browser-facing rpcUrl when one is
// configured (RD-1031). MetaMask sources its RPC URL from this field, so it
// must reflect the privacy-proxy public /rpc endpoint rather than a localhost
// fallback.
func TestHandleGetChainInfo_ExposesBrowserRPCURL(t *testing.T) {
	tests := []struct {
		name        string
		browserRPC  string
		wantRPCURL  string
		wantPresent bool
	}{
		{
			name:        "configured public proxy rpc url is returned",
			browserRPC:  "https://proxy.example.com/rpc",
			wantRPCURL:  "https://proxy.example.com/rpc",
			wantPresent: true,
		},
		{
			name:        "unset rpc url is omitted, not localhost",
			browserRPC:  "",
			wantRPCURL:  "",
			wantPresent: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ci := chaininfo.NewService(nil, 0)
			ci.SetBrowserRPCURL(tc.browserRPC)

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
			// the omitempty rpcUrl field, not just its value.
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			raw, present := body["rpcUrl"]
			if present != tc.wantPresent {
				t.Fatalf("rpcUrl present = %v, want %v (body=%v)", present, tc.wantPresent, body)
			}
			if tc.wantPresent {
				got, _ := raw.(string)
				if got != tc.wantRPCURL {
					t.Fatalf("rpcUrl = %q, want %q", got, tc.wantRPCURL)
				}
			}

			// Guard against the regression that caused RD-1031: the handler
			// must never emit the old unreachable localhost fallback.
			if got, _ := raw.(string); got == "http://localhost:8545" {
				t.Fatalf("rpcUrl must not be the localhost fallback that broke MetaMask")
			}
		})
	}
}

// TestServerNew_WiresBrowserRPCURL asserts that api.New threads
// ServerConfig.BrowserRPCURL into the chain-info service so the wiring from
// PRIVACY_PROXY_PUBLIC_URL actually reaches the handler.
func TestServerNew_WiresBrowserRPCURL(t *testing.T) {
	const want = "https://proxy.example.com/rpc"

	s := New(nil, nil, nil, nil, nil, 0, &ServerConfig{BrowserRPCURL: want}, nil, nil, nil)

	if got := s.chainInfo.Get().RPCURL; got != want {
		t.Fatalf("chainInfo rpcUrl = %q, want %q", got, want)
	}
}
