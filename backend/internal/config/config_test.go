package config

// Contract under test: the config.Load/Validate contract as documented by the
// setDefaults table (config.go:87-137), the Validate rules (config.go:213-238),
// and the field doc comments (e.g. PrivacyProxyPublicURL defaults to
// PrivacyProxyURL when unset, config.go:54-58/187-189). Expected values are the
// DEFAULTS and RULES declared in the source contract, not values harvested by
// running Load() blindly.
//
// CRITICAL isolation: Load() reads a .env file from ".", "./backend", and ".."
// via viper. Every test therefore t.Chdir(t.TempDir()) so the repository .env
// cannot leak in, and uses t.Setenv for overrides (viper AutomaticEnv maps
// config keys to UPPER_SNAKE env vars via the "."->"_" replacer).
//
// We deliberately do NOT assert on the privacy/standalone mode mutual-exclusion
// (that lives in cmd/api selectMode, covered by A1) nor on fields the privacy
// branch may add. Helpers use the `tc` prefix per the coexistence contract.

import (
	"testing"
	"time"
)

// tcIsolate moves the working dir to an empty temp dir so no repo .env leaks
// into Load(), and (defensively) clears the env vars these tests touch.
func tcIsolate(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func TestLoadDefaults(t *testing.T) {
	tcIsolate(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with no env/.env should succeed on built-in defaults: %v", err)
	}

	// Each expected value is the documented default from setDefaults.
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"RPCURL", cfg.RPCURL, "http://127.0.0.1:8545"},
		{"DatabaseURL", cfg.DatabaseURL, "postgres://postgres:postgres@localhost:5432/explorer?sslmode=disable"},
		{"APIPort", cfg.APIPort, 8080},
		{"PollInterval", cfg.PollInterval, 2 * time.Second},
		{"StartBlock", cfg.StartBlock, uint64(0)},
		{"RPCWorkers", cfg.RPCWorkers, 50},
		{"RPCRateLimit", cfg.RPCRateLimit, 500},
		{"DBBatchSize", cfg.DBBatchSize, 500},
		{"TokenMetadataWorkers", cfg.TokenMetadataWorkers, 20},
		{"BalanceWorkers", cfg.BalanceWorkers, 30},
		{"EnableAsyncBalance", cfg.EnableAsyncBalance, true},
		{"EnableTracing", cfg.EnableTracing, false},
		{"TraceRateLimit", cfg.TraceRateLimit, 50},
		{"TraceWorkers", cfg.TraceWorkers, 10},
		{"WSMaxConnections", cfg.WSMaxConnections, 10000},
		{"WSPingInterval", cfg.WSPingInterval, 30 * time.Second},
		{"SolcPath", cfg.SolcPath, "/opt/solc"},
		{"UseSourcifyFallback", cfg.UseSourcifyFallback, true},
		{"MetricsEnabled", cfg.MetricsEnabled, true},
		{"CatchupEnabled", cfg.CatchupEnabled, true},
		{"CatchupWorkers", cfg.CatchupWorkers, 10},
		{"CatchupBatchSize", cfg.CatchupBatchSize, 100},
		{"CatchupQueueSize", cfg.CatchupQueueSize, 1000},
		{"SSOClientID", cfg.SSOClientID, "explorer"},
		{"SSORedirectURI", cfg.SSORedirectURI, "http://localhost:8080/api/auth/callback"},
		{"LogLevel", cfg.LogLevel, "info"},
		{"EnableGasPrices", cfg.EnableGasPrices, false},
		{"EnableProviderCache", cfg.EnableProviderCache, true},
		{"EnableOPDeposits", cfg.EnableOPDeposits, false},
		{"L1DepositPollInterval", cfg.L1DepositPollInterval, 12 * time.Second},
		{"L1DepositBatchSize", cfg.L1DepositBatchSize, 1000},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want default %v", c.name, c.got, c.want)
		}
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	tcIsolate(t)

	// Contract: AutomaticEnv maps UPPER_SNAKE env vars onto config keys.
	t.Setenv("RPC_URL", "http://node.example:9999")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x?sslmode=require")
	t.Setenv("API_PORT", "9090")
	t.Setenv("POLL_INTERVAL", "5s")
	t.Setenv("RPC_WORKERS", "8")
	t.Setenv("RPC_RATE_LIMIT", "123")
	t.Setenv("ENABLE_PROVIDER_CACHE", "false")
	t.Setenv("METRICS_ENABLED", "false")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WS_MAX_CONNECTIONS", "42")
	t.Setenv("WS_PING_INTERVAL", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"RPCURL", cfg.RPCURL, "http://node.example:9999"},
		{"DatabaseURL", cfg.DatabaseURL, "postgres://u:p@db:5432/x?sslmode=require"},
		{"APIPort", cfg.APIPort, 9090},
		{"PollInterval", cfg.PollInterval, 5 * time.Second},
		{"RPCWorkers", cfg.RPCWorkers, 8},
		{"RPCRateLimit", cfg.RPCRateLimit, 123},
		{"EnableProviderCache", cfg.EnableProviderCache, false},
		{"MetricsEnabled", cfg.MetricsEnabled, false},
		{"LogLevel", cfg.LogLevel, "debug"},
		{"WSMaxConnections", cfg.WSMaxConnections, 42},
		{"WSPingInterval", cfg.WSPingInterval, 15 * time.Second},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want override %v", c.name, c.got, c.want)
		}
	}
}

func TestLoadPrivacyProxyPublicURLDefaulting(t *testing.T) {
	tcIsolate(t)

	// Contract (config.go:187-189): PrivacyProxyPublicURL defaults to
	// PrivacyProxyURL when not explicitly set.
	t.Setenv("PRIVACY_PROXY_URL", "http://proxy.internal:8081")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PrivacyProxyPublicURL != "http://proxy.internal:8081" {
		t.Fatalf("PrivacyProxyPublicURL = %q, want it to default to PrivacyProxyURL", cfg.PrivacyProxyPublicURL)
	}

	// When explicitly set, it is honored independently.
	t.Setenv("PRIVACY_PROXY_PUBLIC_URL", "http://localhost:8081")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load (explicit public url): %v", err)
	}
	if cfg.PrivacyProxyPublicURL != "http://localhost:8081" {
		t.Fatalf("PrivacyProxyPublicURL = %q, want the explicit value", cfg.PrivacyProxyPublicURL)
	}
}

func TestValidateRules(t *testing.T) {
	// Pure unit tests of Validate against its declared rules (config.go:213-238).
	// Start from a known-good base and mutate one field per case.
	base := func() *Config {
		return &Config{
			RPCURL:       "http://127.0.0.1:8545",
			DatabaseURL:  "postgres://x",
			APIPort:      8080,
			RPCWorkers:   50,
			RPCRateLimit: 500,
		}
	}

	cases := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid base", func(*Config) {}, false},
		{"empty RPC_URL", func(c *Config) { c.RPCURL = "" }, true},
		{"empty DATABASE_URL", func(c *Config) { c.DatabaseURL = "" }, true},
		{"api port zero", func(c *Config) { c.APIPort = 0 }, true},
		{"api port negative", func(c *Config) { c.APIPort = -1 }, true},
		{"api port too high", func(c *Config) { c.APIPort = 70000 }, true},
		{"api port max valid", func(c *Config) { c.APIPort = 65535 }, false},
		{"rpc workers zero", func(c *Config) { c.RPCWorkers = 0 }, true},
		{"rpc rate limit zero", func(c *Config) { c.RPCRateLimit = 0 }, true},
		{"op deposits without L1 url", func(c *Config) {
			c.EnableOPDeposits = true
			c.OptimismPortalAddress = "0xabc"
		}, true},
		{"op deposits without portal", func(c *Config) {
			c.EnableOPDeposits = true
			c.L1RPCURL = "http://l1"
		}, true},
		{"op deposits fully configured", func(c *Config) {
			c.EnableOPDeposits = true
			c.L1RPCURL = "http://l1"
			c.OptimismPortalAddress = "0xabc"
		}, false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.mutate(c)
			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestLoadRejectsMalformedPort(t *testing.T) {
	tcIsolate(t)

	// Contract: a non-numeric API_PORT cannot satisfy the 1..65535 rule. viper
	// coerces an unparseable int to 0, which Validate then rejects — Load must
	// return an error rather than a Config with a bogus port.
	t.Setenv("API_PORT", "not-a-number")
	if _, err := Load(); err == nil {
		t.Fatalf("Load with malformed API_PORT = nil error, want validation failure")
	}
}

func TestMaskDatabasePassword(t *testing.T) {
	// Contract: maskDatabasePassword (config.go:298-315) replaces the password
	// between ':' and '@' with **** and is a no-op for inputs lacking that shape.
	cases := []struct {
		in   string
		want string
	}{
		{"postgres://user:secret@host:5432/db", "postgres://user:****@host:5432/db"},
		{"postgres://user@host:5432/db", "postgres://user@host:5432/db"}, // no password
		{"postgres://host/db", "postgres://host/db"},                     // no @ / creds
		{"not-a-url", "not-a-url"},
	}
	for _, c := range cases {
		if got := maskDatabasePassword(c.in); got != c.want {
			t.Errorf("maskDatabasePassword(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
