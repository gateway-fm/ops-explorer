package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// addressPrivacyMiddleware checks address visibility before allowing access.
// It extracts the {address} URL param, calls the privacy-proxy, and returns
// 403 if the address is not accessible to the current viewer.
func (s *Server) addressPrivacyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address := chi.URLParam(r, "address")
		if address == "" {
			next.ServeHTTP(w, r)
			return
		}

		vis := s.checkAddressVisibility(r, address)
		if vis != nil && !vis.Visible {
			http.Error(w, "address is not accessible", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
