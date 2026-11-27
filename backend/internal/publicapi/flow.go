package publicapi

import (
	"net/http"

	"explorer/internal/api"
	"explorer/internal/txflow"

	"github.com/go-chi/chi/v5"
)

// handleGetTransactionFlow returns the transaction as ordered plain-language steps,
// built only from indexed data.
func (s *Server) handleGetTransactionFlow(w http.ResponseWriter, r *http.Request) {
	hash, ok := api.NormalizeTxHash(chi.URLParam(r, "hash"))
	if !ok {
		writeError(w, "invalid transaction hash", http.StatusBadRequest)
		return
	}
	flow, err := txflow.Load(r.Context(), s.provider, hash, txflow.Options{Currency: s.currency})
	if err != nil {
		if api.IsNotFound(err) {
			writeError(w, "transaction not found", http.StatusNotFound)
			return
		}
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, flow)
}
