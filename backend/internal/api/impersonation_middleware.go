package api

import (
	"context"
	"net/http"
	"strings"

	"explorer/pkg/log"
)

// ImpersonateHeader is the request header the frontend sets when the admin is
// in "View as user" mode. The BFF translates this into a target DID via the
// impersonation store and rewrites outbound proxy requests to the
// /api/v1/admin/impersonate/<target_did>/... URL tree on privacy-proxy.
//
// The token MUST NOT be forwarded to privacy-proxy — privacy-proxy gates
// impersonation on the URL path + the caller's JWT, not on this header.
const ImpersonateHeader = "X-Impersonate-Token"

// impersonationCtxKey is the request-context key used to carry the resolved
// impersonation session through the request lifecycle (handlers → outbound
// data provider). Defined as a private type to avoid collisions.
type impersonationCtxKey struct{}

// WithImpersonation attaches the impersonation session to a context so that
// downstream callers (e.g. ProxyDataProvider) can look it up.
func WithImpersonation(ctx context.Context, session ImpersonationSession) context.Context {
	return context.WithValue(ctx, impersonationCtxKey{}, session)
}

// ImpersonationFromContext returns the impersonation session attached to the
// context (if any). The second return value is false when no impersonation is
// active, in which case callers must use the normal request path.
func ImpersonationFromContext(ctx context.Context) (ImpersonationSession, bool) {
	if ctx == nil {
		return ImpersonationSession{}, false
	}
	session, ok := ctx.Value(impersonationCtxKey{}).(ImpersonationSession)
	return session, ok
}

// impersonationMiddleware translates a valid X-Impersonate-Token header into
// a context-attached ImpersonationSession.
//
// Defense-in-depth checks:
//
//  1. Inbound write methods on JSON-RPC paths are rejected outright; the
//     privacy-proxy already blocks these but failing closed at the BFF makes
//     misuse impossible even if the proxy is misconfigured.
//  2. The caller's JWT subject (DID) must match the AdminDID embedded in the
//     session at mint time. A stolen token cannot be replayed by a different
//     user because they cannot satisfy this check.
//  3. The header is stripped from the in-memory request before handlers see
//     it, so it can never accidentally leak onward.
//
// The privacy-proxy side is the authoritative gate (it enforces same-org,
// tier-2 admin, and audit logging); these checks are intentionally redundant.
func (s *Server) impersonationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(ImpersonateHeader)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		// Always strip the header so it never reaches privacy-proxy and
		// never accidentally leaks into outbound calls or proxy logs.
		r.Header.Del(ImpersonateHeader)

		if s.impersonations == nil {
			// Feature disabled — silently ignore the header rather than 500.
			next.ServeHTTP(w, r)
			return
		}

		session, err := s.impersonations.Lookup(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid or expired impersonation token")
			return
		}

		// Verify the caller's JWT subject still matches the admin who minted
		// the token. Without this check, an attacker who captured the token
		// could use it from a different (lower-privileged) account.
		callerDID := s.GetAuthDID(r)
		if callerDID == "" || !strings.EqualFold(callerDID, session.AdminDID) {
			// P-3: do NOT log the caller/admin DID pair — it links two real
			// identities in the explorer's own stderr. The mismatch itself is
			// the useful security signal; the DIDs are not needed to act on it.
			log.Warn("impersonation: caller DID does not match token's admin DID (rejected)")
			writeError(w, http.StatusUnauthorized, "Impersonation token is not bound to this caller")
			return
		}

		// Server-side write-method guard. The privacy-proxy gates writes too,
		// but rejecting before forwarding keeps the audit log clean.
		if isImpersonationDisallowedMethod(r.Method) {
			writeError(w, http.StatusMethodNotAllowed, "Write requests are not allowed in view-as mode")
			return
		}

		ctx := WithImpersonation(r.Context(), session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isImpersonationDisallowedMethod returns true for HTTP methods that are
// considered state-changing for the explorer BFF surface. Impersonation is a
// read-only diagnostic view; the privacy-proxy translates write JSON-RPC into
// debug_traceCall server-side, but for the explorer's REST surface there is
// no equivalent — we simply refuse them.
func isImpersonationDisallowedMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// applyImpersonationPath rewrites an outbound request path to its
// admin-impersonate equivalent when the request context carries an active
// "View as user" session. RD-994 — the bound org is prepended as /in/<org_id>.
// For example, with target DID "did:p:42" and org "org-7":
//
//	/api/v1/explorer/blocks  ->  /api/v1/admin/impersonate/did:p:42/in/org-7/api/v1/explorer/blocks
//	/api/v1/explorer/stats   ->  /api/v1/admin/impersonate/did:p:42/in/org-7/api/v1/explorer/stats
//
// The org is read from the bound session (the token store), never from the
// inbound request — a session minted for one org cannot be redirected to
// another by tampering with the request path.
//
// When no impersonation is active the path is returned unchanged. If a session
// is somehow active without a bound org (should be impossible — Mint requires
// it), the path is also returned unchanged so the proxy's bare-route 400
// surfaces rather than a malformed /in// path. Used by ProxyDataProvider so
// every chain-data read transparently honours the active view-as session.
func applyImpersonationPath(ctx context.Context, path string) string {
	session, ok := ImpersonationFromContext(ctx)
	if !ok || session.TargetDID == "" || session.OrgID == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// PathEscape the DID and org — DIDs include colons which are technically
	// reserved but accepted in path segments; escaping is defensive against
	// future DID-method changes that introduce slashes or query-significant
	// chars. Org IDs are UUIDs today but we escape them too for the same
	// reason.
	return "/api/v1/admin/impersonate/" + pathEscapeDID(session.TargetDID) +
		"/in/" + pathEscapeDID(session.OrgID) + path
}

// pathEscapeDID escapes a DID for use as a single path segment. Defined
// separately so test cases for problematic DIDs (slashes, percent-encoded
// chars) can target it in isolation if the DID method ever changes.
func pathEscapeDID(did string) string {
	// We can't use url.PathEscape directly because we want colons to stay as
	// colons (RFC 3986 allows them in pchar). Easiest to swap in/out: build
	// the result by escaping just the bytes that would otherwise break path
	// parsing on privacy-proxy.
	replacer := strings.NewReplacer(
		"/", "%2F",
		"?", "%3F",
		"#", "%23",
	)
	return replacer.Replace(did)
}
