package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"explorer/internal/privacy"
	"explorer/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
)

// getViewerIdentity extracts the viewer identity from request (DID from auth cookie only, SSO-only)
func (s *Server) getViewerIdentity(r *http.Request) privacy.ViewerIdentity {
	return privacy.ViewerIdentity{
		DID: s.GetAuthDID(r),
	}
}

// handleGetViewableAddresses returns all addresses viewable by the authenticated user
func (s *Server) handleGetViewableAddresses(w http.ResponseWriter, r *http.Request) {
	viewer := s.getViewerIdentity(r)

	// DID is required (SSO-only)
	if viewer.DID == "" {
		http.Error(w, "authentication required (sign in via Privado SSO)", http.StatusBadRequest)
		return
	}

	if !s.privacyClient.IsEnabled() {
		// Privacy not configured - return empty response
		writeJSON(w, privacy.ViewableAddressesResponse{
			ViewerDID:          viewer.DID,
			OwnAddresses:       []privacy.OwnAddress{},
			DisclosedAddresses: []privacy.DisclosedAddress{},
		})
		return
	}

	result, err := s.privacyClient.GetViewableAddressesWithIdentity(r.Context(), viewer)
	if err != nil {
		http.Error(w, "failed to get viewable addresses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// handleCheckAddressVisibility checks if a specific address is visible to the user
func (s *Server) handleCheckAddressVisibility(w http.ResponseWriter, r *http.Request) {
	viewer := s.getViewerIdentity(r)

	// DID is required (SSO-only)
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
		// Privacy not configured - all addresses visible
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
		http.Error(w, "failed to check address visibility: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

// handleBatchCheckAddresses checks visibility of multiple addresses at once
func (s *Server) handleBatchCheckAddresses(w http.ResponseWriter, r *http.Request) {
	viewer := s.getViewerIdentity(r)

	// DID is required (SSO-only)
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

	if !s.privacyClient.IsEnabled() {
		// Privacy not configured - all addresses visible
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
		http.Error(w, "failed to check addresses: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"results": results})
}

// checkAddressVisibility is a helper for use in other handlers.
// Returns nil if privacy is not enabled (no gating needed).
// Otherwise always calls the privacy-proxy, even for anonymous viewers.
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

// filterAddressesForVisibility filters a list of addresses based on visibility
// Returns map of address -> display info
func (s *Server) filterAddressesForVisibility(r *http.Request, addresses []string) map[string]AddressDisplayInfo {
	viewer := s.getViewerIdentity(r)
	result := make(map[string]AddressDisplayInfo)

	// If no identity or privacy not enabled, all addresses are visible
	if viewer.DID == "" || !s.privacyClient.IsEnabled() {
		for _, addr := range addresses {
			result[addr] = AddressDisplayInfo{
				Address:     addr,
				DisplayName: addr,
				IsPrivate:   false,
			}
		}
		return result
	}

	// Check visibility in batch
	visibilities, err := s.privacyClient.CheckAddressesWithIdentity(r.Context(), viewer, addresses)
	if err != nil {
		// Fail open on error
		for _, addr := range addresses {
			result[addr] = AddressDisplayInfo{
				Address:     addr,
				DisplayName: addr,
				IsPrivate:   false,
			}
		}
		return result
	}

	for _, addr := range addresses {
		vis, ok := visibilities[strings.ToLower(addr)]
		if !ok || !vis.Visible {
			result[addr] = AddressDisplayInfo{
				Address:     addr,
				DisplayName: "[PRIVATE]",
				IsPrivate:   true,
			}
		} else {
			displayName := addr
			if vis.Level == privacy.VisibilityPseudonymous && vis.Pseudonym != nil {
				displayName = *vis.Pseudonym
			}
			result[addr] = AddressDisplayInfo{
				Address:     addr,
				DisplayName: displayName,
				IsPrivate:   false,
				Visibility:  vis,
			}
		}
	}

	return result
}

// AddressDisplayInfo contains display information for an address
type AddressDisplayInfo struct {
	Address     string                     `json:"address"`
	DisplayName string                     `json:"display_name"`
	IsPrivate   bool                       `json:"is_private"`
	Visibility  *privacy.AddressVisibility `json:"visibility,omitempty"`
}

// GrantedAddressResponse is the response for viewing an address via a disclosure grant
// SECURITY: Addresses are redacted based on disclosure_level before being sent to the frontend.
type GrantedAddressResponse struct {
	// Display address - pseudonym for pseudonymous, "[REDACTED]" for redacted, real for full
	DisplayAddress  string `json:"display_address"`
	DisclosureLevel string `json:"disclosure_level"`
	GrantID         string `json:"grant_id"`
	// Address info (balance, txCount) - always included
	Balance    string `json:"balance"`
	TxCount    int64  `json:"tx_count"`
	IsContract bool   `json:"is_contract"`
}

// handleGetGrantedAddress returns address info for a disclosed address via grant
// GET /api/privacy/grant/:grant_id/:address_id
// SECURITY: This endpoint uses opaque address_id - the real address is never exposed to the frontend
// for pseudonymous/redacted disclosures.
func (s *Server) handleGetGrantedAddress(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantId")
	addressID := chi.URLParam(r, "addressId")

	if grantID == "" || addressID == "" {
		http.Error(w, "grant_id and address_id are required", http.StatusBadRequest)
		return
	}

	// SECURITY: Require authenticated viewer to prevent unauthorized access (IDOR)
	viewer := s.getViewerIdentity(r)
	if viewer.DID == "" {
		http.Error(w, "authentication required (sign in via Privado SSO)", http.StatusUnauthorized)
		return
	}

	if !s.privacyClient.IsEnabled() {
		http.Error(w, "privacy service not enabled", http.StatusServiceUnavailable)
		return
	}

	// Resolve the address_id to get real address and disclosure level
	resolved, err := s.privacyClient.ResolveAddressID(r.Context(), grantID, addressID)
	if err != nil {
		// Check if it's a not found or forbidden error
		errStr := err.Error()
		if strings.Contains(errStr, "not found") {
			http.Error(w, "grant or address not found", http.StatusNotFound)
			return
		}
		if strings.Contains(errStr, "access denied") || strings.Contains(errStr, "revoked") || strings.Contains(errStr, "expired") {
			http.Error(w, errStr, http.StatusForbidden)
			return
		}
		http.Error(w, "failed to resolve address: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Fetch address stats using the real address (backend only)
	stats, err := s.db.GetAddressStats(ctx, resolved.RealAddress)
	if err != nil {
		http.Error(w, "failed to get address stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch balance from RPC
	balance, err := s.rpc.GetBalance(ctx, common.HexToAddress(resolved.RealAddress))
	if err != nil {
		http.Error(w, "failed to get balance: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if it's a contract
	code, err := s.rpc.GetCode(ctx, common.HexToAddress(resolved.RealAddress))
	if err != nil {
		http.Error(w, "failed to check contract status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Determine display address based on disclosure level
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
		Balance:         balance.String(),
		TxCount:         int64(stats.TxCount),
		IsContract:      len(code) > 0,
	}

	writeJSON(w, response)
}

// PseudonymizedTransaction is a transaction with addresses pseudonymized for privacy
// SECURITY: All addresses are replaced with pseudonyms - the real addresses are never exposed
type PseudonymizedTransaction struct {
	// TxHash is hidden for non-full disclosures to prevent lookup on other explorers
	TxHash         *string          `json:"tx_hash,omitempty"`
	BlockNumber    uint64           `json:"block_number"`
	BlockTimestamp uint64           `json:"block_timestamp"`
	From           string           `json:"from"`
	To             *string          `json:"to"`
	Value          types.JSONString `json:"value"`
	GasUsed        uint64           `json:"gas_used"`
	Status         int              `json:"status"`
	// Direction relative to the disclosed address: "in", "out", or "self"
	Direction string `json:"direction"`
}

// PseudonymizedTransactionsResponse is the response for granted address transactions
type PseudonymizedTransactionsResponse struct {
	Transactions    []PseudonymizedTransaction `json:"transactions"`
	DisclosureLevel string                     `json:"disclosure_level"`
	// AddressLabels maps pseudonyms to their roles for UI display
	// e.g., {"Address-KDCM": "This Address", "External-1": "External Address"}
	AddressLabels map[string]string `json:"address_labels"`
	HasMore       bool              `json:"has_more"`
}

// handleGetGrantedAddressTransactions returns pseudonymized transactions for a disclosed address
// GET /api/privacy/grant/:grant_id/:address_id/transactions
// SECURITY: Addresses are pseudonymized based on disclosure_level before being sent to the frontend.
// For non-full disclosures, tx hashes are hidden to prevent lookup on other explorers.
func (s *Server) handleGetGrantedAddressTransactions(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantId")
	addressID := chi.URLParam(r, "addressId")

	if grantID == "" || addressID == "" {
		http.Error(w, "grant_id and address_id are required", http.StatusBadRequest)
		return
	}

	// SECURITY: Require authenticated viewer to prevent unauthorized access (IDOR)
	viewer := s.getViewerIdentity(r)
	if viewer.DID == "" {
		http.Error(w, "authentication required (sign in via Privado SSO)", http.StatusUnauthorized)
		return
	}

	if !s.privacyClient.IsEnabled() {
		http.Error(w, "privacy service not enabled", http.StatusServiceUnavailable)
		return
	}

	// Resolve the address_id to get real address and disclosure level
	resolved, err := s.privacyClient.ResolveAddressID(r.Context(), grantID, addressID)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not found") {
			http.Error(w, "grant or address not found", http.StatusNotFound)
			return
		}
		if strings.Contains(errStr, "access denied") || strings.Contains(errStr, "revoked") || strings.Contains(errStr, "expired") {
			http.Error(w, errStr, http.StatusForbidden)
			return
		}
		http.Error(w, "failed to resolve address: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// For redacted disclosures, don't show transactions at all
	if resolved.DisclosureLevel == "redacted" {
		writeJSON(w, PseudonymizedTransactionsResponse{
			Transactions:    []PseudonymizedTransaction{},
			DisclosureLevel: "redacted",
			AddressLabels:   map[string]string{},
			HasMore:         false,
		})
		return
	}

	ctx := r.Context()

	// Parse pagination params
	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	// Normalize the address to match database format (checksummed)
	normalizedAddress := common.HexToAddress(resolved.RealAddress).Hex()

	// Fetch transactions using the real address (backend only)
	txs, err := s.db.GetTransactionsByAddress(ctx, normalizedAddress, limit+1, beforeBlock)
	if err != nil {
		http.Error(w, "failed to get transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if there are more results
	hasMore := len(txs) > limit
	if hasMore {
		txs = txs[:limit]
	}

	// Determine the disclosed address pseudonym
	disclosedPseudonym := resolved.RealAddress // For full disclosure, use real address
	if resolved.DisclosureLevel == "pseudonymous" {
		disclosedPseudonym = resolved.Pseudonym
		if disclosedPseudonym == "" {
			disclosedPseudonym = "Address-Unknown"
		}
	}

	// Build pseudonym mapping for external addresses
	// For full disclosure: use real addresses
	// For pseudonymous: use consistent pseudonyms like "External-1", "External-2"
	externalPseudonyms := make(map[string]string)
	externalCounter := 1
	addressLabels := map[string]string{
		disclosedPseudonym: "This Address",
	}

	getExternalPseudonym := func(addr string) string {
		if resolved.DisclosureLevel == "full" {
			return addr
		}
		normalizedAddr := strings.ToLower(addr)
		if pseudo, exists := externalPseudonyms[normalizedAddr]; exists {
			return pseudo
		}
		pseudo := generateExternalPseudonym(addr, grantID, externalCounter)
		externalPseudonyms[normalizedAddr] = pseudo
		addressLabels[pseudo] = "External Address"
		externalCounter++
		return pseudo
	}

	// Pseudonymize transactions
	normalizedDisclosed := strings.ToLower(resolved.RealAddress)
	pseudoTxs := make([]PseudonymizedTransaction, 0, len(txs))

	for _, tx := range txs {
		pseudoTx := PseudonymizedTransaction{
			BlockNumber:    tx.BlockNumber,
			BlockTimestamp: tx.BlockTimestamp,
			Value:          tx.Value,
			GasUsed:        tx.GasUsed,
			Status:         tx.Status,
		}

		// Only include tx hash for full disclosure
		if resolved.DisclosureLevel == "full" {
			pseudoTx.TxHash = &tx.Hash
		}

		// Pseudonymize from address
		if strings.ToLower(tx.From) == normalizedDisclosed {
			pseudoTx.From = disclosedPseudonym
		} else {
			pseudoTx.From = getExternalPseudonym(tx.From)
		}

		// Pseudonymize to address
		if tx.To != nil {
			if strings.ToLower(*tx.To) == normalizedDisclosed {
				to := disclosedPseudonym
				pseudoTx.To = &to
			} else {
				to := getExternalPseudonym(*tx.To)
				pseudoTx.To = &to
			}
		}

		// Determine direction
		fromIsDisclosed := strings.ToLower(tx.From) == normalizedDisclosed
		toIsDisclosed := tx.To != nil && strings.ToLower(*tx.To) == normalizedDisclosed
		if fromIsDisclosed && toIsDisclosed {
			pseudoTx.Direction = "self"
		} else if fromIsDisclosed {
			pseudoTx.Direction = "out"
		} else {
			pseudoTx.Direction = "in"
		}

		pseudoTxs = append(pseudoTxs, pseudoTx)
	}

	writeJSON(w, PseudonymizedTransactionsResponse{
		Transactions:    pseudoTxs,
		DisclosureLevel: resolved.DisclosureLevel,
		AddressLabels:   addressLabels,
		HasMore:         hasMore,
	})
}

// generateExternalPseudonym creates a consistent pseudonym for an external address
// The pseudonym is derived from a hash of the address + grantID to ensure consistency
// within the same grant but different across grants
func generateExternalPseudonym(address, grantID string, counter int) string {
	// Use hash to ensure same address always gets same pseudonym within a grant
	h := sha256.New()
	h.Write([]byte(strings.ToLower(address)))
	h.Write([]byte(":"))
	h.Write([]byte(grantID))
	hash := hex.EncodeToString(h.Sum(nil))[:4]
	return "External-" + strings.ToUpper(hash)
}
