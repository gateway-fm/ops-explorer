package opdeposits

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"explorer/internal/db"
	"explorer/internal/log"
	"explorer/internal/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TransactionDeposited event signature:
// TransactionDeposited(address indexed from, address indexed to, uint256 indexed version, bytes opaqueData)
var transactionDepositedTopic = crypto.Keccak256Hash([]byte("TransactionDeposited(address,address,uint256,bytes)"))

// FetcherConfig holds configuration for the L1 deposit fetcher.
type FetcherConfig struct {
	L1RPCURL              string
	OptimismPortalAddress common.Address
	PollInterval          time.Duration
	BatchSize             int
	StartBlock            uint64 // 0 = auto-detect from DB
	ConfirmationBlocks    uint64 // L1 confirmation buffer (default: 12)
}

// Fetcher watches L1 for TransactionDeposited events and populates the op_deposits table.
type Fetcher struct {
	db     *db.DB
	config *FetcherConfig

	l1Client *ethclient.Client
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewFetcher creates a new L1 deposit fetcher.
func NewFetcher(database *db.DB, cfg *FetcherConfig) *Fetcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Fetcher{
		db:     database,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the L1 event watching loop.
func (f *Fetcher) Start(ctx context.Context) error {
	client, err := ethclient.Dial(f.config.L1RPCURL)
	if err != nil {
		return fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}
	f.l1Client = client

	go f.run(ctx)
	return nil
}

// Stop gracefully stops the fetcher.
func (f *Fetcher) Stop() {
	f.cancel()
}

func (f *Fetcher) run(ctx context.Context) {
	ticker := time.NewTicker(f.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			if err := f.poll(ctx); err != nil {
				log.Error("op_deposits: poll error", "error", err)
			}
		}
	}
}

func (f *Fetcher) poll(ctx context.Context) error {
	// Determine start block
	fromBlock, err := f.getStartBlock(ctx)
	if err != nil {
		return err
	}

	// Get L1 chain head
	l1Head, err := f.l1Client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L1 block number: %w", err)
	}

	// Apply confirmation buffer
	safeHead := l1Head
	if safeHead > f.config.ConfirmationBlocks {
		safeHead -= f.config.ConfirmationBlocks
	} else {
		return nil // L1 not far enough ahead
	}

	if fromBlock > safeHead {
		return nil // Already caught up
	}

	// Process in batches
	batchSize := uint64(f.config.BatchSize)
	for batchStart := fromBlock; batchStart <= safeHead; batchStart += batchSize {
		batchEnd := batchStart + batchSize - 1
		if batchEnd > safeHead {
			batchEnd = safeHead
		}

		deposits, err := f.fetchDepositEvents(ctx, batchStart, batchEnd)
		if err != nil {
			return fmt.Errorf("failed to fetch deposit events [%d, %d]: %w", batchStart, batchEnd, err)
		}

		if len(deposits) > 0 {
			if err := f.db.InsertOPDepositsBatch(ctx, deposits); err != nil {
				return fmt.Errorf("failed to insert deposits: %w", err)
			}
			log.Info("op_deposits: indexed deposits",
				"l1_from", batchStart,
				"l1_to", batchEnd,
				"count", len(deposits))
		}
	}

	return nil
}

func (f *Fetcher) getStartBlock(ctx context.Context) (uint64, error) {
	// Check DB for last indexed L1 block
	lastIndexed, err := f.db.GetLastIndexedL1Block(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get last indexed L1 block: %w", err)
	}

	if lastIndexed > 0 {
		return lastIndexed + 1, nil
	}

	// Use configured start block
	if f.config.StartBlock > 0 {
		return f.config.StartBlock, nil
	}

	// Default: start from current L1 head minus confirmation blocks
	l1Head, err := f.l1Client.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get L1 block number: %w", err)
	}
	if l1Head > f.config.ConfirmationBlocks {
		return l1Head - f.config.ConfirmationBlocks, nil
	}
	return 0, nil
}

func (f *Fetcher) fetchDepositEvents(ctx context.Context, fromBlock, toBlock uint64) ([]*types.OPDeposit, error) {
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Addresses: []common.Address{f.config.OptimismPortalAddress},
		Topics:    [][]common.Hash{{transactionDepositedTopic}},
	}

	logs, err := f.l1Client.FilterLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	var deposits []*types.OPDeposit

	for _, logEntry := range logs {
		if len(logEntry.Topics) < 4 {
			continue
		}

		// Parse indexed fields
		fromAddr := common.BytesToAddress(logEntry.Topics[1].Bytes())
		toAddr := common.BytesToAddress(logEntry.Topics[2].Bytes())
		// version := logEntry.Topics[3] — unused but kept for documentation

		// Parse opaqueData from log data
		if len(logEntry.Data) < 64 {
			continue
		}

		// ABI decode: bytes is encoded as offset (32) + length (32) + data
		offset := new(big.Int).SetBytes(logEntry.Data[:32]).Uint64()
		if offset+32 > uint64(len(logEntry.Data)) {
			continue
		}
		length := new(big.Int).SetBytes(logEntry.Data[offset : offset+32]).Uint64()
		if offset+32+length > uint64(len(logEntry.Data)) {
			continue
		}
		opaqueData := logEntry.Data[offset+32 : offset+32+length]

		// Parse opaque data fields
		mint, value, gasLimit, isCreation, data, err := ParseOpaqueData(opaqueData)
		if err != nil {
			log.Warn("op_deposits: failed to parse opaque data",
				"tx", logEntry.TxHash.Hex(),
				"error", err)
			continue
		}

		// Compute deposit destination
		var depositTo *common.Address
		if !isCreation {
			depositTo = &toAddr
		}

		// Compute source hash
		sourceHash := ComputeSourceHash(logEntry.BlockHash, uint64(logEntry.Index))

		// Compute L2 deposit tx hash
		l2TxHash, err := ComputeL2DepositTxHash(sourceHash, fromAddr, depositTo, mint, value, gasLimit, false, data)
		if err != nil {
			log.Warn("op_deposits: failed to compute L2 tx hash",
				"l1_tx", logEntry.TxHash.Hex(),
				"error", err)
			continue
		}

		// Get L1 block timestamp
		l1Block, err := f.l1Client.HeaderByNumber(ctx, new(big.Int).SetUint64(logEntry.BlockNumber))
		var l1Timestamp *uint64
		if err == nil && l1Block != nil {
			ts := l1Block.Time
			l1Timestamp = &ts
		}

		deposits = append(deposits, &types.OPDeposit{
			L2TxHash:        l2TxHash.Hex(),
			L1BlockNumber:   logEntry.BlockNumber,
			L1BlockTimestamp: l1Timestamp,
			L1TxHash:        logEntry.TxHash.Hex(),
			L1TxOrigin:      fromAddr.Hex(),
		})
	}

	return deposits, nil
}
