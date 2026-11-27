package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"explorer/internal/txflow"
	"explorer/internal/types"
)

const flowTestHash = "0xabababababababababababababababababababababababababababababababab"

// flowStub serves one transaction. It does not implement IndexBlock or any RPC
// method: the flow endpoint must never index on demand or reach the node.
type flowStub struct {
	DataProvider
	tx    *types.Transaction
	txErr error
}

func (s *flowStub) GetTransaction(context.Context, string) (*types.Transaction, error) {
	return s.tx, s.txErr
}
func (s *flowStub) GetInternalTransactionsByTx(context.Context, string) ([]types.InternalTransaction, error) {
	return nil, nil
}
func (s *flowStub) GetLogsByTransaction(context.Context, string) ([]types.Log, error) {
	return nil, nil
}
func (s *flowStub) GetTransfersByTransaction(context.Context, string) ([]types.TokenTransfer, error) {
	return nil, nil
}
func (s *flowStub) GetContract(context.Context, string) (*types.Contract, error) { return nil, nil }
func (s *flowStub) GetToken(context.Context, string) (*types.Token, error)       { return nil, nil }

func TestHandleGetTransactionFlow(t *testing.T) {
	to := "0x2222222222222222222222222222222222222222"
	tx := &types.Transaction{Hash: flowTestHash, From: "0x1111111111111111111111111111111111111111", To: &to, Value: "1000000000000000000", Status: 1}

	t.Run("ok", func(t *testing.T) {
		s := &Server{provider: &flowStub{tx: tx}, cfg: &ServerConfig{NetworkCurrency: "tTEST"}}
		w := httptest.NewRecorder()
		s.handleGetTransactionFlow(w, hpChiReq(http.MethodGet, "/", map[string]string{"hash": flowTestHash}))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		var f txflow.Flow
		if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
			t.Fatal(err)
		}
		if len(f.Steps) != 1 || !strings.Contains(f.Steps[0].Text, "sent 1 tTEST") {
			t.Fatalf("steps = %+v", f.Steps)
		}
	})

	t.Run("hash without 0x", func(t *testing.T) {
		s := &Server{provider: &flowStub{tx: tx}}
		w := httptest.NewRecorder()
		s.handleGetTransactionFlow(w, hpChiReq(http.MethodGet, "/", map[string]string{"hash": strings.TrimPrefix(flowTestHash, "0x")}))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "sent 1 ETH") {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("bad hash", func(t *testing.T) {
		s := &Server{provider: &flowStub{tx: tx}}
		w := httptest.NewRecorder()
		s.handleGetTransactionFlow(w, hpChiReq(http.MethodGet, "/", map[string]string{"hash": "0xabc"}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d", w.Code)
		}
	})

	t.Run("not indexed is 404 without indexing", func(t *testing.T) {
		s := &Server{provider: &flowStub{}}
		w := httptest.NewRecorder()
		s.handleGetTransactionFlow(w, hpChiReq(http.MethodGet, "/", map[string]string{"hash": flowTestHash}))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status %d", w.Code)
		}
	})

	t.Run("caller-scoped denial is 404", func(t *testing.T) {
		s := &Server{provider: &flowStub{txErr: errProviderNotFound}}
		w := httptest.NewRecorder()
		s.handleGetTransactionFlow(w, hpChiReq(http.MethodGet, "/", map[string]string{"hash": flowTestHash}))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status %d", w.Code)
		}
	})
}
