package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"explorer/internal/privacy"

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
		slog.Warn("failed to get linked addresses from proxy", "error", err)
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
		slog.Warn("failed to create link challenge from proxy", "error", err)
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
		slog.Warn("failed to verify link with proxy", "error", err)
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
		slog.Warn("failed to unlink address via proxy", "error", err)
		writeError(w, http.StatusBadGateway, "failed to unlink address")
		return
	}

	writeJSON(w, map[string]bool{"success": true})
}
