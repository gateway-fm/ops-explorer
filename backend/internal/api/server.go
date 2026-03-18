package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"explorer/internal/auth"
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
	db            APIDatabase
	rpc           *rpc.Client
	indexer       *indexer.Indexer
	provider      DataProvider
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

type ServerConfig struct {
	SolcPath            string
	UseSourcifyFallback bool
	MetricsEnabled      bool
}

func New(database APIDatabase, rpcClient *rpc.Client, idx *indexer.Indexer, priceService *price.Service, eventBus *events.Bus, port int, cfg *ServerConfig, privacyClient *privacy.Client, ssoClient *auth.SSOClient, provider DataProvider) *Server {
	s := &Server{
		db:            database,
		rpc:           rpcClient,
		indexer:       idx,
		provider:      provider,
		price:         priceService,
		eventBus:      eventBus,
		privacyClient: privacyClient,
		ssoClient:     ssoClient,
		port:          port,
		router:        chi.NewRouter(),
	}

	if cfg != nil && cfg.MetricsEnabled {
		s.metrics = NewMetrics()
	}

	if eventBus != nil {
		s.wsConfig = ws.DefaultConfig()
		s.wsHub = ws.NewHub(eventBus, s.wsConfig.MaxConnections)
		if privacyClient != nil && privacyClient.IsEnabled() {
			s.wsHub.SetPrivacyEnabled(true)
		}
	}

	if cfg != nil && cfg.SolcPath != "" {
		s.verifier = verifier.NewVerifier(database, rpcClient, &verifier.Config{
			SolcPath:            cfg.SolcPath,
			UseSourcifyFallback: cfg.UseSourcifyFallback,
		})
	}

	s.gasTracker = gas.NewTracker(database, nil)

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))

	if s.metrics != nil {
		s.router.Use(metricsMiddleware(s.metrics))
	}

	corsOpts := cors.Options{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Cookie"},
		AllowCredentials: true,
	}
	c := cors.New(corsOpts)
	s.router.Use(c.Handler)

	if s.ssoClient != nil && s.ssoClient.IsEnabled() {
		s.router.Use(s.refreshAuthMiddleware)
		s.router.Use(s.authContextMiddleware)
	}

	s.router.Get("/health", s.handleHealthCheck)
	s.router.Get("/health/live", s.handleLivenessCheck)
	s.router.Get("/health/ready", s.handleReadinessCheck)

	if s.metrics != nil {
		s.router.Handle("/metrics", s.metrics.Handler())
	}

	if s.wsHub != nil {
		s.router.Get("/ws", s.handleWebSocket)
	}

	// /api and /api/v1 serve the same routes
	s.router.Route("/api", func(r chi.Router) {
		s.setupAPIRoutes(r)
	})
	s.router.Route("/api/v1", func(r chi.Router) {
		s.setupAPIRoutes(r)
	})

	s.router.Route("/api/v2", func(r chi.Router) {
		s.setupAPIV2Routes(r)
	})

	if s.ssoClient != nil && s.ssoClient.IsEnabled() {
		s.router.Route("/api/auth", func(r chi.Router) {
			r.Get("/login", s.handleAuthLogin)
			r.Get("/callback", s.handleAuthCallback)
			r.Get("/status", s.handleAuthStatus)
			r.Post("/logout", s.handleAuthLogout)
		})
	}

	if s.privacyClient != nil && s.privacyClient.IsEnabled() {
		s.router.Route("/api/privacy", func(r chi.Router) {
			r.Get("/viewable-addresses", s.handleGetViewableAddresses)
			r.Get("/check-address/{address}", s.handleCheckAddressVisibility)
			r.Post("/check-addresses", s.handleBatchCheckAddresses)
			r.Get("/grant/{grantId}/{addressId}", s.handleGetGrantedAddress)
			r.Get("/grant/{grantId}/{addressId}/transactions", s.handleGetGrantedAddressTransactions)
		})

		s.router.Route("/api/eth", func(r chi.Router) {
			r.Get("/addresses", s.handleGetLinkedAddresses)
			r.Post("/link/challenge", s.handleCreateLinkChallenge)
			r.Post("/link/verify", s.handleVerifyLink)
			r.Delete("/addresses/{address}", s.handleUnlinkAddress)
		})
	}
}

func (s *Server) setupAPIRoutes(r chi.Router) {
	// Public routes — no auth required
	r.Get("/stats", s.handleGetStats)
	r.Get("/stats/tx-history", s.handleGetTransactionHistory)
	r.Get("/price", s.handleGetPrice)
	r.Get("/gas", s.handleGetGasPrices)
	r.Get("/sync", s.handleGetSyncStatus)

	// Block list/detail — publicly accessible but stripped for unauthenticated users
	r.Route("/blocks", func(r chi.Router) {
		r.Get("/", s.handleGetBlocks)
		r.Get("/latest", s.handleGetLatestBlock)
		r.Get("/{number}", s.handleGetBlock)
		// Block internal txs — fully gated
		r.With(s.privacyAuthGateMiddleware).Get("/{number}/internal", s.handleGetBlockInternalTxs)
	})

	// Gated routes — 403 for unauthenticated when privacy is enabled
	r.Group(func(r chi.Router) {
		r.Use(s.privacyAuthGateMiddleware)

		r.Get("/search", s.handleSearch)
		r.Get("/search/suggestions", s.handleSearchSuggestions)

		r.Route("/transactions", func(r chi.Router) {
			r.Get("/", s.handleGetTransactions)
			r.Get("/{hash}", s.handleGetTransaction)
			r.Get("/{hash}/transfers", s.handleGetTransactionTransfers)
			r.Get("/{hash}/logs", s.handleGetTransactionLogs)
			r.Get("/{hash}/internal", s.handleGetTransactionInternalTxs)
		})

		r.Route("/addresses/{address}", func(r chi.Router) {
			r.Use(s.addressPrivacyMiddleware)
			r.Get("/", s.handleGetAddress)
			r.Get("/transactions", s.handleGetAddressTransactions)
			r.Get("/transfers", s.handleGetAddressTransfers)
			r.Get("/contract", s.handleGetContract)
			r.Post("/abi", s.handleUpdateContractABI)
			r.Get("/internal", s.handleGetAddressInternalTxs)
			r.Get("/logs", s.handleGetAddressLogs)
			r.Get("/balances", s.handleGetAddressTokenBalances)
			r.Get("/sourcify", s.handleFetchSourcify)
			r.Get("/sourcify/check", s.handleCheckSourcify)
		})

		r.Route("/verify", func(r chi.Router) {
			r.Post("/", s.handleVerifyContract)
			r.Post("/standard-json", s.handleVerifyStandardJSON)
			r.Get("/compilers", s.handleListCompilers)
		})

		r.Route("/tokens", func(r chi.Router) {
			r.Get("/", s.handleGetTokens)
			r.Route("/{address}", func(r chi.Router) {
				r.Use(s.addressPrivacyMiddleware)
				r.Get("/", s.handleGetToken)
				r.Get("/holders", s.handleGetTokenHolders)
				r.Get("/transfers", s.handleGetTokenTransfers)
			})
		})

		r.Get("/token-transfers", s.handleGetAllTransfers)
		r.Get("/logs", s.handleGetLogs)
		r.Get("/accounts", s.handleGetAccounts)
	})
}

func (s *Server) setupAPIV2Routes(r chi.Router) {
	// V2 handlers detect v2 from the route context and adjust pagination accordingly
	s.setupAPIRoutes(r)
}

func (s *Server) authContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.GetAuthToken(r)
		if token != "" {
			ctx := context.WithValue(r.Context(), rpc.ContextKeyAuthToken, token)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start(ctx context.Context) error {
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
