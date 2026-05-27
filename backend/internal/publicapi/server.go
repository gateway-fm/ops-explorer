package publicapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"explorer/internal/api"
	"explorer/internal/chaininfo"
	"explorer/pkg/log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
)

// Server is the public API server that exposes read-only endpoints.
type Server struct {
	provider    api.DataProvider
	chainInfo   *chaininfo.Service
	port        int
	rateLimiter *RateLimiter
	router      *chi.Mux
}

// NewServer creates a new public API server. The gasTracker parameter
// is retained for call-site compatibility but is ignored — gas prices
// are served via provider.GetGasPrices now.
func NewServer(provider api.DataProvider, gasTracker any, chainInfo *chaininfo.Service, port int, rateLimit int, rateWindow time.Duration) *Server {
	_ = gasTracker
	s := &Server{
		provider:    provider,
		chainInfo:   chainInfo,
		port:        port,
		rateLimiter: NewRateLimiter(rateLimit, rateWindow),
		router:      chi.NewRouter(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(log.HTTPMiddleware)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))

	corsOpts := cors.Options{
		AllowOriginFunc: func(origin string) bool { return true },
		AllowedMethods:  []string{"GET", "OPTIONS"},
		AllowedHeaders:  []string{"Accept", "Content-Type"},
	}
	c := cors.New(corsOpts)
	s.router.Use(c.Handler)

	s.router.Use(s.rateLimiter.Middleware)

	s.router.Get("/health", s.handleHealthCheck)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/openapi.json", s.handleOpenAPISpec)

		// Blocks
		r.Get("/blocks", s.handleGetBlocks)
		r.Get("/blocks/latest", s.handleGetLatestBlock)
		r.Get("/blocks/{number}", s.handleGetBlock)

		// Transactions
		r.Get("/transactions", s.handleGetTransactions)
		r.Get("/transactions/{hash}", s.handleGetTransaction)
		r.Get("/transactions/{hash}/logs", s.handleGetTransactionLogs)
		r.Get("/transactions/{hash}/transfers", s.handleGetTransactionTransfers)
		r.Get("/transactions/{hash}/internal", s.handleGetTransactionInternalTxs)

		// Addresses
		r.Get("/addresses/{address}", s.handleGetAddress)
		r.Get("/addresses/{address}/transactions", s.handleGetAddressTransactions)
		r.Get("/addresses/{address}/transfers", s.handleGetAddressTransfers)
		r.Get("/addresses/{address}/balances", s.handleGetAddressTokenBalances)
		r.Get("/addresses/{address}/internal", s.handleGetAddressInternalTxs)
		r.Get("/addresses/{address}/logs", s.handleGetAddressLogs)
		r.Get("/addresses/{address}/contract", s.handleGetContract)

		// Tokens
		r.Get("/tokens", s.handleGetTokens)
		r.Get("/tokens/{address}", s.handleGetToken)
		r.Get("/tokens/{address}/holders", s.handleGetTokenHolders)
		r.Get("/tokens/{address}/transfers", s.handleGetTokenTransfers)

		// Token Transfers
		r.Get("/token-transfers", s.handleGetAllTransfers)

		// Accounts
		r.Get("/accounts", s.handleGetAccounts)

		// Search
		r.Get("/search", s.handleSearch)
		r.Get("/search/suggestions", s.handleSearchSuggestions)

		// Stats
		r.Get("/stats", s.handleGetStats)
		r.Get("/stats/tx-history", s.handleGetTransactionHistory)
		r.Get("/gas", s.handleGetGasPrices)
		r.Get("/chain-info", s.handleGetChainInfo)
		r.Get("/sync", s.handleGetSyncStatus)

		// Charts
		r.Route("/charts", func(r chi.Router) {
			r.Get("/counters", s.handleGetChartCounters)
			r.Get("/lines", s.handleGetChartLines)
			r.Get("/lines/{id}", s.handleGetChartLine)
		})
	})
}

// Start starts the HTTP server, blocking until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	if s.chainInfo != nil {
		go s.chainInfo.Start(ctx)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
		// Aikido SAST (severity 65 / medium): the public-API listener is
		// internet-facing once deployed; without a header-read deadline it's
		// vulnerable to slowloris-style header-trickle DoS where a client
		// holds a TCP connection by sending one byte at a time. 5s is
		// generous for any legitimate client (HTTP headers are <1KB in
		// practice) and tight enough that slow attackers get pruned. We
		// don't set ReadTimeout / WriteTimeout because some explorer
		// endpoints stream large block ranges and would tail off legit
		// long reads; ReadHeaderTimeout alone closes the slowloris path
		// without affecting payload throughput.
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	return srv.ListenAndServe()
}
