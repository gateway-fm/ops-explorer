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

func (s *Server) getViewerIdentity(r *http.Request) privacy.ViewerIdentity {
	return privacy.ViewerIdentity{
		DID: s.GetAuthDID(r),
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
		http.Error(w, "failed to get viewable addresses: "+err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "failed to check address visibility: "+err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "failed to check addresses: "+err.Error(), http.StatusInternalServerError)
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

func (s *Server) filterAddressesForVisibility(r *http.Request, addresses []string) map[string]AddressDisplayInfo {
	viewer := s.getViewerIdentity(r)
	result := make(map[string]AddressDisplayInfo)

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

type AddressDisplayInfo struct {
	Address     string                     `json:"address"`
	DisplayName string                     `json:"display_name"`
	IsPrivate   bool                       `json:"is_private"`
	Visibility  *privacy.AddressVisibility `json:"visibility,omitempty"`
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

	stats, err := s.db.GetAddressStats(ctx, resolved.RealAddress)
	if err != nil {
		http.Error(w, "failed to get address stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	balance, err := s.rpc.GetBalance(ctx, common.HexToAddress(resolved.RealAddress))
	if err != nil {
		http.Error(w, "failed to get balance: "+err.Error(), http.StatusInternalServerError)
		return
	}

	code, err := s.rpc.GetCode(ctx, common.HexToAddress(resolved.RealAddress))
	if err != nil {
		http.Error(w, "failed to check contract status: "+err.Error(), http.StatusInternalServerError)
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
		Balance:         balance.String(),
		TxCount:         int64(stats.TxCount),
		IsContract:      len(code) > 0,
	}

	writeJSON(w, response)
}

// SECURITY: All addresses are replaced with pseudonyms - the real addresses are never exposed.
// TxHash is hidden for non-full disclosures to prevent lookup on other explorers.
type PseudonymizedTransaction struct {
	TxHash         *string          `json:"tx_hash,omitempty"`
	BlockNumber    uint64           `json:"block_number"`
	BlockTimestamp uint64           `json:"block_timestamp"`
	From           string           `json:"from"`
	To             *string          `json:"to"`
	Value          types.JSONString `json:"value"`
	GasUsed        uint64           `json:"gas_used"`
	Status         int              `json:"status"`
	Direction string `json:"direction"`
}

type PseudonymizedTransactionsResponse struct {
	Transactions    []PseudonymizedTransaction `json:"transactions"`
	DisclosureLevel string                     `json:"disclosure_level"`
	AddressLabels map[string]string `json:"address_labels"`
	HasMore       bool              `json:"has_more"`
}

// SECURITY: Addresses are pseudonymized based on disclosure_level before being sent to the frontend.
// For non-full disclosures, tx hashes are hidden to prevent lookup on other explorers.
func (s *Server) handleGetGrantedAddressTransactions(w http.ResponseWriter, r *http.Request) {
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

	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	normalizedAddress := common.HexToAddress(resolved.RealAddress).Hex()

	txs, err := s.db.GetTransactionsByAddress(ctx, normalizedAddress, limit+1, beforeBlock)
	if err != nil {
		http.Error(w, "failed to get transactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hasMore := len(txs) > limit
	if hasMore {
		txs = txs[:limit]
	}

	disclosedPseudonym := resolved.RealAddress
	if resolved.DisclosureLevel == "pseudonymous" {
		disclosedPseudonym = resolved.Pseudonym
		if disclosedPseudonym == "" {
			disclosedPseudonym = "Address-Unknown"
		}
	}

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

		if resolved.DisclosureLevel == "full" {
			pseudoTx.TxHash = &tx.Hash
		}

		if strings.ToLower(tx.From) == normalizedDisclosed {
			pseudoTx.From = disclosedPseudonym
		} else {
			pseudoTx.From = getExternalPseudonym(tx.From)
		}

		if tx.To != nil {
			if strings.ToLower(*tx.To) == normalizedDisclosed {
				to := disclosedPseudonym
				pseudoTx.To = &to
			} else {
				to := getExternalPseudonym(*tx.To)
				pseudoTx.To = &to
			}
		}

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

// generateExternalPseudonym creates a consistent pseudonym derived from address + grantID,
// ensuring the same address always maps to the same pseudonym within a grant.
func generateExternalPseudonym(address, grantID string, counter int) string {
	h := sha256.New()
	h.Write([]byte(strings.ToLower(address)))
	h.Write([]byte(":"))
	h.Write([]byte(grantID))
	hash := hex.EncodeToString(h.Sum(nil))[:4]
	return "External-" + strings.ToUpper(hash)
}
