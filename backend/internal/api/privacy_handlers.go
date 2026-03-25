package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"explorer/internal/privacy"
	"explorer/pkg/eth/common"
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
		slog.Warn("failed to get viewable addresses", "error", err)
		http.Error(w, "failed to get viewable addresses", http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (s *Server) handleCheckAddressVisibility(w http.ResponseWriter, r *http.Request) {
	viewer := s.getViewerIdentity(r)

	if viewer.DID == "" {
		http.Error(w, "authentication required (sign in via Privado SSO)", http.StatusBadRequest)
		return
	}

	address := chi.URLParam(r, "address")
	if address == "" {
		http.Error(w, "address parameter required", http.StatusBadRequest)
		return
	}

	if !s.privacyClient.IsEnabled() {
		writeJSON(w, privacy.AddressVisibility{
			Address: strings.ToLower(address),
			Visible: true,
			Level:   privacy.VisibilityFull,
			Reason:  privacy.ReasonPublicAddress,
		})
		return
	}

	result, err := s.privacyClient.CheckAddressWithIdentity(r.Context(), viewer, address)
	if err != nil {
		slog.Warn("failed to check address visibility", "address", address, "error", err)
		http.Error(w, "failed to check address visibility", http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (s *Server) handleBatchCheckAddresses(w http.ResponseWriter, r *http.Request) {
	viewer := s.getViewerIdentity(r)

	if viewer.DID == "" {
		http.Error(w, "authentication required (sign in via Privado SSO)", http.StatusBadRequest)
		return
	}

	var req struct {
		Addresses []string `json:"addresses"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Addresses) == 0 {
		http.Error(w, "addresses array is required", http.StatusBadRequest)
		return
	}

	if len(req.Addresses) > 100 {
		http.Error(w, "maximum 100 addresses allowed per request", http.StatusBadRequest)
		return
	}

	for _, addr := range req.Addresses {
		if !common.IsHexAddress(addr) {
			http.Error(w, "invalid address format", http.StatusBadRequest)
			return
		}
	}

	if !s.privacyClient.IsEnabled() {
		results := make(map[string]*privacy.AddressVisibility)
		for _, addr := range req.Addresses {
			results[strings.ToLower(addr)] = &privacy.AddressVisibility{
				Address: strings.ToLower(addr),
				Visible: true,
				Level:   privacy.VisibilityFull,
				Reason:  privacy.ReasonPublicAddress,
			}
		}
		writeJSON(w, map[string]any{"results": results})
		return
	}

	results, err := s.privacyClient.CheckAddressesWithIdentity(r.Context(), viewer, req.Addresses)
	if err != nil {
		http.Error(w, "failed to check address visibility", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"results": results})
}

// checkAddressVisibility returns nil if privacy is not enabled (no gating needed).
// Fails closed: on error returns HIDDEN to prevent leaking private data.
func (s *Server) checkAddressVisibility(r *http.Request, address string) *privacy.AddressVisibility {
	if !s.privacyClient.IsEnabled() {
		return nil
	}

	viewer := s.getViewerIdentity(r)
	if viewer.DID == "" {
		return nil
	}

	vis, err := s.privacyClient.CheckAddressWithIdentity(r.Context(), viewer, address)
	if err != nil {
		// Fail closed - return HIDDEN on error
		return &privacy.AddressVisibility{
			Address: strings.ToLower(address),
			Visible: false,
			Level:   privacy.VisibilityHidden,
			Reason:  privacy.ReasonNoAccess,
		}
	}

	return vis
}


// SECURITY: Addresses are redacted based on disclosure_level before being sent to the frontend.
type GrantedAddressResponse struct {
	DisplayAddress  string `json:"display_address"`
	DisclosureLevel string `json:"disclosure_level"`
	GrantID         string `json:"grant_id"`
	Balance    string `json:"balance"`
	TxCount    int64  `json:"tx_count"`
	IsContract bool   `json:"is_contract"`
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

	resolved, err := s.privacyClient.ResolveAddressID(r.Context(), grantID, addressID)
	if err != nil {
		if errors.Is(err, privacy.ErrNotFound) {
			http.Error(w, "grant or address not found", http.StatusNotFound)
			return
		}
		slog.Warn("failed to resolve address", "grant_id", grantID, "address_id", addressID, "error", err)
		http.Error(w, "failed to resolve address", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	stats, err := s.provider.GetAddressStats(ctx, resolved.RealAddress)
	if err != nil {
		slog.Warn("failed to get address stats", "address", resolved.RealAddress, "error", err)
		http.Error(w, "failed to get address stats", http.StatusInternalServerError)
		return
	}

	balance, err := s.provider.GetBalance(ctx, resolved.RealAddress)
	if err != nil {
		slog.Warn("failed to get balance", "address", resolved.RealAddress, "error", err)
		http.Error(w, "failed to get balance", http.StatusInternalServerError)
		return
	}

	code, err := s.provider.GetCode(ctx, resolved.RealAddress)
	if err != nil {
		slog.Warn("failed to check contract status", "address", resolved.RealAddress, "error", err)
		http.Error(w, "failed to check contract status", http.StatusInternalServerError)
		return
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
		displayAddress = "[REDACTED]"
	default:
		// SECURITY: Fail-safe - treat unknown disclosure levels as redacted
		displayAddress = "[REDACTED]"
	}

	response := GrantedAddressResponse{
		DisplayAddress:  displayAddress,
		DisclosureLevel: resolved.DisclosureLevel,
		GrantID:         resolved.GrantID,
		Balance:         string(*balance),
		TxCount:         int64(stats.TxCount),
		IsContract:      len(code) > 0,
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

	if !s.privacyClient.IsEnabled() {
		http.Error(w, "privacy service not enabled", http.StatusServiceUnavailable)
		return
	}

	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	body, statusCode, err := s.privacyClient.GetGrantTransactions(r.Context(), grantID, addressID, limit, beforeBlock)
	if err != nil {
		slog.Warn("failed to get grant transactions", "grant_id", grantID, "address_id", addressID, "error", err)
		http.Error(w, "failed to get transactions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

// generateExternalPseudonym creates a consistent pseudonym derived from address + grantID,
// generateExternalPseudonym was removed — pseudonymization is now handled
// entirely by the privacy proxy (see GetGrantTransactions).
