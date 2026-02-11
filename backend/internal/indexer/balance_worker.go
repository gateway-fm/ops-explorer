package indexer

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"time"

	"explorer/internal/db"
	"explorer/internal/log"
	"explorer/internal/rpc"
	"explorer/internal/types"

	"github.com/ethereum/go-ethereum/common"
)

// BalanceWork represents a balance tracking work item
type BalanceWork struct {
	Address      common.Address
	TokenAddress common.Address
	BlockNumber  uint64
}

// BalanceWorkerPool manages async balance tracking workers
type BalanceWorkerPool struct {
	db         *db.DB
	rpc        *rpc.Client
	workChan   chan BalanceWork
	numWorkers int
	rateLimit  int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewBalanceWorkerPool creates a new balance worker pool
func NewBalanceWorkerPool(database *db.DB, rpcClient *rpc.Client, numWorkers, rateLimit int) *BalanceWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &BalanceWorkerPool{
		db:         database,
		rpc:        rpcClient,
		workChan:   make(chan BalanceWork, 10000), // Buffer for 10k balance requests
		numWorkers: numWorkers,
		rateLimit:  rateLimit,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start launches the worker goroutines
func (p *BalanceWorkerPool) Start() {
	log.Info("starting balance workers", "count", p.numWorkers)
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop gracefully shuts down the worker pool
func (p *BalanceWorkerPool) Stop() {
	log.Info("stopping balance workers")
	p.cancel()
	close(p.workChan)
	p.wg.Wait()
	log.Info("balance workers stopped")
}

// QueueWork adds a balance tracking request to the queue
// Returns false if the queue is full (non-blocking)
func (p *BalanceWorkerPool) QueueWork(work BalanceWork) bool {
	select {
	case p.workChan <- work:
		return true
	default:
		// Queue full, drop the work item
		return false
	}
}

// QueueWorkBatch adds multiple balance tracking requests
func (p *BalanceWorkerPool) QueueWorkBatch(works []BalanceWork) int {
	queued := 0
	for _, work := range works {
		if p.QueueWork(work) {
			queued++
		}
	}
	return queued
}

// QueueSize returns the current number of items in the queue
func (p *BalanceWorkerPool) QueueSize() int {
	return len(p.workChan)
}

// worker processes balance work items
func (p *BalanceWorkerPool) worker(id int) {
	defer p.wg.Done()

	// Batch accumulator for DB inserts
	batch := make([]*types.Balance, 0, 100)

	for {
		select {
		case <-p.ctx.Done():
			// Flush remaining batch before exit
			if len(batch) > 0 {
				p.flushBatch(batch)
			}
			return
		case work, ok := <-p.workChan:
			if !ok {
				// Channel closed
				if len(batch) > 0 {
					p.flushBatch(batch)
				}
				return
			}

			// Skip zero address
			if work.Address == (common.Address{}) {
				continue
			}

			// Fetch balance
			balance, err := p.fetchBalance(work)
			if err != nil {
				continue // Skip on error
			}

			batch = append(batch, balance)

			// Flush batch when full
			if len(batch) >= 100 {
				p.flushBatch(batch)
				batch = make([]*types.Balance, 0, 100)
			}
		}
	}
}

// fetchBalance queries the token balance for an address
func (p *BalanceWorkerPool) fetchBalance(work BalanceWork) (*types.Balance, error) {
	// balanceOf(address) = 0x70a08231
	addrPadded := common.LeftPadBytes(work.Address.Bytes(), 32)
	callData := append(common.FromHex("0x70a08231"), addrPadded...)

	balanceData, err := p.rpc.CallContract(p.ctx, work.TokenAddress, callData)
	if err != nil {
		return nil, err
	}

	if len(balanceData) < 32 {
		return nil, nil
	}

	balance := new(big.Int).SetBytes(balanceData)

	return &types.Balance{
		Address:      work.Address.Hex(),
		TokenAddress: work.TokenAddress.Hex(),
		BlockNumber:  work.BlockNumber,
		Balance:      types.JSONString(balance.String()),
	}, nil
}

// flushBatch inserts a batch of balances into the database with retry logic
func (p *BalanceWorkerPool) flushBatch(batch []*types.Balance) {
	if len(batch) == 0 {
		return
	}

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := p.db.InsertBalancesBatch(p.ctx, batch)
		if err == nil {
			return
		}

		// Check if it's a retryable error (deadlock or serialization failure)
		errStr := err.Error()
		isRetryable := strings.Contains(errStr, "40P01") || // deadlock
			strings.Contains(errStr, "40001") // serialization failure

		if !isRetryable || attempt == maxRetries-1 {
			log.Error("balance worker: failed to insert batch", "count", len(batch), "error", err)
			return
		}

		// Exponential backoff: 10ms, 20ms, 40ms
		time.Sleep(time.Duration(10<<attempt) * time.Millisecond)
	}
}
