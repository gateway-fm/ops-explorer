package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type invalidatingFake struct {
	*DirectDBProvider
	invalidated []string
}

func (f *invalidatingFake) UpdateContractABI(context.Context, string, json.RawMessage) error {
	return nil
}

func (f *invalidatingFake) InvalidateContract(address string) {
	f.invalidated = append(f.invalidated, address)
}

func TestUpdateContractABIInvalidatesContractCache(t *testing.T) {
	fake := &invalidatingFake{}
	s := &Server{provider: fake}

	const addr = "0x6b7fed77a857e39510d19dd96a553b1387120d3e"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"abi":[]}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("address", addr)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	s.handleUpdateContractABI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if len(fake.invalidated) != 1 || !strings.EqualFold(fake.invalidated[0], addr) {
		t.Fatalf("invalidated = %v, want exactly %s", fake.invalidated, addr)
	}
}

func TestInvalidateContractWithoutCacheIsNoop(t *testing.T) {
	s := &Server{provider: &DirectDBProvider{}}
	s.invalidateContract("0x6b7fed77a857e39510d19dd96a553b1387120d3e")
}
