package api

import (
	"explorer/internal/authv2"
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
			if method != "GET" && method != "HEAD" && method != "POST" && method != "DELETE" {
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
	s.router.Group(func(r chi.Router) {
		r.Use(v2WalletDeadline)
		r.Use(session.Protected)
		r.Get("/api/eth/addresses", s.handleV2Wallet)
		r.Post("/api/eth/link/challenge", s.handleV2Wallet)
		r.Post("/api/eth/link/verify", s.handleV2Wallet)
		r.Delete("/api/eth/addresses/{address}", s.handleV2Wallet)
	})
	for _, prefix := range []string{"/api", "/api/v1", "/api/v2"} {
		s.router.Group(func(r chi.Router) {
			r.Use(v2WalletDeadline)
			r.Use(session.Protected)
			r.Route(prefix, s.setupV2DisclosureRoutes)
		})
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
	}
	s.router.Group(func(r chi.Router) { r.Use(v2ReadDeadline); r.Use(session.Protected); s.setupV2ReadRoutes(r) })
	return s, nil
}

// Handler supports normal TLS/reverse-proxy composition without exposing any
// route that was not explicitly mounted by the selected constructor.
func (s *Server) Handler() http.Handler { return s.router }
