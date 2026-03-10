package rpc

import (
	"context"
	"math/big"
	"sync"
	"time"

	"explorer/internal/log"
	explorerTypes "explorer/internal/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

type Client struct {
	eth *ethclient.Client
	raw *rpc.Client // For trace_* and debug_* calls
}

func New(url string) (*Client, error) {
	raw, err := rpc.Dial(url)
	if err != nil {
		return nil, err
	}
	eth := ethclient.NewClient(raw)
	return &Client{eth: eth, raw: raw}, nil
}

func (c *Client) Raw() *rpc.Client {
	return c.raw
}

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	return c.eth.BlockNumber(ctx)
}

func (c *Client) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return c.eth.BlockByNumber(ctx, number)
}

func (c *Client) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return c.eth.TransactionReceipt(ctx, txHash)
}

func (c *Client) TransactionByHash(ctx context.Context, txHash common.Hash) (*types.Transaction, bool, error) {
	return c.eth.TransactionByHash(ctx, txHash)
}

func (c *Client) GetBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	return c.eth.BalanceAt(ctx, address, nil)
}

func (c *Client) GetCode(ctx context.Context, address common.Address) ([]byte, error) {
	return c.eth.CodeAt(ctx, address, nil)
}

func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	return c.eth.ChainID(ctx)
}

func (c *Client) CallContract(ctx context.Context, to common.Address, data []byte) ([]byte, error) {
	msg := ethereum.CallMsg{
		To:   &to,
		Data: data,
	}
	return c.eth.CallContract(ctx, msg, nil)
}

func (c *Client) GetTransactionByHash(ctx context.Context, hash common.Hash) (*explorerTypes.Transaction, error) {
	tx, isPending, err := c.eth.TransactionByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	receipt, err := c.eth.TransactionReceipt(ctx, hash)
	if err != nil {
		log.Warn("failed to fetch receipt for tx lookup", "tx", hash.Hex(), "error", err)
	}

	chainID, err := c.eth.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	from, err := types.Sender(types.LatestSignerForChainID(chainID), tx)
	if err != nil {
		return nil, err
	}

	var to *string
	if tx.To() != nil {
		toStr := tx.To().Hex()
		to = &toStr
	}

	inputData := ""
	if len(tx.Data()) > 0 {
		if len(tx.Data()) >= 4 {
			inputData = common.Bytes2Hex(tx.Data()[:4])
		} else {
			inputData = common.Bytes2Hex(tx.Data())
		}
	}

	var blockNum uint64
	var txIndex int
	var gasUsed uint64
	var status int

	if receipt != nil {
		if !isPending && receipt.BlockNumber != nil {
			blockNum = receipt.BlockNumber.Uint64()
		}
		txIndex = int(receipt.TransactionIndex)
		gasUsed = receipt.GasUsed
		status = int(receipt.Status)
	} else {
		gasUsed = tx.Gas()
		status = 1
	}

	return &explorerTypes.Transaction{
		Hash:        tx.Hash().Hex(),
		BlockNumber: blockNum,
		TxIndex:     txIndex,
		From:        from.Hex(),
		To:          to,
		Value:       explorerTypes.JSONString(tx.Value().String()),
		GasUsed:     gasUsed,
		GasPrice:    tx.GasPrice().Uint64(),
		InputData:   inputData,
		Status:      status,
		CreatedAt:   time.Now(),
	}, nil
}

func (c *Client) GetBlockByNumber(ctx context.Context, number uint64) (*explorerTypes.Block, error) {
	block, err := c.eth.BlockByNumber(ctx, new(big.Int).SetUint64(number))
	if err != nil {
		return nil, err
	}

	return &explorerTypes.Block{
		Number:           block.NumberU64(),
		Hash:             block.Hash().Hex(),
		ParentHash:       block.ParentHash().Hex(),
		Timestamp:        block.Time(),
		GasUsed:          block.GasUsed(),
		GasLimit:         block.GasLimit(),
		TransactionCount: len(block.Transactions()),
		CreatedAt:        time.Now(),
	}, nil
}

func (c *Client) GetBlockTransactions(ctx context.Context, number uint64) ([]*explorerTypes.Transaction, error) {
	block, err := c.eth.BlockByNumber(ctx, new(big.Int).SetUint64(number))
	if err != nil {
		return nil, err
	}

	chainID, err := c.eth.ChainID(ctx)
	if err != nil {
		return nil, err
	}

	txs := make([]*explorerTypes.Transaction, 0, len(block.Transactions()))
	for idx, tx := range block.Transactions() {
		receipt, err := c.eth.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			continue // Skip transactions we can't get receipts for
		}

		from, err := types.Sender(types.LatestSignerForChainID(chainID), tx)
		if err != nil {
			continue
		}

		var to *string
		if tx.To() != nil {
			toStr := tx.To().Hex()
			to = &toStr
		}

		inputData := ""
		if len(tx.Data()) > 0 {
			if len(tx.Data()) >= 4 {
				inputData = common.Bytes2Hex(tx.Data()[:4])
			} else {
				inputData = common.Bytes2Hex(tx.Data())
			}
		}

		txs = append(txs, &explorerTypes.Transaction{
			Hash:        tx.Hash().Hex(),
			BlockNumber: number,
			TxIndex:     idx,
			From:        from.Hex(),
			To:          to,
			Value:       explorerTypes.JSONString(tx.Value().String()),
			GasUsed:     receipt.GasUsed,
			GasPrice:    tx.GasPrice().Uint64(),
			InputData:   inputData,
			Status:      int(receipt.Status),
			CreatedAt:   time.Now(),
		})
	}

	return txs, nil
}

type ReceiptResult struct {
	TxHash  common.Hash
	Receipt *types.Receipt
	Err     error
}

// FetchReceiptsBatch fetches transaction receipts in parallel with rate limiting.
// Individual receipt failures are logged and skipped rather than failing the entire batch.
// This handles RPC nodes (e.g. op-reth) that return errors for receipt queries due to
// missing L1 block info or other node-specific issues.
func (c *Client) FetchReceiptsBatch(ctx context.Context, txHashes []common.Hash, workers int, rateLimit int) (map[common.Hash]*types.Receipt, error) {
	if len(txHashes) == 0 {
		return make(map[common.Hash]*types.Receipt), nil
	}

	limiter := rate.NewLimiter(rate.Limit(rateLimit), rateLimit/10+1)

	results := make(map[common.Hash]*types.Receipt)
	var mu sync.Mutex
	var failCount int

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	for _, hash := range txHashes {
		hash := hash // capture loop variable
		g.Go(func() error {
			if err := limiter.Wait(ctx); err != nil {
				return err
			}

			receipt, err := c.eth.TransactionReceipt(ctx, hash)
			if err != nil {
				mu.Lock()
				failCount++
				mu.Unlock()
				log.Warn("failed to fetch receipt, tx will be indexed without receipt data",
					"tx", hash.Hex(), "error", err)
				return nil
			}

			mu.Lock()
			results[hash] = receipt
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if failCount > 0 {
		log.Warn("some receipts could not be fetched",
			"failed", failCount, "total", len(txHashes))
	}

	return results, nil
}

type TokenMetadataResult struct {
	Address  common.Address
	Symbol   string
	Name     *string
	Decimals int
	Err      error
}

func (c *Client) FetchTokenMetadataBatch(ctx context.Context, addresses []common.Address, workers int, rateLimit int) (map[common.Address]*TokenMetadataResult, error) {
	if len(addresses) == 0 {
		return make(map[common.Address]*TokenMetadataResult), nil
	}

	limiter := rate.NewLimiter(rate.Limit(rateLimit), rateLimit/10+1)

	results := make(map[common.Address]*TokenMetadataResult)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	for _, addr := range addresses {
		addr := addr // capture loop variable
		g.Go(func() error {
			result := &TokenMetadataResult{Address: addr}

			// Fetch symbol (0x95d89b41)
			if err := limiter.Wait(ctx); err != nil {
				result.Err = err
				mu.Lock()
				results[addr] = result
				mu.Unlock()
				return nil // Don't fail the whole batch
			}
			symbolData, err := c.CallContract(ctx, addr, common.FromHex("0x95d89b41"))
			if err != nil {
				result.Symbol = "UNKNOWN"
			} else {
				result.Symbol = parseStringResult(symbolData)
				if result.Symbol == "" {
					result.Symbol = "UNKNOWN"
				}
			}

			// Fetch name (0x06fdde03)
			if err := limiter.Wait(ctx); err != nil {
				mu.Lock()
				results[addr] = result
				mu.Unlock()
				return nil
			}
			nameData, err := c.CallContract(ctx, addr, common.FromHex("0x06fdde03"))
			if err == nil {
				name := parseStringResult(nameData)
				if name != "" {
					result.Name = &name
				}
			}

			// Fetch decimals (0x313ce567)
			if err := limiter.Wait(ctx); err != nil {
				mu.Lock()
				results[addr] = result
				mu.Unlock()
				return nil
			}
			decimalsData, err := c.CallContract(ctx, addr, common.FromHex("0x313ce567"))
			if err == nil && len(decimalsData) >= 32 {
				result.Decimals = int(new(big.Int).SetBytes(decimalsData).Int64())
			} else {
				result.Decimals = 18 // Default
			}

			mu.Lock()
			results[addr] = result
			mu.Unlock()
			return nil
		})
	}

	g.Wait() // Errors are captured in results
	return results, nil
}

func parseStringResult(data []byte) string {
	if len(data) < 64 {
		result := string(data)
		for i := 0; i < len(result); i++ {
			if result[i] == 0 {
				return result[:i]
			}
		}
		return result
	}

	offset := new(big.Int).SetBytes(data[:32]).Uint64()
	if offset >= uint64(len(data)) {
		return ""
	}

	if offset+32 > uint64(len(data)) {
		return ""
	}

	length := new(big.Int).SetBytes(data[offset : offset+32]).Uint64()
	if offset+32+length > uint64(len(data)) {
		return ""
	}

	result := string(data[offset+32 : offset+32+length])
	for i := 0; i < len(result); i++ {
		if result[i] == 0 {
			return result[:i]
		}
	}
	return result
}

func (c *Client) FetchBalanceBatch(ctx context.Context, requests []BalanceRequest, workers int, rateLimit int) map[string]*big.Int {
	if len(requests) == 0 {
		return make(map[string]*big.Int)
	}

	limiter := rate.NewLimiter(rate.Limit(rateLimit), rateLimit/10+1)
	results := make(map[string]*big.Int)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	for _, req := range requests {
		req := req
		g.Go(func() error {
			if err := limiter.Wait(ctx); err != nil {
				return nil
			}

			// balanceOf(address) = 0x70a08231
			addrPadded := common.LeftPadBytes(req.Address.Bytes(), 32)
			callData := append(common.FromHex("0x70a08231"), addrPadded...)

			balanceData, err := c.CallContract(ctx, req.TokenAddress, callData)
			if err != nil || len(balanceData) < 32 {
				return nil
			}

			balance := new(big.Int).SetBytes(balanceData)
			key := req.Address.Hex() + ":" + req.TokenAddress.Hex()

			mu.Lock()
			results[key] = balance
			mu.Unlock()
			return nil
		})
	}

	g.Wait()
	return results
}

type BalanceRequest struct {
	Address      common.Address
	TokenAddress common.Address
	BlockNumber  uint64
}

func (c *Client) SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error) {
	return c.eth.SubscribeNewHead(ctx, ch)
}

type RawBlockResponse struct {
	TotalDifficulty string `json:"totalDifficulty"`
}

func (c *Client) GetTotalDifficulty(ctx context.Context, blockNumber uint64) string {
	var result RawBlockResponse
	err := c.raw.CallContext(ctx, &result, "eth_getBlockByNumber", toHex(blockNumber), false)
	if err != nil || result.TotalDifficulty == "" {
		return "0"
	}
	td := new(big.Int)
	if _, ok := td.SetString(result.TotalDifficulty, 0); !ok {
		return "0"
	}
	return td.String()
}

func toHex(n uint64) string {
	return "0x" + new(big.Int).SetUint64(n).Text(16)
}
