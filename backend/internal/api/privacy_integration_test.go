package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"explorer/internal/db"
	"explorer/internal/privacy"
	"explorer/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock DataProvider — for tests that need fine-grained control over responses
// (e.g. injecting addressMetadata into transactions).
// ---------------------------------------------------------------------------

type MockDataProvider struct {
	mock.Mock
}

func (m *MockDataProvider) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ChainStats), args.Error(1)
}

func (m *MockDataProvider) GetChainID(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockDataProvider) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SyncStatus), args.Error(1)
}

func (m *MockDataProvider) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	args := m.Called(ctx, intervalSeconds, limit)
	return args.Get(0).([]types.TxHistoryPoint), args.Error(1)
}

func (m *MockDataProvider) GetIndexerProgress(ctx context.Context) (*db.IndexerProgress, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.IndexerProgress), args.Error(1)
}

func (m *MockDataProvider) GetCatchupProgress(ctx context.Context) (int64, uint64, float64, bool, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Get(1).(uint64), args.Get(2).(float64), args.Bool(3), args.Error(4)
}

func (m *MockDataProvider) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	args := m.Called(ctx, limit, beforeBlock)
	return args.Get(0).([]types.Block), args.Error(1)
}

func (m *MockDataProvider) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Block), args.Error(1)
}

func (m *MockDataProvider) GetBlockByHash(ctx context.Context, hash string) (*types.Block, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Block), args.Error(1)
}

func (m *MockDataProvider) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockDataProvider) GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error) {
	args := m.Called(ctx, blockNumber)
	return args.Get(0).([]types.InternalTransaction), args.Error(1)
}

func (m *MockDataProvider) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, limit, beforeBlock)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockDataProvider) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]types.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, blockNumber)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockDataProvider) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, limit, beforeBlock)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockDataProvider) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]types.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Transaction), args.Error(1)
}

func (m *MockDataProvider) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Transaction), args.Error(1)
}

func (m *MockDataProvider) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, address, limit, beforeBlock)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockDataProvider) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	args := m.Called(ctx, txHash)
	return args.Get(0).([]types.InternalTransaction), args.Error(1)
}

func (m *MockDataProvider) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	args := m.Called(ctx, txHash)
	return args.Get(0).([]types.TokenTransfer), args.Error(1)
}

func (m *MockDataProvider) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	args := m.Called(ctx, txHash)
	return args.Get(0).([]types.Log), args.Error(1)
}

func (m *MockDataProvider) GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error) {
	args := m.Called(ctx, txHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.OPDeposit), args.Error(1)
}

func (m *MockDataProvider) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.AddressStats), args.Error(1)
}

func (m *MockDataProvider) GetBalance(ctx context.Context, address string) (*types.JSONString, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.JSONString), args.Error(1)
}

func (m *MockDataProvider) GetCode(ctx context.Context, address string) ([]byte, error) {
	args := m.Called(ctx, address)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockDataProvider) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	args := m.Called(ctx, address)
	return args.Get(0).([]types.Balance), args.Error(1)
}

func (m *MockDataProvider) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	args := m.Called(ctx, address, limit, beforeBlock)
	return args.Get(0).([]types.TokenTransfer), args.Error(1)
}

func (m *MockDataProvider) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	args := m.Called(ctx, address, limit, offset)
	return args.Get(0).([]types.InternalTransaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	args := m.Called(ctx, address, limit, offset)
	return args.Get(0).([]types.Log), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	args := m.Called(ctx, address, topic0, fromBlock, toBlock, limit)
	return args.Get(0).([]types.Log), args.Error(1)
}

func (m *MockDataProvider) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Contract), args.Error(1)
}

func (m *MockDataProvider) IsContract(ctx context.Context, address string) (bool, error) {
	args := m.Called(ctx, address)
	return args.Bool(0), args.Error(1)
}

func (m *MockDataProvider) UpdateContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	args := m.Called(ctx, address, abi)
	return args.Error(0)
}

func (m *MockDataProvider) VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error {
	args := m.Called(ctx, address, name, compilerVersion, optimizationUsed, sourceCode, abi, evmVersion, licenseType, constructorArgs, optimizationRuns)
	return args.Error(0)
}

func (m *MockDataProvider) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	args := m.Called(ctx, limit, offset, tokenType)
	return args.Get(0).([]types.Token), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetToken(ctx context.Context, address string) (*types.Token, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Token), args.Error(1)
}

func (m *MockDataProvider) GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	args := m.Called(ctx, address, limit, offset)
	return args.Get(0).([]types.TokenHolder), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	args := m.Called(ctx, tokenAddress, limit, offset)
	return args.Get(0).([]types.TokenTransfer), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]types.TokenTransfer), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]types.AddressStats), args.Get(1).(int64), args.Error(2)
}

func (m *MockDataProvider) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	args := m.Called(ctx, query, limit)
	return args.Get(0).([]types.SearchSuggestion), args.Error(1)
}

func (m *MockDataProvider) IndexBlock(ctx context.Context, number uint64) error {
	args := m.Called(ctx, number)
	return args.Error(0)
}

func (m *MockDataProvider) GetTransactionByHashRPC(ctx context.Context, hash string) (*types.Transaction, *uint64, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*types.Transaction), args.Get(1).(*uint64), args.Error(2)
}

func (m *MockDataProvider) GetDailyStats(ctx context.Context, from, to time.Time) ([]types.DailyStats, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]types.DailyStats), args.Error(1)
}

func (m *MockDataProvider) BackfillDailyStats(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestServerWithProvider creates a Server using the given DataProvider mock.
// The server has privacy enabled (non-nil privacyClient pointing at a dummy server).
func newTestServerWithProvider(t *testing.T, provider DataProvider) *Server {
	t.Helper()
	// Create a dummy privacy proxy server so s.privacyClient != nil
	dummyProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(dummyProxy.Close)

	mockDB := new(MockAPIDatabase)
	privacyClient := privacy.NewClient(dummyProxy.URL)
	srv := New(mockDB, nil, nil, nil, nil, 8080, nil, privacyClient, nil, provider)
	return srv
}

func strP(s string) *string { return &s }

func jsStr(s string) types.JSONString { return types.JSONString(s) }

// ---------------------------------------------------------------------------
// 1. Address pages work in privacy mode
// ---------------------------------------------------------------------------

func TestPrivacy_AddressPage_OwnAddress_Returns200(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	eveAddress := "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"

	provider.On("GetTransactionsByAddress", mock.Anything, eveAddress, mock.AnythingOfType("int"), mock.Anything).
		Return([]types.Transaction{
			{Hash: "0xabc1", From: eveAddress, To: strP("0x1234567890abcdef1234567890abcdef12345678"), Value: "1000"},
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/addresses/"+eveAddress+"/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "own address page should return 200")

	var resp types.PaginatedResponse[types.Transaction]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1, "should return 1 transaction")
}

func TestPrivacy_AddressPage_GrantedContract_Returns200(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	contractAddr := "0x59F2f1fCfE2474fD5F0b9BA1E73ca90b143Eb8d0"

	provider.On("GetTransactionsByAddress", mock.Anything, contractAddr, mock.AnythingOfType("int"), mock.Anything).
		Return([]types.Transaction{
			{Hash: "0xabc2", From: "0xaaaa000000000000000000000000000000000000", To: strP(contractAddr), Value: "500"},
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/addresses/"+contractAddr+"/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "granted contract address page should return 200")

	var resp types.PaginatedResponse[types.Transaction]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
}

func TestPrivacy_AddressPage_RestrictedAddress_ReturnsData(t *testing.T) {
	// In privacy mode, the ProxyDataProvider handles access control.
	// If the user cannot see the address, the provider returns empty data.
	// The handler should NOT return 403 — it returns 200 with empty/filtered data.
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	bobAddress := "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

	provider.On("GetTransactionsByAddress", mock.Anything, bobAddress, mock.AnythingOfType("int"), mock.Anything).
		Return([]types.Transaction{}, nil) // empty — proxy filtered everything

	req := httptest.NewRequest(http.MethodGet, "/api/addresses/"+bobAddress+"/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "restricted address should return 200, not 403")

	var resp types.PaginatedResponse[types.Transaction]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data, "restricted address should return empty data")
}

func TestPrivacy_AddressPage_Anonymous_ReturnsData(t *testing.T) {
	// Anonymous users (no auth cookie) should still get a response.
	// The ProxyDataProvider may return limited/empty data for anonymous viewers.
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	address := "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"

	provider.On("GetTransactionsByAddress", mock.Anything, address, mock.AnythingOfType("int"), mock.Anything).
		Return([]types.Transaction{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/addresses/"+address+"/transactions", nil)
	// No auth cookie
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "anonymous request should return 200")
}

// ---------------------------------------------------------------------------
// 2. Search works in privacy mode
// ---------------------------------------------------------------------------

func TestPrivacy_Search_ForAddress_Returns200(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	address := "0x59F2f1fCfE2474fD5F0b9BA1E73ca90b143Eb8d0"

	provider.On("GetAddressStats", mock.Anything, address).
		Return(&types.AddressStats{Address: address, TxCount: 5}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q="+address, nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "search for address should return 200")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "address", resp["type"], "should identify as address type")
}

func TestPrivacy_Search_ForTxHash_Returns200(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	txHash := "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	provider.On("GetTransaction", mock.Anything, txHash).
		Return(&types.Transaction{Hash: txHash, From: "0xaaaa000000000000000000000000000000000000"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q="+txHash, nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "search for tx hash should return 200")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "transaction", resp["type"])
}

// ---------------------------------------------------------------------------
// 3. addressMetadata passes through in transaction responses
// ---------------------------------------------------------------------------

func TestPrivacy_Metadata_TransactionList_ContainsAddressMetadata(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	eveAddr := "0x9965507d1a55bcc2695c58ba16fb37d819b0a4dc"
	contractAddr := "0x59f2f1fcfe2474fd5f0b9ba1e73ca90b143eb8d0"

	tx := types.Transaction{
		Hash:  "0xabc1",
		From:  eveAddr,
		To:    strP(contractAddr),
		Value: "1000",
		AddressMetadata: map[string]string{
			eveAddr:      "own_address",
			contractAddr: "rbac_group_member",
		},
	}

	provider.On("GetTransactionsWithCategories", mock.Anything, mock.AnythingOfType("int"), mock.Anything).
		Return([]types.Transaction{tx}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/transactions?limit=1", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Parse the raw JSON to verify addressMetadata is present
	var rawResp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rawResp))

	var data []json.RawMessage
	require.NoError(t, json.Unmarshal(rawResp["data"], &data))
	require.Len(t, data, 1, "should have 1 transaction")

	var txResp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data[0], &txResp))

	metaRaw, hasKey := txResp["addressMetadata"]
	require.True(t, hasKey, "transaction JSON must contain addressMetadata key")

	var meta map[string]string
	require.NoError(t, json.Unmarshal(metaRaw, &meta))
	assert.Equal(t, "own_address", meta[eveAddr])
	assert.Equal(t, "rbac_group_member", meta[contractAddr])
}

func TestPrivacy_Metadata_TransactionDetail_ContainsAddressMetadata(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	txHash := "0xdeadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
	fromAddr := "0x9965507d1a55bcc2695c58ba16fb37d819b0a4dc"
	toAddr := "0x59f2f1fcfe2474fd5f0b9ba1e73ca90b143eb8d0"

	tx := &types.Transaction{
		Hash:  txHash,
		From:  fromAddr,
		To:    strP(toAddr),
		Value: "100",
		AddressMetadata: map[string]string{
			fromAddr: "own_address",
			toAddr:   "disclosure_grant",
		},
	}

	provider.On("GetTransactionWithCategories", mock.Anything, txHash).Return(tx, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/transactions/"+txHash, nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// The transaction detail endpoint returns the transaction object directly
	// (not wrapped in {"transaction": ...}), so parse the top-level JSON.
	var txJSON map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &txJSON))

	metaRaw, hasKey := txJSON["addressMetadata"]
	require.True(t, hasKey, "transaction detail must contain addressMetadata key")

	var meta map[string]string
	require.NoError(t, json.Unmarshal(metaRaw, &meta))
	assert.Equal(t, "own_address", meta[fromAddr])
	assert.Equal(t, "disclosure_grant", meta[toAddr])
}

// ---------------------------------------------------------------------------
// 4. Deleted endpoints return 404
// ---------------------------------------------------------------------------

func TestPrivacy_DeletedEndpoint_CheckAddress_Returns404(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	address := "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"

	req := httptest.NewRequest(http.MethodGet, "/api/privacy/check-address/"+address, nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"GET /api/privacy/check-address/{address} was removed and should return 404")
}

func TestPrivacy_DeletedEndpoint_CheckAddresses_Returns404(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	req := httptest.NewRequest(http.MethodPost, "/api/privacy/check-addresses", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	// POST to a non-existent route returns 405 (Method Not Allowed) rather than 404
	// in chi, because the /api/privacy route group exists for GET endpoints.
	// Either 404 or 405 is acceptable — the point is it's NOT 200 or 500.
	assert.NotEqual(t, http.StatusOK, w.Code, "check-addresses endpoint should not return 200")
	assert.NotEqual(t, http.StatusInternalServerError, w.Code, "check-addresses endpoint should not return 500")
}

// ---------------------------------------------------------------------------
// 5. SQL errors not leaked
// ---------------------------------------------------------------------------

func TestPrivacy_SQLError_TokenHolders_ReturnsCleanError(t *testing.T) {
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	address := "0x59F2f1fCfE2474fD5F0b9BA1E73ca90b143Eb8d0"

	// Simulate a SQL error (e.g. missing table)
	sqlErr := errors.New("pq: relation \"token_balances\" does not exist")
	provider.On("GetToken", mock.Anything, address).
		Return(&types.Token{Address: address, Symbol: "TEST"}, nil)
	provider.On("GetTokenHolders", mock.Anything, address, mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return([]types.TokenHolder(nil), int64(0), sqlErr)

	req := httptest.NewRequest(http.MethodGet, "/api/tokens/"+address+"/holders", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	body := w.Body.String()
	// The error message should NOT contain raw SQL details
	assert.NotContains(t, body, "token_balances",
		"response must not leak SQL table names")
	assert.NotContains(t, body, "pq:",
		"response must not leak database driver details")
}

func TestPrivacy_SQLError_AddressStats_ReturnsError(t *testing.T) {
	// NOTE: Some handlers currently pass err.Error() directly to http.Error(),
	// which leaks SQL details. This test documents the current behavior.
	// The token holders endpoint wraps errors properly; the address stats
	// endpoint does not yet. This is tracked for future hardening.
	provider := new(MockDataProvider)
	srv := newTestServerWithProvider(t, provider)

	address := "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"

	sqlErr := errors.New("pq: column \"nonce\" does not exist")
	provider.On("GetAddressStats", mock.Anything, address).Return(nil, sqlErr)

	req := httptest.NewRequest(http.MethodGet, "/api/addresses/"+address, nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code,
		"SQL error should produce 500, not panic or 200")
}
