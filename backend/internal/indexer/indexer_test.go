package indexer

import (
	"context"
	"math/big"
	"testing"
	"time"

	"explorer/internal/db"
	"explorer/internal/rpc"
	"explorer/internal/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock of the Database interface
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockDatabase) GetBlockCount(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDatabase) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Block), args.Error(1)
}

func (m *MockDatabase) DeleteBlock(ctx context.Context, number uint64) error {
	args := m.Called(ctx, number)
	return args.Error(0)
}

func (m *MockDatabase) InsertBlock(ctx context.Context, b *types.Block) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *MockDatabase) InsertBlockDataBatch(ctx context.Context, b *db.BlockData) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *MockDatabase) UpdateSyncStatus(ctx context.Context, lastIndexed uint64, isSyncing bool) error {
	args := m.Called(ctx, lastIndexed, isSyncing)
	return args.Error(0)
}

func (m *MockDatabase) HasBlock(ctx context.Context, number uint64) (bool, error) {
	args := m.Called(ctx, number)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) DeleteMissingRangeByBlock(ctx context.Context, blockNum uint64) error {
	args := m.Called(ctx, blockNum)
	return args.Error(0)
}

func (m *MockDatabase) GetMissingRangesBatch(ctx context.Context, batchSize int) ([]db.BlockRange, error) {
	args := m.Called(ctx, batchSize)
	return args.Get(0).([]db.BlockRange), args.Error(1)
}

func (m *MockDatabase) RebuildAddressStats(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDatabase) InsertTransaction(ctx context.Context, tx *types.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *MockDatabase) InsertContract(ctx context.Context, l *types.Contract) error {
	args := m.Called(ctx, l)
	return args.Error(0)
}

func (m *MockDatabase) UpsertAddressStats(ctx context.Context, address string, blockNumber uint64, isContract bool) error {
	args := m.Called(ctx, address, blockNumber, isContract)
	return args.Error(0)
}

func (m *MockDatabase) IsContract(ctx context.Context, address string) (bool, error) {
	args := m.Called(ctx, address)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) InsertLog(ctx context.Context, l *types.Log) error {
	args := m.Called(ctx, l)
	return args.Error(0)
}

func (m *MockDatabase) InsertTokenTransfer(ctx context.Context, t *types.TokenTransfer) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockDatabase) UpdateAddressStatsCounters(ctx context.Context, address string, internalTxCount int, tokenTransferCount int) error {
	args := m.Called(ctx, address, internalTxCount, tokenTransferCount)
	return args.Error(0)
}

func (m *MockDatabase) InsertBalance(ctx context.Context, b *types.Balance) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *MockDatabase) GetToken(ctx context.Context, address string) (*types.Token, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Token), args.Error(1)
}

func (m *MockDatabase) InsertToken(ctx context.Context, t *types.Token) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockDatabase) GetMinMaxIndexedBlocks(ctx context.Context) (uint64, uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Get(1).(uint64), args.Error(2)
}

func (m *MockDatabase) GetTotalMissingBlocks(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDatabase) GetIndexerProgress(ctx context.Context) (*db.IndexerProgress, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.IndexerProgress), args.Error(1)
}

func (m *MockDatabase) UpdateIndexerProgress(ctx context.Context, minBlock, maxBlock uint64, backfillComplete bool) error {
	args := m.Called(ctx, minBlock, maxBlock, backfillComplete)
	return args.Error(0)
}

func (m *MockDatabase) FindMissingBlocksInRange(ctx context.Context, fromBlock, toBlock uint64) ([]db.BlockRange, error) {
	args := m.Called(ctx, fromBlock, toBlock)
	return args.Get(0).([]db.BlockRange), args.Error(1)
}

func (m *MockDatabase) SaveMissingRanges(ctx context.Context, ranges []db.BlockRange) error {
	args := m.Called(ctx, ranges)
	return args.Error(0)
}

func (m *MockDatabase) GetAllTokenAddresses(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockDatabase) InsertBalancesBatch(ctx context.Context, balances []*types.Balance) error {
	args := m.Called(ctx, balances)
	return args.Error(0)
}

// MockRPCClient is a mock of the RPCClient interface
type MockRPCClient struct {
	mock.Mock
}

func (m *MockRPCClient) BlockNumber(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockRPCClient) BlockByNumber(ctx context.Context, number *big.Int) (*ethtypes.Block, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ethtypes.Block), args.Error(1)
}

func (m *MockRPCClient) RawBlockByNumber(ctx context.Context, number uint64) (*rpc.RawBlock, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rpc.RawBlock), args.Error(1)
}

func (m *MockRPCClient) RawBlockHash(ctx context.Context, number uint64) (string, error) {
	args := m.Called(ctx, number)
	return args.String(0), args.Error(1)
}

func (m *MockRPCClient) FetchReceiptsBatch(ctx context.Context, txHashes []common.Hash, workers int, rateLimit int) (map[common.Hash]*ethtypes.Receipt, error) {
	args := m.Called(ctx, txHashes, workers, rateLimit)
	return args.Get(0).(map[common.Hash]*ethtypes.Receipt), args.Error(1)
}

func (m *MockRPCClient) GetTotalDifficulty(ctx context.Context, blockNumber uint64) string {
	args := m.Called(ctx, blockNumber)
	return args.String(0)
}

func (m *MockRPCClient) CheckTracingSupport(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockRPCClient) SubscribeNewHead(ctx context.Context, ch chan<- *ethtypes.Header) (ethereum.Subscription, error) {
	args := m.Called(ctx, ch)
	return args.Get(0).(ethereum.Subscription), args.Error(1)
}

func (m *MockRPCClient) CallContract(ctx context.Context, to common.Address, data []byte) ([]byte, error) {
	args := m.Called(ctx, to, data)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockRPCClient) GetCode(ctx context.Context, address common.Address) ([]byte, error) {
	args := m.Called(ctx, address)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockRPCClient) FetchTracesBatch(ctx context.Context, txHashes []common.Hash, startBlock, endBlock uint64, workers, rateLimit int) ([]*types.InternalTransaction, error) {
	args := m.Called(ctx, txHashes, startBlock, endBlock, workers, rateLimit)
	return args.Get(0).([]*types.InternalTransaction), args.Error(1)
}

func (m *MockRPCClient) FetchTokenMetadataBatch(ctx context.Context, addresses []common.Address, workers int, rateLimit int) (map[common.Address]*rpc.TokenMetadataResult, error) {
	args := m.Called(ctx, addresses, workers, rateLimit)
	return args.Get(0).(map[common.Address]*rpc.TokenMetadataResult), args.Error(1)
}

func (m *MockRPCClient) ChainID(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockRPCClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*ethtypes.Receipt, error) {
	args := m.Called(ctx, txHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ethtypes.Receipt), args.Error(1)
}

func TestIndexer_GetCatchupProgress(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)
	idx := New(mockDB, mockRPC, time.Second, 0)

	processed, total, percent, isRunning := idx.GetCatchupProgress()
	assert.Equal(t, int64(0), processed)
	assert.Equal(t, uint64(0), total)
	assert.Equal(t, 100.0, percent)
	assert.False(t, isRunning)
}

func TestIndexer_DetectReorg(t *testing.T) {
	ctx := context.Background()

	t.Run("no reorg", func(t *testing.T) {
		mockDB := new(MockDatabase)
		mockRPC := new(MockRPCClient)
		idx := New(mockDB, mockRPC, time.Second, 0)

		blockNum := uint64(100)
		header := &ethtypes.Header{
			Number: big.NewInt(int64(blockNum)),
		}
		chainBlock := ethtypes.NewBlockWithHeader(header)
		expectedHash := chainBlock.Hash().Hex()

		mockDB.On("GetBlock", ctx, mock.Anything).Return(&types.Block{Hash: expectedHash}, nil)
		mockRPC.On("BlockByNumber", ctx, mock.Anything).Return(chainBlock, nil)

		reorgAt, err := idx.detectReorg(ctx, blockNum)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), reorgAt)
		mockDB.AssertExpectations(t)
		mockRPC.AssertExpectations(t)
	})

	t.Run("reorg detected at parent", func(t *testing.T) {
		mockDB := new(MockDatabase)
		mockRPC := new(MockRPCClient)
		idx := New(mockDB, mockRPC, time.Second, 0)

		blockNum := uint64(100)

		// Current block matches
		header := &ethtypes.Header{Number: big.NewInt(100)}
		block := ethtypes.NewBlockWithHeader(header)
		mockDB.On("GetBlock", ctx, uint64(100)).Return(&types.Block{Hash: block.Hash().Hex()}, nil).Once()
		mockRPC.On("BlockByNumber", ctx, big.NewInt(100)).Return(block, nil).Once()

		// Parent doesn't match
		mockDB.On("GetBlock", ctx, uint64(99)).Return(&types.Block{Hash: "0xdb_parent"}, nil).Once()

		parentHeader := &ethtypes.Header{Number: big.NewInt(99)}
		parentBlock := ethtypes.NewBlockWithHeader(parentHeader)
		mockRPC.On("BlockByNumber", ctx, big.NewInt(99)).Return(parentBlock, nil).Once()

		reorgAt, err := idx.detectReorg(ctx, blockNum)
		assert.NoError(t, err)
		assert.Equal(t, uint64(2), reorgAt) // depth 1 (parent) + 1
		mockDB.AssertExpectations(t)
		mockRPC.AssertExpectations(t)
	})
}

func TestIndexer_ProcessBlock(t *testing.T) {
	ctx := context.Background()

	t.Run("standard block (empty)", func(t *testing.T) {
		mockDB := new(MockDatabase)
		mockRPC := new(MockRPCClient)
		idx := New(mockDB, mockRPC, time.Second, 0)

		blockNum := uint64(200)

		header := &ethtypes.Header{
			Number:     big.NewInt(int64(blockNum)),
			Time:       uint64(time.Now().Unix()),
			ParentHash: common.HexToHash("0x123"),
		}
		ethBlock := ethtypes.NewBlock(header, &ethtypes.Body{}, nil, trie.NewStackTrie(nil))

		mockRPC.On("BlockByNumber", ctx, big.NewInt(int64(blockNum))).Return(ethBlock, nil).Once()
		mockRPC.On("GetTotalDifficulty", ctx, blockNum).Return("1000").Once()

		mockDB.On("InsertBlock", ctx, mock.MatchedBy(func(b *types.Block) bool {
			return b.Number == blockNum
		})).Return(nil).Once()

		err := idx.processBlock(ctx, blockNum)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
		mockRPC.AssertExpectations(t)
	})

	t.Run("block with transactions", func(t *testing.T) {
		mockDB := new(MockDatabase)
		mockRPC := new(MockRPCClient)
		idx := New(mockDB, mockRPC, time.Second, 0)

		blockNum := uint64(201)
		toAddr := common.HexToAddress("0x222")
		privKey, _ := crypto.GenerateKey()
		tx := ethtypes.NewTransaction(0, toAddr, big.NewInt(0), 21000, big.NewInt(1), nil)
		signedTx, _ := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(big.NewInt(1)), privKey)

		header := &ethtypes.Header{
			Number: big.NewInt(int64(blockNum)),
			Time:   uint64(time.Now().Unix()),
		}
		ethBlock := ethtypes.NewBlock(header, &ethtypes.Body{Transactions: []*ethtypes.Transaction{signedTx}}, nil, trie.NewStackTrie(nil))

		mockRPC.On("BlockByNumber", ctx, big.NewInt(int64(blockNum))).Return(ethBlock, nil).Once()
		mockRPC.On("GetTotalDifficulty", ctx, blockNum).Return("1001").Once()
		mockRPC.On("ChainID", ctx).Return(big.NewInt(1), nil).Once()
		mockRPC.On("FetchReceiptsBatch", ctx, []common.Hash{signedTx.Hash()}, mock.Anything, mock.Anything).Return(map[common.Hash]*ethtypes.Receipt{
			signedTx.Hash(): {Status: 1, TxHash: signedTx.Hash()},
		}, nil).Once()

		mockDB.On("InsertBlockDataBatch", ctx, mock.MatchedBy(func(b *db.BlockData) bool {
			return b.Block.Number == blockNum && len(b.Transactions) == 1
		})).Return(nil).Once()

		err := idx.processBlock(ctx, blockNum)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
		mockRPC.AssertExpectations(t)
	})
}
