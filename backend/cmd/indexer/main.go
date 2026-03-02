package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"explorer/internal/config"
	"explorer/internal/db"
	"explorer/internal/indexer"
	"explorer/internal/log"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutting down indexer")
		cancel()
	}()

	log.Info("starting indexer worker")
	if err := idx.Start(ctx); err != nil {
		log.Error("indexer error", "error", err)
	}
}
