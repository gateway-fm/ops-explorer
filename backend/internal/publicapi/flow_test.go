package publicapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"explorer/internal/api"
	"explorer/internal/types"
)

type flowProvider struct {
	api.DataProvider
	tx *types.Transaction
}

func (p *flowProvider) GetTransaction(context.Context, string) (*types.Transaction, error) {
	return p.tx, nil
}
func (p *flowProvider) GetInternalTransactionsByTx(context.Context, string) ([]types.InternalTransaction, error) {
	return nil, nil
}
func (p *flowProvider) GetLogsByTransaction(context.Context, string) ([]types.Log, error) {
	return nil, nil
}
func (p *flowProvider) GetTransfersByTransaction(context.Context, string) ([]types.TokenTransfer, error) {
	return nil, nil
}
func (p *flowProvider) GetContract(context.Context, string) (*types.Contract, error) { return nil, nil }
func (p *flowProvider) GetToken(context.Context, string) (*types.Token, error)       { return nil, nil }

func TestTransactionFlowRoute(t *testing.T) {
	hash := "0x" + strings.Repeat("ab", 32)
	to := "0x2222222222222222222222222222222222222222"
	s := tcServer(&flowProvider{tx: &types.Transaction{Hash: hash, From: to, To: &to, Value: "2000000000000000000", Status: 1}})
	s.SetNetworkCurrency("tTEST")

	w := tcReq(s, "/api/v1/transactions/"+hash+"/flow", "10.9.0.1:1")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "sent 2 tTEST") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if w := tcReq(s, "/api/v1/transactions/0xzz/flow", "10.9.0.2:1"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad hash status %d", w.Code)
	}
	s = tcServer(&flowProvider{})
	if w := tcReq(s, "/api/v1/transactions/"+hash+"/flow", "10.9.0.3:1"); w.Code != http.StatusNotFound {
		t.Fatalf("missing tx status %d", w.Code)
	}
}
