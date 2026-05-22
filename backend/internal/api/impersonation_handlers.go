package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"explorer/pkg/log"

	"github.com/go-chi/chi/v5"
)

// startImpersonationRequest is the body of POST /api/impersonation/start.
type startImpersonationRequest struct {
	TargetDID string `json:"target_did"`
}

// startImpersonationResponse is the JSON returned on a successful mint. The
// browser stores `token` in URL state and the impersonation context; it never
// receives the target DID through a query string.
type startImpersonationResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	// TargetDID is echoed back so the UI can render the banner ("Viewing as
	// <DID>") without making a second round-trip. It lives in the response
	// body only — never in the URL.
	TargetDID string `json:"target_did"`
}

// handleStartImpersonation mints a short-lived opaque token that maps to a
// target DID. The endpoint requires a valid auth cookie (the BFF cannot
// directly verify the tier-2 admin flag because the JWT does not carry RBAC
// claims), so authority is verified by *probing* privacy-proxy: we call a
// benign read endpoint under the impersonation URL prefix and let the proxy
// decide whether the admin is allowed to view-as this target. If the proxy
// rejects (non-2xx), we mirror the status to the client without minting.
//
// Cross-org and not-tier-2-admin cases both surface as 404 on the proxy side
// (to avoid leaking the existence of the target), and we propagate that
// shape here.
func (s *Server) handleStartImpersonation(w http.ResponseWriter, r *http.Request) {
	if s.impersonations == nil {
		writeError(w, http.StatusServiceUnavailable, "View-as impersonation is not enabled")
		return
	}

	adminDID := s.GetAuthDID(r)
	if adminDID == "" {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	adminToken := s.GetAuthToken(r)
	if adminToken == "" {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req startImpersonationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.TargetDID = strings.TrimSpace(req.TargetDID)
	if req.TargetDID == "" {
		writeError(w, http.StatusBadRequest, "target_did is required")
		return
	}
	// Defensive: a self-view-as is meaningless and is rejected by the proxy as
	// well. Mirror that here so we don't even bother probing.
	if strings.EqualFold(req.TargetDID, adminDID) {
		writeError(w, http.StatusBadRequest, "Cannot impersonate yourself")
		return
	}

	// Probe privacy-proxy to confirm the caller is allowed to view-as this
	// target. Same-org + tier-2 admin checks live on the proxy side; we just
	// surface the result. Cross-org / non-admin / unknown target all return
	// 404 (no info leak).
	status, err := s.probeImpersonationGate(r.Context(), adminToken, req.TargetDID)
	if err != nil {
		log.Warn("impersonation probe failed", "err", err)
		writeError(w, http.StatusBadGateway, "Failed to verify impersonation eligibility")
		return
	}
	switch {
	case status == http.StatusOK:
		// allowed — fall through to mint.
	case status == http.StatusForbidden:
		writeError(w, http.StatusForbidden, "Not authorized to impersonate")
		return
	case status == http.StatusUnauthorized:
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	case status == http.StatusNotFound:
		// Hide existence: pretend the target is not visible.
		writeError(w, http.StatusNotFound, "Target user not found")
		return
	default:
		writeError(w, http.StatusBadGateway, "Failed to verify impersonation eligibility")
		return
	}

	token, expiresAt, err := s.impersonations.Mint(r.Context(), ImpersonationSession{
		AdminDID:  adminDID,
		TargetDID: req.TargetDID,
	}, DefaultImpersonationTTL)
	if err != nil {
		log.Error("impersonation mint failed", "err", err)
		writeError(w, http.StatusInternalServerError, "Failed to start impersonation")
		return
	}

	writeJSON(w, startImpersonationResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		TargetDID: req.TargetDID,
	})
}

// handleStopImpersonation revokes the given token. Idempotent: revoking an
// unknown token returns 204 so the frontend can "stop" cleanly even if the
// session was already cleared by GC or another tab.
func (s *Server) handleStopImpersonation(w http.ResponseWriter, r *http.Request) {
	if s.impersonations == nil {
		writeError(w, http.StatusServiceUnavailable, "View-as impersonation is not enabled")
		return
	}
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	// Best-effort revoke; the call cannot fail in the current in-memory
	// implementation but we still honour the interface contract.
	_ = s.impersonations.Revoke(r.Context(), token)
	w.WriteHeader(http.StatusNoContent)
}

// handleGetImpersonation returns the session metadata for a known token,
// scoped to the caller. Used by the frontend's cold-mount restore path so
// it can populate the banner with the real target DID after a page refresh
// without leaving the placeholder in place.
//
// The caller's auth-cookie subject must match the AdminDID embedded in the
// session at mint time — same replay defense as the middleware. Unknown or
// foreign tokens return 404 (we never leak the existence of someone else's
// session).
func (s *Server) handleGetImpersonation(w http.ResponseWriter, r *http.Request) {
	if s.impersonations == nil {
		writeError(w, http.StatusServiceUnavailable, "View-as impersonation is not enabled")
		return
	}
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	callerDID := s.GetAuthDID(r)
	if callerDID == "" {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	session, err := s.impersonations.Lookup(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}
	if !strings.EqualFold(session.AdminDID, callerDID) {
		// 404 not 403: don't leak that the token exists.
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}
	writeJSON(w, startImpersonationResponse{
		Token:     token,
		ExpiresAt: session.ExpiresAt,
		TargetDID: session.TargetDID,
	})
}

// probeImpersonationGate makes a HEAD-like read call under the impersonation
// URL prefix and returns the upstream status code. Privacy-proxy decides the
// outcome:
//
//   - 200 → caller is a tier-2 admin in the same org as target_did.
//   - 404 → cross-org, not a tier-2 admin, or unknown target (same shape).
//   - 403 → policy rejects (e.g. self-impersonation, locked target).
//   - 401 → admin's JWT is no longer valid.
//
// We deliberately use a chain-info-shaped read (a tiny endpoint that exists
// on the proxy explorer surface) so the probe is cheap and side-effect-free.
// The path is the impersonation prefix + the standard explorer chain-id path.
func (s *Server) probeImpersonationGate(ctx context.Context, adminToken, targetDID string) (int, error) {
	if s.privacyProxyURL == "" {
		return 0, fmt.Errorf("privacy proxy URL not configured")
	}
	endpoint, err := url.JoinPath(
		s.privacyProxyURL,
		"/api/v1/admin/impersonate/",
		targetDID,
		"/api/v1/explorer/chain-id",
	)
	if err != nil {
		return 0, fmt.Errorf("build probe URL: %w", err)
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("build probe request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return 0, fmt.Errorf("probe request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// httpClient returns the HTTP client used for outbound impersonation probes.
// Held as a method so tests can swap it via a server field if needed; today
// it just lazily instantiates a sensible default.
func (s *Server) httpClient() *http.Client {
	if s.impersonationHTTP != nil {
		return s.impersonationHTTP
	}
	return defaultImpersonationHTTPClient
}

// defaultImpersonationHTTPClient is the shared client for probe calls. A
// short timeout avoids tying up handler goroutines if privacy-proxy is slow.
var defaultImpersonationHTTPClient = &http.Client{Timeout: 10 * time.Second}
