package api

import (
	"context"
	"explorer/internal/authv2"
	"explorer/internal/rpc"
	"explorer/pkg/log"
	"github.com/go-chi/chi/v5"
	"net/http"
)

// NewV2 deliberately bypasses New: no V1 SSO/JWKS, impersonation store,
// verification database, websocket hub, raw RPC or chain-info poller is built.
// H1 owns the actual upstream current-rights REST implementation.
func NewV2(session *authv2.Service, provider *V2ReadClient, port int) (*Server, error) {
	if !v2PrivacyBuild || session == nil || !session.Ready() || provider == nil || port < 1 || port > 65535 {
		return nil, authv2.ErrInvalid
	}
	if provider.origin != session.OPSOrigin() {
		return nil, authv2.ErrInvalid
	}
	s := &Server{v2: session, v2Reads: provider, port: port, router: chi.NewRouter(), cfg: &ServerConfig{PrivacyMode: true, CookieSecure: "true", CORSAllowedOrigins: []string{session.Origin()}}}
	s.router.Use(log.HTTPMiddleware)
	// The ingress wraps the router itself. Its registered-route preflight check
	// therefore sees the same routes as real requests without running handlers.
	s.router.Use(func(next http.Handler) http.Handler {
		return session.Boundary(next, func(path, method string) bool {
			if method != "GET" && method != "HEAD" && method != "POST" {
				return false
			}
			return s.router.Match(chi.NewRouteContext(), method, path)
		})
	})
	s.router.Get("/health", s.handleHealthCheck)
	s.router.Get("/health/live", s.handleLivenessCheck)
	s.router.Get("/health/ready", s.handleReadinessCheck)
	s.router.Get("/version", s.handleVersion)
	for _, route := range []string{"/api/auth/login", "/api/auth/callback", "/api/auth/status", "/api/auth/logout"} {
		method := "GET"
		if route == "/api/auth/login" || route == "/api/auth/logout" {
			method = "POST"
		}
		s.router.Method(method, route, http.HandlerFunc(session.Auth))
	}
	for _, prefix := range []string{"/api", "/api/v1", "/api/v2"} {
		s.router.Get(prefix+"/version", s.handleVersion)
		s.router.Get(prefix+"/chain-info", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery != "" {
				writeError(w, 400, "invalid_request")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(session.SetupJSON())
		})
		s.router.Group(func(r chi.Router) {
			r.Use(session.Protected)
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := context.WithValue(r.Context(), rpc.ContextKeyAuthToken, authv2.Token(r.Context()))
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			r.Route(prefix, func(r chi.Router) { s.setupV2ReadRoutes(r) })
		})
	}
	return s, nil
}

func (s *Server) setupV2ReadRoutes(r chi.Router) {
	// These exact existing read families are fail-closed in this auth component.
	// They never pass through legacy numeric/offset DTOs or raw-indexer fallbacks.
	// The separate current-privacy REST adapter supplies their wire contract.
	for _, path := range []string{"/stats", "/stats/tx-history", "/sync", "/search", "/search/suggestions", "/blocks", "/blocks/", "/blocks/latest", "/blocks/{number}", "/blocks/{number}/internal", "/transactions", "/transactions/", "/transactions/{hash}", "/transactions/{hash}/transfers", "/transactions/{hash}/logs", "/transactions/{hash}/internal", "/addresses/{address}", "/addresses/{address}/", "/addresses/{address}/transactions", "/addresses/{address}/transfers", "/addresses/{address}/contract", "/addresses/{address}/contract/uml", "/addresses/{address}/internal", "/addresses/{address}/logs", "/addresses/{address}/balances", "/tokens", "/tokens/", "/tokens/{address}", "/tokens/{address}/", "/tokens/{address}/holders", "/tokens/{address}/inventory", "/tokens/{address}/transfers", "/token-transfers", "/logs", "/accounts"} {
		r.Get(path, func(w http.ResponseWriter, r *http.Request) { writeError(w, 503, "private_data_unavailable") })
	}
}

// Handler supports normal TLS/reverse-proxy composition without exposing any
// route that was not explicitly mounted by the selected constructor.
func (s *Server) Handler() http.Handler { return s.router }
