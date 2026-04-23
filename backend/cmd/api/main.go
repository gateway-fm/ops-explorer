package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"explorer/internal/api"
	"explorer/internal/api/indexerclient"
	"explorer/internal/auth"
	"explorer/internal/config"
	"explorer/internal/db"
	"explorer/internal/events"
	"explorer/internal/log"
	"explorer/internal/price"
	"explorer/internal/privacy"
	"explorer/internal/rpc"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config", "error", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", "error", err)
	}
	defer database.Close()

	// Parse hidden transaction types — prefer env var, fall back to config default
	hiddenTxTypesRaw := cfg.HiddenTxTypes
	if v := os.Getenv("HIDDEN_TX_TYPES"); v != "" {
		hiddenTxTypesRaw = v
	}
	if hiddenTxTypesRaw == "" {
		hiddenTxTypesRaw = "126" // Default: hide OP deposit system transactions
	}
	if hiddenTxTypesRaw != "" {
		for _, s := range strings.Split(hiddenTxTypesRaw, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			n, err := strconv.Atoi(s)
			if err != nil {
				log.Warn("invalid hidden_tx_types value, skipping", "value", s, "error", err)
				continue
			}
			database.HiddenTxTypes = append(database.HiddenTxTypes, n)
		}
		if len(database.HiddenTxTypes) > 0 {
			log.Info("hiding transaction types from listings", "types", database.HiddenTxTypes)
		}
	}

	if err := database.Migrate(); err != nil {
		log.Fatal("failed to run migrations", "error", err)
	}

	rpcClient, err := rpc.New(cfg.RPCURL)
	if err != nil {
		log.Fatal("failed to create rpc client", "error", err)
	}

	eventBus := events.NewBus()
	defer eventBus.Close()

	priceService := price.NewService("ethereum", "usd", 60*time.Second)
	priceService.SetEventBus(eventBus)

	serverCfg := &api.ServerConfig{
		SolcPath:            cfg.SolcPath,
		UseSourcifyFallback: cfg.UseSourcifyFallback,
		MetricsEnabled:      cfg.MetricsEnabled,
	}

	var privacyClient *privacy.Client
	var ssoClient *auth.SSOClient
	var dataProvider api.DataProvider

	// Block-explorer no longer runs its own indexer (RD-855 Phase 6).
	// A chain-data source — either chain-indexer over gRPC (INDEXER_URL)
	// or privacy-proxy's explorer REST surface (PRIVACY_PROXY_URL) — is
	// required. Fail fast with a clear message if neither is set.
	switch {
	case cfg.PrivacyProxyURL != "" && cfg.IndexerURL == "":
		// Classic proxy mode: block-explorer gets chain data by calling
		// privacy-proxy's REST API (privacy-proxy applies redaction).
		privacyClient = privacy.NewClient(cfg.PrivacyProxyURL)
		ssoClient = auth.NewSSOClient(cfg.PrivacyProxyURL, cfg.PrivacyProxyPublicURL, cfg.SSOClientID, cfg.SSORedirectURI)
		dataProvider = api.NewProxyDataProvider(cfg.PrivacyProxyURL)
		log.Info("proxy mode enabled", "proxy_url", cfg.PrivacyProxyURL)

	case cfg.IndexerURL != "":
		// Standalone + chain-indexer: reads go to the indexer over gRPC.
		// The embedded DirectDBProvider now serves only block-explorer's
		// own state (ABI/verification), no chain data.
		if cfg.PrivacyProxyURL != "" {
			privacyClient = privacy.NewClient(cfg.PrivacyProxyURL)
			ssoClient = auth.NewSSOClient(cfg.PrivacyProxyURL, cfg.PrivacyProxyPublicURL, cfg.SSOClientID, cfg.SSORedirectURI)
		}
		fallback := api.NewDirectDBProvider(database, rpcClient, nil)
		ip, err := indexerclient.New(indexerclient.Config{IndexerURL: cfg.IndexerURL}, fallback)
		if err != nil {
			log.Fatal("failed to construct indexerclient provider", "error", err)
		}
		dataProvider = ip
		log.Info("indexer-backed standalone mode", "indexer_url", cfg.IndexerURL)

	default:
		log.Fatal("neither INDEXER_URL nor PRIVACY_PROXY_URL is set — block-explorer needs a chain-data source. Point INDEXER_URL at a chain-indexer gRPC endpoint (standalone mode) or PRIVACY_PROXY_URL at a privacy-proxy instance (privacy mode).")
	}

	server := api.New(database, rpcClient, nil, priceService, eventBus, cfg.APIPort, serverCfg, privacyClient, ssoClient, dataProvider)

	ctx, cancel := context.WithCancel(context.Background())
	priceService.StartBackgroundRefresh(ctx)
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutting down API server")
		cancel()
	}()

	log.Info("starting API server", "port", cfg.APIPort, "mode", "api-only")
	if err := server.Start(ctx); err != nil {
		log.Fatal("server error", "error", err)
	}
}
