package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"explorer/internal/auth"
	"explorer/internal/chaininfo"
	"explorer/internal/events"
	"explorer/internal/price"
	"explorer/internal/privacy"
	"explorer/internal/rpc"
	"explorer/internal/verifier"
	ws "explorer/internal/websocket"
	"explorer/pkg/log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
	"golang.org/x/sync/singleflight"
)

type Server struct {
	db                   APIDatabase
	rpc                  *rpc.Client
	provider             DataProvider
	price                *price.Service
	eventBus             *events.Bus
	verifier             *verifier.Verifier
	chainInfo            *chaininfo.Service
	metrics              *Metrics
	wsHub                *ws.Hub
	wsConfig             *ws.Config
	privacyClient        *privacy.Client
	privacyProxyURL      string // base URL used for the impersonation probe (RD-928)
	ssoClient            *auth.SSOClient
	postLoginRedirectURL string
	gasPricesEnabled     bool
	port                 int
	router               *chi.Mux
	etherscanResults     sync.Map // guid -> *etherscanVerifyResult

	// refreshGroup collapses concurrent access-token refreshes for the same
	// session into a single privacy-proxy /refresh call. privacy-proxy rotates
	// refresh tokens single-use, so without this a page load's parallel
	// requests would race — one rotates the token and the rest get 401s, which
	// would otherwise tear the session down. Keyed by the refresh token.
	// See refreshAuthMiddleware.
	refreshGroup singleflight.Group

	// impersonations is the session store backing the "View as user" (RD-928)
	// admin diagnostic flow. nil when the feature is not configured (e.g.
	// privacy proxy URL not set), in which case start/stop endpoints return
	// 503 and the impersonationMiddleware is a no-op.
	impersonations ImpersonationStore
	// impersonationHTTP is an optional HTTP client for the eligibility probe
	// against privacy-proxy. Tests inject a stub here; production code leaves
	// it nil and uses the default client.
	impersonationHTTP *http.Client
}

type ServerConfig struct {
	SolcPath             string
	UseSourcifyFallback  bool
	MetricsEnabled       bool
	PostLoginRedirectURL string
	EnableGasPrices      bool
}

// New constructs the api Server. The idx parameter is retained for
// backwards compatibility with existing callers but is ignored —
// block-explorer no longer runs its own indexer (RD-855 Phase 6).
// Chain data comes from chain-indexer via the DataProvider.
func New(database APIDatabase, rpcClient *rpc.Client, idx any, priceService *price.Service, eventBus *events.Bus, port int, cfg *ServerConfig, privacyClient *privacy.Client, ssoClient *auth.SSOClient, provider DataProvider) *Server {
	_ = idx
	s := &Server{
		db:            database,
		rpc:           rpcClient,
		provider:      provider,
		price:         priceService,
		eventBus:      eventBus,
		privacyClient: privacyClient,
		ssoClient:     ssoClient,
		port:          port,
		router:        chi.NewRouter(),
	}

	if privacyClient != nil && privacyClient.IsEnabled() {
		s.privacyProxyURL = privacyClient.BaseURL()
		// "View as user" (RD-928): only meaningful when privacy-proxy is
		// reachable — without it there is nothing to impersonate against.
		s.impersonations = NewMemoryImpersonationStore(0)
	}

	if cfg != nil {
		s.postLoginRedirectURL = cfg.PostLoginRedirectURL
		s.gasPricesEnabled = cfg.EnableGasPrices
	}

	if cfg != nil && cfg.MetricsEnabled {
		s.metrics = NewMetrics()
	}

	if eventBus != nil {
		s.wsConfig = ws.DefaultConfig()
		s.wsHub = ws.NewHub(eventBus, s.wsConfig.MaxConnections)
	}

	if cfg != nil && cfg.SolcPath != "" {
		s.verifier = verifier.NewVerifier(database, rpcClient, &verifier.Config{
			SolcPath:            cfg.SolcPath,
			UseSourcifyFallback: cfg.UseSourcifyFallback,
		})
	}

	// gas prices are served via provider.GetGasPrices; no in-process tracker.
	s.chainInfo = chaininfo.NewService(rpcClient, 10*time.Minute)

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(log.HTTPMiddleware)
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

	// /api and /api/v1 serve the same routes.
	// impersonationMiddleware is mounted here so chain-data reads are
	// transparently routed through privacy-proxy's admin-impersonate prefix
	// when X-Impersonate-Token is present. The middleware is a no-op when
	// the feature is disabled or the header is absent.
	s.router.Route("/api", func(r chi.Router) {
		r.Use(s.impersonationMiddleware)
		s.setupAPIRoutes(r)
	})
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(s.impersonationMiddleware)
		s.setupAPIRoutes(r)
	})

	s.router.Route("/api/v2", func(r chi.Router) {
		r.Use(s.impersonationMiddleware)
		s.setupAPIV2Routes(r)
	})

	// "View as user" (RD-928) session management. Mounted outside the
	// impersonation middleware: starting / stopping a session must always be
	// authenticated as the real admin user, never under another view-as token.
	if s.impersonations != nil {
		s.router.Route("/api/impersonation", func(r chi.Router) {
			r.Post("/start", s.handleStartImpersonation)
			r.Delete("/{token}", s.handleStopImpersonation)
			// Cold-mount restore: lets the frontend translate ?as=<token>
			// back into the target DID after a page refresh without leaking
			// it through the URL.
			r.Get("/{token}", s.handleGetImpersonation)
		})
	}

	if s.ssoClient != nil && s.ssoClient.IsEnabled() {
		s.router.Route("/api/auth", func(r chi.Router) {
			r.Get("/login", s.handleAuthLogin)
			r.Get("/callback", s.handleAuthCallback)
			r.Get("/status", s.handleAuthStatus)
			r.Post("/logout", s.handleAuthLogout)
		})
	} else {
		s.router.Get("/api/auth/status", handleAuthStatusDisabled)
		s.router.Get("/api/v1/auth/status", handleAuthStatusDisabled)
	}

	if s.privacyClient != nil && s.privacyClient.IsEnabled() {
		s.router.Route("/api/privacy", func(r chi.Router) {
			r.Get("/viewable-addresses", s.handleGetViewableAddresses)
			r.Get("/grant/{grantId}/{addressId}", s.handleGetGrantedAddress)
			r.Get("/grant/{grantId}/{addressId}/transactions", s.handleGetGrantedAddressTransactions)
			r.Get("/grant/{grantId}/activity", s.handleGetGrantActivityLogs)
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
	// Etherscan-compatible RPC API (hardhat verify, forge verify-contract)
	// is registered via the build-tagged verification helper so privacy
	// builds can compile it out entirely. See verification_routes_*.go.
	// (In standalone mode this also adds /verify/* below.)
	s.registerVerificationAPI(r)

	r.Get("/stats", s.handleGetStats)
	r.Get("/chain-info", s.handleGetChainInfo)
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
		r.Get("/{number}/internal", s.handleGetBlockInternalTxs)
	})

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
		// Sourcify lookups are also build-tagged (privacy build = no-op).
		s.registerSourcifyAddressRoutes(r)
	})

	// /verify/* moved into the build-tagged registerVerificationAPI helper
	// above (call site at the top of this function). Privacy builds skip it.

	r.Route("/tokens", func(r chi.Router) {
		r.Get("/", s.handleGetTokens)
		r.Route("/{address}", func(r chi.Router) {
			// No addressPrivacyMiddleware here — the privacy proxy already handles
			// token visibility (returns redacted fields for non-Full access, 404 for
			// Hidden). Blocking at the explorer level would prevent fetching token
			// decimals needed for correct value formatting.
			r.Get("/", s.handleGetToken)
			r.Get("/holders", s.handleGetTokenHolders)
			r.Get("/transfers", s.handleGetTokenTransfers)
		})
	})

	r.Get("/token-transfers", s.handleGetAllTransfers)
	r.Get("/logs", s.handleGetLogs)
	r.Get("/accounts", s.handleGetAccounts)

	r.Route("/charts", func(r chi.Router) {
		r.Get("/counters", s.handleGetChartCounters)
		r.Get("/lines", s.handleGetChartLines)
		r.Get("/lines/{id}", s.handleGetChartLine)
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

	if s.chainInfo != nil {
		go s.chainInfo.Start(ctx)
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.router,
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
