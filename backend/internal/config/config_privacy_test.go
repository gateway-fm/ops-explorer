package config

import (
	"strings"
	"testing"
)

// W-1: privacy mode is FAIL-CLOSED on CORS — CORS_ALLOWED_ORIGINS is REQUIRED.
// With PRIVACY_PROXY_URL set and no allowlist, config.Load must return an error
// at startup rather than fall back to reflect-any-Origin + credentials
// (PROD_READINESS_AUDIT §W-1). Standalone with an empty allowlist must still
// load (permissive + warn, preserving current behavior).
func TestCORSAllowlistRequiredInPrivacyMode(t *testing.T) {
	t.Run("privacy mode + empty allowlist -> Load error (fail-closed)", func(t *testing.T) {
		t.Setenv("PRIVACY_PROXY_URL", "https://proxy.example.com")
		t.Setenv("INDEXER_URL", "")
		t.Setenv("CORS_ALLOWED_ORIGINS", "")

		_, err := Load()
		if err == nil {
			t.Fatal("expected Load to fail-closed when privacy mode has no CORS_ALLOWED_ORIGINS, got nil error")
		}
		if !strings.Contains(strings.ToUpper(err.Error()), "CORS_ALLOWED_ORIGINS") {
			t.Errorf("error should name CORS_ALLOWED_ORIGINS, got: %v", err)
		}
	})

	t.Run("privacy mode + allowlist set -> loads", func(t *testing.T) {
		t.Setenv("PRIVACY_PROXY_URL", "https://proxy.example.com")
		t.Setenv("INDEXER_URL", "")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load should succeed with a non-empty allowlist in privacy mode: %v", err)
		}
		if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "https://app.example.com" {
			t.Errorf("CORSAllowedOrigins = %#v, want [https://app.example.com]", cfg.CORSAllowedOrigins)
		}
	})

	t.Run("standalone + empty allowlist -> loads (permissive)", func(t *testing.T) {
		t.Setenv("PRIVACY_PROXY_URL", "")
		t.Setenv("INDEXER_URL", "indexer:50051")
		t.Setenv("CORS_ALLOWED_ORIGINS", "")

		if _, err := Load(); err != nil {
			t.Fatalf("standalone with empty allowlist must still load, got: %v", err)
		}
	})

	t.Run("allowlist parses comma-separated and trims spaces", func(t *testing.T) {
		t.Setenv("PRIVACY_PROXY_URL", "")
		t.Setenv("INDEXER_URL", "indexer:50051")
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.example.com, https://b.example.com ,https://c.example.com")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		want := []string{"https://a.example.com", "https://b.example.com", "https://c.example.com"}
		if len(cfg.CORSAllowedOrigins) != len(want) {
			t.Fatalf("CORSAllowedOrigins = %#v, want %#v", cfg.CORSAllowedOrigins, want)
		}
		for i := range want {
			if cfg.CORSAllowedOrigins[i] != want[i] {
				t.Errorf("CORSAllowedOrigins[%d] = %q, want %q", i, cfg.CORSAllowedOrigins[i], want[i])
			}
		}
	})
}

// P-4: CoinGecko egress must be off by default in privacy mode (a
// confidential/air-gapped deployment must not make unexpected third-party
// requests), but on by default in standalone. The coin id / currency must be
// configurable (PROD_READINESS_AUDIT §P-4).
func TestEnablePriceDefault_PrivacyVsStandalone(t *testing.T) {
	t.Run("privacy mode defaults price OFF", func(t *testing.T) {
		// Only PRIVACY_PROXY_URL set; ENABLE_PRICE left unset.
		t.Setenv("PRIVACY_PROXY_URL", "https://proxy.example.com")
		t.Setenv("INDEXER_URL", "")
		// W-1: privacy mode now requires CORS_ALLOWED_ORIGINS to Load.
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.EnablePrice {
			t.Error("EnablePrice must default to FALSE in privacy mode (PRIVACY_PROXY_URL set)")
		}
	})

	t.Run("standalone mode defaults price ON", func(t *testing.T) {
		t.Setenv("PRIVACY_PROXY_URL", "")
		t.Setenv("INDEXER_URL", "indexer:50051")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !cfg.EnablePrice {
			t.Error("EnablePrice must default to TRUE in standalone mode")
		}
	})

	t.Run("explicit ENABLE_PRICE=true overrides the privacy default", func(t *testing.T) {
		t.Setenv("PRIVACY_PROXY_URL", "https://proxy.example.com")
		t.Setenv("ENABLE_PRICE", "true")
		// W-1: privacy mode now requires CORS_ALLOWED_ORIGINS to Load.
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !cfg.EnablePrice {
			t.Error("explicit ENABLE_PRICE=true must win over the privacy-mode default")
		}
	})

	t.Run("explicit ENABLE_PRICE=false overrides the standalone default", func(t *testing.T) {
		t.Setenv("PRIVACY_PROXY_URL", "")
		t.Setenv("INDEXER_URL", "indexer:50051")
		t.Setenv("ENABLE_PRICE", "false")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.EnablePrice {
			t.Error("explicit ENABLE_PRICE=false must win over the standalone default")
		}
	})
}

func TestPriceCoinAndCurrencyDefaults(t *testing.T) {
	t.Setenv("PRIVACY_PROXY_URL", "")
	t.Setenv("INDEXER_URL", "indexer:50051")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PriceCoinID != "ethereum" {
		t.Errorf("PriceCoinID default = %q, want %q", cfg.PriceCoinID, "ethereum")
	}
	if cfg.PriceCurrency != "usd" {
		t.Errorf("PriceCurrency default = %q, want %q", cfg.PriceCurrency, "usd")
	}
}

func TestPriceCoinIsConfigurable(t *testing.T) {
	t.Setenv("PRIVACY_PROXY_URL", "")
	t.Setenv("INDEXER_URL", "indexer:50051")
	t.Setenv("PRICE_COIN_ID", "matic-network")
	t.Setenv("PRICE_CURRENCY", "eur")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PriceCoinID != "matic-network" {
		t.Errorf("PriceCoinID = %q, want %q", cfg.PriceCoinID, "matic-network")
	}
	if cfg.PriceCurrency != "eur" {
		t.Errorf("PriceCurrency = %q, want %q", cfg.PriceCurrency, "eur")
	}
}
