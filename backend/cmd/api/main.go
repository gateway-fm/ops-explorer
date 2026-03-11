package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"explorer/internal/api"
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
	if cfg.PrivacyEnabled && cfg.PrivacyProxyURL != "" {
		privacyClient = privacy.NewClient(cfg.PrivacyProxyURL)
		ssoClient = auth.NewSSOClient(cfg.PrivacyProxyURL, cfg.PrivacyProxyPublicURL, cfg.SSOClientID, cfg.SSORedirectURI)
		log.Info("privacy integration enabled", "proxy_url", cfg.PrivacyProxyURL)
	}

	var dataProvider api.DataProvider
	if cfg.PrivacyProxyURL != "" {
		dataProvider = api.NewProxyDataProvider(cfg.PrivacyProxyURL)
	} else {
		dataProvider = api.NewDirectDBProvider(database, rpcClient, nil)
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
