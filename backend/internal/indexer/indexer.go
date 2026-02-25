package indexer

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"explorer/internal/db"
	"explorer/internal/events"
	"explorer/internal/log"
	"explorer/internal/rpc"
	"explorer/internal/types"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Config holds parallelization settings for the indexer
type Config struct {
	RPCWorkers           int
	RPCRateLimit         int
	DBBatchSize          int
	TokenMetadataWorkers int
	BalanceWorkers       int
	EnableAsyncBalance   bool

	// Tracing settings
	EnableTracing  bool
	TraceRateLimit int
	TraceWorkers   int

	// Catchup mode settings
	CatchupEnabled   bool
	CatchupWorkers   int
	CatchupBatchSize int
	CatchupQueueSize int

	// Skip address stats during catchup (avoids deadlocks, rebuild after)
	SkipAddressStats bool

	// OP Stack deposit transaction support
	EnableOPDeposits bool
}

// ERC20 Transfer event topic
var transferTopic = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

// Zero address for detecting mints/burns
var zeroAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")

type Indexer struct {
	db           *db.DB
	rpc          *rpc.Client
	pollInterval time.Duration
	startBlock   uint64

	// Channel for on-demand block indexing requests
	indexRequests chan indexRequest

	// Parallelization config
	config *Config

	// Caches
	tokenCache    *TokenCache
	contractCache *ContractCache

	// Async balance worker pool
	balanceWorkers *BalanceWorkerPool

	// Tracing support
	tracingSupported bool

	// Event bus for publishing events
	eventBus *events.Bus

	// Missing range collector for gap discovery
	missingRangeCollector *MissingRangeCollector

	// Catchup and realtime indexers
	catchupIndexer  *CatchupIndexer
	realtimeIndexer *RealtimeIndexer

	// Mode tracking
	catchupRunning bool
}

// SetEventBus sets the event bus for publishing indexer events
func (i *Indexer) SetEventBus(bus *events.Bus) {
	i.eventBus = bus
}

// GetCatchupProgress returns the catchup progress if catchup is running
func (i *Indexer) GetCatchupProgress() (processed int64, total uint64, percentComplete float64, isRunning bool) {
	if i.catchupIndexer == nil || !i.catchupRunning {
		return 0, 0, 100, false
	}
	processed, total, percentComplete = i.catchupIndexer.Progress()
	return processed, total, percentComplete, true
}

// IsCatchupRunning returns true if catchup indexing is in progress
func (i *Indexer) IsCatchupRunning() bool {
	return i.catchupRunning
}

// GetMissingRangeProgress returns the missing range collector progress
func (i *Indexer) GetMissingRangeProgress() (minFetched, maxFetched, chainHead uint64, backfillComplete bool, totalMissing int64) {
	if i.missingRangeCollector == nil {
		return 0, 0, 0, true, 0
	}
	minFetched, maxFetched, chainHead, backfillComplete = i.missingRangeCollector.GetProgress()
	totalMissing, _ = i.missingRangeCollector.GetTotalMissingBlocks(context.Background())
	return
}

type indexRequest struct {
	blockNumber uint64
	done        chan error
}

func New(database *db.DB, rpcClient *rpc.Client, pollInterval time.Duration, startBlock uint64) *Indexer {
	// Default config for backwards compatibility
	return NewWithConfig(database, rpcClient, pollInterval, startBlock, &Config{
		RPCWorkers:           50,
		RPCRateLimit:         500,
		DBBatchSize:          500,
		TokenMetadataWorkers: 20,
		BalanceWorkers:       30,
		EnableAsyncBalance:   true,
		EnableTracing:        false,
		TraceRateLimit:       50,
		TraceWorkers:         10,
		CatchupEnabled:       true,
		CatchupWorkers:       10,
		CatchupBatchSize:     100,
		CatchupQueueSize:     1000,
	})
}

func NewWithConfig(database *db.DB, rpcClient *rpc.Client, pollInterval time.Duration, startBlock uint64, cfg *Config) *Indexer {
	idx := &Indexer{
		db:            database,
		rpc:           rpcClient,
		pollInterval:  pollInterval,
		startBlock:    startBlock,
		indexRequests: make(chan indexRequest, 100),
		config:        cfg,
		tokenCache:    NewTokenCache(),
		contractCache: NewContractCache(),
	}

	// Initialize balance workers if async balance is enabled
	if cfg.EnableAsyncBalance && cfg.BalanceWorkers > 0 {
		idx.balanceWorkers = NewBalanceWorkerPool(database, rpcClient, cfg.BalanceWorkers, cfg.RPCRateLimit)
	}

	return idx
}

// IndexBlock indexes a specific block on-demand and waits for completion.
// This is used by the API when a user requests data that hasn't been indexed yet.
func (i *Indexer) IndexBlock(ctx context.Context, blockNumber uint64) error {
	done := make(chan error, 1)
	select {
	case i.indexRequests <- indexRequest{blockNumber: blockNumber, done: done}:
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

func (i *Indexer) Start(ctx context.Context) error {
	lastIndexed, err := i.db.GetLatestBlockNumber(ctx)
	if err != nil {
		log.Error("failed to get latest indexed block", "error", err)
	}

	// Pre-load token cache from database
	if err := i.tokenCache.LoadFromDB(ctx, i.db); err != nil {
		log.Warn("failed to pre-load token cache", "error", err)
	} else {
		log.Info("loaded tokens into cache", "count", i.tokenCache.Size())
	}

	// Check tracing support if enabled
	if i.config.EnableTracing {
		supported, err := i.rpc.CheckTracingSupport(ctx)
		if err != nil {
			log.Warn("failed to check tracing support", "error", err)
		} else if supported {
			i.tracingSupported = true
			log.Info("tracing enabled: node supports debug_traceBlockByNumber")
		} else {
			log.Warn("tracing requested but node does not support it, disabling")
			i.tracingSupported = false
		}
	}

	// Start balance workers if enabled
	if i.balanceWorkers != nil {
		i.balanceWorkers.Start()
	}

	// Get latest block on chain
	latestOnChain, err := i.rpc.BlockNumber(ctx)
	if err != nil {
		return err
	}

	// Get block count to determine if we have gaps
	blockCount, err := i.db.GetBlockCount(ctx)
	if err != nil {
		log.Warn("failed to get block count", "error", err)
	}

	// Calculate expected blocks vs actual (to detect gaps)
	var expectedBlocks uint64
	if lastIndexed > i.startBlock {
		expectedBlocks = lastIndexed - i.startBlock + 1
	}
	hasGaps := blockCount < int64(expectedBlocks)

	// Calculate forward catchup needed
	blocksToSync := uint64(0)
	if latestOnChain > lastIndexed {
		blocksToSync = latestOnChain - lastIndexed
	}

	log.Info("indexer starting",
		"last_indexed", lastIndexed,
		"chain_head", latestOnChain,
		"blocks_indexed", blockCount,
		"expected_blocks", expectedBlocks,
		"has_gaps", hasGaps,
		"blocks_behind", blocksToSync)

	// Update sync status
	i.db.UpdateSyncStatus(ctx, lastIndexed, true)

	// Use catchup mode with missing range collector if:
	// 1. CatchupEnabled is true AND
	// 2. Either we have gaps OR we're significantly behind chain head
	catchupThreshold := uint64(100)
	useCatchup := i.config.CatchupEnabled && (hasGaps || blocksToSync > catchupThreshold)

	if useCatchup {
		// Start with missing range collector for intelligent backfilling
		return i.startWithMissingRangeCollector(ctx, latestOnChain)
	}

	// Use the standard single-loop mode for small gaps
	return i.startStandardMode(ctx, lastIndexed)
}

// startWithCatchup runs both catchup and realtime indexers in parallel
func (i *Indexer) startWithCatchup(ctx context.Context, lastIndexed, latestOnChain uint64) error {
	log.Info("starting catchup mode",
		"from_block", lastIndexed+1,
		"to_block", latestOnChain,
		"workers", i.config.CatchupWorkers)

	// Create catchup indexer
	catchupCfg := &CatchupConfig{
		Workers:   i.config.CatchupWorkers,
		BatchSize: i.config.CatchupBatchSize,
		QueueSize: i.config.CatchupQueueSize,
	}

	i.catchupIndexer = NewCatchupIndexer(
		i.db, i.rpc, catchupCfg, i.config,
		i.tokenCache, i.contractCache, i.balanceWorkers,
		i.tracingSupported,
	)
	i.catchupIndexer.SetEventBus(i.eventBus)

	// Create realtime indexer
	realtimeCfg := &RealtimeConfig{
		ConfirmationBlocks: 1,
		PollInterval:       i.pollInterval,
	}

	i.realtimeIndexer = NewRealtimeIndexer(
		i.db, i.rpc, realtimeCfg, i.config,
		i.tokenCache, i.contractCache, i.balanceWorkers,
		i.tracingSupported,
	)
	i.realtimeIndexer.SetEventBus(i.eventBus)

	// Set up catchup completion callback
	i.catchupRunning = true
	i.catchupIndexer.SetOnComplete(func() {
		i.catchupRunning = false
		log.Info("catchup indexing completed, realtime indexer continues")
	})

	// Start catchup indexer (processes historical blocks)
	// Target block is the latest on-chain when we started, minus confirmation blocks
	targetBlock := latestOnChain - 1
	if err := i.catchupIndexer.Start(ctx, lastIndexed+1, targetBlock); err != nil {
		return err
	}

	// Start realtime indexer (handles new blocks)
	// It will start from the target block and process new blocks as they come
	go func() {
		if err := i.realtimeIndexer.Start(ctx, targetBlock); err != nil {
			log.Error("realtime indexer error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Stop both indexers
	if i.catchupIndexer != nil {
		i.catchupIndexer.Stop()
	}
	if i.realtimeIndexer != nil {
		i.realtimeIndexer.Stop()
	}
	if i.balanceWorkers != nil {
		i.balanceWorkers.Stop()
	}

	return ctx.Err()
}

// startWithMissingRangeCollector uses the Blockscout-style missing range collector
// to discover and backfill all gaps from genesis to chain head
func (i *Indexer) startWithMissingRangeCollector(ctx context.Context, latestOnChain uint64) error {
	log.Info("starting with missing range collector",
		"start_block", i.startBlock,
		"chain_head", latestOnChain,
		"workers", i.config.CatchupWorkers)

	// Create missing range collector configuration
	collectorCfg := &MissingRangeCollectorConfig{
		BatchSize:            100000,                  // Scan 100k blocks at a time
		BackwardScanInterval: 10 * time.Millisecond,  // Fast initial backward scan
		ForwardScanInterval:  1 * time.Minute,        // Check for new blocks every minute
		FirstBlock:           i.startBlock,           // Start from configured block (usually 0)
		LastBlock:            latestOnChain,          // Scan up to current chain head
	}

	// Create and start the missing range collector
	i.missingRangeCollector = NewMissingRangeCollector(i.db, i.rpc, collectorCfg)
	if err := i.missingRangeCollector.Start(ctx); err != nil {
		return err
	}

	// Create catchup indexer
	catchupCfg := &CatchupConfig{
		Workers:   i.config.CatchupWorkers,
		BatchSize: i.config.CatchupBatchSize,
		QueueSize: i.config.CatchupQueueSize,
	}

	i.catchupIndexer = NewCatchupIndexer(
		i.db, i.rpc, catchupCfg, i.config,
		i.tokenCache, i.contractCache, i.balanceWorkers,
		i.tracingSupported,
	)
	i.catchupIndexer.SetEventBus(i.eventBus)
	i.catchupIndexer.SetCollector(i.missingRangeCollector)

	// Create realtime indexer for new blocks
	realtimeCfg := &RealtimeConfig{
		ConfirmationBlocks: 1,
		PollInterval:       i.pollInterval,
	}

	i.realtimeIndexer = NewRealtimeIndexer(
		i.db, i.rpc, realtimeCfg, i.config,
		i.tokenCache, i.contractCache, i.balanceWorkers,
		i.tracingSupported,
	)
	i.realtimeIndexer.SetEventBus(i.eventBus)

	// Set up catchup completion callback
	i.catchupRunning = true
	i.catchupIndexer.SetOnComplete(func() {
		i.catchupRunning = false
		log.Info("catchup indexing completed, realtime indexer continues")
	})

	// Start catchup indexer (processes missing blocks from collector)
	if err := i.catchupIndexer.Start(ctx, i.startBlock, latestOnChain); err != nil {
		return err
	}

	// Start realtime indexer (handles new blocks as they arrive)
	go func() {
		if err := i.realtimeIndexer.Start(ctx, latestOnChain); err != nil {
			log.Error("realtime indexer error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Stop all components
	if i.missingRangeCollector != nil {
		i.missingRangeCollector.Stop()
	}
	if i.catchupIndexer != nil {
		i.catchupIndexer.Stop()
	}
	if i.realtimeIndexer != nil {
		i.realtimeIndexer.Stop()
	}
	if i.balanceWorkers != nil {
		i.balanceWorkers.Stop()
	}

	return ctx.Err()
}

// startStandardMode runs the original single-loop indexer mode
func (i *Indexer) startStandardMode(ctx context.Context, lastIndexed uint64) error {
	log.Info("starting standard mode",
		"workers", i.config.RPCWorkers,
		"rate_limit", i.config.RPCRateLimit,
		"tracing", i.tracingSupported)

	ticker := time.NewTicker(i.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Stop balance workers gracefully
			if i.balanceWorkers != nil {
				i.balanceWorkers.Stop()
			}
			i.db.UpdateSyncStatus(ctx, lastIndexed, false)
			return ctx.Err()

		case req := <-i.indexRequests:
			// On-demand indexing request from API
			err := i.processBlock(ctx, req.blockNumber)
			req.done <- err

		case <-ticker.C:
			latestOnChain, err := i.rpc.BlockNumber(ctx)
			if err != nil {
				log.Error("failed to get latest block number", "error", err)
				continue
			}

			// Check for reorg on the last indexed block before continuing
			if lastIndexed > 0 {
				reorgDepth, err := i.detectReorg(ctx, lastIndexed)
				if err != nil {
					log.Error("failed to check for reorg", "error", err)
				} else if reorgDepth > 0 {
					log.Warn("detected reorg, reverting blocks", "depth", reorgDepth)
					if err := i.handleReorg(ctx, lastIndexed-reorgDepth+1); err != nil {
						log.Error("failed to handle reorg", "error", err)
					} else {
						lastIndexed = lastIndexed - reorgDepth
					}
				}
			}

			for blockNum := lastIndexed + 1; blockNum <= latestOnChain; blockNum++ {
				// Check for on-demand requests between blocks to stay responsive
				select {
				case req := <-i.indexRequests:
					err := i.processBlock(ctx, req.blockNumber)
					req.done <- err
				default:
				}

				if err := i.processBlock(ctx, blockNum); err != nil {
					log.Error("failed to process block", "block", blockNum, "error", err)
					break
				}
				lastIndexed = blockNum

				// Update sync status every 10 blocks
				if blockNum%10 == 0 {
					i.db.UpdateSyncStatus(ctx, lastIndexed, true)
				}
			}
		}
	}
}

// detectReorg checks if recent blocks have been reorganized
// Returns the depth of the reorg (number of blocks to revert), or 0 if no reorg
func (i *Indexer) detectReorg(ctx context.Context, blockNumber uint64) (uint64, error) {
	// Check the last few blocks for reorg
	maxReorgCheck := uint64(10) // Check up to 10 blocks back
	for depth := uint64(0); depth < maxReorgCheck && blockNumber > depth; depth++ {
		checkBlock := blockNumber - depth

		// Get stored block
		storedBlock, err := i.db.GetBlock(ctx, checkBlock)
		if err != nil || storedBlock == nil {
			continue
		}

		// Get chain block hash — use raw path when OP deposits are enabled
		var chainHash string
		if i.config.EnableOPDeposits {
			chainHash, err = i.rpc.RawBlockHash(ctx, checkBlock)
		} else {
			var chainBlock *ethtypes.Block
			chainBlock, err = i.rpc.BlockByNumber(ctx, big.NewInt(int64(checkBlock)))
			if err == nil {
				chainHash = chainBlock.Hash().Hex()
			}
		}
		if err != nil {
			return 0, err
		}

		// Compare hashes
		if storedBlock.Hash != chainHash {
			log.Warn("reorg detected",
				"block", checkBlock,
				"stored_hash", storedBlock.Hash[:16],
				"chain_hash", chainHash[:16])
			return depth + 1, nil
		}
	}
	return 0, nil
}

// handleReorg reverts blocks from the given block number onwards
func (i *Indexer) handleReorg(ctx context.Context, fromBlock uint64) error {
	log.Info("reverting blocks due to reorg", "from_block", fromBlock)

	// Delete blocks (cascades to transactions, logs, transfers due to FK constraints)
	lastIndexed, err := i.db.GetLatestBlockNumber(ctx)
	if err != nil {
		return err
	}

	for blockNum := lastIndexed; blockNum >= fromBlock; blockNum-- {
		if err := i.db.DeleteBlock(ctx, blockNum); err != nil {
			log.Error("failed to delete block", "block", blockNum, "error", err)
			return err
		}
		log.Debug("reverted block", "block", blockNum)
	}

	return nil
}

func (i *Indexer) processBlock(ctx context.Context, number uint64) error {
	// Use raw JSON-RPC path for OP Stack chains to handle deposit transactions (type 0x7E)
	if i.config.EnableOPDeposits {
		return i.processBlockRaw(ctx, number)
	}

	block, err := i.rpc.BlockByNumber(ctx, big.NewInt(int64(number)))
	if err != nil {
		return err
	}

	// Use parallel processing for blocks with transactions
	txCount := len(block.Transactions())
	if txCount > 0 {
		return i.processBlockParallel(ctx, block)
	}

	// Empty block - just insert the block record
	var baseFeePerGas *uint64
	if block.BaseFee() != nil {
		baseFee := block.BaseFee().Uint64()
		baseFeePerGas = &baseFee
	}

	// Get total difficulty (separate RPC call for post-merge compatibility)
	totalDifficulty := i.rpc.GetTotalDifficulty(ctx, block.NumberU64())

	b := &types.Block{
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
	}

	return i.db.InsertBlock(ctx, b)
}

// processBlockRaw fetches and processes a block using raw JSON-RPC to handle
// OP Stack deposit transactions (type 0x7E) that go-ethereum cannot unmarshal.
func (i *Indexer) processBlockRaw(ctx context.Context, number uint64) error {
	rawBlock, err := i.rpc.RawBlockByNumber(ctx, number)
	if err != nil {
		return err
	}

	if len(rawBlock.Transactions) > 0 {
		return i.processBlockParallelRaw(ctx, rawBlock)
	}

	// Empty block - just insert the block record
	b := &types.Block{
		Number:           rawBlock.NumberU64(),
		Hash:             rawBlock.Hash.Hex(),
		ParentHash:       rawBlock.ParentHash.Hex(),
		Timestamp:        uint64(rawBlock.Timestamp),
		GasUsed:          uint64(rawBlock.GasUsed),
		GasLimit:         uint64(rawBlock.GasLimit),
		BaseFeePerGas:    rawBlock.BaseFeeU64(),
		TransactionCount: 0,
		Size:             uint64(rawBlock.Size),
		Difficulty:       rawBlock.DifficultyString(),
		TotalDifficulty:  rawBlock.TotalDifficultyString(),
		Nonce:            rawBlock.NonceHex(),
		Miner:            rawBlock.Miner.Hex(),
		ExtraData:        common.Bytes2Hex(rawBlock.ExtraData),
		StateRoot:        rawBlock.StateRoot.Hex(),
		TransactionsRoot: rawBlock.TransactionsRoot.Hex(),
		ReceiptsRoot:     rawBlock.ReceiptsRoot.Hex(),
	}

	return i.db.InsertBlock(ctx, b)
}

// processBlockParallelRaw processes a raw block with transactions.
// This mirrors processBlockParallel but reads from RawTransaction fields
// instead of go-ethereum's *types.Transaction methods, enabling support
// for OP Stack deposit transactions (type 0x7E).
func (i *Indexer) processBlockParallelRaw(ctx context.Context, rawBlock *rpc.RawBlock) error {
	start := time.Now()
	blockNumber := rawBlock.NumberU64()
	rawTxs := rawBlock.Transactions
	blockTimestamp := uint64(rawBlock.Timestamp)

	// 1. Collect all transaction hashes
	txHashes := make([]common.Hash, len(rawTxs))
	for idx, tx := range rawTxs {
		txHashes[idx] = tx.Hash
	}

	// 2. Fetch all receipts in parallel (receipts work fine for type 0x7E)
	receipts, err := i.rpc.FetchReceiptsBatch(ctx, txHashes, i.config.RPCWorkers, i.config.RPCRateLimit)
	if err != nil {
		return err
	}

	// 3. Build block data structure
	blockData := &db.BlockData{
		Block: &types.Block{
			Number:           blockNumber,
			Hash:             rawBlock.Hash.Hex(),
			ParentHash:       rawBlock.ParentHash.Hex(),
			Timestamp:        blockTimestamp,
			GasUsed:          uint64(rawBlock.GasUsed),
			GasLimit:         uint64(rawBlock.GasLimit),
			BaseFeePerGas:    rawBlock.BaseFeeU64(),
			TransactionCount: len(rawTxs),
			Size:             uint64(rawBlock.Size),
			Difficulty:       rawBlock.DifficultyString(),
			TotalDifficulty:  rawBlock.TotalDifficultyString(),
			Nonce:            rawBlock.NonceHex(),
			Miner:            rawBlock.Miner.Hex(),
			ExtraData:        common.Bytes2Hex(rawBlock.ExtraData),
			StateRoot:        rawBlock.StateRoot.Hex(),
			TransactionsRoot: rawBlock.TransactionsRoot.Hex(),
			ReceiptsRoot:     rawBlock.ReceiptsRoot.Hex(),
		},
		Transactions:         make([]*types.Transaction, 0, len(rawTxs)),
		Logs:                 make([]*types.Log, 0),
		Transfers:            make([]*types.TokenTransfer, 0),
		Contracts:            make([]*types.Contract, 0),
		Tokens:               make([]*types.Token, 0),
		InternalTransactions: make([]*types.InternalTransaction, 0),
		AddressStats:         make(map[string]*db.AddressStatsDelta),
		SkipAddressStats:     i.config.SkipAddressStats,
	}

	// Track new tokens that need metadata fetching
	newTokenAddresses := make([]common.Address, 0)
	newTokenTxHashes := make(map[common.Address]string)

	// Track balance work for async processing
	balanceWork := make([]BalanceWork, 0)

	// 4. Process all transactions and receipts
	for idx, rawTx := range rawTxs {
		receipt, ok := receipts[rawTx.Hash]
		if !ok {
			log.Warn("missing receipt for tx", "tx", rawTx.Hash.Hex())
			continue
		}

		// From is always present in JSON-RPC response (computed by the node)
		from := rawTx.From.Hex()

		var to *string
		if rawTx.To != nil {
			toStr := rawTx.To.Hex()
			to = &toStr
		}

		// Store full input data
		inputData := ""
		if len(rawTx.Input) > 0 {
			inputData = common.Bytes2Hex(rawTx.Input)
		}

		// Nonce: nil for deposit txs (store as 0)
		var nonce uint64
		if rawTx.Nonce != nil {
			nonce = uint64(*rawTx.Nonce)
		}

		gasLimit := uint64(rawTx.Gas)
		txType := int(rawTx.Type)

		// Gas price: nil/0 for deposit txs
		var gasPrice uint64
		if rawTx.GasPrice != nil {
			gasPrice = rawTx.GasPrice.ToInt().Uint64()
		}

		// Value
		value := "0"
		if rawTx.Value != nil {
			value = rawTx.Value.ToInt().String()
		}

		// EIP-1559 fields (naturally nil for deposits)
		var maxFeePerGas, maxPriorityFeePerGas *uint64
		if rawTx.MaxFeePerGas != nil {
			mfpg := rawTx.MaxFeePerGas.ToInt().Uint64()
			maxFeePerGas = &mfpg
		}
		if rawTx.MaxPriorityFeePerGas != nil {
			mpfpg := rawTx.MaxPriorityFeePerGas.ToInt().Uint64()
			maxPriorityFeePerGas = &mpfpg
		}

		var txError *string
		if receipt.Status == 0 {
			errMsg := "transaction reverted"
			txError = &errMsg
		}

		// Add transaction
		blockData.Transactions = append(blockData.Transactions, &types.Transaction{
			Hash:                 rawTx.Hash.Hex(),
			BlockNumber:          blockNumber,
			TxIndex:              idx,
			From:                 from,
			To:                   to,
			Value:                types.JSONString(value),
			GasUsed:              receipt.GasUsed,
			GasPrice:             gasPrice,
			GasLimit:             &gasLimit,
			MaxFeePerGas:         maxFeePerGas,
			MaxPriorityFeePerGas: maxPriorityFeePerGas,
			Nonce:                &nonce,
			TxType:               txType,
			InputData:            inputData,
			Status:               int(receipt.Status),
			Error:                txError,
		})

		// Update address stats for sender
		i.updateAddressStatsDelta(blockData.AddressStats, from, blockNumber, false)

		// Update address stats for recipient
		if to != nil {
			isContract := i.contractCache.Has(*to)
			i.updateAddressStatsDelta(blockData.AddressStats, *to, blockNumber, isContract)
		}

		// Check for contract creation
		if rawTx.To == nil && receipt.ContractAddress != (common.Address{}) {
			contractAddr := receipt.ContractAddress.Hex()
			code, err := i.rpc.GetCode(ctx, receipt.ContractAddress)
			if err == nil && len(code) > 0 {
				bytecodeHash := common.BytesToHash(common.FromHex(common.Bytes2Hex(code))).Hex()
				blockData.Contracts = append(blockData.Contracts, &types.Contract{
					Address:      contractAddr,
					Bytecode:     common.Bytes2Hex(code),
					BytecodeHash: &bytecodeHash,
					Creator:      from,
					CreationTx:   rawTx.Hash.Hex(),
					BlockNumber:  blockNumber,
					IsVerified:   false,
				})
				i.contractCache.Add(contractAddr)
				i.updateAddressStatsDelta(blockData.AddressStats, contractAddr, blockNumber, true)
			}
		}

		// Process logs (identical to non-raw path — receipts are the same)
		for _, logEntry := range receipt.Logs {
			var topic0, topic1, topic2, topic3 *string
			if len(logEntry.Topics) > 0 {
				t := logEntry.Topics[0].Hex()
				topic0 = &t
			}
			if len(logEntry.Topics) > 1 {
				t := logEntry.Topics[1].Hex()
				topic1 = &t
			}
			if len(logEntry.Topics) > 2 {
				t := logEntry.Topics[2].Hex()
				topic2 = &t
			}
			if len(logEntry.Topics) > 3 {
				t := logEntry.Topics[3].Hex()
				topic3 = &t
			}

			blockData.Logs = append(blockData.Logs, &types.Log{
				TxHash:      rawTx.Hash.Hex(),
				LogIndex:    int(logEntry.Index),
				Address:     logEntry.Address.Hex(),
				Topic0:      topic0,
				Topic1:      topic1,
				Topic2:      topic2,
				Topic3:      topic3,
				Data:        common.Bytes2Hex(logEntry.Data),
				BlockNumber: blockNumber,
				Timestamp:   &blockTimestamp,
				Removed:     logEntry.Removed,
			})

			// Process token transfers
			if len(logEntry.Topics) >= 3 && logEntry.Topics[0] == transferTopic {
				fromAddr := common.BytesToAddress(logEntry.Topics[1].Bytes())
				toAddr := common.BytesToAddress(logEntry.Topics[2].Bytes())

				transferType := types.TransferTypeTransfer
				if fromAddr == zeroAddress {
					transferType = types.TransferTypeMint
				} else if toAddr == zeroAddress {
					transferType = types.TransferTypeBurn
				}

				tokenType := types.TokenTypeERC20
				var tokenID *string
				if len(logEntry.Topics) == 4 {
					tokenType = types.TokenTypeERC721
					tid := logEntry.Topics[3].Big().String()
					tokenID = &tid
				}

				transferValue := "0"
				if tokenType == types.TokenTypeERC20 && len(logEntry.Data) > 0 {
					transferValue = new(big.Int).SetBytes(logEntry.Data).String()
				} else if tokenType == types.TokenTypeERC721 {
					transferValue = "1"
				}

				blockData.Transfers = append(blockData.Transfers, &types.TokenTransfer{
					TxHash:       rawTx.Hash.Hex(),
					LogIndex:     int(logEntry.Index),
					TokenAddress: logEntry.Address.Hex(),
					From:         fromAddr.Hex(),
					To:           toAddr.Hex(),
					Value:        types.JSONString(transferValue),
					BlockNumber:  blockNumber,
					Timestamp:    &blockTimestamp,
					TransferType: transferType,
					TokenType:    tokenType,
					TokenID:      tokenID,
					IsInternal:   false,
				})

				// Queue balance tracking
				if fromAddr != zeroAddress {
					balanceWork = append(balanceWork, BalanceWork{
						Address:      fromAddr,
						TokenAddress: logEntry.Address,
						BlockNumber:  blockNumber,
					})
				}
				if toAddr != zeroAddress {
					balanceWork = append(balanceWork, BalanceWork{
						Address:      toAddr,
						TokenAddress: logEntry.Address,
						BlockNumber:  blockNumber,
					})
				}

				// Check if this is a new token
				if !i.tokenCache.Has(logEntry.Address.Hex()) {
					if _, exists := newTokenTxHashes[logEntry.Address]; !exists {
						newTokenAddresses = append(newTokenAddresses, logEntry.Address)
						newTokenTxHashes[logEntry.Address] = rawTx.Hash.Hex()
					}
				}

				// Update transfer counts in address stats
				if delta, ok := blockData.AddressStats[strings.ToLower(from)]; ok {
					delta.TokenTransferDelta++
				}
				if to != nil {
					if delta, ok := blockData.AddressStats[strings.ToLower(*to)]; ok {
						delta.TokenTransferDelta++
					}
				}
			}
		}
	}

	// 5. Fetch internal transactions (traces) if enabled
	if i.tracingSupported && len(txHashes) > 0 {
		internalTxs, err := i.rpc.FetchTracesBatch(ctx, txHashes, blockNumber, blockTimestamp,
			i.config.TraceWorkers, i.config.TraceRateLimit)
		if err != nil {
			log.Warn("failed to fetch traces for block", "block", blockNumber, "error", err)
		} else if len(internalTxs) > 0 {
			blockData.InternalTransactions = internalTxs

			for _, it := range internalTxs {
				i.updateAddressStatsDeltaInternal(blockData.AddressStats, it.From, blockNumber)
				if it.To != nil {
					i.updateAddressStatsDeltaInternal(blockData.AddressStats, *it.To, blockNumber)
				}
			}
		}
	}

	// 6. Fetch metadata for new tokens in parallel
	if len(newTokenAddresses) > 0 {
		tokenMetadata, err := i.rpc.FetchTokenMetadataBatch(ctx, newTokenAddresses, i.config.TokenMetadataWorkers, i.config.RPCRateLimit)
		if err != nil {
			log.Warn("failed to fetch token metadata batch", "error", err)
		} else {
			for addr, meta := range tokenMetadata {
				if meta.Err != nil {
					continue
				}
				txHash := newTokenTxHashes[addr]
				blockData.Tokens = append(blockData.Tokens, &types.Token{
					Address:     addr.Hex(),
					Symbol:      meta.Symbol,
					Name:        meta.Name,
					Decimals:    meta.Decimals,
					TokenType:   types.TokenTypeERC20,
					BlockNumber: blockNumber,
					CreationTx:  &txHash,
				})
				i.tokenCache.Add(addr.Hex())
			}
		}
	}

	// 7. Insert all data atomically
	if err := i.db.InsertBlockDataBatch(ctx, blockData); err != nil {
		return err
	}

	// 8. Queue balance work for async processing
	if i.balanceWorkers != nil && len(balanceWork) > 0 {
		queued := i.balanceWorkers.QueueWorkBatch(balanceWork)
		if queued < len(balanceWork) {
			log.Warn("balance queue full, dropped items", "dropped", len(balanceWork)-queued)
		}
	}

	// 9. Publish events
	if i.eventBus != nil {
		i.eventBus.PublishNewBlock(blockData.Block)
		for _, tx := range blockData.Transactions {
			i.eventBus.PublishNewTransaction(tx)
		}
	}

	// Log progress
	elapsed := time.Since(start)
	if blockNumber%100 == 0 || elapsed > 2*time.Second {
		log.Info("indexed block (raw)",
			"block", blockNumber,
			"txs", len(rawTxs),
			"transfers", len(blockData.Transfers),
			"new_tokens", len(blockData.Tokens),
			"elapsed", elapsed)
	}

	return nil
}

// processBlockParallel processes a block using parallel RPC calls and batch DB inserts
func (i *Indexer) processBlockParallel(ctx context.Context, block *ethtypes.Block) error {
	start := time.Now()
	blockNumber := block.NumberU64()
	txs := block.Transactions()
	blockTimestamp := block.Time()

	// Get chain ID for transaction signing (needed for legacy txs with chainID 0)
	chainID, err := i.rpc.ChainID(ctx)
	if err != nil {
		return err
	}
	signer := ethtypes.LatestSignerForChainID(chainID)

	// 1. Collect all transaction hashes
	txHashes := make([]common.Hash, len(txs))
	for idx, tx := range txs {
		txHashes[idx] = tx.Hash()
	}

	// 2. Fetch all receipts in parallel
	receipts, err := i.rpc.FetchReceiptsBatch(ctx, txHashes, i.config.RPCWorkers, i.config.RPCRateLimit)
	if err != nil {
		return err
	}

	// 3. Build block data structure
	var baseFeePerGas *uint64
	if block.BaseFee() != nil {
		baseFee := block.BaseFee().Uint64()
		baseFeePerGas = &baseFee
	}

	// Get total difficulty (separate RPC call for post-merge compatibility)
	totalDifficulty := i.rpc.GetTotalDifficulty(ctx, blockNumber)

	blockData := &db.BlockData{
		Block: &types.Block{
			Number:           blockNumber,
			Hash:             block.Hash().Hex(),
			ParentHash:       block.ParentHash().Hex(),
			Timestamp:        blockTimestamp,
			GasUsed:          block.GasUsed(),
			GasLimit:         block.GasLimit(),
			BaseFeePerGas:    baseFeePerGas,
			TransactionCount: len(txs),
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
		Transactions:         make([]*types.Transaction, 0, len(txs)),
		Logs:                 make([]*types.Log, 0),
		Transfers:            make([]*types.TokenTransfer, 0),
		Contracts:            make([]*types.Contract, 0),
		Tokens:               make([]*types.Token, 0),
		InternalTransactions: make([]*types.InternalTransaction, 0),
		AddressStats:         make(map[string]*db.AddressStatsDelta),
		SkipAddressStats:     i.config.SkipAddressStats,
	}

	// Track new tokens that need metadata fetching
	newTokenAddresses := make([]common.Address, 0)
	newTokenTxHashes := make(map[common.Address]string)

	// Track balance work for async processing
	balanceWork := make([]BalanceWork, 0)

	// 4. Process all transactions and receipts
	for idx, tx := range txs {
		receipt, ok := receipts[tx.Hash()]
		if !ok {
			log.Warn("missing receipt for tx", "tx", tx.Hash().Hex())
			continue
		}

		// Get sender using pre-fetched signer
		from, err := ethtypes.Sender(signer, tx)
		if err != nil {
			log.Warn("failed to get sender for tx", "tx", tx.Hash().Hex(), "error", err)
			continue
		}

		var to *string
		if tx.To() != nil {
			toStr := tx.To().Hex()
			to = &toStr
		}

		// Store full input data
		inputData := ""
		if len(tx.Data()) > 0 {
			inputData = common.Bytes2Hex(tx.Data())
		}

		nonce := tx.Nonce()
		gasLimit := tx.Gas()
		txType := int(tx.Type())

		var maxFeePerGas, maxPriorityFeePerGas *uint64
		if tx.Type() == ethtypes.DynamicFeeTxType {
			if tx.GasFeeCap() != nil {
				mfpg := tx.GasFeeCap().Uint64()
				maxFeePerGas = &mfpg
			}
			if tx.GasTipCap() != nil {
				mpfpg := tx.GasTipCap().Uint64()
				maxPriorityFeePerGas = &mpfpg
			}
		}

		var txError *string
		if receipt.Status == 0 {
			errMsg := "transaction reverted"
			txError = &errMsg
		}

		// Add transaction
		blockData.Transactions = append(blockData.Transactions, &types.Transaction{
			Hash:                 tx.Hash().Hex(),
			BlockNumber:          blockNumber,
			TxIndex:              idx,
			From:                 from.Hex(),
			To:                   to,
			Value:                types.JSONString(tx.Value().String()),
			GasUsed:              receipt.GasUsed,
			GasPrice:             tx.GasPrice().Uint64(),
			GasLimit:             &gasLimit,
			MaxFeePerGas:         maxFeePerGas,
			MaxPriorityFeePerGas: maxPriorityFeePerGas,
			Nonce:                &nonce,
			TxType:               txType,
			InputData:            inputData,
			Status:               int(receipt.Status),
			Error:                txError,
		})

		// Update address stats for sender
		i.updateAddressStatsDelta(blockData.AddressStats, from.Hex(), blockNumber, false)

		// Update address stats for recipient
		if to != nil {
			isContract := i.contractCache.Has(*to)
			i.updateAddressStatsDelta(blockData.AddressStats, *to, blockNumber, isContract)
		}

		// Check for contract creation
		if tx.To() == nil && receipt.ContractAddress != (common.Address{}) {
			contractAddr := receipt.ContractAddress.Hex()
			code, err := i.rpc.GetCode(ctx, receipt.ContractAddress)
			if err == nil && len(code) > 0 {
				bytecodeHash := common.BytesToHash(common.FromHex(common.Bytes2Hex(code))).Hex()
				blockData.Contracts = append(blockData.Contracts, &types.Contract{
					Address:      contractAddr,
					Bytecode:     common.Bytes2Hex(code),
					BytecodeHash: &bytecodeHash,
					Creator:      from.Hex(),
					CreationTx:   tx.Hash().Hex(),
					BlockNumber:  blockNumber,
					IsVerified:   false,
				})
				i.contractCache.Add(contractAddr)
				i.updateAddressStatsDelta(blockData.AddressStats, contractAddr, blockNumber, true)
			}
		}

		// Process logs
		for _, logEntry := range receipt.Logs {
			var topic0, topic1, topic2, topic3 *string
			if len(logEntry.Topics) > 0 {
				t := logEntry.Topics[0].Hex()
				topic0 = &t
			}
			if len(logEntry.Topics) > 1 {
				t := logEntry.Topics[1].Hex()
				topic1 = &t
			}
			if len(logEntry.Topics) > 2 {
				t := logEntry.Topics[2].Hex()
				topic2 = &t
			}
			if len(logEntry.Topics) > 3 {
				t := logEntry.Topics[3].Hex()
				topic3 = &t
			}

			blockData.Logs = append(blockData.Logs, &types.Log{
				TxHash:      tx.Hash().Hex(),
				LogIndex:    int(logEntry.Index),
				Address:     logEntry.Address.Hex(),
				Topic0:      topic0,
				Topic1:      topic1,
				Topic2:      topic2,
				Topic3:      topic3,
				Data:        common.Bytes2Hex(logEntry.Data),
				BlockNumber: blockNumber,
				Timestamp:   &blockTimestamp,
				Removed:     logEntry.Removed,
			})

			// Process token transfers
			if len(logEntry.Topics) >= 3 && logEntry.Topics[0] == transferTopic {
				fromAddr := common.BytesToAddress(logEntry.Topics[1].Bytes())
				toAddr := common.BytesToAddress(logEntry.Topics[2].Bytes())

				transferType := types.TransferTypeTransfer
				if fromAddr == zeroAddress {
					transferType = types.TransferTypeMint
				} else if toAddr == zeroAddress {
					transferType = types.TransferTypeBurn
				}

				tokenType := types.TokenTypeERC20
				var tokenID *string
				if len(logEntry.Topics) == 4 {
					tokenType = types.TokenTypeERC721
					tid := logEntry.Topics[3].Big().String()
					tokenID = &tid
				}

				value := "0"
				if tokenType == types.TokenTypeERC20 && len(logEntry.Data) > 0 {
					value = new(big.Int).SetBytes(logEntry.Data).String()
				} else if tokenType == types.TokenTypeERC721 {
					value = "1"
				}

				blockData.Transfers = append(blockData.Transfers, &types.TokenTransfer{
					TxHash:       tx.Hash().Hex(),
					LogIndex:     int(logEntry.Index),
					TokenAddress: logEntry.Address.Hex(),
					From:         fromAddr.Hex(),
					To:           toAddr.Hex(),
					Value:        types.JSONString(value),
					BlockNumber:  blockNumber,
					Timestamp:    &blockTimestamp,
					TransferType: transferType,
					TokenType:    tokenType,
					TokenID:      tokenID,
					IsInternal:   false,
				})

				// Queue balance tracking (async if enabled)
				if fromAddr != zeroAddress {
					balanceWork = append(balanceWork, BalanceWork{
						Address:      fromAddr,
						TokenAddress: logEntry.Address,
						BlockNumber:  blockNumber,
					})
				}
				if toAddr != zeroAddress {
					balanceWork = append(balanceWork, BalanceWork{
						Address:      toAddr,
						TokenAddress: logEntry.Address,
						BlockNumber:  blockNumber,
					})
				}

				// Check if this is a new token
				if !i.tokenCache.Has(logEntry.Address.Hex()) {
					// Check if we already queued this token in this block
					if _, exists := newTokenTxHashes[logEntry.Address]; !exists {
						newTokenAddresses = append(newTokenAddresses, logEntry.Address)
						newTokenTxHashes[logEntry.Address] = tx.Hash().Hex()
					}
				}

				// Update transfer counts in address stats
				if delta, ok := blockData.AddressStats[strings.ToLower(from.Hex())]; ok {
					delta.TokenTransferDelta++
				}
				if to != nil {
					if delta, ok := blockData.AddressStats[strings.ToLower(*to)]; ok {
						delta.TokenTransferDelta++
					}
				}
			}
		}
	}

	// 5. Fetch internal transactions (traces) if enabled
	if i.tracingSupported && len(txHashes) > 0 {
		internalTxs, err := i.rpc.FetchTracesBatch(ctx, txHashes, blockNumber, blockTimestamp,
			i.config.TraceWorkers, i.config.TraceRateLimit)
		if err != nil {
			log.Warn("failed to fetch traces for block", "block", blockNumber, "error", err)
		} else if len(internalTxs) > 0 {
			blockData.InternalTransactions = internalTxs

			// Update internal tx counts in address stats
			for _, it := range internalTxs {
				i.updateAddressStatsDeltaInternal(blockData.AddressStats, it.From, blockNumber)
				if it.To != nil {
					i.updateAddressStatsDeltaInternal(blockData.AddressStats, *it.To, blockNumber)
				}
			}
		}
	}

	// 6. Fetch metadata for new tokens in parallel
	if len(newTokenAddresses) > 0 {
		tokenMetadata, err := i.rpc.FetchTokenMetadataBatch(ctx, newTokenAddresses, i.config.TokenMetadataWorkers, i.config.RPCRateLimit)
		if err != nil {
			log.Warn("failed to fetch token metadata batch", "error", err)
		} else {
			for addr, meta := range tokenMetadata {
				if meta.Err != nil {
					continue
				}
				txHash := newTokenTxHashes[addr]
				blockData.Tokens = append(blockData.Tokens, &types.Token{
					Address:     addr.Hex(),
					Symbol:      meta.Symbol,
					Name:        meta.Name,
					Decimals:    meta.Decimals,
					TokenType:   types.TokenTypeERC20,
					BlockNumber: blockNumber,
					CreationTx:  &txHash,
				})
				i.tokenCache.Add(addr.Hex())
			}
		}
	}

	// 7. Insert all data atomically
	if err := i.db.InsertBlockDataBatch(ctx, blockData); err != nil {
		return err
	}

	// 8. Queue balance work for async processing
	if i.balanceWorkers != nil && len(balanceWork) > 0 {
		queued := i.balanceWorkers.QueueWorkBatch(balanceWork)
		if queued < len(balanceWork) {
			log.Warn("balance queue full, dropped items", "dropped", len(balanceWork)-queued)
		}
	}

	// 9. Publish events to the event bus
	if i.eventBus != nil {
		// Publish new block event
		i.eventBus.PublishNewBlock(blockData.Block)

		// Publish new transaction events
		for _, tx := range blockData.Transactions {
			i.eventBus.PublishNewTransaction(tx)
		}
	}

	// Log progress
	elapsed := time.Since(start)
	if blockNumber%100 == 0 || elapsed > 2*time.Second {
		log.Info("indexed block",
			"block", blockNumber,
			"txs", len(txs),
			"transfers", len(blockData.Transfers),
			"new_tokens", len(blockData.Tokens),
			"elapsed", elapsed)
	}

	return nil
}

// updateAddressStatsDelta updates or creates an address stats delta entry
func (i *Indexer) updateAddressStatsDelta(stats map[string]*db.AddressStatsDelta, address string, blockNumber uint64, isContract bool) {
	key := strings.ToLower(address)
	if delta, ok := stats[key]; ok {
		delta.TxCountDelta++
		if isContract {
			delta.IsContract = true
		}
	} else {
		stats[key] = &db.AddressStatsDelta{
			Address:      address,
			TxCountDelta: 1,
			IsContract:   isContract,
			BlockNumber:  blockNumber,
		}
	}
}

// updateAddressStatsDeltaInternal updates address stats for internal transactions
func (i *Indexer) updateAddressStatsDeltaInternal(stats map[string]*db.AddressStatsDelta, address string, blockNumber uint64) {
	key := strings.ToLower(address)
	if delta, ok := stats[key]; ok {
		delta.InternalTxCountDelta++
	} else {
		stats[key] = &db.AddressStatsDelta{
			Address:              address,
			InternalTxCountDelta: 1,
			BlockNumber:          blockNumber,
		}
	}
}

func (i *Indexer) processTransaction(ctx context.Context, tx *ethtypes.Transaction, block *ethtypes.Block, idx int) error {
	receipt, err := i.rpc.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return err
	}

	from, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return err
	}

	var to *string
	if tx.To() != nil {
		toStr := tx.To().Hex()
		to = &toStr
	}

	// Store full input data
	inputData := ""
	if len(tx.Data()) > 0 {
		inputData = common.Bytes2Hex(tx.Data())
	}

	// Extract additional transaction fields
	nonce := tx.Nonce()
	gasLimit := tx.Gas()
	txType := int(tx.Type())

	// Extract EIP-1559 fields if available
	var maxFeePerGas, maxPriorityFeePerGas *uint64
	if tx.Type() == ethtypes.DynamicFeeTxType {
		if tx.GasFeeCap() != nil {
			mfpg := tx.GasFeeCap().Uint64()
			maxFeePerGas = &mfpg
		}
		if tx.GasTipCap() != nil {
			mpfpg := tx.GasTipCap().Uint64()
			maxPriorityFeePerGas = &mpfpg
		}
	}

	// Check for errors and revert reasons
	var txError, revertReason *string
	if receipt.Status == 0 {
		// Transaction failed - try to get revert reason
		errMsg := "transaction reverted"
		txError = &errMsg
		// Note: Getting revert reason requires calling the transaction again
		// which can be expensive, so we leave it for future enhancement
	}

	t := &types.Transaction{
		Hash:                 tx.Hash().Hex(),
		BlockNumber:          block.NumberU64(),
		TxIndex:              idx,
		From:                 from.Hex(),
		To:                   to,
		Value:                types.JSONString(tx.Value().String()),
		GasUsed:              receipt.GasUsed,
		GasPrice:             tx.GasPrice().Uint64(),
		GasLimit:             &gasLimit,
		MaxFeePerGas:         maxFeePerGas,
		MaxPriorityFeePerGas: maxPriorityFeePerGas,
		Nonce:                &nonce,
		TxType:               txType,
		InputData:            inputData,
		Status:               int(receipt.Status),
		Error:                txError,
		RevertReason:         revertReason,
	}

	if err := i.db.InsertTransaction(ctx, t); err != nil {
		return err
	}

	// Check for contract creation
	isContractCreation := tx.To() == nil && receipt.ContractAddress != (common.Address{})
	if isContractCreation {
		contractAddr := receipt.ContractAddress.Hex()
		// Fetch deployed bytecode
		code, err := i.rpc.GetCode(ctx, receipt.ContractAddress)
		if err != nil {
			log.Error("failed to get contract code", "address", contractAddr, "error", err)
		} else if len(code) > 0 {
			// Compute bytecode hash
			bytecodeHash := common.BytesToHash(common.FromHex(common.Bytes2Hex(code))).Hex()

			contract := &types.Contract{
				Address:      contractAddr,
				Bytecode:     common.Bytes2Hex(code),
				BytecodeHash: &bytecodeHash,
				Creator:      from.Hex(),
				CreationTx:   tx.Hash().Hex(),
				BlockNumber:  block.NumberU64(),
				IsVerified:   false,
			}
			if err := i.db.InsertContract(ctx, contract); err != nil {
				log.Error("failed to insert contract", "address", contractAddr, "error", err)
			} else {
				log.Info("indexed contract", "address", contractAddr, "creator", from.Hex())
			}

			// Update address stats marking it as a contract
			if err := i.db.UpsertAddressStats(ctx, contractAddr, block.NumberU64(), true); err != nil {
				log.Error("failed to update address stats for contract", "address", contractAddr, "error", err)
			}
		}
	}

	// Update address stats for sender
	if err := i.db.UpsertAddressStats(ctx, from.Hex(), block.NumberU64(), false); err != nil {
		log.Error("failed to update address stats", "address", from.Hex(), "error", err)
	}

	// Update address stats for recipient
	if to != nil {
		// Check if recipient is a contract
		isContract, _ := i.db.IsContract(ctx, *to)
		if err := i.db.UpsertAddressStats(ctx, *to, block.NumberU64(), isContract); err != nil {
			log.Error("failed to update address stats", "address", *to, "error", err)
		}
	}

	// Block timestamp for logs and transfers
	blockTimestamp := block.Time()

	// Process all logs
	for _, logEntry := range receipt.Logs {
		var topic0, topic1, topic2, topic3 *string
		if len(logEntry.Topics) > 0 {
			t := logEntry.Topics[0].Hex()
			topic0 = &t
		}
		if len(logEntry.Topics) > 1 {
			t := logEntry.Topics[1].Hex()
			topic1 = &t
		}
		if len(logEntry.Topics) > 2 {
			t := logEntry.Topics[2].Hex()
			topic2 = &t
		}
		if len(logEntry.Topics) > 3 {
			t := logEntry.Topics[3].Hex()
			topic3 = &t
		}

		logRecord := &types.Log{
			TxHash:      tx.Hash().Hex(),
			LogIndex:    int(logEntry.Index),
			Address:     logEntry.Address.Hex(),
			Topic0:      topic0,
			Topic1:      topic1,
			Topic2:      topic2,
			Topic3:      topic3,
			Data:        common.Bytes2Hex(logEntry.Data),
			BlockNumber: block.NumberU64(),
			Timestamp:   &blockTimestamp,
			Removed:     logEntry.Removed,
		}
		if err := i.db.InsertLog(ctx, logRecord); err != nil {
			log.Error("failed to insert log", "error", err)
		}
	}

	// Process token transfers
	transferCount := 0
	for _, logEntry := range receipt.Logs {
		if len(logEntry.Topics) >= 3 && logEntry.Topics[0] == transferTopic {
			fromAddr := common.BytesToAddress(logEntry.Topics[1].Bytes())
			toAddr := common.BytesToAddress(logEntry.Topics[2].Bytes())

			// Determine transfer type
			transferType := types.TransferTypeTransfer
			if fromAddr == zeroAddress {
				transferType = types.TransferTypeMint
			} else if toAddr == zeroAddress {
				transferType = types.TransferTypeBurn
			}

			// Determine token type (ERC20 vs ERC721)
			// ERC721 transfers have 4 topics (with tokenId in topic[3])
			// ERC20 transfers have 3 topics
			tokenType := types.TokenTypeERC20
			var tokenID *string
			if len(logEntry.Topics) == 4 {
				tokenType = types.TokenTypeERC721
				tid := logEntry.Topics[3].Big().String()
				tokenID = &tid
			}

			// Parse value from data for ERC20 transfers
			value := "0"
			if tokenType == types.TokenTypeERC20 && len(logEntry.Data) > 0 {
				value = new(big.Int).SetBytes(logEntry.Data).String()
			} else if tokenType == types.TokenTypeERC721 {
				// For ERC721, the "value" is typically 1 (one NFT)
				value = "1"
			}

			transfer := &types.TokenTransfer{
				TxHash:       tx.Hash().Hex(),
				LogIndex:     int(logEntry.Index),
				TokenAddress: logEntry.Address.Hex(),
				From:         fromAddr.Hex(),
				To:           toAddr.Hex(),
				Value:        types.JSONString(value),
				BlockNumber:  block.NumberU64(),
				Timestamp:    &blockTimestamp,
				TransferType: transferType,
				TokenType:    tokenType,
				TokenID:      tokenID,
				IsInternal:   false,
			}
			if err := i.db.InsertTokenTransfer(ctx, transfer); err != nil {
				log.Error("failed to insert token transfer", "error", err)
			} else {
				transferCount++

				// Track balances for sender and receiver
				i.trackBalance(ctx, fromAddr, logEntry.Address, block.NumberU64())
				i.trackBalance(ctx, toAddr, logEntry.Address, block.NumberU64())
			}

			// Try to detect and index token metadata
			i.maybeIndexToken(ctx, logEntry.Address, block.NumberU64(), tx.Hash().Hex())
		}
	}

	// Update token transfer counts in address stats
	if transferCount > 0 {
		i.db.UpdateAddressStatsCounters(ctx, from.Hex(), 0, transferCount)
		if to != nil {
			i.db.UpdateAddressStatsCounters(ctx, *to, 0, transferCount)
		}
	}

	return nil
}

// trackBalance queries and stores the balance for an address on a token at the given block
// This is used by the legacy sequential processing path
func (i *Indexer) trackBalance(ctx context.Context, addr common.Address, tokenAddress common.Address, blockNumber uint64) {
	// Skip zero address
	if addr == zeroAddress {
		return
	}

	// If async balance workers are enabled, queue the work
	if i.balanceWorkers != nil {
		i.balanceWorkers.QueueWork(BalanceWork{
			Address:      addr,
			TokenAddress: tokenAddress,
			BlockNumber:  blockNumber,
		})
		return
	}

	// Fallback to synchronous balance tracking
	addrPadded := common.LeftPadBytes(addr.Bytes(), 32)
	callData := append(common.FromHex("0x70a08231"), addrPadded...)

	balanceData, err := i.rpc.CallContract(ctx, tokenAddress, callData)
	if err != nil {
		return
	}

	if len(balanceData) < 32 {
		return
	}

	balance := new(big.Int).SetBytes(balanceData)

	balanceRecord := &types.Balance{
		Address:      addr.Hex(),
		TokenAddress: tokenAddress.Hex(),
		BlockNumber:  blockNumber,
		Balance:      types.JSONString(balance.String()),
	}

	if err := i.db.InsertBalance(ctx, balanceRecord); err != nil {
		log.Error("failed to insert balance", "address", addr.Hex(), "error", err)
	}
}

// maybeIndexToken attempts to detect and index token metadata for an ERC20/ERC721 token
// This is used by the legacy sequential processing path
func (i *Indexer) maybeIndexToken(ctx context.Context, tokenAddress common.Address, blockNumber uint64, txHash string) {
	// Check cache first (fast path)
	if i.tokenCache.Has(tokenAddress.Hex()) {
		return
	}

	// Double-check database (in case cache was not preloaded)
	existing, err := i.db.GetToken(ctx, tokenAddress.Hex())
	if err == nil && existing != nil {
		i.tokenCache.Add(tokenAddress.Hex())
		return
	}

	// Try to fetch token metadata
	symbol, name, decimals, err := i.fetchTokenMetadata(ctx, tokenAddress)
	if err != nil {
		return
	}

	token := &types.Token{
		Address:     tokenAddress.Hex(),
		Symbol:      symbol,
		Name:        name,
		Decimals:    decimals,
		TokenType:   types.TokenTypeERC20,
		BlockNumber: blockNumber,
		CreationTx:  &txHash,
	}

	if err := i.db.InsertToken(ctx, token); err != nil {
		log.Error("failed to insert token", "address", tokenAddress.Hex(), "error", err)
	} else {
		i.tokenCache.Add(tokenAddress.Hex())
		log.Info("indexed token", "symbol", symbol, "address", tokenAddress.Hex())
	}
}

// fetchTokenMetadata attempts to fetch ERC20 token metadata
func (i *Indexer) fetchTokenMetadata(ctx context.Context, tokenAddress common.Address) (symbol string, name *string, decimals int, err error) {
	// ERC20 symbol() = 0x95d89b41
	symbolData, err := i.rpc.CallContract(ctx, tokenAddress, common.FromHex("0x95d89b41"))
	if err != nil {
		return "", nil, 0, err
	}
	symbol = parseStringResult(symbolData)
	if symbol == "" {
		symbol = "UNKNOWN"
	}

	// ERC20 name() = 0x06fdde03
	nameData, err := i.rpc.CallContract(ctx, tokenAddress, common.FromHex("0x06fdde03"))
	if err == nil {
		n := parseStringResult(nameData)
		if n != "" {
			name = &n
		}
	}

	// ERC20 decimals() = 0x313ce567
	decimalsData, err := i.rpc.CallContract(ctx, tokenAddress, common.FromHex("0x313ce567"))
	if err == nil && len(decimalsData) >= 32 {
		decimals = int(new(big.Int).SetBytes(decimalsData).Int64())
	} else {
		decimals = 18 // Default to 18 decimals
	}

	return symbol, name, decimals, nil
}

// parseStringResult parses a string from Solidity ABI-encoded data
func parseStringResult(data []byte) string {
	if len(data) < 64 {
		// Try to parse as raw bytes
		return strings.TrimRight(string(data), "\x00")
	}

	// ABI-encoded string: offset (32 bytes) + length (32 bytes) + data
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

	return strings.TrimRight(string(data[offset+32:offset+32+length]), "\x00")
}
