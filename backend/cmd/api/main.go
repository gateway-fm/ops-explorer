package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"explorer/internal/api"
	"explorer/internal/config"
	"explorer/internal/db"
	"explorer/internal/events"
	"explorer/internal/log"
	"explorer/internal/price"
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

	// Create event bus for real-time updates
	eventBus := events.NewBus()
	defer eventBus.Close()

	// Create price service (ETH price from CoinGecko, refresh every 60 seconds)
	priceService := price.NewService("ethereum", "usd", 60*time.Second)
	priceService.SetEventBus(eventBus)

	// Create server config
	serverCfg := &api.ServerConfig{
		SolcPath:            cfg.SolcPath,
		UseSourcifyFallback: cfg.UseSourcifyFallback,
		MetricsEnabled:      cfg.MetricsEnabled,
	}

	// API-only mode: no indexer required
	server := api.New(database, rpcClient, nil, priceService, eventBus, cfg.APIPort, serverCfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Start background price refresh
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
