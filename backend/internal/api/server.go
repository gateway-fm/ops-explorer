package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"explorer/internal/auth"
	"explorer/internal/db"
	"explorer/internal/events"
	"explorer/internal/gas"
	"explorer/internal/indexer"
	"explorer/internal/price"
	"explorer/internal/privacy"
	"explorer/internal/rpc"
	"explorer/internal/verifier"
	ws "explorer/internal/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
)

type Server struct {
	db            *db.DB
	rpc           *rpc.Client
	indexer       *indexer.Indexer
	price         *price.Service
	eventBus      *events.Bus
	verifier      *verifier.Verifier
	gasTracker    *gas.Tracker
	metrics       *Metrics
	wsHub         *ws.Hub
	wsConfig      *ws.Config
	privacyClient *privacy.Client
	ssoClient     *auth.SSOClient
	port          int
	router        *chi.Mux
}

// ServerConfig holds server configuration options
type ServerConfig struct {
	SolcPath            string
	UseSourcifyFallback bool
	MetricsEnabled      bool
}

func New(database *db.DB, rpcClient *rpc.Client, idx *indexer.Indexer, priceService *price.Service, eventBus *events.Bus, port int, cfg *ServerConfig, privacyClient *privacy.Client, ssoClient *auth.SSOClient) *Server {
	s := &Server{
		db:            database,
		rpc:           rpcClient,
		indexer:       idx,
		price:         priceService,
		eventBus:      eventBus,
		privacyClient: privacyClient,
		ssoClient:     ssoClient,
		port:          port,
		router:        chi.NewRouter(),
	}

	// Set up metrics if enabled
	if cfg != nil && cfg.MetricsEnabled {
		s.metrics = NewMetrics()
	}

	// Set up WebSocket if event bus is available
	if eventBus != nil {
		s.wsConfig = ws.DefaultConfig()
		s.wsHub = ws.NewHub(eventBus, s.wsConfig.MaxConnections)
	}

	// Set up verifier if solc path is provided
	if cfg != nil && cfg.SolcPath != "" {
		s.verifier = verifier.NewVerifier(database, rpcClient, &verifier.Config{
			SolcPath:            cfg.SolcPath,
			UseSourcifyFallback: cfg.UseSourcifyFallback,
		})
	}

	// Set up gas tracker
	s.gasTracker = gas.NewTracker(database, nil)

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))

	// Add metrics middleware if enabled
	if s.metrics != nil {
		s.router.Use(metricsMiddleware(s.metrics))
	}

	corsOpts := cors.Options{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Cookie"},
		AllowCredentials: true,
	}
	c := cors.New(corsOpts)
	s.router.Use(c.Handler)

	// Add auth refresh middleware if SSO is enabled
	if s.ssoClient != nil && s.ssoClient.IsEnabled() {
		s.router.Use(s.refreshAuthMiddleware)
	}

	// Health check endpoints
	s.router.Get("/health", s.handleHealthCheck)
	s.router.Get("/health/live", s.handleLivenessCheck)
	s.router.Get("/health/ready", s.handleReadinessCheck)

	// Metrics endpoint
	if s.metrics != nil {
		s.router.Handle("/metrics", s.metrics.Handler())
	}

	// WebSocket endpoint
	if s.wsHub != nil {
		s.router.Get("/ws", s.handleWebSocket)
	}

	// API v1 routes (current)
	s.router.Route("/api", func(r chi.Router) {
		s.setupAPIRoutes(r)
	})

	// API v1 explicit versioning (alias to /api)
	s.router.Route("/api/v1", func(r chi.Router) {
		s.setupAPIRoutes(r)
	})

	// API v2 routes (with standardized cursor pagination)
	s.router.Route("/api/v2", func(r chi.Router) {
		s.setupAPIV2Routes(r)
	})

	// Auth routes (only if SSO is enabled)
	if s.ssoClient != nil && s.ssoClient.IsEnabled() {
		s.router.Route("/api/auth", func(r chi.Router) {
			r.Post("/login", s.handleAuthLogin)
			r.Get("/callback", s.handleAuthCallback)
			r.Get("/status", s.handleAuthStatus)
			r.Post("/logout", s.handleAuthLogout)
			r.Get("/session/{id}/status", s.handleAuthSessionStatus)
		})
	}

	// Privacy routes (only if privacy client is enabled)
	if s.privacyClient != nil && s.privacyClient.IsEnabled() {
		s.router.Route("/api/privacy", func(r chi.Router) {
			r.Get("/viewable-addresses", s.handleGetViewableAddresses)
			r.Get("/check-address/{address}", s.handleCheckAddressVisibility)
			r.Post("/check-addresses", s.handleBatchCheckAddresses)
			r.Get("/grant/{grantId}/{addressId}", s.handleGetGrantedAddress)
			r.Get("/grant/{grantId}/{addressId}/transactions", s.handleGetGrantedAddressTransactions)
		})
	}
}

func (s *Server) setupAPIRoutes(r chi.Router) {
	r.Get("/stats", s.handleGetStats)
	r.Get("/stats/tx-history", s.handleGetTransactionHistory)
	r.Get("/price", s.handleGetPrice)
	r.Get("/gas", s.handleGetGasPrices)
	r.Get("/sync", s.handleGetSyncStatus)
	r.Get("/search", s.handleSearch)
	r.Get("/search/suggestions", s.handleSearchSuggestions)

	r.Route("/blocks", func(r chi.Router) {
		r.Get("/", s.handleGetBlocks)
		r.Get("/latest", s.handleGetLatestBlock)
		r.Get("/{number}", s.handleGetBlock)
	})

	r.Route("/transactions", func(r chi.Router) {
		r.Get("/", s.handleGetTransactions)
		r.Get("/{hash}", s.handleGetTransaction)
		r.Get("/{hash}/transfers", s.handleGetTransactionTransfers)
		r.Get("/{hash}/logs", s.handleGetTransactionLogs)
		r.Get("/{hash}/internal", s.handleGetTransactionInternalTxs)
	})

	r.Route("/addresses", func(r chi.Router) {
		r.Get("/{address}", s.handleGetAddress)
		r.Get("/{address}/transactions", s.handleGetAddressTransactions)
		r.Get("/{address}/transfers", s.handleGetAddressTransfers)
		r.Get("/{address}/contract", s.handleGetContract)
		r.Post("/{address}/abi", s.handleUpdateContractABI)
		r.Get("/{address}/internal", s.handleGetAddressInternalTxs)
		r.Get("/{address}/logs", s.handleGetAddressLogs)
		r.Get("/{address}/balances", s.handleGetAddressTokenBalances)
		r.Get("/{address}/sourcify", s.handleFetchSourcify)
		r.Get("/{address}/sourcify/check", s.handleCheckSourcify)
	})

	// Contract verification
	r.Post("/verify", s.handleVerifyContract)
	r.Get("/verify/compilers", s.handleListCompilers)

	r.Route("/tokens", func(r chi.Router) {
		r.Get("/", s.handleGetTokens)
		r.Get("/{address}", s.handleGetToken)
		r.Get("/{address}/holders", s.handleGetTokenHolders)
		r.Get("/{address}/transfers", s.handleGetTokenTransfers)
	})

	r.Get("/token-transfers", s.handleGetAllTransfers)
	r.Get("/logs", s.handleGetLogs)
	r.Get("/accounts", s.handleGetAccounts)
}

// setupAPIV2Routes sets up API v2 routes with standardized cursor pagination
func (s *Server) setupAPIV2Routes(r chi.Router) {
	// V2 uses the same handlers but with cursor-based pagination responses
	// The handlers detect v2 from the route context and adjust accordingly
	s.setupAPIRoutes(r)
}

func (s *Server) Start(ctx context.Context) error {
	// Start WebSocket hub if available
	if s.wsHub != nil {
		go s.wsHub.Run(ctx)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	return srv.ListenAndServe()
}
