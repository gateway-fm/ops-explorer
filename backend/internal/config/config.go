package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the explorer backend
type Config struct {
	// Core settings
	RPCURL       string        `mapstructure:"rpc_url"`
	DatabaseURL  string        `mapstructure:"database_url"`
	APIPort      int           `mapstructure:"api_port"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
	StartBlock   uint64        `mapstructure:"start_block"`

	// Parallelization settings
	RPCWorkers           int  `mapstructure:"rpc_workers"`            // Number of concurrent RPC workers
	RPCRateLimit         int  `mapstructure:"rpc_rate_limit"`         // Max RPC requests per second
	DBBatchSize          int  `mapstructure:"db_batch_size"`          // Max items per DB batch insert
	TokenMetadataWorkers int  `mapstructure:"token_metadata_workers"` // Workers for token metadata fetching
	BalanceWorkers       int  `mapstructure:"balance_workers"`        // Workers for async balance tracking
	EnableAsyncBalance   bool `mapstructure:"enable_async_balance"`   // Enable async balance tracking

	// Tracing settings
	EnableTracing  bool `mapstructure:"enable_tracing"`   // Enable internal transaction tracing
	TraceRateLimit int  `mapstructure:"trace_rate_limit"` // Max trace RPC calls per second
	TraceWorkers   int  `mapstructure:"trace_workers"`    // Parallel workers for tracing

	// WebSocket settings
	WSMaxConnections int           `mapstructure:"ws_max_connections"` // Max WebSocket connections
	WSPingInterval   time.Duration `mapstructure:"ws_ping_interval"`   // WebSocket ping interval

	// Verification settings
	SolcPath            string `mapstructure:"solc_path"`             // Path to solc binaries
	UseSourcifyFallback bool   `mapstructure:"use_sourcify_fallback"` // Try Sourcify first for public chains

	// Metrics settings
	MetricsEnabled bool `mapstructure:"metrics_enabled"` // Enable Prometheus metrics

	// Catchup mode settings
	CatchupEnabled   bool `mapstructure:"catchup_enabled"`    // Enable catchup mode for faster historical indexing
	CatchupWorkers   int  `mapstructure:"catchup_workers"`    // Number of parallel block processing workers
	CatchupBatchSize int  `mapstructure:"catchup_batch_size"` // Number of blocks to process in each batch
	CatchupQueueSize int  `mapstructure:"catchup_queue_size"` // Size of the catchup work queue

	// Privacy settings (opt-in: PrivacyEnabled must be true AND PrivacyProxyURL must be set)
	PrivacyEnabled  bool   `mapstructure:"privacy_enabled"`   // Master toggle for all privacy features
	PrivacyProxyURL string `mapstructure:"privacy_proxy_url"` // URL of the privacy-proxy service
	SSOClientID     string `mapstructure:"sso_client_id"`     // OAuth client ID for SSO
	SSORedirectURI  string `mapstructure:"sso_redirect_uri"`  // OAuth redirect URI for SSO callback

	// OP Stack deposit transaction settings
	EnableOPDeposits      bool          `mapstructure:"enable_op_deposits"`       // Enable OP Stack deposit tx support
	L1RPCURL              string        `mapstructure:"l1_rpc_url"`              // L1 RPC URL for deposit event fetching
	OptimismPortalAddress string        `mapstructure:"optimism_portal_address"` // OptimismPortal contract address on L1
	L1DepositPollInterval time.Duration `mapstructure:"l1_deposit_poll_interval"` // L1 polling interval
	L1DepositBatchSize    int           `mapstructure:"l1_deposit_batch_size"`    // L1 log fetch batch size
	L1DepositStartBlock   uint64        `mapstructure:"l1_deposit_start_block"`   // L1 start block (0 = auto-detect)
}

// setDefaults sets all default values for configuration
func setDefaults(v *viper.Viper) {
	// Core settings
	v.SetDefault("rpc_url", "http://127.0.0.1:8545")
	v.SetDefault("database_url", "postgres://postgres:postgres@localhost:5432/explorer?sslmode=disable")
	v.SetDefault("api_port", 8080)
	v.SetDefault("poll_interval", "2s")
	v.SetDefault("start_block", 0)

	// Parallelization settings
	v.SetDefault("rpc_workers", 50)
	v.SetDefault("rpc_rate_limit", 500)
	v.SetDefault("db_batch_size", 500)
	v.SetDefault("token_metadata_workers", 20)
	v.SetDefault("balance_workers", 30)
	v.SetDefault("enable_async_balance", true)

	// Tracing settings
	v.SetDefault("enable_tracing", false)
	v.SetDefault("trace_rate_limit", 50)
	v.SetDefault("trace_workers", 10)

	// WebSocket settings
	v.SetDefault("ws_max_connections", 10000)
	v.SetDefault("ws_ping_interval", "30s")

	// Verification settings
	v.SetDefault("solc_path", "/opt/solc")
	v.SetDefault("use_sourcify_fallback", true)

	// Metrics settings
	v.SetDefault("metrics_enabled", true)

	// Catchup mode settings
	v.SetDefault("catchup_enabled", true)
	v.SetDefault("catchup_workers", 10)
	v.SetDefault("catchup_batch_size", 100)
	v.SetDefault("catchup_queue_size", 1000)

	// Privacy settings (disabled by default)
	v.SetDefault("privacy_enabled", false)
	v.SetDefault("privacy_proxy_url", "")
	v.SetDefault("sso_client_id", "explorer")
	v.SetDefault("sso_redirect_uri", "http://localhost:8080/api/auth/callback")

	// OP Stack deposit settings (disabled by default)
	v.SetDefault("enable_op_deposits", false)
	v.SetDefault("l1_rpc_url", "")
	v.SetDefault("optimism_portal_address", "")
	v.SetDefault("l1_deposit_poll_interval", "12s")
	v.SetDefault("l1_deposit_batch_size", 1000)
	v.SetDefault("l1_deposit_start_block", 0)
}

// Load loads configuration from environment variables with defaults
// Priority: Environment variables > .env file > defaults
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults first
	setDefaults(v)

	// Configure environment variable reading
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Try to read from .env file (optional)
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./backend")
	v.AddConfigPath("..")

	// Read config file if it exists (ignore error if not found)
	_ = v.ReadInConfig()

	// Create config struct
	cfg := &Config{}

	// Manually bind each config value to handle type conversions properly
	cfg.RPCURL = v.GetString("rpc_url")
	cfg.DatabaseURL = v.GetString("database_url")
	cfg.APIPort = v.GetInt("api_port")
	cfg.PollInterval = v.GetDuration("poll_interval")
	cfg.StartBlock = v.GetUint64("start_block")

	// Parallelization settings
	cfg.RPCWorkers = v.GetInt("rpc_workers")
	cfg.RPCRateLimit = v.GetInt("rpc_rate_limit")
	cfg.DBBatchSize = v.GetInt("db_batch_size")
	cfg.TokenMetadataWorkers = v.GetInt("token_metadata_workers")
	cfg.BalanceWorkers = v.GetInt("balance_workers")
	cfg.EnableAsyncBalance = v.GetBool("enable_async_balance")

	// Tracing settings
	cfg.EnableTracing = v.GetBool("enable_tracing")
	cfg.TraceRateLimit = v.GetInt("trace_rate_limit")
	cfg.TraceWorkers = v.GetInt("trace_workers")

	// WebSocket settings
	cfg.WSMaxConnections = v.GetInt("ws_max_connections")
	cfg.WSPingInterval = v.GetDuration("ws_ping_interval")

	// Verification settings
	cfg.SolcPath = v.GetString("solc_path")
	cfg.UseSourcifyFallback = v.GetBool("use_sourcify_fallback")

	// Metrics settings
	cfg.MetricsEnabled = v.GetBool("metrics_enabled")

	// Catchup mode settings
	cfg.CatchupEnabled = v.GetBool("catchup_enabled")
	cfg.CatchupWorkers = v.GetInt("catchup_workers")
	cfg.CatchupBatchSize = v.GetInt("catchup_batch_size")
	cfg.CatchupQueueSize = v.GetInt("catchup_queue_size")

	// Privacy settings
	cfg.PrivacyEnabled = v.GetBool("privacy_enabled")
	cfg.PrivacyProxyURL = v.GetString("privacy_proxy_url")
	cfg.SSOClientID = v.GetString("sso_client_id")
	cfg.SSORedirectURI = v.GetString("sso_redirect_uri")

	// OP Stack deposit settings
	cfg.EnableOPDeposits = v.GetBool("enable_op_deposits")
	cfg.L1RPCURL = v.GetString("l1_rpc_url")
	cfg.OptimismPortalAddress = v.GetString("optimism_portal_address")
	cfg.L1DepositPollInterval = v.GetDuration("l1_deposit_poll_interval")
	cfg.L1DepositBatchSize = v.GetInt("l1_deposit_batch_size")
	cfg.L1DepositStartBlock = v.GetUint64("l1_deposit_start_block")

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.RPCURL == "" {
		return fmt.Errorf("RPC_URL is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.APIPort <= 0 || c.APIPort > 65535 {
		return fmt.Errorf("API_PORT must be between 1 and 65535")
	}
	if c.RPCWorkers <= 0 {
		return fmt.Errorf("RPC_WORKERS must be greater than 0")
	}
	if c.RPCRateLimit <= 0 {
		return fmt.Errorf("RPC_RATE_LIMIT must be greater than 0")
	}
	if c.EnableOPDeposits {
		if c.L1RPCURL == "" {
			return fmt.Errorf("L1_RPC_URL is required when ENABLE_OP_DEPOSITS is true")
		}
		if c.OptimismPortalAddress == "" {
			return fmt.Errorf("OPTIMISM_PORTAL_ADDRESS is required when ENABLE_OP_DEPOSITS is true")
		}
	}
	return nil
}

// String returns a string representation of the config (for logging)
// Sensitive values are masked
func (c *Config) String() string {
	// Mask database URL password
	maskedDBURL := maskDatabasePassword(c.DatabaseURL)

	return fmt.Sprintf(`Config{
  Core:
    RPC_URL: %s
    DATABASE_URL: %s
    API_PORT: %d
    POLL_INTERVAL: %s
    START_BLOCK: %d
  Parallelization:
    RPC_WORKERS: %d
    RPC_RATE_LIMIT: %d
    DB_BATCH_SIZE: %d
    TOKEN_METADATA_WORKERS: %d
    BALANCE_WORKERS: %d
    ENABLE_ASYNC_BALANCE: %t
  Tracing:
    ENABLE_TRACING: %t
    TRACE_RATE_LIMIT: %d
    TRACE_WORKERS: %d
  WebSocket:
    WS_MAX_CONNECTIONS: %d
    WS_PING_INTERVAL: %s
  Verification:
    SOLC_PATH: %s
    USE_SOURCIFY_FALLBACK: %t
  Metrics:
    METRICS_ENABLED: %t
  Catchup:
    CATCHUP_ENABLED: %t
    CATCHUP_WORKERS: %d
    CATCHUP_BATCH_SIZE: %d
    CATCHUP_QUEUE_SIZE: %d
  Privacy:
    PRIVACY_ENABLED: %t
    PRIVACY_PROXY_URL: %s
    SSO_CLIENT_ID: %s
    SSO_REDIRECT_URI: %s
  OP Stack:
    ENABLE_OP_DEPOSITS: %t
    L1_RPC_URL: %s
    OPTIMISM_PORTAL_ADDRESS: %s
    L1_DEPOSIT_POLL_INTERVAL: %s
    L1_DEPOSIT_BATCH_SIZE: %d
    L1_DEPOSIT_START_BLOCK: %d
}`,
		c.RPCURL, maskedDBURL, c.APIPort, c.PollInterval, c.StartBlock,
		c.RPCWorkers, c.RPCRateLimit, c.DBBatchSize, c.TokenMetadataWorkers, c.BalanceWorkers, c.EnableAsyncBalance,
		c.EnableTracing, c.TraceRateLimit, c.TraceWorkers,
		c.WSMaxConnections, c.WSPingInterval,
		c.SolcPath, c.UseSourcifyFallback,
		c.MetricsEnabled,
		c.CatchupEnabled, c.CatchupWorkers, c.CatchupBatchSize, c.CatchupQueueSize,
		c.PrivacyEnabled, c.PrivacyProxyURL, c.SSOClientID, c.SSORedirectURI,
		c.EnableOPDeposits, c.L1RPCURL, c.OptimismPortalAddress, c.L1DepositPollInterval, c.L1DepositBatchSize, c.L1DepositStartBlock,
	)
}

// maskDatabasePassword masks the password in a database URL
func maskDatabasePassword(url string) string {
	// Find password between :// and @
	start := strings.Index(url, "://")
	if start == -1 {
		return url
	}
	atSign := strings.Index(url[start:], "@")
	if atSign == -1 {
		return url
	}

	// Find the colon after the username
	userPassPart := url[start+3 : start+atSign]
	colonPos := strings.Index(userPassPart, ":")
	if colonPos == -1 {
		return url
	}

	// Mask the password
	return url[:start+3+colonPos+1] + "****" + url[start+atSign:]
}
