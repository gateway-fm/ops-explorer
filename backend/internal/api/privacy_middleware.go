package api

import (
	"net/http"
)

// addressPrivacyMiddleware is a no-op pass-through. Address-level privacy
// filtering is handled by the ProxyDataProvider — all data requests go through
// the privacy-proxy which applies per-user visibility. The check-address
// endpoint was removed (PR #97) as it was superseded by inlined addressMetadata.
func (s *Server) addressPrivacyMiddleware(next http.Handler) http.Handler {
	return next
}
