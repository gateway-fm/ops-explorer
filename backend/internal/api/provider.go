package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"explorer/internal/db"
	"explorer/internal/indexer"
	"explorer/internal/rpc"
	"explorer/internal/types"

	"explorer/pkg/eth/common"
)

type DataProvider interface {
	GetChainStats(ctx context.Context) (*types.ChainStats, error)
	GetChainID(ctx context.Context) (uint64, error)
	GetSyncStatus(ctx context.Context) (*types.SyncStatus, error)
	GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error)
	GetIndexerProgress(ctx context.Context) (*db.IndexerProgress, error)
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
}

type DirectDBProvider struct {
	db      APIDatabase
	rpc     *rpc.Client
	indexer *indexer.Indexer
}

func NewDirectDBProvider(db APIDatabase, rpc *rpc.Client, idx *indexer.Indexer) *DirectDBProvider {
	return &DirectDBProvider{
		db:      db,
		rpc:     rpc,
		indexer: idx,
	}
}

func (p *DirectDBProvider) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	return p.db.GetChainStats(ctx)
}

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

func (p *DirectDBProvider) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	return p.db.GetSyncStatus(ctx)
}

func (p *DirectDBProvider) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	return p.db.GetTransactionHistory(ctx, intervalSeconds, limit)
}

func (p *DirectDBProvider) GetIndexerProgress(ctx context.Context) (*db.IndexerProgress, error) {
	return p.db.GetIndexerProgress(ctx)
}

func (p *DirectDBProvider) GetCatchupProgress(ctx context.Context) (processed int64, total uint64, percentComplete float64, isRunning bool, err error) {
	if p.indexer == nil {
		return 0, 0, 0, false, nil
	}
	processed, total, percentComplete, isRunning = p.indexer.GetCatchupProgress()
	return
}

func (p *DirectDBProvider) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	return p.db.GetBlocks(ctx, limit, beforeBlock)
}

func (p *DirectDBProvider) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	return p.db.GetBlock(ctx, number)
}

func (p *DirectDBProvider) GetBlockByHash(ctx context.Context, hash string) (*types.Block, error) {
	return p.db.GetBlockByHash(ctx, hash)
}

func (p *DirectDBProvider) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	return p.db.GetLatestBlockNumber(ctx)
}

func (p *DirectDBProvider) GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error) {
	return p.db.GetInternalTransactionsByBlock(ctx, blockNumber)
}

func (p *DirectDBProvider) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return p.db.GetTransactions(ctx, limit, beforeBlock)
}

func (p *DirectDBProvider) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	return p.db.GetTransactionsPaginated(ctx, page, pageSize)
}

func (p *DirectDBProvider) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	return p.db.GetTransactionsByBlock(ctx, blockNumber)
}

func (p *DirectDBProvider) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return p.db.GetTransactionsWithCategories(ctx, limit, beforeBlock)
}

func (p *DirectDBProvider) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	return p.db.GetTransactionsPaginatedWithCategories(ctx, page, pageSize)
}

func (p *DirectDBProvider) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	return p.db.GetTransactionWithCategories(ctx, hash)
}

func (p *DirectDBProvider) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	return p.db.GetTransaction(ctx, hash)
}

func (p *DirectDBProvider) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return p.db.GetTransactionsByAddress(ctx, address, limit, beforeBlock)
}

func (p *DirectDBProvider) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	return p.db.GetInternalTransactionsByTx(ctx, txHash)
}

func (p *DirectDBProvider) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	return p.db.GetTransfersByTransaction(ctx, txHash)
}

func (p *DirectDBProvider) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	return p.db.GetLogsByTransaction(ctx, txHash)
}

func (p *DirectDBProvider) GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error) {
	return p.db.GetOPDeposit(ctx, txHash)
}

func (p *DirectDBProvider) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	return p.db.GetAddressStats(ctx, address)
}

func (p *DirectDBProvider) GetBalance(ctx context.Context, address string) (*types.JSONString, error) {
	if p.rpc == nil {
		return nil, fmt.Errorf("rpc not available")
	}
	bal, err := p.rpc.GetBalance(ctx, common.HexToAddress(address))
	if err != nil {
		return nil, err
	}
	js := types.JSONString(bal.String())
	return &js, nil
}

func (p *DirectDBProvider) GetCode(ctx context.Context, address string) ([]byte, error) {
	if p.rpc == nil {
		return nil, fmt.Errorf("rpc not available")
	}
	return p.rpc.GetCode(ctx, common.HexToAddress(address))
}

func (p *DirectDBProvider) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	return p.db.GetTokenBalances(ctx, address)
}

func (p *DirectDBProvider) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	return p.db.GetTransfersByAddress(ctx, address, limit, beforeBlock)
}

func (p *DirectDBProvider) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	return p.db.GetInternalTransactionsByAddress(ctx, address, limit, offset)
}

func (p *DirectDBProvider) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	return p.db.GetLogsByAddress(ctx, address, limit, offset)
}

func (p *DirectDBProvider) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	return p.db.GetLogs(ctx, address, topic0, fromBlock, toBlock, limit)
}

func (p *DirectDBProvider) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	return p.db.GetContract(ctx, address)
}

func (p *DirectDBProvider) IsContract(ctx context.Context, address string) (bool, error) {
	return p.db.IsContract(ctx, address)
}

func (p *DirectDBProvider) UpdateContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	return p.db.SetContractABI(ctx, address, abi)
}

func (p *DirectDBProvider) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	return p.db.GetTokens(ctx, limit, offset, tokenType)
}

func (p *DirectDBProvider) GetToken(ctx context.Context, address string) (*types.Token, error) {
	return p.db.GetToken(ctx, address)
}

func (p *DirectDBProvider) GetTokenHolders(ctx context.Context, address string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	return p.db.GetTokenHolders(ctx, address, limit, offset)
}

func (p *DirectDBProvider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	return p.db.GetTransfersByToken(ctx, tokenAddress, limit, offset)
}

func (p *DirectDBProvider) GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	return p.db.GetAllTransfers(ctx, limit, offset)
}

func (p *DirectDBProvider) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	return p.db.GetAccountsPaginated(ctx, page, pageSize)
}

func (p *DirectDBProvider) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	return p.db.SearchSuggestions(ctx, query, limit)
}

func (p *DirectDBProvider) IndexBlock(ctx context.Context, number uint64) error {
	if p.indexer == nil {
		return fmt.Errorf("indexer not available")
	}
	return p.indexer.IndexBlock(ctx, number)
}

func (p *DirectDBProvider) VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error {
	return p.db.VerifyContract(ctx, address, name, compilerVersion, optimizationUsed, sourceCode, abi, evmVersion, licenseType, constructorArgs, optimizationRuns)
}

func (p *DirectDBProvider) GetTransactionByHashRPC(ctx context.Context, hash string) (*types.Transaction, *uint64, error) {
	if p.rpc == nil {
		return nil, nil, fmt.Errorf("rpc not available")
	}
	tx, _, err := p.rpc.TransactionByHash(ctx, common.HexToHash(hash))
	if err != nil {
		return nil, nil, err
	}
	receipt, err := p.rpc.TransactionReceipt(ctx, common.HexToHash(hash))
	if err != nil {
		return nil, nil, err
	}

	t := &types.Transaction{
		Hash: tx.Hash.Hex(),
	}
	var blockNumber *uint64
	if receipt.BlockNumber != nil {
		bn := receipt.BlockNumber.Uint64()
		blockNumber = &bn
	}
	return t, blockNumber, nil
}

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

func (p *ProxyDataProvider) GetIndexerProgress(ctx context.Context) (*db.IndexerProgress, error) {
	var pr db.IndexerProgress
	err := p.doRequest(ctx, "GET", "/api/v1/explorer/sync/indexer-progress", nil, &pr)
	return &pr, err
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

func (p *ProxyDataProvider) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	q := fmt.Sprintf("?limit=%d&offset=%d&type=%s", limit, offset, tokenType)
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

func (p *ProxyDataProvider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	var res struct {
		Data  []types.TokenTransfer `json:"data"`
		Total int64                 `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/tokens/%s/transfers?limit=%d&offset=%d", tokenAddress, limit, offset), nil, &res)
	return res.Data, res.Total, err
}

func (p *ProxyDataProvider) GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	var res struct {
		Data  []types.TokenTransfer `json:"data"`
		Total int64                 `json:"total"`
	}
	err := p.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/explorer/transfers?limit=%d&offset=%d", limit, offset), nil, &res)
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
