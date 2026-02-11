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
	"explorer/internal/indexer"
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

	// Create indexer with parallelization and catchup config
	idxCfg := &indexer.Config{
		RPCWorkers:           cfg.RPCWorkers,
		RPCRateLimit:         cfg.RPCRateLimit,
		DBBatchSize:          cfg.DBBatchSize,
		TokenMetadataWorkers: cfg.TokenMetadataWorkers,
		BalanceWorkers:       cfg.BalanceWorkers,
		EnableAsyncBalance:   cfg.EnableAsyncBalance,
		EnableTracing:        cfg.EnableTracing,
		TraceRateLimit:       cfg.TraceRateLimit,
		TraceWorkers:         cfg.TraceWorkers,
		CatchupEnabled:       cfg.CatchupEnabled,
		CatchupWorkers:       cfg.CatchupWorkers,
		CatchupBatchSize:     cfg.CatchupBatchSize,
		CatchupQueueSize:     cfg.CatchupQueueSize,
	}
	idx := indexer.NewWithConfig(database, rpcClient, cfg.PollInterval, cfg.StartBlock, idxCfg)

	// Set the event bus for the indexer
	idx.SetEventBus(eventBus)

	// Create price service (ETH price from CoinGecko, refresh every 60 seconds)
	priceService := price.NewService("ethereum", "usd", 60*time.Second)

	// Set the event bus for the price service
	priceService.SetEventBus(eventBus)

	// Create server config
	serverCfg := &api.ServerConfig{
		SolcPath:            cfg.SolcPath,
		UseSourcifyFallback: cfg.UseSourcifyFallback,
		MetricsEnabled:      cfg.MetricsEnabled,
	}

	server := api.New(database, rpcClient, idx, priceService, eventBus, cfg.APIPort, serverCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background price refresh
	priceService.StartBackgroundRefresh(ctx)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutting down")
		cancel()
	}()

	go func() {
		if err := idx.Start(ctx); err != nil {
			log.Error("indexer error", "error", err)
		}
	}()

	log.Info("starting API server", "port", cfg.APIPort)
	if err := server.Start(ctx); err != nil {
		log.Fatal("server error", "error", err)
	}
}
