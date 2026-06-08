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
	"explorer/internal/config"
	"explorer/internal/db"
	"explorer/internal/events"
	"explorer/internal/price"
	"explorer/internal/rpc"
	"explorer/pkg/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config", "error", err)
	}

	// P-3: in privacy mode, mask on-chain identifiers in HTTP access-log paths
	// so logs/SIEM cannot correlate an authenticated DID with the addresses /
	// grants it viewed. Standalone serves public data, so paths stay intact.
	log.RedactHTTPPaths = cfg.PrivacyProxyURL != ""

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

	// P-4: build the price service from config (coin id / currency are
	// configurable per chain) but only wire the event bus + background
	// refresher when ENABLE_PRICE is on. In privacy mode this defaults off
	// (config.Load), so a confidential/air-gapped deployment makes no
	// CoinGecko egress unless the operator opts in. The /price endpoint still
	// exists; it just serves no data (empty cache) when disabled.
	priceService := price.NewService(cfg.PriceCoinID, cfg.PriceCurrency, 60*time.Second)
	if cfg.EnablePrice {
		priceService.SetEventBus(eventBus)
	} else {
		log.Info("price refresher disabled (ENABLE_PRICE=false) — no CoinGecko egress",
			"privacy_mode", cfg.PrivacyProxyURL != "")
	}

	serverCfg := &api.ServerConfig{
		SolcPath:             cfg.SolcPath,
		UseSourcifyFallback:  cfg.UseSourcifyFallback,
		MetricsEnabled:       cfg.MetricsEnabled,
		PostLoginRedirectURL: cfg.PostLoginRedirectURL,
		EnableGasPrices:      cfg.EnableGasPrices,
		CORSAllowedOrigins:   cfg.CORSAllowedOrigins,
		PrivacyMode:          cfg.PrivacyProxyURL != "",
		SSOJWKSURL:           cfg.SSOJWKSURL,
		SSOIssuer:            cfg.SSOIssuer,
		SSOAudience:          cfg.SSOAudience,
	}

	// chooseProvider decides which chain-data source the api will serve
	// out of. It has two build-tagged implementations:
	//   provider_standalone.go  (default)  — supports both INDEXER_URL
	//                                         (chain-indexer gRPC) and
	//                                         PRIVACY_PROXY_URL modes.
	//   provider_privacy.go     (-tags privacy) — supports ONLY
	//                                         PRIVACY_PROXY_URL; the
	//                                         indexerclient package is
	//                                         not compiled into the
	//                                         binary. Used for
	//                                         privacy-mode deployments
	//                                         where direct chain-indexer
	//                                         access would bypass
	//                                         redaction.
	privacyClient, ssoClient, dataProvider := chooseProvider(cfg, database, rpcClient)

	server := api.New(database, rpcClient, nil, priceService, eventBus, cfg.APIPort, serverCfg, privacyClient, ssoClient, dataProvider)

	ctx, cancel := context.WithCancel(context.Background())
	if cfg.EnablePrice {
		// P-4: only the configured-on path makes outbound CoinGecko requests.
		priceService.StartBackgroundRefresh(ctx)
	}
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
