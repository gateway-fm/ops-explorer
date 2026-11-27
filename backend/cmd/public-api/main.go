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
	"explorer/internal/version"
	"explorer/pkg/log"
)

func main() {
	// Emit the build identity first thing so every deployment's logs say
	// exactly which binary is running. Values are injected via -ldflags at
	// build time; a plain `go build` logs version=dev.
	log.Info("build info", "version", version.Version, "commit", version.Commit, "built", version.BuildTime)

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

	// Proxies in front of the service that append to X-Forwarded-For (an
	// ingress gateway, the frontend's nginx). The rate limiter keys on the
	// address the outermost one appended; 0 = key on the TCP peer.
	trustedHops := publicapi.DefaultTrustedHops
	if v := os.Getenv("RATE_LIMIT_TRUSTED_HOPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			trustedHops = n
		} else {
			log.Fatal("RATE_LIMIT_TRUSTED_HOPS must be a whole number >= 0", "value", v)
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
	// RD-1031 (Option B): surface the privacy-proxy public base URL only as a
	// hint for the MetaMask setup dialog's jwt-injector --upstream field — NOT
	// a wallet RPC target. Stored WITHOUT a "/rpc" suffix; omitted when unset.
	if publicURL := strings.TrimSpace(os.Getenv("PRIVACY_PROXY_PUBLIC_URL")); publicURL != "" {
		chainInfo.SetPrivacyProxyPublicURL(strings.TrimSuffix(publicURL, "/"))
	}

	server := publicapi.NewServer(dataProvider, nil, chainInfo, port, rateLimit, rateWindow)
	server.SetTrustedProxyHops(trustedHops)
	// Title, currency and link origin of the `/llm.md` pages. All optional.
	// NETWORK_NAME / NETWORK_CURRENCY are the compose inputs the frontend's
	// VITE_NETWORK_NAME / VITE_NETWORK_CURRENCY come from, passed to this
	// service too; without EXPLORER_PUBLIC_URL, links use the request's host.
	server.SetSiteInfo(publicapi.SiteInfo{
		NetworkName:    os.Getenv("NETWORK_NAME"),
		CurrencySymbol: os.Getenv("NETWORK_CURRENCY"),
		PublicURL:      os.Getenv("EXPLORER_PUBLIC_URL"),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutting down public API server")
		cancel()
	}()

	log.Info("starting public API server", "port", port, "rateLimit", rateLimit, "rateWindow", rateWindow.String(), "trustedHops", trustedHops)
	if err := server.Start(ctx); err != nil {
		log.Fatal("server error", "error", err)
	}
}
