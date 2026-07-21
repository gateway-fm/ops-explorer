package api

import (
	"errors"
	"net/http"

	"explorer/internal/privacy"
	"explorer/internal/types"
	"explorer/pkg/log"

	"github.com/go-chi/chi/v5"
)

func (s *Server) getViewerIdentity(r *http.Request) privacy.ViewerIdentity {
	return privacy.ViewerIdentity{
		DID:      s.GetAuthDID(r),
		JWTToken: s.GetAuthToken(r),
	}
}

func (s *Server) handleGetViewableAddresses(w http.ResponseWriter, r *http.Request) {
	viewer := s.getViewerIdentity(r)

	if viewer.DID == "" {
		http.Error(w, "authentication required (sign in via Privado SSO)", http.StatusBadRequest)
		return
	}

	if !s.privacyClient.IsEnabled() {
		writeJSON(w, privacy.ViewableAddressesResponse{
			ViewerDID:          viewer.DID,
			OwnAddresses:       []privacy.OwnAddress{},
			DisclosedAddresses: []privacy.DisclosedAddress{},
		})
		return
	}

	result, err := s.privacyClient.GetViewableAddressesWithIdentity(r.Context(), viewer)
	if err != nil {
		// P-3: do NOT log viewer_did — it would let log/SIEM access correlate
		// an authenticated DID with the addresses/grants it viewed.
		log.Warn("privacy: get viewable addresses failed", "error", err)
		http.Error(w, "failed to get viewable addresses", http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// SECURITY: Addresses are redacted based on disclosure_level before being sent to the frontend.
type GrantedAddressResponse struct {
	DisplayAddress  string   `json:"display_address"`
	DisclosureLevel string   `json:"disclosure_level"`
	GrantID         string   `json:"grant_id"`
	Balance         string   `json:"balance"`
	TxCount         int64    `json:"tx_count"`
	IsContract      bool     `json:"is_contract"`
	ScopeMethods    []string `json:"scope_methods,omitempty"` // Grant scope methods (e.g. "transaction_history", "activity_logs")
}

// SECURITY: This endpoint uses opaque address_id - the real address is never exposed to the frontend
// for pseudonymous/redacted disclosures.
func (s *Server) handleGetGrantedAddress(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantId")
	addressID := chi.URLParam(r, "addressId")

	if grantID == "" || addressID == "" {
		http.Error(w, "grant_id and address_id are required", http.StatusBadRequest)
		return
	}

	// Require authenticated viewer to prevent unauthorized access (IDOR)
	viewer := s.getViewerIdentity(r)
	if viewer.DID == "" {
		http.Error(w, "authentication required (sign in via Privado SSO)", http.StatusUnauthorized)
		return
	}

	if !s.privacyClient.IsEnabled() {
		http.Error(w, "privacy service not enabled", http.StatusServiceUnavailable)
		return
	}

	resolved, err := s.privacyClient.ResolveAddressID(r.Context(), grantID, addressID, viewer.JWTToken)
	if err != nil {
		if errors.Is(err, privacy.ErrNotFound) {
			http.Error(w, "grant or address not found", http.StatusNotFound)
			return
		}
		// P-3: grant_id / address_id are privacy-sensitive identifiers — omit them.
		log.Warn("privacy: resolve grant address failed", "error", err)
		http.Error(w, "failed to resolve address", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Stats/balance/code may not exist for EOAs or pseudonymous/redacted addresses — that's OK
	var stats *types.AddressStats
	var balance *types.JSONString
	var code []byte
	if resolved.RealAddress != "" {
		stats, _ = s.provider.GetAddressStats(ctx, resolved.RealAddress)
		balance, _ = s.provider.GetBalance(ctx, resolved.RealAddress)
		code, _ = s.provider.GetCode(ctx, resolved.RealAddress)
	}

	// SECURITY: Never expose real address for non-full disclosures
	var displayAddress string
	switch resolved.DisclosureLevel {
	case "full":
		displayAddress = resolved.RealAddress
	case "pseudonymous":
		displayAddress = resolved.Pseudonym
		if displayAddress == "" {
			displayAddress = "Address-Unknown"
		}
	case "redacted":
		displayAddress = "[PRIVATE]"
	default:
		// SECURITY: Fail-safe - treat unknown disclosure levels as redacted
		displayAddress = "[PRIVATE]"
	}

	var txCount int64
	if stats != nil {
		txCount = int64(stats.TxCount)
	}
	var balanceStr string
	if balance != nil {
		balanceStr = string(*balance)
	}

	response := GrantedAddressResponse{
		DisplayAddress:  displayAddress,
		DisclosureLevel: resolved.DisclosureLevel,
		GrantID:         resolved.GrantID,
		Balance:         balanceStr,
		TxCount:         txCount,
		IsContract:      len(code) > 0,
		ScopeMethods:    resolved.ScopeMethods,
	}

	writeJSON(w, response)
}

// SECURITY: All addresses are replaced with pseudonyms - the real addresses are never exposed.
// TxHash is hidden for non-full disclosures to prevent lookup on other explorers.
// PseudonymizedTransactionsResponse is now generated entirely by the privacy proxy.
// The explorer just forwards the proxy's JSON response as-is.

// handleGetGrantedAddressTransactions proxies the request to the privacy proxy's
// grant transactions endpoint. The proxy handles all pseudonymization — the
// explorer never sees real addresses for non-full disclosures.
func (s *Server) handleGetGrantedAddressTransactions(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantId")
	addressID := chi.URLParam(r, "addressId")

	if grantID == "" || addressID == "" {
		http.Error(w, "grant_id and address_id are required", http.StatusBadRequest)
		return
	}

	// Require authenticated viewer to prevent unauthorized access (IDOR).
	// Grant IDs are UUIDs but could be leaked via browser history or logs.
	viewer := s.getViewerIdentity(r)
	if viewer.DID == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	if !s.privacyClient.IsEnabled() {
		http.Error(w, "privacy service not enabled", http.StatusServiceUnavailable)
		return
	}

	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	body, statusCode, err := s.privacyClient.GetGrantTransactions(
		r.Context(), grantID, addressID, viewer.JWTToken, limit, beforeBlock,
	)
	if err != nil {
		// P-3: grant_id / address_id are privacy-sensitive identifiers — omit them.
		log.Warn("privacy: get grant transactions failed", "error", err)
		http.Error(w, "failed to get transactions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

// handleGetGrantActivityLogs proxies grant activity log requests to the privacy proxy.
// The viewer's JWT is forwarded so the proxy can verify grant holder identity.
func (s *Server) handleGetGrantActivityLogs(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantId")
	if grantID == "" {
		http.Error(w, "grant_id is required", http.StatusBadRequest)
		return
	}

	viewer := s.getViewerIdentity(r)
	if viewer.DID == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	if !s.privacyClient.IsEnabled() {
		http.Error(w, "privacy service not enabled", http.StatusServiceUnavailable)
		return
	}

	limit := parseLimit(r)
	offset := parseOffset(r)

	body, statusCode, err := s.privacyClient.GetGrantActivityLogs(r.Context(), grantID, viewer.JWTToken, limit, offset)
	if err != nil {
		// P-3: grant_id is a privacy-sensitive identifier — omit it.
		log.Warn("privacy: get grant activity logs failed", "error", err)
		http.Error(w, "failed to get activity logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

// generateExternalPseudonym creates a consistent pseudonym derived from address + grantID,
// generateExternalPseudonym was removed — pseudonymization is now handled
// entirely by the privacy proxy (see GetGrantTransactions).
