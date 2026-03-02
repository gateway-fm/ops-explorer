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

type RealtimeConfig struct {
	ConfirmationBlocks int           // Blocks to wait before processing (default: 1)
	PollInterval       time.Duration // Fallback poll interval if WS fails
}

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

func (r *RealtimeIndexer) SetEventBus(bus *events.Bus) {
	r.eventBus = bus
}

func (r *RealtimeIndexer) SetLastProcessedBlock(blockNum uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastProcessedBlock = blockNum
}

func (r *RealtimeIndexer) GetLastProcessedBlock() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastProcessedBlock
}

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

func (r *RealtimeIndexer) Start(ctx context.Context, startBlock uint64) error {
	r.SetLastProcessedBlock(startBlock)

	log.Info("realtime: starting",
		"from_block", startBlock,
		"confirmation_blocks", r.config.ConfirmationBlocks)

	headers := make(chan *ethtypes.Header)
	sub, err := r.rpc.SubscribeNewHead(ctx, headers)
	if err != nil {
		log.Warn("realtime: WebSocket subscription failed, falling back to polling", "error", err)
		return r.runPollingMode(ctx)
	}

	log.Info("realtime: WebSocket subscription active")
	return r.runSubscriptionMode(ctx, headers, sub)
}

func (r *RealtimeIndexer) Stop() {
	r.cancel()
}

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
			err := r.processBlock(ctx, req.blockNumber)
			req.done <- err

		case header := <-headers:
			if header == nil {
				continue
			}

			latestBlock := header.Number.Uint64()
			safeBlock := latestBlock - uint64(r.config.ConfirmationBlocks)

			lastProcessed := r.GetLastProcessedBlock()
			for blockNum := lastProcessed + 1; blockNum <= safeBlock; blockNum++ {
				select {
				case req := <-r.indexRequests:
					err := r.processBlock(ctx, req.blockNumber)
					req.done <- err
				default:
				}

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

				if blockNum%10 == 0 {
					r.db.UpdateSyncStatus(ctx, blockNum, true)
				}
			}
		}
	}
}

// runPollingMode is the fallback when WebSocket subscriptions are unavailable
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
				select {
				case req := <-r.indexRequests:
					err := r.processBlock(ctx, req.blockNumber)
					req.done <- err
				default:
				}

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

				if blockNum%10 == 0 {
					r.db.UpdateSyncStatus(ctx, blockNum, true)
				}
			}
		}
	}
}

func (r *RealtimeIndexer) processBlock(ctx context.Context, number uint64) error {
	// Use raw JSON-RPC path for OP Stack chains
	if r.idxCfg.EnableOPDeposits {
		return r.processBlockRaw(ctx, number)
	}

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

func (r *RealtimeIndexer) processBlockRaw(ctx context.Context, number uint64) error {
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

	return idx.processBlockRaw(ctx, number)
}

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

	if r.eventBus != nil {
		r.eventBus.PublishNewBlock(b.Block)
	}

	return r.db.InsertBlockDataBatch(ctx, b)
}

func (r *RealtimeIndexer) processBlockWithTxs(ctx context.Context, block *ethtypes.Block) error {
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

func (r *RealtimeIndexer) detectReorg(ctx context.Context, blockNumber uint64) (uint64, error) {
	maxReorgCheck := uint64(10)
	for depth := uint64(0); depth < maxReorgCheck && blockNumber > depth; depth++ {
		checkBlock := blockNumber - depth

		storedBlock, err := r.db.GetBlock(ctx, checkBlock)
		if err != nil || storedBlock == nil {
			continue
		}

		// Use raw path when OP deposits are enabled
		var chainHash string
		if r.idxCfg.EnableOPDeposits {
			chainHash, err = r.rpc.RawBlockHash(ctx, checkBlock)
		} else {
			var chainBlock *ethtypes.Block
			chainBlock, err = r.rpc.BlockByNumber(ctx, big.NewInt(int64(checkBlock)))
			if err == nil {
				chainHash = chainBlock.Hash().Hex()
			}
		}
		if err != nil {
			return 0, err
		}

		if storedBlock.Hash != chainHash {
			log.Warn("realtime: reorg detected",
				"block", checkBlock,
				"stored_hash", storedBlock.Hash[:16],
				"chain_hash", chainHash[:16])
			return depth + 1, nil
		}
	}
	return 0, nil
}

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
