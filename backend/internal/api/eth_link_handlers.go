package api

import (
	"encoding/json"
	"net/http"

	"explorer/internal/privacy"
	"explorer/pkg/log"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetLinkedAddresses(w http.ResponseWriter, r *http.Request) {
	token := s.GetAuthToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	result, err := s.privacyClient.GetLinkedAddresses(r.Context(), token)
	if err != nil {
		log.Warn("eth-link: get linked addresses via privacy-proxy failed", "error", err)
		writeError(w, http.StatusBadGateway, "failed to get linked addresses")
		return
	}

	writeJSON(w, result)
}

func (s *Server) handleCreateLinkChallenge(w http.ResponseWriter, r *http.Request) {
	token := s.GetAuthToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	result, err := s.privacyClient.CreateLinkChallenge(r.Context(), token)
	if err != nil {
		log.Warn("eth-link: create challenge via privacy-proxy failed", "error", err)
		writeError(w, http.StatusBadGateway, "failed to create link challenge")
		return
	}

	writeJSON(w, result)
}

func (s *Server) handleVerifyLink(w http.ResponseWriter, r *http.Request) {
	token := s.GetAuthToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req privacy.VerifyLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Nonce == "" || req.Address == "" || req.Signature == "" {
		writeError(w, http.StatusBadRequest, "nonce, address, and signature are required")
		return
	}

	result, err := s.privacyClient.VerifyLink(r.Context(), token, req)
	if err != nil {
		// P-3: do NOT log the wallet address being linked — pairs a DID with a
		// real on-chain address in the explorer's own stderr.
		log.Warn("eth-link: verify signature via privacy-proxy failed", "error", err)
		writeError(w, http.StatusBadGateway, "failed to verify link")
		return
	}

	writeJSON(w, result)
}

func (s *Server) handleUnlinkAddress(w http.ResponseWriter, r *http.Request) {
	token := s.GetAuthToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	address := chi.URLParam(r, "address")
	if address == "" {
		writeError(w, http.StatusBadRequest, "address parameter required")
		return
	}

	if err := s.privacyClient.UnlinkAddress(r.Context(), token, address); err != nil {
		// P-3: do NOT log the wallet address being unlinked (see handleVerifyLink).
		log.Warn("eth-link: unlink address via privacy-proxy failed", "error", err)
		writeError(w, http.StatusBadGateway, "failed to unlink address")
		return
	}

	writeJSON(w, map[string]bool{"success": true})
}
