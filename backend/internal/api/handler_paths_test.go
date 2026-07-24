package api

// Meaningful-path handler contract tests (audit §2 / plan §2). Built with a
// keyed &Server{provider: stub} (NOT api.New) + chi route context, mirroring
// privacy_handlers_test.go. Expected values come from the handler contract and
// the parse-helper definitions, not from harvested output. Helpers use the `hp`
// prefix to avoid colliding with the privacy branch (`ph`) and the existing
// `tc` helpers.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"explorer/internal/types"

	"github.com/go-chi/chi/v5"
)

var errHP = errors.New("provider boom")

// hpStub is a configurable api.DataProvider. The embedded nil interface gives
// the full method set; only the funcs a test sets are invoked. A method whose
// func is nil and which gets called will panic with a nil-deref — keeping the
// tests honest about which provider calls each handler makes.
type hpStub struct {
	DataProvider

	chainStats   *types.ChainStats
	chainStatErr error

	txsCat    []types.Transaction
	txsCatErr error
	txsPag    []types.Transaction
	txsPagTot int64
	txsPagErr error

	block     *types.Block
	blockErr  error
	indexErr  error
	blockTxs  []types.Transaction
	addrStats *types.AddressStats
	tx        *types.Transaction
	txErr     error
	contract  *types.Contract
}

type hpRestrictedTxStub struct{ DataProvider }

func (s *hpRestrictedTxStub) CallerScoped() {}
func (s *hpRestrictedTxStub) GetTransactionWithCategories(context.Context, string) (*types.Transaction, error) {
	return nil, errProviderNotFound
}

func (s *hpStub) GetChainStats(context.Context) (*types.ChainStats, error) {
	return s.chainStats, s.chainStatErr
}
func (s *hpStub) GetTransactionsWithCategories(context.Context, int, *uint64) ([]types.Transaction, error) {
	return s.txsCat, s.txsCatErr
}
func (s *hpStub) GetTransactionsPaginatedWithCategories(context.Context, int, int) ([]types.Transaction, int64, error) {
	return s.txsPag, s.txsPagTot, s.txsPagErr
}
func (s *hpStub) GetBlock(context.Context, uint64) (*types.Block, error) {
	return s.block, s.blockErr
}
func (s *hpStub) IndexBlock(context.Context, uint64) error { return s.indexErr }
func (s *hpStub) GetTransactionsByBlock(context.Context, uint64) ([]types.Transaction, error) {
	return s.blockTxs, nil
}
func (s *hpStub) GetAddressStats(context.Context, string) (*types.AddressStats, error) {
	return s.addrStats, nil
}
func (s *hpStub) GetTransaction(context.Context, string) (*types.Transaction, error) {
	return s.tx, s.txErr
}
func (s *hpStub) GetContract(context.Context, string) (*types.Contract, error) {
	return s.contract, nil
}
func (s *hpStub) GetInternalTransactionsByBlock(context.Context, uint64) ([]types.InternalTransaction, error) {
	return nil, nil // nil -> handler must coerce to []
}
func (s *hpStub) GetLogsByTransaction(context.Context, string) ([]types.Log, error) {
	return nil, nil
}

func hpChiReq(method, target string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// =============================================================================
// A — parsing helpers
// =============================================================================

func TestParseLimit_Contract(t *testing.T) {
	// Contract (handlers.go:778): valid 1..100 honored; 0/neg/>100/garbage ->
	// defaultLimit (25). The >100 -> 25 (not clamp-to-100) is surprising; pin it.
	cases := map[string]int{
		"":      defaultLimit,
		"1":     1,
		"100":   100,
		"0":     defaultLimit,
		"-5":    defaultLimit,
		"101":   defaultLimit, // NOT 100
		"99999": defaultLimit,
		"abc":   defaultLimit,
		"10.5":  defaultLimit,
	}
	for q, want := range cases {
		r := httptest.NewRequest(http.MethodGet, "/x?limit="+q, nil)
		if got := parseLimit(r); got != want {
			t.Errorf("parseLimit(limit=%q) = %d, want %d", q, got, want)
		}
	}
}

func TestParseBeforeBlock_Contract(t *testing.T) {
	// Valid uint -> &v; empty/garbage/negative -> nil.
	if got := parseBeforeBlock(httptest.NewRequest(http.MethodGet, "/x?before=42", nil)); got == nil || *got != 42 {
		t.Errorf("before=42 -> %v, want &42", got)
	}
	for _, q := range []string{"", "abc", "-1", "0x10"} {
		if got := parseBeforeBlock(httptest.NewRequest(http.MethodGet, "/x?before="+q, nil)); got != nil {
			t.Errorf("before=%q -> %v, want nil", q, *got)
		}
	}
	// before=0 is a valid uint and must round-trip as &0.
	if got := parseBeforeBlock(httptest.NewRequest(http.MethodGet, "/x?before=0", nil)); got == nil || *got != 0 {
		t.Errorf("before=0 -> %v, want &0", got)
	}
}

// =============================================================================
// A — handleGetTransactions cursor-vs-paginated fork
// =============================================================================

func TestHandleGetTransactions_CursorMode(t *testing.T) {
	// No ?page= -> cursor fork -> GetTransactionsWithCategories; envelope is the
	// cursor PaginatedResponse (has hasMore, not total/totalPages).
	s := &Server{provider: &hpStub{txsCat: []types.Transaction{{Hash: "0xa", BlockNumber: 5}}}}
	w := httptest.NewRecorder()
	s.handleGetTransactions(w, httptest.NewRequest(http.MethodGet, "/api/transactions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var env map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if _, ok := env["hasMore"]; !ok {
		t.Errorf("cursor mode envelope missing hasMore: %s", w.Body.String())
	}
	if _, ok := env["totalPages"]; ok {
		t.Errorf("cursor mode envelope must NOT have totalPages: %s", w.Body.String())
	}
}

func TestHandleGetTransactions_PaginatedFork(t *testing.T) {
	// ?page= present -> paginated fork -> offset envelope with total/totalPages.
	s := &Server{provider: &hpStub{
		txsPag:    []types.Transaction{{Hash: "0xa", BlockNumber: 5}},
		txsPagTot: 53,
	}}
	w := httptest.NewRecorder()
	s.handleGetTransactions(w, httptest.NewRequest(http.MethodGet, "/api/transactions?page=2&pageSize=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp types.OffsetPaginatedResponse[types.Transaction]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 53 || resp.Page != 2 || resp.PageSize != 10 {
		t.Errorf("envelope = %+v, want total=53 page=2 pageSize=10", resp)
	}
	if resp.TotalPages != 6 { // ceil(53/10)
		t.Errorf("TotalPages = %d, want 6", resp.TotalPages)
	}
}

func TestHandleGetTransactions_ProviderError500(t *testing.T) {
	s := &Server{provider: &hpStub{txsCatErr: errHP}}
	w := httptest.NewRecorder()
	s.handleGetTransactions(w, httptest.NewRequest(http.MethodGet, "/api/transactions", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", w.Code)
	}
	// Audit §2B flag: the raw provider error string is leaked to clients. Lock
	// the current behavior so a future redaction is a conscious change.
	if !strings.Contains(w.Body.String(), errHP.Error()) {
		t.Errorf("body = %q, expected to (currently) leak provider error %q", w.Body.String(), errHP.Error())
	}
}

// =============================================================================
// A — handleGetBlock validation + not-found -> IndexBlock ladder
// =============================================================================

func TestHandleGetBlock_InvalidNumber400(t *testing.T) {
	s := &Server{provider: &hpStub{}}
	w := httptest.NewRecorder()
	s.handleGetBlock(w, hpChiReq(http.MethodGet, "/api/blocks/notanumber", map[string]string{"number": "notanumber"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestHandleGetBlock_NotFoundThenIndexFails404(t *testing.T) {
	// block nil + IndexBlock error -> 404.
	s := &Server{provider: &hpStub{block: nil, indexErr: errHP}}
	w := httptest.NewRecorder()
	s.handleGetBlock(w, hpChiReq(http.MethodGet, "/api/blocks/99", map[string]string{"number": "99"}))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", w.Code)
	}
}

func TestHandleGetBlock_Success(t *testing.T) {
	s := &Server{provider: &hpStub{
		block:    &types.Block{Number: 7},
		blockTxs: []types.Transaction{{Hash: "0xa"}},
	}}
	w := httptest.NewRecorder()
	s.handleGetBlock(w, hpChiReq(http.MethodGet, "/api/blocks/7", map[string]string{"number": "7"}))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var env struct {
		Block        *types.Block        `json:"block"`
		Transactions []types.Transaction `json:"transactions"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Block == nil || env.Block.Number != 7 || len(env.Transactions) != 1 {
		t.Errorf("envelope = %+v", env)
	}
}

func TestHandleGetTransaction_CallerScopedNotFoundReturnsOpaque404WithoutReindex(t *testing.T) {
	s := &Server{provider: &hpRestrictedTxStub{}}
	w := httptest.NewRecorder()
	s.handleGetTransaction(w, hpChiReq(
		http.MethodGet,
		"/api/transactions/0xabc",
		map[string]string{"hash": "0xabc"},
	))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want privacy-preserving 404 (body=%s)", w.Code, w.Body.String())
	}
}

// =============================================================================
// A — handleSearch classification
// =============================================================================

func TestHandleSearch_Classification(t *testing.T) {
	const addr = "0x52908400098527886E0F7030069857D2E4169EE7"
	const txHash = "0x" + "ab" + "00000000000000000000000000000000000000000000000000000000000000" // 66 chars

	t.Run("empty-400", func(t *testing.T) {
		s := &Server{provider: &hpStub{}}
		w := httptest.NewRecorder()
		s.handleSearch(w, httptest.NewRequest(http.MethodGet, "/api/search?q=%20", nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400", w.Code)
		}
	})

	t.Run("numeric-block", func(t *testing.T) {
		s := &Server{provider: &hpStub{block: &types.Block{Number: 100}}}
		w := httptest.NewRecorder()
		s.handleSearch(w, httptest.NewRequest(http.MethodGet, "/api/search?q=100", nil))
		assertSearchType(t, w, "block")
	})

	t.Run("66hex-transaction", func(t *testing.T) {
		if len(txHash) != 66 {
			t.Fatalf("test hash is %d chars, need 66", len(txHash))
		}
		s := &Server{provider: &hpStub{tx: &types.Transaction{Hash: txHash}}}
		w := httptest.NewRecorder()
		s.handleSearch(w, httptest.NewRequest(http.MethodGet, "/api/search?q="+txHash, nil))
		assertSearchType(t, w, "transaction")
	})

	t.Run("hex-address", func(t *testing.T) {
		s := &Server{provider: &hpStub{addrStats: &types.AddressStats{Address: addr}}}
		w := httptest.NewRecorder()
		s.handleSearch(w, httptest.NewRequest(http.MethodGet, "/api/search?q="+addr, nil))
		assertSearchType(t, w, "address")
	})

	t.Run("garbage-404", func(t *testing.T) {
		// Not numeric, not 66-hex, not an address -> 404. (No provider calls.)
		s := &Server{provider: &hpStub{}}
		w := httptest.NewRecorder()
		s.handleSearch(w, httptest.NewRequest(http.MethodGet, "/api/search?q=hello-world", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status %d, want 404", w.Code)
		}
	})
}

func assertSearchType(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Type != want {
		t.Errorf("type = %q, want %q", env.Type, want)
	}
}

// =============================================================================
// Lock current reality: V2 pagination is a no-op (server.go:297-300).
// setupAPIV2Routes just calls setupAPIRoutes, so /api/v2 is byte-identical to
// /api/v1 despite the comment claiming v2 "adjusts pagination". Pin it so a
// future divergence is a conscious change, not an accident.
// =============================================================================

func TestAPIV2RoutesIdenticalToV1(t *testing.T) {
	s := &Server{} // route registration touches no providers

	walk := func(setup func(chi.Router)) []string {
		r := chi.NewRouter()
		setup(r)
		var routes []string
		_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			routes = append(routes, method+" "+route)
			return nil
		})
		return routes
	}

	v1 := walk(s.setupAPIRoutes)
	v2 := walk(s.setupAPIV2Routes)
	sort.Strings(v1)
	sort.Strings(v2)

	if len(v1) != len(v2) {
		t.Fatalf("v1 has %d routes, v2 has %d — V2 is supposed to be a no-op copy", len(v1), len(v2))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Errorf("route set differs at %d: v1=%q v2=%q", i, v1[i], v2[i])
		}
	}
}

// =============================================================================
// B — nil slice -> [] (must never marshal as null)
// =============================================================================

func TestHandlers_NilSliceMarshalsAsEmptyArray(t *testing.T) {
	s := &Server{provider: &hpStub{}}

	t.Run("block-internal-txs", func(t *testing.T) {
		w := httptest.NewRecorder()
		s.handleGetBlockInternalTxs(w, hpChiReq(http.MethodGet, "/api/blocks/5/internal", map[string]string{"number": "5"}))
		assertJSONArrayNotNull(t, w)
	})

	t.Run("transaction-logs", func(t *testing.T) {
		w := httptest.NewRecorder()
		s.handleGetTransactionLogs(w, hpChiReq(http.MethodGet, "/api/transactions/0xabc/logs", map[string]string{"hash": "0xabc"}))
		assertJSONArrayNotNull(t, w)
	})
}

func assertJSONArrayNotNull(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := strings.TrimSpace(w.Body.String())
	if body == "null" {
		t.Fatalf("body is null, want [] (nil slice must coerce to empty array)")
	}
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("body %q is not a JSON array: %v", body, err)
	}
	if len(arr) != 0 {
		t.Errorf("array len = %d, want 0", len(arr))
	}
}

// =============================================================================
// C — handleGetStats: error -> minimal stats with PrivacyEnabled preserved
// =============================================================================

func TestHandleGetStats_ErrorStillSetsPrivacyEnabled(t *testing.T) {
	// Contract (handlers.go:27-49): a provider error must NOT fail the request;
	// it returns a zero-value ChainStats with PrivacyEnabled set from whether a
	// privacyClient is wired. Here standalone (no privacyClient) -> false.
	s := &Server{provider: &hpStub{chainStatErr: errHP}}
	w := httptest.NewRecorder()
	s.handleGetStats(w, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (graceful)", w.Code)
	}
	var stats types.ChainStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats.PrivacyEnabled {
		t.Error("PrivacyEnabled = true, want false (no privacyClient = standalone)")
	}
	if stats.TotalBlocks != 0 {
		t.Errorf("TotalBlocks = %d, want 0 (minimal stats on error)", stats.TotalBlocks)
	}
}
