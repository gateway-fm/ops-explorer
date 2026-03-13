package indexer

import (
	"context"
	"fmt"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"explorer/internal/db"
	"explorer/internal/types"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockSubscription implements ethereum.Subscription for testing SubscribeNewHead failure paths.
type mockSubscription struct {
	errCh chan error
}

func newMockSubscription() *mockSubscription {
	return &mockSubscription{errCh: make(chan error, 1)}
}

func (s *mockSubscription) Unsubscribe() {}
func (s *mockSubscription) Err() <-chan error { return s.errCh }

func newTestCatchupIndexer(mockDB *MockDatabase, mockRPC *MockRPCClient) *CatchupIndexer {
	cfg := &CatchupConfig{
		Workers:   1,
		BatchSize: 100,
		QueueSize: 100,
	}
	idxCfg := &Config{
		RPCWorkers:   1,
		RPCRateLimit: 100,
	}
	return NewCatchupIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)
}

func TestCatchupIndexer_WorkerProcessesBlock(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	catchup := newTestCatchupIndexer(mockDB, mockRPC)

	blockNum := uint64(5)

	// Block does not exist in DB
	mockDB.On("HasBlock", mock.Anything, blockNum).Return(false, nil).Once()

	// processBlock: RPC returns empty block
	header := &ethtypes.Header{
		Number: big.NewInt(int64(blockNum)),
		Time:   uint64(time.Now().Unix()),
	}
	ethBlock := ethtypes.NewBlock(header, &ethtypes.Body{}, nil, trie.NewStackTrie(nil))

	mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(int64(blockNum))).Return(ethBlock, nil).Once()
	mockRPC.On("GetTotalDifficulty", mock.Anything, blockNum).Return("100").Once()
	mockDB.On("InsertBlockDataBatch", mock.Anything, mock.MatchedBy(func(b *db.BlockData) bool {
		return b.Block.Number == blockNum
	})).Return(nil).Once()

	// After processing, DeleteMissingRangeByBlock is called (no collector set)
	mockDB.On("DeleteMissingRangeByBlock", mock.Anything, blockNum).Return(nil).Once()

	// Send block to work queue and run worker
	ctx, cancel := context.WithCancel(context.Background())
	catchup.ctx = ctx
	catchup.cancel = cancel

	go func() {
		catchup.workQueue <- blockNum
		// Close after sending to stop worker
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	catchup.wg.Add(1)
	catchup.worker(0)

	mockDB.AssertExpectations(t)
	mockRPC.AssertExpectations(t)
}

func TestCatchupIndexer_WorkerSkipsExistingBlock(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	catchup := newTestCatchupIndexer(mockDB, mockRPC)

	blockNum := uint64(5)

	// Block already exists
	mockDB.On("HasBlock", mock.Anything, blockNum).Return(true, nil).Once()

	// Since no collector is set, DeleteMissingRangeByBlock should NOT be called
	// (the worker only calls collector.MarkBlockProcessed or db.DeleteMissingRangeByBlock
	//  when processing succeeds OR when block exists with collector set)
	// With no collector, and block exists, it just increments processed and continues.

	ctx, cancel := context.WithCancel(context.Background())
	catchup.ctx = ctx
	catchup.cancel = cancel

	go func() {
		catchup.workQueue <- blockNum
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	catchup.wg.Add(1)
	catchup.worker(0)

	assert.Equal(t, int64(1), atomic.LoadInt64(&catchup.processedBlocks))

	// processBlock should NOT have been called (no BlockByNumber)
	mockRPC.AssertNotCalled(t, "BlockByNumber", mock.Anything, mock.Anything)
	mockDB.AssertExpectations(t)
}

func TestCatchupIndexer_WorkerSkipsExistingBlock_WithCollector(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	catchup := newTestCatchupIndexer(mockDB, mockRPC)

	// Set up a collector so MarkBlockProcessed is called
	collectorDB := new(MockDatabase)
	collectorRPC := new(MockRPCClient)
	collector := NewMissingRangeCollector(collectorDB, collectorRPC, nil)
	catchup.collector = collector

	blockNum := uint64(5)

	// Block already exists
	mockDB.On("HasBlock", mock.Anything, blockNum).Return(true, nil).Once()

	// collector.MarkBlockProcessed calls collector.db.DeleteMissingRangeByBlock
	collectorDB.On("DeleteMissingRangeByBlock", mock.Anything, blockNum).Return(nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	catchup.ctx = ctx
	catchup.cancel = cancel

	go func() {
		catchup.workQueue <- blockNum
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	catchup.wg.Add(1)
	catchup.worker(0)

	assert.Equal(t, int64(1), atomic.LoadInt64(&catchup.processedBlocks))
	mockRPC.AssertNotCalled(t, "BlockByNumber", mock.Anything, mock.Anything)
	collectorDB.AssertCalled(t, "DeleteMissingRangeByBlock", mock.Anything, blockNum)
}

func TestCatchupIndexer_WorkerRetriesOnError(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	catchup := newTestCatchupIndexer(mockDB, mockRPC)

	blockNum := uint64(5)

	// Block doesn't exist
	mockDB.On("HasBlock", mock.Anything, blockNum).Return(false, nil).Once()

	// processBlock fails (RPC error)
	mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(int64(blockNum))).Return(nil, fmt.Errorf("connection refused")).Once()

	ctx, cancel := context.WithCancel(context.Background())
	catchup.ctx = ctx
	catchup.cancel = cancel

	go func() {
		catchup.workQueue <- blockNum
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	catchup.wg.Add(1)
	catchup.worker(0)

	// processedBlocks should NOT have been incremented on error
	assert.Equal(t, int64(0), atomic.LoadInt64(&catchup.processedBlocks))

	// DeleteMissingRangeByBlock should NOT have been called
	mockDB.AssertNotCalled(t, "DeleteMissingRangeByBlock", mock.Anything, mock.Anything)
}

func TestCatchupIndexer_BlockProducer_QueuesAllBlocksInRange(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	cfg := &CatchupConfig{
		Workers:   2,
		BatchSize: 100,
		QueueSize: 100,
	}
	idxCfg := &Config{RPCWorkers: 1, RPCRateLimit: 100}
	catchup := NewCatchupIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)

	// Set up a collector
	collectorDB := new(MockDatabase)
	collectorRPC := new(MockRPCClient)
	collector := NewMissingRangeCollector(collectorDB, collectorRPC, nil)
	catchup.collector = collector

	// GetMissingRangesBatch: first call returns [1,9], subsequent calls return empty
	// First call returns [1,9], subsequent calls return empty
	collectorDB.On("GetMissingRangesBatch", mock.Anything, 100).
		Return([]db.BlockRange{{FromNumber: 1, ToNumber: 9}}, nil).Once()
	collectorDB.On("GetMissingRangesBatch", mock.Anything, 100).
		Return([]db.BlockRange{}, nil)

	// For each block 1-9: HasBlock returns false, processBlock works, MarkBlockProcessed called
	for blockNum := uint64(1); blockNum <= 9; blockNum++ {
		mockDB.On("HasBlock", mock.Anything, blockNum).Return(false, nil).Once()

		header := &ethtypes.Header{
			Number: big.NewInt(int64(blockNum)),
			Time:   uint64(time.Now().Unix()),
		}
		ethBlock := ethtypes.NewBlock(header, &ethtypes.Body{}, nil, trie.NewStackTrie(nil))

		mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(int64(blockNum))).Return(ethBlock, nil).Once()
		mockRPC.On("GetTotalDifficulty", mock.Anything, blockNum).Return("100").Once()
		mockDB.On("InsertBlockDataBatch", mock.Anything, mock.MatchedBy(func(b *db.BlockData) bool {
			return b.Block.Number >= 1 && b.Block.Number <= 9
		})).Return(nil).Maybe()

		// collector.MarkBlockProcessed -> collectorDB.DeleteMissingRangeByBlock
		collectorDB.On("DeleteMissingRangeByBlock", mock.Anything, blockNum).Return(nil).Once()
	}

	// GetTotalMissingBlocks for initial count
	collectorDB.On("GetTotalMissingBlocks", mock.Anything).Return(int64(9), nil).Maybe()

	// RebuildAddressStats called when idle for 3 polls
	mockDB.On("RebuildAddressStats", mock.Anything).Return(nil).Maybe()

	err := catchup.Start(context.Background(), 1, 9)
	assert.NoError(t, err)

	// Wait for all blocks to be processed (with timeout)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&catchup.processedBlocks) >= 9 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	processed := atomic.LoadInt64(&catchup.processedBlocks)
	assert.Equal(t, int64(9), processed, "all 9 blocks should be processed")

	catchup.Stop()

	// Verify all blocks 1-9 had MarkBlockProcessed called
	for blockNum := uint64(1); blockNum <= 9; blockNum++ {
		collectorDB.AssertCalled(t, "DeleteMissingRangeByBlock", mock.Anything, blockNum)
	}
}

func TestCatchupIndexer_BlockProducer_MultipleRanges(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	cfg := &CatchupConfig{
		Workers:   2,
		BatchSize: 100,
		QueueSize: 200,
	}
	idxCfg := &Config{RPCWorkers: 1, RPCRateLimit: 100}
	catchup := NewCatchupIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)

	collectorDB := new(MockDatabase)
	collectorRPC := new(MockRPCClient)
	collector := NewMissingRangeCollector(collectorDB, collectorRPC, nil)
	catchup.collector = collector

	// First call returns two ranges (newest first), subsequent calls return empty
	collectorDB.On("GetMissingRangesBatch", mock.Anything, 100).
		Return([]db.BlockRange{
			{FromNumber: 100, ToNumber: 110},
			{FromNumber: 1, ToNumber: 9},
		}, nil).Once()
	collectorDB.On("GetMissingRangesBatch", mock.Anything, 100).
		Return([]db.BlockRange{}, nil)

	// Set up expectations for ALL blocks: 100-110 and 1-9
	allBlocks := make([]uint64, 0)
	for b := uint64(100); b <= 110; b++ {
		allBlocks = append(allBlocks, b)
	}
	for b := uint64(1); b <= 9; b++ {
		allBlocks = append(allBlocks, b)
	}

	for _, blockNum := range allBlocks {
		mockDB.On("HasBlock", mock.Anything, blockNum).Return(false, nil).Once()

		header := &ethtypes.Header{
			Number: big.NewInt(int64(blockNum)),
			Time:   uint64(time.Now().Unix()),
		}
		ethBlock := ethtypes.NewBlock(header, &ethtypes.Body{}, nil, trie.NewStackTrie(nil))

		mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(int64(blockNum))).Return(ethBlock, nil).Once()
		mockRPC.On("GetTotalDifficulty", mock.Anything, blockNum).Return("100").Once()
		mockDB.On("InsertBlockDataBatch", mock.Anything, mock.MatchedBy(func(b *db.BlockData) bool {
			return b.Block.Number >= 1 && b.Block.Number <= 110
		})).Return(nil).Maybe()
		collectorDB.On("DeleteMissingRangeByBlock", mock.Anything, blockNum).Return(nil).Once()
	}

	collectorDB.On("GetTotalMissingBlocks", mock.Anything).Return(int64(20), nil).Maybe()
	mockDB.On("RebuildAddressStats", mock.Anything).Return(nil).Maybe()

	totalExpected := int64(11 + 9) // blocks 100-110 (11) + blocks 1-9 (9)

	err := catchup.Start(context.Background(), 1, 110)
	assert.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&catchup.processedBlocks) >= totalExpected {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	processed := atomic.LoadInt64(&catchup.processedBlocks)
	assert.Equal(t, totalExpected, processed, "all 20 blocks should be processed")

	catchup.Stop()
}

func TestCatchupIndexer_BlockProducer_GoesIdleWhenNoRanges(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	cfg := &CatchupConfig{
		Workers:   1,
		BatchSize: 100,
		QueueSize: 100,
	}
	idxCfg := &Config{RPCWorkers: 1, RPCRateLimit: 100}
	catchup := NewCatchupIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)

	// No missing ranges
	mockDB.On("GetMissingRangesBatch", mock.Anything, 100).Return([]db.BlockRange{}, nil)
	mockDB.On("GetTotalMissingBlocks", mock.Anything).Return(int64(0), nil).Maybe()

	completeCalled := make(chan struct{}, 1)
	catchup.SetOnComplete(func() {
		select {
		case completeCalled <- struct{}{}:
		default:
		}
	})

	err := catchup.Start(context.Background(), 0, 0)
	assert.NoError(t, err)

	// onComplete should NOT be called when processedBlocks == 0 (no work was done)
	// The code checks: if !completionCalled && idleCount >= 3 && processed > 0
	select {
	case <-completeCalled:
		t.Fatal("onComplete should not be called when no blocks were processed")
	case <-time.After(200 * time.Millisecond):
		// Expected: no completion callback since processedBlocks == 0
	}

	catchup.Stop()
}

// --- Realtime Indexer Tests ---

func TestRealtimeIndexer_DetectReorg_BlockNotInDB_Continues(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	cfg := &RealtimeConfig{
		ConfirmationBlocks: 1,
		PollInterval:       100 * time.Millisecond,
	}
	idxCfg := &Config{}

	rt := NewRealtimeIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)

	blockNum := uint64(50)

	// GetBlock returns nil for all blocks checked — blocks not yet in DB
	mockDB.On("GetBlock", mock.Anything, mock.Anything).Return(nil, nil)

	reorgDepth, err := rt.detectReorg(context.Background(), blockNum)
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), reorgDepth, "no reorg should be detected when blocks are not in DB")
}

func TestRealtimeIndexer_DetectReorg_RPCError_DoesNotTriggerDeletion(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	cfg := &RealtimeConfig{
		ConfirmationBlocks: 1,
		PollInterval:       100 * time.Millisecond,
	}
	idxCfg := &Config{}

	rt := NewRealtimeIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)

	blockNum := uint64(50)

	// DB has the block
	mockDB.On("GetBlock", mock.Anything, blockNum).Return(&types.Block{
		Number: blockNum,
		Hash:   "0xabc123",
	}, nil).Once()

	// RPC fails
	mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(int64(blockNum))).Return(nil, fmt.Errorf("connection refused")).Once()

	reorgDepth, err := rt.detectReorg(context.Background(), blockNum)

	// detectReorg returns error — caller should handle gracefully
	assert.Error(t, err)
	assert.Equal(t, uint64(0), reorgDepth)

	// DeleteBlock should NEVER be called
	mockDB.AssertNotCalled(t, "DeleteBlock", mock.Anything, mock.Anything)
}

func TestRealtimeIndexer_ProcessingLoop_SkipsBlock0(t *testing.T) {
	// The realtime indexer loop is:
	//   for blockNum := lastProcessed + 1; blockNum <= safeBlock
	//
	// When lastProcessed=0, the loop starts at block 1.
	// Block 0 (genesis) is NEVER processed by the realtime indexer.
	//
	// This is by design: the catchup/missing-range system should handle block 0.
	// But if the missing range collector ALSO misses block 0, it won't be indexed.

	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	cfg := &RealtimeConfig{
		ConfirmationBlocks: 1,
		PollInterval:       50 * time.Millisecond,
	}
	idxCfg := &Config{}

	rt := NewRealtimeIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)
	rt.SetLastProcessedBlock(0)

	// Simulate polling mode: chain is at block 10, safe = 9
	// First poll returns block 10
	mockRPC.On("BlockNumber", mock.Anything).Return(uint64(10), nil)

	// For blocks 1 through 9:
	for blockNum := uint64(1); blockNum <= 9; blockNum++ {
		// detectReorg: GetBlock for blockNum-1 returns nil (not in DB yet) — continue
		mockDB.On("GetBlock", mock.Anything, blockNum-1).Return(nil, nil).Maybe()

		header := &ethtypes.Header{
			Number:     big.NewInt(int64(blockNum)),
			Time:       uint64(time.Now().Unix()),
			ParentHash: common.HexToHash(fmt.Sprintf("0x%064x", blockNum-1)),
		}
		ethBlock := ethtypes.NewBlock(header, &ethtypes.Body{}, nil, trie.NewStackTrie(nil))

		mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(int64(blockNum))).Return(ethBlock, nil).Once()
		mockRPC.On("GetTotalDifficulty", mock.Anything, blockNum).Return("100").Once()
		mockDB.On("InsertBlock", mock.Anything, mock.MatchedBy(func(b *types.Block) bool {
			return b.Number == blockNum
		})).Return(nil).Maybe()
		mockDB.On("InsertBlockDataBatch", mock.Anything, mock.MatchedBy(func(b *db.BlockData) bool {
			return b.Block.Number == blockNum
		})).Return(nil).Maybe()
		mockDB.On("UpdateSyncStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	}

	// Subscribe fails, falls back to polling
	mockRPC.On("SubscribeNewHead", mock.Anything, mock.Anything).Return(newMockSubscription(), fmt.Errorf("not supported"))

	// Run in a goroutine with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go rt.Start(ctx, 0)

	// Wait for processing
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if rt.GetLastProcessedBlock() >= 9 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Block 0 should NEVER have been requested from RPC
	mockRPC.AssertNotCalled(t, "BlockByNumber", mock.Anything, big.NewInt(0))

	// Blocks 1-9 SHOULD have been processed
	lastProcessed := rt.GetLastProcessedBlock()
	assert.GreaterOrEqual(t, lastProcessed, uint64(9),
		"blocks 1-9 should have been processed, but lastProcessed=%d", lastProcessed)

	rt.Stop()
}

func TestRealtimeIndexer_ReorgError_DoesNotDeleteBlocks(t *testing.T) {
	// When detectReorg returns an error, the polling loop should:
	// 1. Log the error
	// 2. Continue processing the block (NOT delete anything)
	//
	// This tests the behavior in runPollingMode lines 235-238:
	//   if err != nil { log.Error(...) }
	//   else if reorgDepth > 0 { ... handleReorg ... }
	//
	// On error: no reorg handling, falls through to processBlock

	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)

	cfg := &RealtimeConfig{
		ConfirmationBlocks: 1,
		PollInterval:       50 * time.Millisecond,
	}
	idxCfg := &Config{}

	rt := NewRealtimeIndexer(mockDB, mockRPC, cfg, idxCfg, NewTokenCache(), NewContractCache(), nil, false)
	rt.SetLastProcessedBlock(4)

	// Chain at block 6, safe = 5
	mockRPC.On("BlockNumber", mock.Anything).Return(uint64(6), nil)

	// detectReorg for block 4 (blockNum=5, check parent=4):
	// GetBlock returns a stored block, but RPC fails
	mockDB.On("GetBlock", mock.Anything, uint64(4)).Return(&types.Block{
		Number: 4,
		Hash:   "0xstoredblock4hash",
	}, nil)
	mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(4)).Return(nil, fmt.Errorf("RPC unavailable")).Once()

	// Despite reorg check error, block 5 should still be processed
	header5 := &ethtypes.Header{
		Number: big.NewInt(5),
		Time:   uint64(time.Now().Unix()),
	}
	ethBlock5 := ethtypes.NewBlock(header5, &ethtypes.Body{}, nil, trie.NewStackTrie(nil))

	mockRPC.On("BlockByNumber", mock.Anything, big.NewInt(5)).Return(ethBlock5, nil).Once()
	mockRPC.On("GetTotalDifficulty", mock.Anything, uint64(5)).Return("100").Once()
	mockDB.On("InsertBlock", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockDB.On("InsertBlockDataBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockDB.On("UpdateSyncStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// SubscribeNewHead fails -> polling mode
	mockRPC.On("SubscribeNewHead", mock.Anything, mock.Anything).Return(newMockSubscription(), fmt.Errorf("not supported"))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go rt.Start(ctx, 4)

	// Wait for block 5 to be processed
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if rt.GetLastProcessedBlock() >= 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// CRITICAL: DeleteBlock should NEVER be called when detectReorg errors
	mockDB.AssertNotCalled(t, "DeleteBlock", mock.Anything, mock.Anything)

	// Block 5 should have been processed despite the reorg check error
	assert.GreaterOrEqual(t, rt.GetLastProcessedBlock(), uint64(5))

	rt.Stop()
}
