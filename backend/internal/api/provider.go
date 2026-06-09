package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	GetTokens(ctx context.Context, limit int, offset int, tokenType, search string) ([]types.Token, int64, error)
	GetToken(ctx context.Context, address string) (*types.Token, error)
	GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error)
	GetTokenInventory(ctx context.Context, address string, tokenID string, limit int, offset int) ([]types.TokenInventoryItem, int64, error)
	GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error)
	// GetAllTransfers is the global transfer feed. tokenType is "ERC20"/"ERC721"/
	// "ERC1155" to filter by standard, or "" for all.
	GetAllTransfers(ctx context.Context, tokenType string, limit int, offset int) ([]types.TokenTransfer, int64, error)

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
func (p *DirectDBProvider) GetTokens(ctx context.Context, limit int, offset int, tokenType, search string) ([]types.Token, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetToken(ctx context.Context, address string) (*types.Token, error) {
	return nil, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTokenInventory(ctx context.Context, address string, tokenID string, limit int, offset int) ([]types.TokenInventoryItem, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	return nil, 0, ErrChainDataNotAvailable
}
func (p *DirectDBProvider) GetAllTransfers(ctx context.Context, tokenType string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
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
	baseURL string
	client  *http.Client
}

func NewProxyDataProvider(baseURL string) *ProxyDataProvider {
	return &ProxyDataProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *ProxyDataProvider) doRequest(ctx context.Context, method, path string, body io.Reader, result any) error {
	// If the caller is in "View as user" mode (RD-928), rewrite the outbound
	// path under the admin-impersonation prefix so privacy-proxy applies the
	// target user's visibility rules + audit-logs the access. The
	// X-Impersonate-Token header (which only the BFF knows how to interpret)
	// is intentionally NOT forwarded — the proxy gates on the URL.
	path = applyImpersonationPath(ctx, path)
	url := p.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if token, ok := ctx.Value(rpc.ContextKeyAuthToken).(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("resource not found")
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxy request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

func (p *ProxyDataProvider) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	var stats types.ChainStats
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/stats", nil, &stats)
	return &stats, err
}

func (p *ProxyDataProvider) GetChainID(ctx context.Context) (uint64, error) {
	var res struct {
		ChainID uint64 `json:"chain_id"`
	}
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/chain-id", nil, &res)
	return res.ChainID, err
}

func (p *ProxyDataProvider) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	var ss types.SyncStatus
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/sync/status", nil, &ss)
	return &ss, err
}

func (p *ProxyDataProvider) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	var h []types.TxHistoryPoint
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/stats/tx-history?interval=%d&limit=%d", intervalSeconds, limit), nil, &h)
	return h, err
}

func (p *ProxyDataProvider) GetCatchupProgress(ctx context.Context) (processed int64, total uint64, percentComplete float64, isRunning bool, err error) {
	var res struct {
		Processed       int64   `json:"processed"`
		Total           uint64  `json:"total"`
		PercentComplete float64 `json:"percentComplete"`
		IsRunning       bool    `json:"isRunning"`
	}
	err = p.doRequest(ctx, "GET", "/api/v1/explorer/sync/catchup", nil, &res)
	return res.Processed, res.Total, res.PercentComplete, res.IsRunning, err
}

func (p *ProxyDataProvider) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	q := fmt.Sprintf("?limit=%d", limit)
	if beforeBlock != nil {
		q += fmt.Sprintf("&before=%d", *beforeBlock)
	}
	var b []types.Block
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/blocks"+q, nil, &b)
	return b, err
}

func (p *ProxyDataProvider) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	var b types.Block
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/blocks/%d", number), nil, &b)
	return &b, err
}

func (p *ProxyDataProvider) GetBlockByHash(ctx context.Context, hash string) (*types.Block, error) {
	var b types.Block
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/blocks/hash/%s", hash), nil, &b)
	return &b, err
}

func (p *ProxyDataProvider) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	var r struct {
		Number uint64 `json:"number"`
	}
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/blocks/latest/number", nil, &r)
	return r.Number, err
}

func (p *ProxyDataProvider) GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error) {
	var t []types.InternalTransaction
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/blocks/%d/internal", blockNumber), nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	q := fmt.Sprintf("?limit=%d", limit)
	if beforeBlock != nil {
		q += fmt.Sprintf("&before=%d", *beforeBlock)
	}
	var t []types.Transaction
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/transactions"+q, nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	var res struct {
		Data  []types.Transaction `json:"data"`
		Total int64               `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/paginated?page=%d&pageSize=%d", page, pageSize), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	var t []types.Transaction
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/blocks/%d/transactions", blockNumber), nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	q := fmt.Sprintf("?limit=%d&with_categories=true", limit)
	if beforeBlock != nil {
		q += fmt.Sprintf("&before=%d", *beforeBlock)
	}
	var t []types.Transaction
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/transactions"+q, nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	var res struct {
		Data  []types.Transaction `json:"data"`
		Total int64               `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/paginated?page=%d&pageSize=%d&with_categories=true", page, pageSize), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	var t types.Transaction
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/%s?with_categories=true", hash), nil, &t)
	return &t, err
}

func (p *ProxyDataProvider) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	var t types.Transaction
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/%s", hash), nil, &t)
	return &t, err
}

func (p *ProxyDataProvider) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	q := fmt.Sprintf("?limit=%d", limit)
	if beforeBlock != nil {
		q += fmt.Sprintf("&before=%d", *beforeBlock)
	}
	var t []types.Transaction
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/transactions", address)+q, nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	var t []types.InternalTransaction
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/%s/internal", txHash), nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	var t []types.TokenTransfer
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/%s/transfers", txHash), nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	var l []types.Log
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/%s/logs", txHash), nil, &l)
	return l, err
}

func (p *ProxyDataProvider) GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error) {
	var d types.OPDeposit
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/%s/op-deposit", txHash), nil, &d)
	return &d, err
}

func (p *ProxyDataProvider) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	var s types.AddressStats
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/stats", address), nil, &s)
	return &s, err
}

func (p *ProxyDataProvider) GetBalance(ctx context.Context, address string) (*types.JSONString, error) {
	var b types.JSONString
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/balance", address), nil, &b)
	return &b, err
}

func (p *ProxyDataProvider) GetCode(ctx context.Context, address string) ([]byte, error) {
	var c []byte
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/code", address), nil, &c)
	return c, err
}

func (p *ProxyDataProvider) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	var b []types.Balance
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/balances", address), nil, &b)
	return b, err
}

func (p *ProxyDataProvider) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	q := fmt.Sprintf("?limit=%d", limit)
	if beforeBlock != nil {
		q += fmt.Sprintf("&before=%d", *beforeBlock)
	}
	var t []types.TokenTransfer
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/transfers", address)+q, nil, &t)
	return t, err
}

func (p *ProxyDataProvider) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	var res struct {
		Data  []types.InternalTransaction `json:"data"`
		Total int64                       `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/internal?limit=%d&offset=%d", address, limit, offset), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	var res struct {
		Data  []types.Log `json:"data"`
		Total int64       `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/logs?limit=%d&offset=%d", address, limit, offset), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	q := fmt.Sprintf("?limit=%d", limit)
	if address != nil {
		q += "&address=" + *address
	}
	if topic0 != nil {
		q += "&topic0=" + *topic0
	}
	if fromBlock != nil {
		q += fmt.Sprintf("&from=%d", *fromBlock)
	}
	if toBlock != nil {
		q += fmt.Sprintf("&to=%d", *toBlock)
	}
	var l []types.Log
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/logs"+q, nil, &l)
	return l, err
}

func (p *ProxyDataProvider) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	var c types.Contract
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/contract", address), nil, &c)
	return &c, err
}

func (p *ProxyDataProvider) IsContract(ctx context.Context, address string) (bool, error) {
	var r struct {
		IsContract bool `json:"is_contract"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/addresses/%s/is-contract", address), nil, &r)
	return r.IsContract, err
}

func (p *ProxyDataProvider) UpdateContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	return p.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/explorer/addresses/%s/abi", address), bytes.NewReader(abi), nil)
}

func (p *ProxyDataProvider) GetTokens(ctx context.Context, limit int, offset int, tokenType, search string) ([]types.Token, int64, error) {
	q := fmt.Sprintf("?limit=%d&offset=%d&type=%s", limit, offset, url.QueryEscape(tokenType))
	if search != "" {
		q += "&search=" + url.QueryEscape(search)
	}
	var res struct {
		Data  []types.Token `json:"data"`
		Total int64         `json:"total"`
	}
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/tokens"+q, nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetToken(ctx context.Context, address string) (*types.Token, error) {
	var t types.Token
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/tokens/%s", address), nil, &t)
	return &t, err
}

func (p *ProxyDataProvider) GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	var res struct {
		Data  []types.TokenHolder `json:"data"`
		Total int64               `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/tokens/%s/holders?limit=%d&offset=%d", address, limit, offset), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetTokenInventory(ctx context.Context, address string, tokenID string, limit int, offset int) ([]types.TokenInventoryItem, int64, error) {
	var res struct {
		Data  []types.TokenInventoryItem `json:"data"`
		Total int64                      `json:"total"`
	}
	path := fmt.Sprintf("/api/v1/explorer/tokens/%s/inventory?limit=%d&offset=%d", address, limit, offset)
	if tokenID != "" {
		path += "&tokenId=" + url.QueryEscape(tokenID)
	}
	err := p.doRequest(ctx, "GET", path, nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	var res struct {
		Data  []types.TokenTransfer `json:"data"`
		Total int64                 `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/tokens/%s/transfers?limit=%d&offset=%d", tokenAddress, limit, offset), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetAllTransfers(ctx context.Context, tokenType string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	var res struct {
		Data  []types.TokenTransfer `json:"data"`
		Total int64                 `json:"total"`
	}
	path := fmt.Sprintf("/api/v1/explorer/transfers?limit=%d&offset=%d", limit, offset)
	if tokenType != "" {
		path += "&type=" + url.QueryEscape(tokenType)
	}
	err := p.doRequest(ctx, "GET", path, nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	var res struct {
		Data  []types.AddressStats `json:"data"`
		Total int64                `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/accounts?page=%d&pageSize=%d", page, pageSize), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	var s []types.SearchSuggestion
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/search/suggestions?q=%s&limit=%d", query, limit), nil, &s)
	return s, err
}

func (p *ProxyDataProvider) IndexBlock(ctx context.Context, number uint64) error {
	return p.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/explorer/index/block/%d", number), nil, nil)
}

func (p *ProxyDataProvider) VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error {
	reqBody := map[string]any{
		"address":          address,
		"name":             name,
		"compilerVersion":  compilerVersion,
		"optimizationUsed": optimizationUsed,
		"sourceCode":       sourceCode,
		"abi":              abi,
		"evmVersion":       evmVersion,
		"licenseType":      licenseType,
		"constructorArgs":  constructorArgs,
		"optimizationRuns": optimizationRuns,
	}
	body, _ := json.Marshal(reqBody)
	return p.doRequest(ctx, "POST", "/api/v1/explorer/contracts/verify", bytes.NewReader(body), nil)
}

func (p *ProxyDataProvider) GetTransactionByHashRPC(ctx context.Context, hash string) (*types.Transaction, *uint64, error) {
	var res struct {
		Transaction *types.Transaction `json:"transaction"`
		BlockNumber *uint64            `json:"block_number"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transactions/%s/rpc", hash), nil, &res)
	return res.Transaction, res.BlockNumber, err
}

func (p *ProxyDataProvider) GetDailyStats(ctx context.Context, from, to time.Time) ([]types.DailyStats, error) {
	var stats []types.DailyStats
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/charts/lines/new_txns?from=%s&to=%s", from.Format("2006-01-02"), to.Format("2006-01-02")), nil, &stats)
	return stats, err
}

func (p *ProxyDataProvider) BackfillDailyStats(ctx context.Context) error {
	return p.doRequest(ctx, "POST", "/api/v1/explorer/charts/backfill", nil, nil)
}
// Compile-time assertions.
var (
	_ DataProvider = (*DirectDBProvider)(nil)
	_ DataProvider = (*ProxyDataProvider)(nil)
	_ = bytes.Buffer{}
)


// GetGasPrices: privacy-proxy does not expose a gas-prices endpoint on
// its REST explorer surface, so the BFF cannot forward it. Returning
// ErrChainDataNotAvailable is correct — the /api/gas endpoint in
// privacy-mode block-explorer returns 500 with the explanation. If
// privacy-proxy grows this endpoint, implement a doRequest here.
func (p *ProxyDataProvider) GetGasPrices(ctx context.Context, numBlocks int) (*uint64, *uint64, *uint64, *uint64, error) {
	return nil, nil, nil, nil, ErrChainDataNotAvailable
}
