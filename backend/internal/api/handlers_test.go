package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/internal/db"
	"explorer/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAPIDatabase is a mock implementation of APIDatabase
type MockAPIDatabase struct {
	mock.Mock
}

func (m *MockAPIDatabase) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ChainStats), args.Error(1)
}

func (m *MockAPIDatabase) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	args := m.Called(ctx, limit, beforeBlock)
	return args.Get(0).([]types.Block), args.Error(1)
}

func (m *MockAPIDatabase) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Block), args.Error(1)
}

func (m *MockAPIDatabase) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, limit, beforeBlock)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockAPIDatabase) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]types.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, blockNumber)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockAPIDatabase) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Transaction), args.Error(1)
}

func (m *MockAPIDatabase) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.AddressStats), args.Error(1)
}

func (m *MockAPIDatabase) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, address, limit, beforeBlock)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockAPIDatabase) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Contract), args.Error(1)
}

func (m *MockAPIDatabase) InsertContract(ctx context.Context, c *types.Contract) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *MockAPIDatabase) SetContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	args := m.Called(ctx, address, abi)
	return args.Error(0)
}

func (m *MockAPIDatabase) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	args := m.Called(ctx, address, limit, beforeBlock)
	return args.Get(0).([]types.TokenTransfer), args.Error(1)
}

func (m *MockAPIDatabase) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	args := m.Called(ctx, txHash)
	return args.Get(0).([]types.TokenTransfer), args.Error(1)
}

func (m *MockAPIDatabase) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	args := m.Called(ctx, txHash)
	return args.Get(0).([]types.Log), args.Error(1)
}

func (m *MockAPIDatabase) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]types.AddressStats), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	args := m.Called(ctx, intervalSeconds, limit)
	return args.Get(0).([]types.TxHistoryPoint), args.Error(1)
}

func (m *MockAPIDatabase) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	args := m.Called(ctx, query, limit)
	return args.Get(0).([]types.SearchSuggestion), args.Error(1)
}

func (m *MockAPIDatabase) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	args := m.Called(ctx, limit, offset, tokenType)
	return args.Get(0).([]types.Token), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetToken(ctx context.Context, address string) (*types.Token, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Token), args.Error(1)
}

func (m *MockAPIDatabase) GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	args := m.Called(ctx, address, limit, offset)
	return args.Get(0).([]types.TokenHolder), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	args := m.Called(ctx, tokenAddress, limit, offset)
	return args.Get(0).([]types.TokenTransfer), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]types.TokenTransfer), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	args := m.Called(ctx, txHash)
	return args.Get(0).([]types.InternalTransaction), args.Error(1)
}

func (m *MockAPIDatabase) GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error) {
	args := m.Called(ctx, blockNumber)
	return args.Get(0).([]types.InternalTransaction), args.Error(1)
}

func (m *MockAPIDatabase) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	args := m.Called(ctx, address, limit, offset)
	return args.Get(0).([]types.InternalTransaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	args := m.Called(ctx, address, topic0, fromBlock, toBlock, limit)
	return args.Get(0).([]types.Log), args.Error(1)
}

func (m *MockAPIDatabase) GetIndexerProgress(ctx context.Context) (*db.IndexerProgress, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.IndexerProgress), args.Error(1)
}

func (m *MockAPIDatabase) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockAPIDatabase) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SyncStatus), args.Error(1)
}

func (m *MockAPIDatabase) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	args := m.Called(ctx, address, limit, offset)
	return args.Get(0).([]types.Log), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	args := m.Called(ctx, address)
	return args.Get(0).([]types.Balance), args.Error(1)
}

func (m *MockAPIDatabase) VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error {
	args := m.Called(ctx, address, name, compilerVersion, optimizationUsed, sourceCode, abi, evmVersion, licenseType, constructorArgs, optimizationRuns)
	return args.Error(0)
}

func (m *MockAPIDatabase) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	args := m.Called(ctx, limit, beforeBlock)
	return args.Get(0).([]types.Transaction), args.Error(1)
}

func (m *MockAPIDatabase) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]types.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockAPIDatabase) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Transaction), args.Error(1)
}

func (m *MockAPIDatabase) GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error) {
	args := m.Called(ctx, txHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.OPDeposit), args.Error(1)
}

func (m *MockAPIDatabase) IsContract(ctx context.Context, address string) (bool, error) {
	args := m.Called(ctx, address)
	return args.Bool(0), args.Error(1)
}

func (m *MockAPIDatabase) GetGasPercentiles(ctx context.Context, numBlocks int, slowPct, avgPct, fastPct float64) (*db.GasPercentiles, error) {
	args := m.Called(ctx, numBlocks, slowPct, avgPct, fastPct)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.GasPercentiles), args.Error(1)
}

func TestHandleGetStats(t *testing.T) {
	mockDB := new(MockAPIDatabase)
	srv := New(mockDB, nil, nil, nil, nil, 8080, nil, nil, nil)

	stats := &types.ChainStats{
		TotalBlocks:       100,
		TotalTransactions: 1000,
	}

	mockDB.On("GetChainStats", mock.Anything).Return(stats, nil)

	req, _ := http.NewRequest("GET", "/api/stats", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response types.ChainStats
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, stats.TotalBlocks, response.TotalBlocks)
}

func TestHandleGetBlock(t *testing.T) {
	mockDB := new(MockAPIDatabase)
	srv := New(mockDB, nil, nil, nil, nil, 8080, nil, nil, nil)

	block := &types.Block{
		Number: 123,
		Hash:   "0xabc",
	}

	mockDB.On("GetBlock", mock.Anything, uint64(123)).Return(block, nil)
	mockDB.On("GetTransactionsByBlock", mock.Anything, uint64(123)).Return([]types.Transaction{}, nil)
	mockDB.On("GetInternalTransactionsByBlock", mock.Anything, uint64(123)).Return([]types.InternalTransaction{}, nil)

	req, _ := http.NewRequest("GET", "/api/v1/blocks/123", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response struct {
		Block        types.Block         `json:"block"`
		Transactions []types.Transaction `json:"transactions"`
	}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, block.Number, response.Block.Number)
	assert.Equal(t, block.Hash, response.Block.Hash)
}
