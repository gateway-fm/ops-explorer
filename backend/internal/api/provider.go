package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"explorer/internal/db"
	"explorer/internal/rpc"
	"explorer/internal/types"

	"explorer/pkg/eth/common"
)

// DataProvider is the read surface the api handlers depend on. Implementations:
//
//   - *DirectDBProvider  — minimal SQL-backed provider that only answers
//                          the block-explorer-local concerns (contract
//                          verification write paths, node-RPC helpers).
//                          Chain-data reads return ErrChainDataNotAvailable.
//                          NEVER use this alone in production; pair with
//                          indexerclient.Provider or use ProxyDataProvider.
//   - *ProxyDataProvider — proxies to privacy-proxy's REST API.
//   - *indexerclient.Provider — gRPC to chain-indexer for reads, embeds
//                               *DirectDBProvider for writes + verification.
type DataProvider interface {
	GetChainStats(ctx context.Context) (*types.ChainStats, error)
	GetChainID(ctx context.Context) (uint64, error)
	GetSyncStatus(ctx context.Context) (*types.SyncStatus, error)
	GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error)
	GetCatchupProgress(ctx context.Context) (processed int64, total uint64, percentComplete float64, isRunning bool, err error)

	GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error)
	GetBlock(ctx context.Context, number uint64) (*types.Block, error)
	GetBlockByHash(ctx context.Context, hash string) (*types.Block, error)
	GetLatestBlockNumber(ctx context.Context) (uint64, error)
	GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error)

	GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error)
	GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error)
	GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error)
	GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error)
	GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error)
	GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error)
	GetTransaction(ctx context.Context, hash string) (*types.Transaction, error)
	GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error)
	GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error)
	GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error)
	GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error)
	GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error)

	GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error)
	GetBalance(ctx context.Context, address string) (*types.JSONString, error)
	GetCode(ctx context.Context, address string) ([]byte, error)
	GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error)
	GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error)
	GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error)
	GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error)
	GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error)
	GetContract(ctx context.Context, address string) (*types.Contract, error)
	IsContract(ctx context.Context, address string) (bool, error)
	UpdateContractABI(ctx context.Context, address string, abi json.RawMessage) error
	VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error

	GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error)
	GetToken(ctx context.Context, address string) (*types.Token, error)
	GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error)
	GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error)
	GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error)

	GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error)
	SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error)

	IndexBlock(ctx context.Context, number uint64) error
	GetTransactionByHashRPC(ctx context.Context, hash string) (*types.Transaction, *uint64, error)

	GetDailyStats(ctx context.Context, from, to time.Time) ([]types.DailyStats, error)
	BackfillDailyStats(ctx context.Context) error

	GetGasPrices(ctx context.Context, numBlocks int) (slow, normal, fast *uint64, baseFee *uint64, err error)
}

// ErrChainDataNotAvailable is returned by DirectDBProvider for any chain-
// data read — the method signatures still exist so DirectDBProvider
// satisfies DataProvider, but chain data lives in chain-indexer now and
// needs the indexerclient.Provider wrapper (or ProxyDataProvider).
var ErrChainDataNotAvailable = errors.New("chain-data requires chain-indexer (INDEXER_URL) or privacy-proxy (PRIVACY_PROXY_URL); direct-DB mode no longer serves chain reads")

// DirectDBProvider implements the verification + node-RPC methods directly
// against block-explorer's DB and RPC client. All chain-data read methods
// return ErrChainDataNotAvailable — those must be served by a wrapping
// provider (indexerclient.Provider) or a routed provider (ProxyDataProvider).
type DirectDBProvider struct {
	db  *db.DB
	rpc *rpc.Client
}

// NewDirectDBProvider constructs the SQL+RPC-backed provider. The idx
// parameter is retained for call-site compatibility but ignored.
func NewDirectDBProvider(database *db.DB, rpcClient *rpc.Client, idx any) *DirectDBProvider {
	_ = idx
	return &DirectDBProvider{db: database, rpc: rpcClient}
}

// ----- Chain-data reads: all return ErrChainDataNotAvailable. -----

func (p *DirectDBProvider) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetCatchupProgress(ctx context.Context) (int64, uint64, float64, bool, error) {
	// Not an error; block-explorer's own indexer is retired and a running
	// chain-indexer is cared for elsewhere. Report "not running" cleanly.
	return 0, 0, 0, false, nil
}
func (p *DirectDBProvider) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetBlockByHash(ctx context.Context, hash string) (*types.Block, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	return 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) IsContract(ctx context.Context, address string) (bool, error) {
	return false, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetToken(ctx context.Context, address string) (*types.Token, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetDailyStats(ctx context.Context, from, to time.Time) ([]types.DailyStats, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetGasPrices(ctx context.Context, numBlocks int) (*uint64, *uint64, *uint64, *uint64, error) {
	return nil, nil, nil, nil, ErrChainDataNotAvailable
}

// ----- Admin / indexer triggers: not supported in this mode. -----

func (p *DirectDBProvider) IndexBlock(ctx context.Context, number uint64) error {
	return fmt.Errorf("block-explorer does not own the indexer; trigger re-index via chain-indexer")
}
func (p *DirectDBProvider) BackfillDailyStats(ctx context.Context) error {
	return fmt.Errorf("daily-stats backfill is owned by chain-indexer")
}

// ----- Contract verification: real implementations backed by db -----

// GetContract returns the verification metadata for an address. Chain
// facts (bytecode, creator, creation tx, block) come from chain-indexer
// via indexerclient.Provider's override; this method sets only the
// verification fields. Returns nil, nil if not verified.
func (p *DirectDBProvider) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	if p.db == nil {
		return nil, ErrChainDataNotAvailable
	}
	return p.db.GetContractVerification(ctx, address)
}

func (p *DirectDBProvider) UpdateContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	if p.db == nil {
		return ErrChainDataNotAvailable
	}
	return p.db.SetContractABI(ctx, address, abi)
}

func (p *DirectDBProvider) VerifyContract(
	ctx context.Context,
	address, name, compilerVersion string,
	optimizationUsed bool,
	sourceCode string,
	abi json.RawMessage,
	evmVersion, licenseType, constructorArgs string,
	optimizationRuns int,
) error {
	if p.db == nil {
		return ErrChainDataNotAvailable
	}
	return p.db.VerifyContract(ctx, address, name, compilerVersion, optimizationUsed, sourceCode, abi, evmVersion, licenseType, constructorArgs, optimizationRuns)
}

// ----- Node-RPC helpers: call the EVM node directly. -----

func (p *DirectDBProvider) GetChainID(ctx context.Context) (uint64, error) {
	if p.rpc == nil {
		return 0, fmt.Errorf("rpc not available")
	}
	id, err := p.rpc.ChainID(ctx)
	if err != nil {
		return 0, err
	}
	return id.Uint64(), nil
}

func (p *DirectDBProvider) GetBalance(ctx context.Context, address string) (*types.JSONString, error) {
	if p.rpc == nil {
		return nil, fmt.Errorf("rpc not available")
	}
	bal, err := p.rpc.GetBalance(ctx, common.HexToAddress(address))
	if err != nil {
		return nil, err
	}
	s := types.JSONString(bal.String())
	return &s, nil
}

func (p *DirectDBProvider) GetCode(ctx context.Context, address string) ([]byte, error) {
	if p.rpc == nil {
		return nil, fmt.Errorf("rpc not available")
	}
	return p.rpc.GetCode(ctx, common.HexToAddress(address))
}

func (p *DirectDBProvider) GetTransactionByHashRPC(ctx context.Context, hash string) (*types.Transaction, *uint64, error) {
	if p.rpc == nil {
		return nil, nil, fmt.Errorf("rpc not available")
	}
	tx, err := p.rpc.GetTransactionByHash(ctx, common.HexToHash(hash))
	if err != nil {
		return nil, nil, err
	}
	if tx == nil {
		return nil, nil, nil
	}
	// Pending txs don't have a block number; rpc leaves BlockNumber zero.
	bn := tx.BlockNumber
	return tx, &bn, nil
}

// ----- ProxyDataProvider unchanged ----- everything below was the existing
// implementation for "proxy mode" where block-explorer calls privacy-proxy's
// REST API for chain data. It's preserved verbatim.

// ProxyDataProvider proxies all data requests to the privacy-proxy
// (block-explorer in privacy mode). Chain data, addresses, contracts —
// all come from the upstream proxy REST API.
type ProxyDataProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewProxyDataProvider(baseURL string) *ProxyDataProvider {
	return &ProxyDataProvider{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *ProxyDataProvider) doRequest(ctx context.Context, method, path string, body io.Reader, result any) error {
	url := p.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}

// Most methods on ProxyDataProvider return ErrChainDataNotAvailable too,
// because routing each one through the proxy REST API is out of scope for
// RD-855 Phase 6 — the privacy-proxy integration path used ProxyDataProvider
// only for a handful of endpoints prior. When running in privacy mode
// today, block-explorer's api is not deployed at all (see
// privacy-proxy/deployments/privacy/).
func (p *ProxyDataProvider) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	var s types.ChainStats
	if err := p.doRequest(ctx, "GET", "/api/stats", nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Remaining ProxyDataProvider methods: return ErrChainDataNotAvailable.
// Block-explorer in privacy mode is frontend-only; the api doesn't run.
// Kept as stubs only to satisfy DataProvider.

func (p *ProxyDataProvider) GetChainID(ctx context.Context) (uint64, error) {
	return 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetCatchupProgress(ctx context.Context) (int64, uint64, float64, bool, error) {
	return 0, 0, 0, false, nil
}
func (p *ProxyDataProvider) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetBlockByHash(ctx context.Context, hash string) (*types.Block, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	return 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetBalance(ctx context.Context, address string) (*types.JSONString, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetCode(ctx context.Context, address string) ([]byte, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) IsContract(ctx context.Context, address string) (bool, error) {
	return false, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) UpdateContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	return ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) VerifyContract(ctx context.Context, address, name, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion, licenseType, constructorArgs string, optimizationRuns int) error {
	return ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetToken(ctx context.Context, address string) (*types.Token, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) IndexBlock(ctx context.Context, number uint64) error {
	return ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetTransactionByHashRPC(ctx context.Context, hash string) (*types.Transaction, *uint64, error) {
	return nil, nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetDailyStats(ctx context.Context, from, to time.Time) ([]types.DailyStats, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) BackfillDailyStats(ctx context.Context) error {
	return ErrChainDataNotAvailable
}
func (p *ProxyDataProvider) GetGasPrices(ctx context.Context, numBlocks int) (*uint64, *uint64, *uint64, *uint64, error) {
	return nil, nil, nil, nil, ErrChainDataNotAvailable
}

// Compile-time assertions.
var (
	_ DataProvider = (*DirectDBProvider)(nil)
	_ DataProvider = (*ProxyDataProvider)(nil)
	_ = bytes.Buffer{}
)
