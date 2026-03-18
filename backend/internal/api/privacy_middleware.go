package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"explorer/internal/privacy"
	"explorer/internal/types"

	"github.com/go-chi/chi/v5"
)

// isPrivacyAuthenticated returns true if privacy is disabled OR the user has a valid DID.
func (s *Server) isPrivacyAuthenticated(r *http.Request) bool {
	if s.privacyClient == nil || !s.privacyClient.IsEnabled() {
		return true
	}
	return s.GetAuthDID(r) != ""
}

// privacyAuthGateMiddleware returns 403 JSON error if privacy is enabled and user is unauthenticated.
func (s *Server) privacyAuthGateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isPrivacyAuthenticated(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "authentication required — sign in via Privado to access this data",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

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

// stripBlockForPrivacy returns a copy of the block with only Number, Hash, and Timestamp populated.
// Address fields use the [PRIVATE] placeholder so the frontend renders them with a padlock icon.
func stripBlockForPrivacy(b *types.Block) types.Block {
	return types.Block{
		Number:    b.Number,
		Hash:      b.Hash,
		Timestamp: b.Timestamp,
		Miner:     "[PRIVATE]",
	}
}

// isPrivateAddress returns true if an address is the [PRIVATE] placeholder
// injected by the privacy proxy during RPC redaction.
func isPrivateAddress(addr string) bool {
	return strings.EqualFold(addr, "[private]") || strings.HasPrefix(strings.ToLower(addr), "[private")
}

// filterTransactionsForPrivacy removes transactions where both from and to addresses
// are hidden from the viewer. An address is considered hidden if it is the [PRIVATE]
// placeholder (redacted by the RPC proxy) or if the privacy proxy says it's not visible.
// Returns the original slice unmodified if privacy is disabled.
func (s *Server) filterTransactionsForPrivacy(r *http.Request, txs []types.Transaction) []types.Transaction {
	if s.privacyClient == nil || !s.privacyClient.IsEnabled() {
		return txs
	}
	if len(txs) == 0 {
		return txs
	}

	viewer := s.getViewerIdentity(r)
	if viewer.DID == "" {
		return []types.Transaction{}
	}

	// Collect unique real addresses (skip [PRIVATE] placeholders)
	addrSet := make(map[string]bool)
	for _, tx := range txs {
		if tx.From != "" && !isPrivateAddress(tx.From) {
			addrSet[strings.ToLower(tx.From)] = true
		}
		if tx.To != nil && *tx.To != "" && !isPrivateAddress(*tx.To) {
			addrSet[strings.ToLower(*tx.To)] = true
		}
	}

	// Batch check visibility for real addresses
	var visibilities map[string]*privacy.AddressVisibility
	addresses := make([]string, 0, len(addrSet))
	for addr := range addrSet {
		addresses = append(addresses, addr)
	}
	if len(addresses) > 0 {
		var err error
		visibilities, err = s.privacyClient.CheckAddressesWithIdentity(r.Context(), viewer, addresses)
		if err != nil {
			// Fail closed
			return []types.Transaction{}
		}
	}

	// Filter: remove txs where both from AND to are hidden
	filtered := make([]types.Transaction, 0, len(txs))
	for _, tx := range txs {
		fromHidden := isPrivateAddress(tx.From)
		if !fromHidden {
			if vis, ok := visibilities[strings.ToLower(tx.From)]; ok && !vis.Visible {
				fromHidden = true
			}
		}

		toHidden := false
		if tx.To != nil && *tx.To != "" {
			toHidden = isPrivateAddress(*tx.To)
			if !toHidden {
				if vis, ok := visibilities[strings.ToLower(*tx.To)]; ok && !vis.Visible {
					toHidden = true
				}
			}
		}

		// Contract creations (to == nil): only check from
		if tx.To == nil {
			toHidden = fromHidden
		}

		if fromHidden && toHidden {
			continue
		}
		filtered = append(filtered, tx)
	}

	return filtered
}
