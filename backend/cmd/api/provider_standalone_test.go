//go:build !privacy

package main

// TDD (red->green) for the standalone build's mode-selection wiring (audit T-2).
//
// New behavior I am defining here: a PURE selectMode(cfg) (string, error) that
// encodes the documented mutual-exclusion contract from provider_standalone.go:
//   - INDEXER_URL and PRIVACY_PROXY_URL are mutually exclusive chain-data modes.
//   - both set      -> error (privacy footgun: chain data would bypass redaction)
//   - PRIVACY only   -> "privacy"
//   - INDEXER only   -> "standalone"
//   - neither set    -> error (no chain-data source)
// selectMode returns an error INSTEAD of log.Fatal so it is unit-testable;
// chooseProvider calls it and log.Fatals on error, leaving startup unchanged.
//
// Also characterizes wrapWithCache against its contract
// (provider_standalone.go:67-74): ENABLE_PROVIDER_CACHE=false returns the inner
// provider unchanged; true returns a *cache.CachingProvider wrapping it.
//
// Per the audit correction, there is NO privacy-build behavioral test here:
// the privacy variant's chooseProvider log.Fatals (-> os.Exit) and does not use
// selectMode, so it is covered by the compile/vet leg in CI (A9), not a unit
// test. Helpers use the `tc` prefix per the coexistence contract.

import (
	"testing"

	"explorer/internal/api"
	"explorer/internal/api/cache"
	"explorer/internal/config"
)

func TestSelectMode(t *testing.T) {
	cases := []struct {
		name      string
		privacy   string
		indexer   string
		wantMode  string
		wantError bool
	}{
		{
			name:      "both set is rejected (privacy footgun)",
			privacy:   "http://proxy:8081",
			indexer:   "indexer:50051",
			wantError: true,
		},
		{
			name:     "privacy only -> privacy mode",
			privacy:  "http://proxy:8081",
			wantMode: modePrivacy,
		},
		{
			name:     "indexer only -> standalone mode",
			indexer:  "indexer:50051",
			wantMode: modeStandalone,
		},
		{
			name:      "neither set is rejected (no chain-data source)",
			wantError: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				PrivacyProxyURL: tt.privacy,
				IndexerURL:      tt.indexer,
			}
			mode, err := selectMode(cfg)
			if tt.wantError {
				if err == nil {
					t.Fatalf("selectMode(privacy=%q,indexer=%q) = (%q, nil), want an error",
						tt.privacy, tt.indexer, mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectMode(privacy=%q,indexer=%q) returned error %v, want mode %q",
					tt.privacy, tt.indexer, err, tt.wantMode)
			}
			if mode != tt.wantMode {
				t.Fatalf("selectMode = %q, want %q", mode, tt.wantMode)
			}
		})
	}
}

// tcNoopProvider is a no-op api.DataProvider used purely as the inner provider
// for wrapWithCache. Methods are never invoked by wrapWithCache itself.
type tcNoopProvider struct {
	api.DataProvider // nil embed; only identity matters here
}

func TestWrapWithCacheDisabled(t *testing.T) {
	// Contract: ENABLE_PROVIDER_CACHE=false -> inner returned unchanged.
	inner := &tcNoopProvider{}
	cfg := &config.Config{EnableProviderCache: false}
	got := wrapWithCache(inner, cfg)
	if got != api.DataProvider(inner) {
		t.Fatalf("wrapWithCache(cache=false) = %T (%p), want the inner provider unchanged (%p)", got, got, inner)
	}
}

func TestWrapWithCacheEnabled(t *testing.T) {
	// Contract: ENABLE_PROVIDER_CACHE=true -> a *cache.CachingProvider wrapping
	// the inner provider.
	inner := &tcNoopProvider{}
	cfg := &config.Config{EnableProviderCache: true}
	got := wrapWithCache(inner, cfg)
	if _, ok := got.(*cache.CachingProvider); !ok {
		t.Fatalf("wrapWithCache(cache=true) = %T, want *cache.CachingProvider", got)
	}
}
