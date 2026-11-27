package api

import (
	"errors"
	"net/http"
	"regexp"

	"explorer/internal/txflow"

	"github.com/go-chi/chi/v5"
)

var txHashPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

// NormalizeTxHash adds a missing 0x prefix and reports whether the result is a
// well-formed transaction hash.
func NormalizeTxHash(hash string) (string, bool) {
	if len(hash) == 64 {
		hash = "0x" + hash
	}
	return hash, txHashPattern.MatchString(hash)
}

// IsNotFound reports whether a provider error means the resource does not exist
// (or, in privacy mode, is outside the caller's visibility).
func IsNotFound(err error) bool {
	return errors.Is(err, errProviderNotFound) || errors.Is(err, txflow.ErrNotFound)
}

// handleGetTransactionFlow returns the transaction as ordered plain-language steps.
// It reads only indexed data through the provider, so it never triggers a trace or
// an on-demand index of the block.
func (s *Server) handleGetTransactionFlow(w http.ResponseWriter, r *http.Request) {
	hash, ok := NormalizeTxHash(chi.URLParam(r, "hash"))
	if !ok {
		http.Error(w, "invalid transaction hash", http.StatusBadRequest)
		return
	}
	currency := ""
	if s.cfg != nil {
		currency = s.cfg.NetworkCurrency
	}
	flow, err := txflow.Load(r.Context(), s.provider, hash, txflow.Options{Currency: currency})
	if err != nil {
		if IsNotFound(err) {
			http.Error(w, "transaction not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, flow)
}
