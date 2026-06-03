//go:build !privacy

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
	"explorer/internal/chaininfo"
	"explorer/internal/db"
	"explorer/internal/publicapi"
	"explorer/internal/rpc"
	"explorer/pkg/log"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Fatal("RPC_URL is required")
	}

	port := 8082
	if p := os.Getenv("PUBLIC_API_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	rateLimit := 100
	if rl := os.Getenv("RATE_LIMIT"); rl != "" {
		if v, err := strconv.Atoi(rl); err == nil {
			rateLimit = v
		}
	}

	rateWindow := 1 * time.Minute
	if rw := os.Getenv("RATE_LIMIT_WINDOW"); rw != "" {
		if d, err := time.ParseDuration(rw); err == nil {
			rateWindow = d
		}
	}

	database, err := db.New(databaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", "error", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatal("failed to run migrations", "error", err)
	}

	rpcClient, err := rpc.New(rpcURL)
	if err != nil {
		log.Fatal("failed to create rpc client", "error", err)
	}

	indexerURL := os.Getenv("INDEXER_URL")
	if indexerURL == "" {
		log.Fatal("INDEXER_URL is required — public-api reads chain data from chain-indexer (RD-855 Phase 6)")
	}
	fallback := api.NewDirectDBProvider(database, rpcClient, nil)
	dataProvider, err := indexerclient.New(indexerclient.Config{IndexerURL: indexerURL}, fallback)
	if err != nil {
		log.Fatal("failed to construct indexerclient provider", "error", err)
	}
	chainInfo := chaininfo.NewService(rpcClient, 10*time.Minute)
	// RD-1031: expose the canonical browser-facing JSON-RPC endpoint so
	// wallets connect to a reachable, redaction-enforcing endpoint. Derived
	// from the proxy public URL (+ "/rpc") when set; empty otherwise so the
	// frontend derives a same-origin URL rather than inventing localhost.
	if publicURL := strings.TrimSpace(os.Getenv("PRIVACY_PROXY_PUBLIC_URL")); publicURL != "" {
		chainInfo.SetBrowserRPCURL(strings.TrimSuffix(publicURL, "/") + "/rpc")
	}

	server := publicapi.NewServer(dataProvider, nil, chainInfo, port, rateLimit, rateWindow)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutting down public API server")
		cancel()
	}()

	log.Info("starting public API server", "port", port, "rateLimit", rateLimit, "rateWindow", rateWindow.String())
	if err := server.Start(ctx); err != nil {
		log.Fatal("server error", "error", err)
	}
}
