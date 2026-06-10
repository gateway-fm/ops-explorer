package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	cfg                  *ServerConfig // retained for CORS allowlist / privacy-mode flags (W-1, A-3)
	// jwtVerifier verifies the auth-cookie JWT signature against privacy-proxy's
	// JWKS (A-2). nil unless SSO_JWKS_URL is configured; when nil, GetAuthDID
	// falls back to display-only ExtractClaims. The impersonation caller-DID
	// binding requires this to be non-nil (the JWKS-required path).
	jwtVerifier          *auth.Verifier
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

	// sourcifyHTTP / sourcifyBaseURL are an optional injectable seam for the
	// Sourcify handlers (handleFetchSourcify/Check/Verify). Production leaves
	// them nil/"" and the handlers use http.DefaultClient + sourcifyAPIBase;
	// tests point them at an httptest server. Mirrors impersonationHTTP.
	sourcifyHTTP    *http.Client
	sourcifyBaseURL string
}

// sourcifyClient returns the HTTP client the Sourcify handlers should use
// (injected stub in tests, default client in production).
func (s *Server) sourcifyClient() *http.Client {
	if s.sourcifyHTTP != nil {
		return s.sourcifyHTTP
	}
	return http.DefaultClient
}

// sourcifyBase returns the Sourcify API base URL (injected in tests, the public
// server const in production).
func (s *Server) sourcifyBase() string {
	if s.sourcifyBaseURL != "" {
		return s.sourcifyBaseURL
	}
	return sourcifyAPIBase
}

type ServerConfig struct {
	SolcPath             string
	UseSourcifyFallback  bool
	MetricsEnabled       bool
	PostLoginRedirectURL string
	EnableGasPrices      bool

	// PrivacyProxyPublicURL is the privacy-proxy PUBLIC BASE url (no "/rpc"
	// suffix), surfaced via GET /chain-info as privacyProxyPublicUrl. It is a
	// hint for the privacy-mode MetaMask setup dialog (jwt-injector --upstream),
	// NOT a wallet RPC target. Empty when not configured (field omitted).
	PrivacyProxyPublicURL string

	// CORSAllowedOrigins is the allowlist of browser Origins permitted to make
	// credentialed cross-origin requests (W-1). When non-empty, only these
	// Origins are reflected into Access-Control-Allow-Origin. When empty in
	// standalone mode the server reflects any Origin (legacy permissive
	// behavior + a startup warning); privacy mode requires a non-empty
	// allowlist (enforced fail-closed in config.Validate).
	CORSAllowedOrigins []string
	// PrivacyMode mirrors cfg.PrivacyProxyURL != "" so the server can apply
	// fail-closed defaults (W-1 CORS, A-3 cookie Secure) without re-reading the
	// process env.
	PrivacyMode bool

	// CookieSecure controls the Secure flag on auth cookies (A-3): "true"
	// always, "false" never, "auto" only when the request is actually HTTPS
	// (r.TLS set or X-Forwarded-Proto: https from a trusted proxy). Privacy mode
	// defaults to "true" (forced); standalone defaults to "auto".
	CookieSecure string

	// JWT signature verification (A-2). When SSOJWKSURL is set, the auth-cookie
	// JWT is signature-verified (alg-confusion-safe) against this JWKS before
	// GetAuthDID trusts the subject; SSOIssuer/SSOAudience are checked only when
	// non-empty. When unset, GetAuthDID is display-only (ExtractClaims) and the
	// impersonation feature is disabled (its caller-DID binding requires a
	// verified DID).
	SSOJWKSURL  string
	SSOIssuer   string
	SSOAudience string
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

	if cfg != nil {
		s.cfg = cfg
		s.postLoginRedirectURL = cfg.PostLoginRedirectURL
		s.gasPricesEnabled = cfg.EnableGasPrices

		// A-2: build the JWKS signature verifier when configured. GetAuthDID
		// uses it to verify the auth-cookie JWT before trusting the subject.
		if cfg.SSOJWKSURL != "" {
			s.jwtVerifier = auth.NewVerifier(auth.VerifierConfig{
				JWKSURL:  cfg.SSOJWKSURL,
				Issuer:   cfg.SSOIssuer,
				Audience: cfg.SSOAudience,
			})
		}
	}

	if privacyClient != nil && privacyClient.IsEnabled() {
		s.privacyProxyURL = privacyClient.BaseURL()
		// "View as user" (RD-928): only meaningful when privacy-proxy is
		// reachable — without it there is nothing to impersonate against.
		//
		// A-2 (JWKS-required path): the impersonation caller-DID binding makes a
		// local authz decision keyed off GetAuthDID, so it must be a VERIFIED
		// DID. Enable the feature only when a JWT verifier is configured
		// (SSO_JWKS_URL set); otherwise leave s.impersonations nil so start/stop
		// return 503 and the middleware is a no-op — fail-closed rather than bind
		// against an unverified, spoofable DID.
		if s.jwtVerifier != nil {
			s.impersonations = NewMemoryImpersonationStore(0)
		} else {
			log.Warn("privacy mode: 'View as user' impersonation is DISABLED because SSO_JWKS_URL is not set — " +
				"the caller-DID binding requires a signature-verified DID (A-2). Set SSO_JWKS_URL to enable it.")
		}
	}

	if cfg != nil && cfg.MetricsEnabled {
		s.metrics = NewMetrics()
	}

	if eventBus != nil {
		s.wsConfig = ws.DefaultConfig()
		s.wsHub = ws.NewHub(eventBus, s.wsConfig.MaxConnections)
	}

	// P-2: in privacy mode the default (!privacy) binary must NOT build the
	// verifier or mount any local-DB write surface — those persist
	// attacker-controlled source/ABI into block-explorer's own Postgres,
	// bypassing privacy-proxy redaction. Skip verifier construction entirely
	// (even when SolcPath is the default /opt/solc) and recommend the
	// -tags privacy image. Route gating happens in setupRoutes via
	// inPrivacyMode(). This is fail-safe: no write surface, no fatal.
	if s.inPrivacyMode() {
		log.Warn("privacy mode: contract-verification + Sourcify write surfaces are disabled; " +
			"build/deploy the `-tags privacy` image to compile them out entirely")
	} else if cfg != nil && cfg.SolcPath != "" {
		s.verifier = verifier.NewVerifier(database, rpcClient, &verifier.Config{
			SolcPath:            cfg.SolcPath,
			UseSourcifyFallback: cfg.UseSourcifyFallback,
		})
	}

	// gas prices are served via provider.GetGasPrices; no in-process tracker.
	s.chainInfo = chaininfo.NewService(rpcClient, 10*time.Minute)
	if cfg != nil {
		// RD-1031 (Option B): surface the privacy-proxy public base URL purely
		// as a hint for the MetaMask setup dialog's jwt-injector --upstream
		// field. We do NOT hand any proxy /rpc endpoint to the wallet — a
		// browser wallet cannot attach the bearer + org path the proxy needs.
		s.chainInfo.SetPrivacyProxyPublicURL(cfg.PrivacyProxyPublicURL)
	}

	s.setupRoutes()
	return s
}

// csrfProtect is a method-gated Origin/Referer-allowlist CSRF middleware (A-1).
// On state-changing methods (POST/PUT/PATCH/DELETE) it requires the request's
// Origin (preferred) or Referer to match the configured CORS allowlist, and
// fails closed when BOTH are absent. Safe methods (GET/HEAD/OPTIONS) always
// pass. When the allowlist is empty (standalone default) it is a no-op — which
// privacy mode never hits because W-1 makes the allowlist mandatory there.
//
// This complements SameSite=Lax (which a top-level cross-site navigation can
// still defeat) with an explicit Origin/Referer check on every cookie-authed
// write.
func (s *Server) csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		var allowed []string
		if s.cfg != nil {
			allowed = s.cfg.CORSAllowedOrigins
		}
		if len(allowed) == 0 {
			// No allowlist configured (standalone) — CSRF check is a no-op.
			next.ServeHTTP(w, r)
			return
		}

		if !originOrRefererAllowed(r, allowed) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// originOrRefererAllowed returns true iff the request's Origin (preferred) or,
// failing that, its Referer origin matches one of the allowed origins. Returns
// false when neither header is present (fail-closed).
func originOrRefererAllowed(r *http.Request, allowed []string) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return originInList(origin, allowed)
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		u, err := url.Parse(ref)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return false
		}
		return originInList(u.Scheme+"://"+u.Host, allowed)
	}
	// Neither Origin nor Referer present on a state-changing request: fail closed.
	return false
}

func originInList(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}

// inPrivacyMode reports whether this server instance is serving privacy mode
// (reads routed through privacy-proxy). Used to gate local-DB write surfaces
// that would bypass privacy-proxy's redaction (P-2). True only when a privacy
// client is wired and enabled — i.e. PRIVACY_PROXY_URL was set.
func (s *Server) inPrivacyMode() bool {
	return s.privacyClient != nil && s.privacyClient.IsEnabled()
}

// corsAllowOriginFunc builds the CORS origin matcher (W-1). With a non-empty
// allowlist, only listed Origins are permitted (exact match) — paired with
// AllowCredentials this avoids the dangerous reflect-any-Origin + credentials
// combination. With an empty allowlist the behavior depends on mode:
//   - privacy mode: this is unreachable in production (config.Validate rejects
//     an empty allowlist), but if it ever happens we fail closed (deny all).
//   - standalone: legacy permissive behavior — reflect any Origin — with a
//     one-time startup warning.
func (s *Server) corsAllowOriginFunc() func(string) bool {
	var allowed []string
	privacyMode := false
	if s.cfg != nil {
		allowed = s.cfg.CORSAllowedOrigins
		privacyMode = s.cfg.PrivacyMode
	}

	if len(allowed) > 0 {
		set := make(map[string]struct{}, len(allowed))
		for _, o := range allowed {
			set[o] = struct{}{}
		}
		return func(origin string) bool {
			_, ok := set[origin]
			return ok
		}
	}

	// Empty allowlist.
	if privacyMode {
		// Defense-in-depth: config.Validate already fails closed here, so this
		// path should never run in privacy mode. Deny all rather than reflect.
		log.Warn("CORS: empty allowlist in privacy mode — denying all cross-origin requests (this should have been rejected at startup)")
		return func(string) bool { return false }
	}
	log.Warn("CORS: no CORS_ALLOWED_ORIGINS configured — reflecting any Origin with credentials (standalone permissive default). Set CORS_ALLOWED_ORIGINS to restrict.")
	return func(string) bool { return true }
}

func (s *Server) setupRoutes() {
	s.router.Use(log.HTTPMiddleware)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(30 * time.Second))

	if s.metrics != nil {
		s.router.Use(metricsMiddleware(s.metrics))
	}

	corsOpts := cors.Options{
		AllowOriginFunc:  s.corsAllowOriginFunc(),
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
			// A-1: CSRF-protect the state-changing impersonation routes
			// (POST /start, DELETE /{token}). Method-gated, so the GET /{token}
			// restore endpoint is never blocked.
			r.Use(s.csrfProtect)
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
			// A-1: CSRF-protect POST /logout (method-gated, so the GET login/
			// callback/status routes pass through).
			r.Use(s.csrfProtect)
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
			// A-1: CSRF-protect the eth-link writes (POST challenge/verify,
			// DELETE address). Method-gated, so GET /addresses passes through.
			r.Use(s.csrfProtect)
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
	//
	// P-2: the privacy build no-ops registerVerificationAPI, but the DEFAULT
	// (!privacy) binary can also serve privacy mode — there the helper is the
	// real one, so gate it at runtime too. These routes persist source/ABI into
	// block-explorer's own Postgres, bypassing privacy-proxy redaction.
	if !s.inPrivacyMode() {
		s.registerVerificationAPI(r)
	}

	r.Get("/stats", s.handleGetStats)
	r.Get("/chain-info", s.handleGetChainInfo)
	r.Get("/stats/tx-history", s.handleGetTransactionHistory)
	// RD-1063: /price and /gas are gated off in privacy mode. The privacy
	// proxy has no viewer-scoped story for them: /gas resolves to
	// ErrChainDataNotAvailable (500) and /price would trigger an outbound
	// CoinGecko fetch on cache-miss (a no-egress hole, S1). Gate at runtime
	// for the default binary too (same pattern as the P-2 verification gate
	// above). In privacy mode both → 404; the frontend already hides/disables
	// them and never issues the request.
	if !s.inPrivacyMode() {
		r.Get("/price", s.handleGetPrice)
		r.Get("/gas", s.handleGetGasPrices)
	}
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
		r.Get("/contract/uml", s.handleGetContractUML)
		// A-1: CSRF-protect the ABI write. In privacy mode this forwards to
		// privacy-proxy (not the local DB — see P-2 audit correction), but it is
		// still a cookie-authed state-changing call, so it needs the
		// Origin/Referer check. csrfProtect is method-gated and a no-op when no
		// allowlist is configured (standalone).
		r.With(s.csrfProtect).Post("/abi", s.handleUpdateContractABI)
		r.Get("/internal", s.handleGetAddressInternalTxs)
		r.Get("/logs", s.handleGetAddressLogs)
		r.Get("/balances", s.handleGetAddressTokenBalances)
		// Sourcify lookups are also build-tagged (privacy build = no-op) and,
		// like /verify, runtime-gated off in privacy mode for the default
		// binary (P-2) — Sourcify writes verified source to the local DB too.
		if !s.inPrivacyMode() {
			s.registerSourcifyAddressRoutes(r)
		}
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
			r.Get("/inventory", s.handleGetTokenInventory)
			r.Get("/transfers", s.handleGetTokenTransfers)
		})
	})

	r.Get("/token-transfers", s.handleGetAllTransfers)
	r.Get("/logs", s.handleGetLogs)
	r.Get("/accounts", s.handleGetAccounts)

	// RD-1063: /charts is gated off in privacy mode — it forwards to a
	// chain-indexer endpoint the privacy proxy doesn't serve (404 upstream),
	// with no viewer-scoping/redaction. Gate at runtime for the default binary
	// too (mirrors the P-2 verification gate above). In privacy mode → 404.
	if !s.inPrivacyMode() {
		r.Route("/charts", func(r chi.Router) {
			r.Get("/counters", s.handleGetChartCounters)
			r.Get("/lines", s.handleGetChartLines)
			r.Get("/lines/{id}", s.handleGetChartLine)
		})
	}
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
