package indexer

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"explorer/internal/db"
	"explorer/internal/events"
	"explorer/internal/log"
	"explorer/internal/rpc"
	"explorer/internal/types"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// CatchupConfig holds configuration for the catchup indexer
type CatchupConfig struct {
	Workers   int // Number of parallel block processing workers
	BatchSize int // Number of blocks to fetch in each batch
	QueueSize int // Size of the work queue
}

// CatchupIndexer handles historical block backfilling with parallel workers
// Now pulls work from MissingRangeCollector instead of sequential ranges
type CatchupIndexer struct {
	db       *db.DB
	rpc      *rpc.Client
	config   *CatchupConfig
	idxCfg   *Config // Main indexer config for RPC settings
	eventBus *events.Bus

	// Missing range collector (shared with main indexer)
	collector *MissingRangeCollector

	// Caches (shared with main indexer)
	tokenCache    *TokenCache
	contractCache *ContractCache

	// Balance workers (shared with main indexer)
	balanceWorkers *BalanceWorkerPool

	// Tracing support
	tracingSupported bool

	// Work queue
	workQueue chan uint64

	// Progress tracking
	processedBlocks int64
	totalMissing    int64
	lastLogTime     time.Time

	// Coordination
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// Completion callback
	onComplete func()
}

// NewCatchupIndexer creates a new catchup indexer
func NewCatchupIndexer(
	database *db.DB,
	rpcClient *rpc.Client,
	cfg *CatchupConfig,
	idxCfg *Config,
	tokenCache *TokenCache,
	contractCache *ContractCache,
	balanceWorkers *BalanceWorkerPool,
	tracingSupported bool,
) *CatchupIndexer {
	ctx, cancel := context.WithCancel(context.Background())
	return &CatchupIndexer{
		db:               database,
		rpc:              rpcClient,
		config:           cfg,
		idxCfg:           idxCfg,
		tokenCache:       tokenCache,
		contractCache:    contractCache,
		balanceWorkers:   balanceWorkers,
		tracingSupported: tracingSupported,
		workQueue:        make(chan uint64, cfg.QueueSize),
		ctx:              ctx,
		cancel:           cancel,
		lastLogTime:      time.Now(),
	}
}

// SetEventBus sets the event bus for publishing indexer events
func (c *CatchupIndexer) SetEventBus(bus *events.Bus) {
	c.eventBus = bus
}

// SetOnComplete sets a callback to be called when catchup completes
func (c *CatchupIndexer) SetOnComplete(fn func()) {
	c.onComplete = fn
}

// SetCollector sets the missing range collector
func (c *CatchupIndexer) SetCollector(collector *MissingRangeCollector) {
	c.collector = collector
}

// Start begins the catchup indexing process
// This runs continuously, polling for missing ranges from the collector.
// It will process existing gaps and continue monitoring for new gaps
// that appear when the chain head advances.
func (c *CatchupIndexer) Start(ctx context.Context, fromBlock, toBlock uint64) error {
	// Get total missing blocks for progress tracking
	if c.collector != nil {
		total, _ := c.collector.GetTotalMissingBlocks(ctx)
		c.totalMissing = total
	} else {
		c.totalMissing = int64(toBlock - fromBlock)
	}

	log.Info("catchup: starting (continuous mode)",
		"initial_missing", c.totalMissing,
		"workers", c.config.Workers)

	// Start workers - they will process blocks from the work queue
	for i := 0; i < c.config.Workers; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}

	// Start the block producer that feeds from missing ranges
	// This runs continuously, polling for new ranges
	c.wg.Add(1)
	go c.blockProducerFromRanges()

	// Note: We don't wait for completion here because catchup runs continuously.
	// The blockProducerFromRanges() handles the completion callback internally
	// when it goes idle (no missing ranges for a while).

	return nil
}

// blockProducerFromRanges fetches missing ranges and enqueues blocks
// This runs continuously, polling for new missing ranges that may be added
// by the forward scanner when the chain head advances
func (c *CatchupIndexer) blockProducerFromRanges() {
	defer c.wg.Done()
	defer close(c.workQueue)

	batchSize := c.config.BatchSize
	if batchSize == 0 {
		batchSize = 100
	}

	// Track idle state for completion callback
	idleCount := 0
	completionCalled := false
	pollInterval := 5 * time.Second // Poll every 5 seconds when idle

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Get a batch of missing ranges from the collector or database
		var ranges []db.BlockRange
		var err error

		if c.collector != nil {
			ranges, err = c.collector.GetMissingRangesBatch(c.ctx, batchSize)
		} else {
			ranges, err = c.db.GetMissingRangesBatch(c.ctx, batchSize)
		}

		if err != nil {
			log.Error("catchup: failed to get missing ranges", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(ranges) == 0 {
			// No missing ranges right now - but keep polling!
			// New ranges may be added when chain head advances
			idleCount++

			// Call completion callback once after being idle for a while
			// This triggers address_stats rebuild but doesn't stop catchup
			if !completionCalled && idleCount >= 3 {
				processed := atomic.LoadInt64(&c.processedBlocks)
				if processed > 0 {
					log.Info("catchup: initial sync complete, continuing to monitor for new gaps",
						"processed", processed)

					// Rebuild address stats after initial catchup
					log.Info("catchup: rebuilding address_stats table")
					if err := c.db.RebuildAddressStats(context.Background()); err != nil {
						log.Error("catchup: failed to rebuild address_stats", "error", err)
					} else {
						log.Info("catchup: address_stats rebuild complete")
					}

					if c.onComplete != nil {
						c.onComplete()
					}
				}
				completionCalled = true
			}

			// Log periodically when idle
			if idleCount%12 == 1 { // Every minute (12 * 5s)
				log.Debug("catchup: waiting for new missing ranges")
			}

			// Wait before polling again
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(pollInterval):
				continue
			}
		}

		// Reset idle counter when we have work
		if idleCount > 0 {
			log.Info("catchup: new missing ranges detected, resuming",
				"ranges", len(ranges))
		}
		idleCount = 0

		// If completion was called but new work appeared, we need to skip address stats
		// for this batch (they'll be updated incrementally or rebuilt later)
		if completionCalled {
			// Reset completion flag - we'll call it again when we go idle
			completionCalled = false
		}

		// Enqueue all blocks from the ranges
		for _, r := range ranges {
			for blockNum := r.FromNumber; blockNum <= r.ToNumber; blockNum++ {
				select {
				case <-c.ctx.Done():
					return
				case c.workQueue <- blockNum:
					// Block enqueued successfully
				}
			}
		}
	}
}

// Stop gracefully stops the catchup indexer
func (c *CatchupIndexer) Stop() {
	c.cancel()
	c.wg.Wait()
}

// Progress returns the current catchup progress
func (c *CatchupIndexer) Progress() (processed int64, total uint64, percentComplete float64) {
	processed = atomic.LoadInt64(&c.processedBlocks)

	// Get current total missing (may have changed)
	if c.collector != nil {
		remaining, _ := c.collector.GetTotalMissingBlocks(c.ctx)
		total = uint64(processed) + uint64(remaining)
	} else {
		total = uint64(c.totalMissing)
	}

	if total > 0 {
		percentComplete = float64(processed) / float64(total) * 100
	}
	return
}

// IsRunning returns true if the catchup indexer is still running
func (c *CatchupIndexer) IsRunning() bool {
	select {
	case <-c.ctx.Done():
		return false
	default:
		return true
	}
}

// worker processes blocks from the work queue
func (c *CatchupIndexer) worker(id int) {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case blockNum, ok := <-c.workQueue:
			if !ok {
				// Work queue closed, exit
				return
			}

			// Check if block already exists (may have been indexed by another process)
			exists, err := c.db.HasBlock(c.ctx, blockNum)
			if err != nil {
				log.Error("catchup: failed to check block existence", "block", blockNum, "error", err)
				continue
			}
			if exists {
				// Block already indexed, mark as processed and continue
				if c.collector != nil {
					c.collector.MarkBlockProcessed(c.ctx, blockNum)
				}
				atomic.AddInt64(&c.processedBlocks, 1)
				continue
			}

			if err := c.processBlock(blockNum); err != nil {
				log.Error("catchup: failed to process block", "worker", id, "block", blockNum, "error", err)
				// Don't mark as processed on error - will be retried
				continue
			}

			// Mark block as processed in collector
			if c.collector != nil {
				c.collector.MarkBlockProcessed(c.ctx, blockNum)
			} else {
				c.db.DeleteMissingRangeByBlock(c.ctx, blockNum)
			}

			processed := atomic.AddInt64(&c.processedBlocks, 1)

			// Log progress periodically
			if time.Since(c.lastLogTime) > 5*time.Second {
				c.lastLogTime = time.Now()

				var remaining int64
				if c.collector != nil {
					remaining, _ = c.collector.GetTotalMissingBlocks(c.ctx)
				} else {
					remaining, _ = c.db.GetTotalMissingBlocks(c.ctx)
				}

				total := processed + remaining
				percent := float64(0)
				if total > 0 {
					percent = float64(processed) / float64(total) * 100
				}

				log.Info("catchup: progress",
					"processed", processed,
					"remaining", remaining,
					"percent", percent)
			}
		}
	}
}

// processBlock processes a single block
func (c *CatchupIndexer) processBlock(number uint64) error {
	block, err := c.rpc.BlockByNumber(c.ctx, big.NewInt(int64(number)))
	if err != nil {
		return err
	}

	txCount := len(block.Transactions())
	if txCount > 0 {
		return c.processBlockWithTxs(block)
	}

	// Empty block - just insert the block record
	return c.insertEmptyBlock(block)
}

// insertEmptyBlock inserts a block with no transactions
func (c *CatchupIndexer) insertEmptyBlock(block *ethtypes.Block) error {
	var baseFeePerGas *uint64
	if block.BaseFee() != nil {
		baseFee := block.BaseFee().Uint64()
		baseFeePerGas = &baseFee
	}

	// Get total difficulty (separate RPC call for post-merge compatibility)
	totalDifficulty := c.rpc.GetTotalDifficulty(c.ctx, block.NumberU64())

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
		SkipAddressStats:     true, // Skip during catchup to avoid deadlocks
	}

	return c.db.InsertBlockDataBatch(c.ctx, b)
}

// processBlockWithTxs processes a block with transactions (similar to main indexer)
func (c *CatchupIndexer) processBlockWithTxs(block *ethtypes.Block) error {
	// Create a config copy with SkipAddressStats enabled
	catchupConfig := *c.idxCfg
	catchupConfig.SkipAddressStats = true

	// Create a temporary Indexer instance to reuse processBlockParallel logic
	// This is a bit of a hack, but it avoids duplicating the complex block processing code
	idx := &Indexer{
		db:               c.db,
		rpc:              c.rpc,
		config:           &catchupConfig,
		tokenCache:       c.tokenCache,
		contractCache:    c.contractCache,
		balanceWorkers:   c.balanceWorkers,
		tracingSupported: c.tracingSupported,
		eventBus:         c.eventBus,
	}

	return idx.processBlockParallel(c.ctx, block)
}
