package config

import "testing"

// P-4: CoinGecko egress must be off by default in privacy mode (a
// confidential/air-gapped deployment must not make unexpected third-party
// requests), but on by default in standalone. The coin id / currency must be
// configurable (PROD_READINESS_AUDIT §P-4).
func TestEnablePriceDefault_PrivacyVsStandalone(t *testing.T) {
	t.Run("privacy mode defaults price OFF", func(t *testing.T) {
		// Only PRIVACY_PROXY_URL set; ENABLE_PRICE left unset.
		t.Setenv("PRIVACY_PROXY_URL", "https://proxy.example.com")
		t.Setenv("INDEXER_URL", "")

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
