package api

import (
	"context"
	"encoding/json"
	"explorer/internal/db"
	"explorer/internal/types"
)

// APIDatabase defines the subset of database methods needed by the API handlers
type APIDatabase interface {
	GetChainStats(ctx context.Context) (*types.ChainStats, error)
	GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error)
	GetBlock(ctx context.Context, number uint64) (*types.Block, error)
	GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error)
	GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error)
	GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error)
	GetTransaction(ctx context.Context, hash string) (*types.Transaction, error)
	GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error)
	GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error)
	GetContract(ctx context.Context, address string) (*types.Contract, error)
	InsertContract(ctx context.Context, c *types.Contract) error
	SetContractABI(ctx context.Context, address string, abi json.RawMessage) error
	GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error)
	GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error)
	GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error)
	GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error)
	GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error)
	SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error)
	GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error)
	GetToken(ctx context.Context, address string) (*types.Token, error)
	GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error)
	GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error)
	GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error)
	GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error)
	GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error)
	GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error)
	GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error)
	GetIndexerProgress(ctx context.Context) (*db.IndexerProgress, error)
	GetLatestBlockNumber(ctx context.Context) (uint64, error)
	GetSyncStatus(ctx context.Context) (*types.SyncStatus, error)
	GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error)
	GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error)
	VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error

	// Complex Category queries
	GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error)
	GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error)
	GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error)
	GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error)
	IsContract(ctx context.Context, address string) (bool, error)
	GetGasPercentiles(ctx context.Context, numBlocks int, slowPct, avgPct, fastPct float64) (*db.GasPercentiles, error)
}
