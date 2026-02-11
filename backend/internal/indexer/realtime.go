package indexer

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"explorer/internal/db"
	"explorer/internal/events"
	"explorer/internal/log"
	"explorer/internal/rpc"
	"explorer/internal/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// RealtimeConfig holds configuration for the realtime indexer
type RealtimeConfig struct {
	ConfirmationBlocks int           // Blocks to wait before processing (default: 1)
	PollInterval       time.Duration // Fallback poll interval if WS fails
}

// RealtimeIndexer handles real-time block indexing using WebSocket subscriptions
type RealtimeIndexer struct {
	db       *db.DB
	rpc      *rpc.Client
	config   *RealtimeConfig
	idxCfg   *Config // Main indexer config for RPC settings
	eventBus *events.Bus

	// Caches (shared with main indexer)
	tokenCache    *TokenCache
	contractCache *ContractCache

	// Balance workers (shared with main indexer)
	balanceWorkers *BalanceWorkerPool

	// Tracing support
	tracingSupported bool

	// State tracking
	lastProcessedBlock uint64
	mu                 sync.RWMutex

	// Coordination
	ctx    context.Context
	cancel context.CancelFunc

	// On-demand indexing requests channel
	indexRequests chan indexRequest
}

// NewRealtimeIndexer creates a new realtime indexer
func NewRealtimeIndexer(
	database *db.DB,
	rpcClient *rpc.Client,
	cfg *RealtimeConfig,
	idxCfg *Config,
	tokenCache *TokenCache,
	contractCache *ContractCache,
	balanceWorkers *BalanceWorkerPool,
	tracingSupported bool,
) *RealtimeIndexer {
	ctx, cancel := context.WithCancel(context.Background())
	return &RealtimeIndexer{
		db:               database,
		rpc:              rpcClient,
		config:           cfg,
		idxCfg:           idxCfg,
		tokenCache:       tokenCache,
		contractCache:    contractCache,
		balanceWorkers:   balanceWorkers,
		tracingSupported: tracingSupported,
		indexRequests:    make(chan indexRequest, 100),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// SetEventBus sets the event bus for publishing indexer events
func (r *RealtimeIndexer) SetEventBus(bus *events.Bus) {
	r.eventBus = bus
}

// SetLastProcessedBlock sets the last processed block number
func (r *RealtimeIndexer) SetLastProcessedBlock(blockNum uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastProcessedBlock = blockNum
}

// GetLastProcessedBlock returns the last processed block number
func (r *RealtimeIndexer) GetLastProcessedBlock() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastProcessedBlock
}

// IndexBlock indexes a specific block on-demand and waits for completion
func (r *RealtimeIndexer) IndexBlock(ctx context.Context, blockNumber uint64) error {
	done := make(chan error, 1)
	select {
	case r.indexRequests <- indexRequest{blockNumber: blockNumber, done: done}:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start begins the realtime indexing process
func (r *RealtimeIndexer) Start(ctx context.Context, startBlock uint64) error {
	r.SetLastProcessedBlock(startBlock)

	log.Info("realtime: starting",
		"from_block", startBlock,
		"confirmation_blocks", r.config.ConfirmationBlocks)

	// Try to use WebSocket subscription for new block headers
	headers := make(chan *ethtypes.Header)
	sub, err := r.rpc.SubscribeNewHead(ctx, headers)
	if err != nil {
		log.Warn("realtime: WebSocket subscription failed, falling back to polling", "error", err)
		return r.runPollingMode(ctx)
	}

	log.Info("realtime: WebSocket subscription active")
	return r.runSubscriptionMode(ctx, headers, sub)
}

// Stop gracefully stops the realtime indexer
func (r *RealtimeIndexer) Stop() {
	r.cancel()
}

// runSubscriptionMode runs the indexer using WebSocket subscriptions
func (r *RealtimeIndexer) runSubscriptionMode(ctx context.Context, headers <-chan *ethtypes.Header, sub ethereum.Subscription) error {
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-r.ctx.Done():
			return r.ctx.Err()

		case err := <-sub.Err():
			log.Warn("realtime: subscription error, switching to polling", "error", err)
			return r.runPollingMode(ctx)

		case req := <-r.indexRequests:
			// On-demand indexing request
			err := r.processBlock(ctx, req.blockNumber)
			req.done <- err

		case header := <-headers:
			if header == nil {
				continue
			}

			latestBlock := header.Number.Uint64()
			safeBlock := latestBlock - uint64(r.config.ConfirmationBlocks)

			// Process blocks up to the safe block
			lastProcessed := r.GetLastProcessedBlock()
			for blockNum := lastProcessed + 1; blockNum <= safeBlock; blockNum++ {
				// Check for on-demand requests between blocks
				select {
				case req := <-r.indexRequests:
					err := r.processBlock(ctx, req.blockNumber)
					req.done <- err
				default:
				}

				// Check for reorg before processing
				if blockNum > 0 {
					reorgDepth, err := r.detectReorg(ctx, blockNum-1)
					if err != nil {
						log.Error("realtime: failed to check for reorg", "error", err)
					} else if reorgDepth > 0 {
						log.Warn("realtime: detected reorg", "depth", reorgDepth)
						if err := r.handleReorg(ctx, blockNum-reorgDepth); err != nil {
							log.Error("realtime: failed to handle reorg", "error", err)
						}
						r.SetLastProcessedBlock(blockNum - reorgDepth - 1)
						continue
					}
				}

				if err := r.processBlock(ctx, blockNum); err != nil {
					log.Error("realtime: failed to process block", "block", blockNum, "error", err)
					break
				}

				r.SetLastProcessedBlock(blockNum)

				// Update sync status every 10 blocks
				if blockNum%10 == 0 {
					r.db.UpdateSyncStatus(ctx, blockNum, true)
				}
			}
		}
	}
}

// runPollingMode runs the indexer using polling (fallback mode)
func (r *RealtimeIndexer) runPollingMode(ctx context.Context) error {
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-r.ctx.Done():
			return r.ctx.Err()

		case req := <-r.indexRequests:
			// On-demand indexing request
			err := r.processBlock(ctx, req.blockNumber)
			req.done <- err

		case <-ticker.C:
			latestOnChain, err := r.rpc.BlockNumber(ctx)
			if err != nil {
				log.Error("realtime: failed to get latest block number", "error", err)
				continue
			}

			safeBlock := latestOnChain - uint64(r.config.ConfirmationBlocks)
			lastProcessed := r.GetLastProcessedBlock()

			for blockNum := lastProcessed + 1; blockNum <= safeBlock; blockNum++ {
				// Check for on-demand requests
				select {
				case req := <-r.indexRequests:
					err := r.processBlock(ctx, req.blockNumber)
					req.done <- err
				default:
				}

				// Check for reorg
				if blockNum > 0 {
					reorgDepth, err := r.detectReorg(ctx, blockNum-1)
					if err != nil {
						log.Error("realtime: failed to check for reorg", "error", err)
					} else if reorgDepth > 0 {
						log.Warn("realtime: detected reorg", "depth", reorgDepth)
						if err := r.handleReorg(ctx, blockNum-reorgDepth); err != nil {
							log.Error("realtime: failed to handle reorg", "error", err)
						}
						r.SetLastProcessedBlock(blockNum - reorgDepth - 1)
						continue
					}
				}

				if err := r.processBlock(ctx, blockNum); err != nil {
					log.Error("realtime: failed to process block", "block", blockNum, "error", err)
					break
				}

				r.SetLastProcessedBlock(blockNum)

				// Update sync status
				if blockNum%10 == 0 {
					r.db.UpdateSyncStatus(ctx, blockNum, true)
				}
			}
		}
	}
}

// processBlock processes a single block
func (r *RealtimeIndexer) processBlock(ctx context.Context, number uint64) error {
	block, err := r.rpc.BlockByNumber(ctx, big.NewInt(int64(number)))
	if err != nil {
		return err
	}

	txCount := len(block.Transactions())
	if txCount > 0 {
		return r.processBlockWithTxs(ctx, block)
	}

	// Empty block - just insert the block record
	return r.insertEmptyBlock(ctx, block)
}

// insertEmptyBlock inserts a block with no transactions
func (r *RealtimeIndexer) insertEmptyBlock(ctx context.Context, block *ethtypes.Block) error {
	var baseFeePerGas *uint64
	if block.BaseFee() != nil {
		baseFee := block.BaseFee().Uint64()
		baseFeePerGas = &baseFee
	}

	// Get total difficulty (separate RPC call for post-merge compatibility)
	totalDifficulty := r.rpc.GetTotalDifficulty(ctx, block.NumberU64())

	b := &db.BlockData{
		Block: &types.Block{
			Number:           block.NumberU64(),
			Hash:             block.Hash().Hex(),
			ParentHash:       block.ParentHash().Hex(),
			Timestamp:        block.Time(),
			GasUsed:          block.GasUsed(),
			GasLimit:         block.GasLimit(),
			BaseFeePerGas:    baseFeePerGas,
			TransactionCount: 0,
			// Additional block fields
			Size:             block.Size(),
			Difficulty:       block.Difficulty().String(),
			TotalDifficulty:  totalDifficulty,
			Nonce:            fmt.Sprintf("0x%016x", block.Nonce()),
			Miner:            block.Coinbase().Hex(),
			ExtraData:        common.Bytes2Hex(block.Extra()),
			StateRoot:        block.Root().Hex(),
			TransactionsRoot: block.TxHash().Hex(),
			ReceiptsRoot:     block.ReceiptHash().Hex(),
		},
		Transactions:         make([]*types.Transaction, 0),
		Logs:                 make([]*types.Log, 0),
		Transfers:            make([]*types.TokenTransfer, 0),
		Contracts:            make([]*types.Contract, 0),
		Tokens:               make([]*types.Token, 0),
		InternalTransactions: make([]*types.InternalTransaction, 0),
		AddressStats:         make(map[string]*db.AddressStatsDelta),
	}

	// Publish new block event
	if r.eventBus != nil {
		r.eventBus.PublishNewBlock(b.Block)
	}

	return r.db.InsertBlockDataBatch(ctx, b)
}

// processBlockWithTxs processes a block with transactions
func (r *RealtimeIndexer) processBlockWithTxs(ctx context.Context, block *ethtypes.Block) error {
	// Create a temporary Indexer instance to reuse processBlockParallel logic
	idx := &Indexer{
		db:               r.db,
		rpc:              r.rpc,
		config:           r.idxCfg,
		tokenCache:       r.tokenCache,
		contractCache:    r.contractCache,
		balanceWorkers:   r.balanceWorkers,
		tracingSupported: r.tracingSupported,
		eventBus:         r.eventBus,
	}

	return idx.processBlockParallel(ctx, block)
}

// detectReorg checks if recent blocks have been reorganized
func (r *RealtimeIndexer) detectReorg(ctx context.Context, blockNumber uint64) (uint64, error) {
	maxReorgCheck := uint64(10)
	for depth := uint64(0); depth < maxReorgCheck && blockNumber > depth; depth++ {
		checkBlock := blockNumber - depth

		storedBlock, err := r.db.GetBlock(ctx, checkBlock)
		if err != nil || storedBlock == nil {
			continue
		}

		chainBlock, err := r.rpc.BlockByNumber(ctx, big.NewInt(int64(checkBlock)))
		if err != nil {
			return 0, err
		}

		if storedBlock.Hash != chainBlock.Hash().Hex() {
			log.Warn("realtime: reorg detected",
				"block", checkBlock,
				"stored_hash", storedBlock.Hash[:16],
				"chain_hash", chainBlock.Hash().Hex()[:16])
			return depth + 1, nil
		}
	}
	return 0, nil
}

// handleReorg reverts blocks from the given block number onwards
func (r *RealtimeIndexer) handleReorg(ctx context.Context, fromBlock uint64) error {
	log.Info("realtime: reverting blocks due to reorg", "from_block", fromBlock)

	lastIndexed, err := r.db.GetLatestBlockNumber(ctx)
	if err != nil {
		return err
	}

	for blockNum := lastIndexed; blockNum >= fromBlock; blockNum-- {
		if err := r.db.DeleteBlock(ctx, blockNum); err != nil {
			log.Error("realtime: failed to delete block", "block", blockNum, "error", err)
			return err
		}
		log.Debug("realtime: reverted block", "block", blockNum)
	}

	return nil
}
