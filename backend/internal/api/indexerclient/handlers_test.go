//go:build !privacy

package indexerclient

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
	"explorer/internal/api"
)

// Contract tests for Provider handlers. These spin up an in-process gRPC
// indexer server via bufconn, wire a Provider to it, and verify each
// handler round-trips a realistic proto response through the mapper layer
// into the explorer types the api handlers expect.
//
// Scope: the handlers that issue gRPC calls and map responses. SQL-backed
// fall-through methods (verification writes, chain-data reads that return
// ErrChainDataNotAvailable) are covered separately via the provider
// integration tests at the db tier.

// ----- Fake indexer server — satisfies indexerv1.IndexerServiceServer
// and returns canned responses controlled by per-test handler fields. -----

type fakeIndexer struct {
	indexerv1.UnimplementedIndexerServiceServer

	getBlock         func(*indexerv1.GetBlockRequest) (*indexerv1.Block, error)
	getLatest        func() (*indexerv1.LatestBlockNumber, error)
	getTransaction   func(*indexerv1.GetTransactionRequest) (*indexerv1.Transaction, error)
	getAddress       func(*indexerv1.GetAddressRequest) (*indexerv1.Address, error)
	getChainStats    func() (*indexerv1.ChainStats, error)
	getSyncStatus    func() (*indexerv1.SyncStatus, error)
	listBlocks       func(*indexerv1.ListBlocksRequest) (*indexerv1.ListBlocksResponse, error)
	listTxs          func(*indexerv1.ListTransactionsRequest) (*indexerv1.ListTransactionsResponse, error)
	listTxsPaginated func(*indexerv1.ListTransactionsPaginatedRequest) (*indexerv1.ListTransactionsPaginatedResponse, error)
	listLogs         func(*indexerv1.ListLogsRequest) (*indexerv1.ListLogsResponse, error)
	listAddresses    func(*indexerv1.ListAddressesRequest) (*indexerv1.ListAddressesResponse, error)
	listTokens       func(*indexerv1.ListTokensRequest) (*indexerv1.ListTokensResponse, error)
	listTransfers    func(*indexerv1.ListTokenTransfersRequest) (*indexerv1.ListTokenTransfersResponse, error)
	listAllTransfers func(*indexerv1.ListAllTokenTransfersRequest) (*indexerv1.ListAllTokenTransfersResponse, error)
	listInternal     func(*indexerv1.ListInternalTransactionsRequest) (*indexerv1.ListInternalTransactionsResponse, error)
	getToken         func(*indexerv1.GetTokenRequest) (*indexerv1.Token, error)
	getContract      func(*indexerv1.GetContractRequest) (*indexerv1.Contract, error)
	search           func(*indexerv1.SearchRequest) (*indexerv1.SearchResponse, error)
	getOPDeposit     func(*indexerv1.GetOPDepositRequest) (*indexerv1.OPDeposit, error)
}

func (f *fakeIndexer) GetBlock(ctx context.Context, req *indexerv1.GetBlockRequest) (*indexerv1.Block, error) {
	if f.getBlock == nil {
		return nil, status.Error(codes.Unimplemented, "GetBlock not set")
	}
	return f.getBlock(req)
}
func (f *fakeIndexer) GetLatestBlockNumber(ctx context.Context, _ *indexerv1.Empty) (*indexerv1.LatestBlockNumber, error) {
	return f.getLatest()
}
func (f *fakeIndexer) GetTransaction(ctx context.Context, req *indexerv1.GetTransactionRequest) (*indexerv1.Transaction, error) {
	return f.getTransaction(req)
}
func (f *fakeIndexer) GetAddress(ctx context.Context, req *indexerv1.GetAddressRequest) (*indexerv1.Address, error) {
	return f.getAddress(req)
}
func (f *fakeIndexer) GetChainStats(ctx context.Context, _ *indexerv1.Empty) (*indexerv1.ChainStats, error) {
	return f.getChainStats()
}
func (f *fakeIndexer) GetSyncStatus(ctx context.Context, _ *indexerv1.Empty) (*indexerv1.SyncStatus, error) {
	return f.getSyncStatus()
}
func (f *fakeIndexer) ListBlocks(ctx context.Context, req *indexerv1.ListBlocksRequest) (*indexerv1.ListBlocksResponse, error) {
	return f.listBlocks(req)
}
func (f *fakeIndexer) ListTransactions(ctx context.Context, req *indexerv1.ListTransactionsRequest) (*indexerv1.ListTransactionsResponse, error) {
	return f.listTxs(req)
}
func (f *fakeIndexer) ListTransactionsPaginated(ctx context.Context, req *indexerv1.ListTransactionsPaginatedRequest) (*indexerv1.ListTransactionsPaginatedResponse, error) {
	return f.listTxsPaginated(req)
}
func (f *fakeIndexer) ListLogs(ctx context.Context, req *indexerv1.ListLogsRequest) (*indexerv1.ListLogsResponse, error) {
	return f.listLogs(req)
}
func (f *fakeIndexer) ListAddresses(ctx context.Context, req *indexerv1.ListAddressesRequest) (*indexerv1.ListAddressesResponse, error) {
	return f.listAddresses(req)
}
func (f *fakeIndexer) ListTokens(ctx context.Context, req *indexerv1.ListTokensRequest) (*indexerv1.ListTokensResponse, error) {
	return f.listTokens(req)
}
func (f *fakeIndexer) ListTokenTransfers(ctx context.Context, req *indexerv1.ListTokenTransfersRequest) (*indexerv1.ListTokenTransfersResponse, error) {
	return f.listTransfers(req)
}
func (f *fakeIndexer) ListAllTokenTransfers(ctx context.Context, req *indexerv1.ListAllTokenTransfersRequest) (*indexerv1.ListAllTokenTransfersResponse, error) {
	return f.listAllTransfers(req)
}
func (f *fakeIndexer) ListInternalTransactions(ctx context.Context, req *indexerv1.ListInternalTransactionsRequest) (*indexerv1.ListInternalTransactionsResponse, error) {
	return f.listInternal(req)
}
func (f *fakeIndexer) GetToken(ctx context.Context, req *indexerv1.GetTokenRequest) (*indexerv1.Token, error) {
	return f.getToken(req)
}
func (f *fakeIndexer) GetContract(ctx context.Context, req *indexerv1.GetContractRequest) (*indexerv1.Contract, error) {
	return f.getContract(req)
}
func (f *fakeIndexer) Search(ctx context.Context, req *indexerv1.SearchRequest) (*indexerv1.SearchResponse, error) {
	return f.search(req)
}
func (f *fakeIndexer) GetOPDeposit(ctx context.Context, req *indexerv1.GetOPDepositRequest) (*indexerv1.OPDeposit, error) {
	return f.getOPDeposit(req)
}

// setupProvider stands up the fake server on a bufconn listener and
// returns a Provider wired to it. The fake's handler fields are set by
// the caller before invoking the Provider method under test.
func setupProvider(t *testing.T, fake *fakeIndexer) *Provider {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	indexerv1.RegisterIndexerServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() {
		srv.Stop()
		_ = conn.Close()
		_ = lis.Close()
	})
	return &Provider{
		DirectDBProvider: api.NewDirectDBProvider(nil, nil, nil), // fallback unused for these tests
		client:           indexerv1.NewIndexerServiceClient(conn),
		conn:             conn,
	}
}

// ----- Tests -----

func TestProvider_GetBlock(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getBlock: func(req *indexerv1.GetBlockRequest) (*indexerv1.Block, error) {
			if req.GetNumber() != 42 {
				t.Errorf("unexpected number: %d", req.GetNumber())
			}
			return &indexerv1.Block{
				Number:           42,
				Hash:             "0xblock",
				Timestamp:        timestamppb.New(timeOf(1_700_000_000)),
				GasUsed:          21000,
				TransactionCount: 3,
			}, nil
		},
	})
	b, err := p.GetBlock(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
	if b == nil || b.Number != 42 || b.TransactionCount != 3 {
		t.Fatalf("got %+v", b)
	}
}

func TestProvider_GetBlock_NotFound(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getBlock: func(req *indexerv1.GetBlockRequest) (*indexerv1.Block, error) {
			return nil, status.Error(codes.NotFound, "nope")
		},
	})
	b, err := p.GetBlock(context.Background(), 999)
	if err != nil {
		t.Fatalf("NotFound should be swallowed, got err: %v", err)
	}
	if b != nil {
		t.Errorf("expected nil block, got %+v", b)
	}
}

func TestProvider_GetLatestBlockNumber(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getLatest: func() (*indexerv1.LatestBlockNumber, error) {
			return &indexerv1.LatestBlockNumber{Number: 1234}, nil
		},
	})
	n, err := p.GetLatestBlockNumber(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 1234 {
		t.Errorf("got %d", n)
	}
}

func TestProvider_GetTransaction(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getTransaction: func(req *indexerv1.GetTransactionRequest) (*indexerv1.Transaction, error) {
			return &indexerv1.Transaction{
				Hash:             "0xtx",
				BlockNumber:      5,
				TransactionIndex: 0,
				From:             "0xfrom",
				To:               "0xto",
				Value:            &indexerv1.BigInt{Value: "1000"},
				Status:           indexerv1.TransactionStatus_TRANSACTION_STATUS_SUCCESS,
			}, nil
		},
	})
	tx, err := p.GetTransaction(context.Background(), "0xtx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tx == nil || tx.Hash != "0xtx" || tx.To == nil || *tx.To != "0xto" {
		t.Errorf("got %+v", tx)
	}
	if tx.Status != 1 {
		t.Errorf("status: %d", tx.Status)
	}
}

func TestProvider_GetAddressStats(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getAddress: func(req *indexerv1.GetAddressRequest) (*indexerv1.Address, error) {
			return &indexerv1.Address{
				Address:    "0xa",
				IsContract: true,
				TxCountIn:  2,
				TxCountOut: 8,
			}, nil
		},
	})
	a, err := p.GetAddressStats(context.Background(), "0xa")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if a == nil || !a.IsContract || a.TxCount != 10 {
		t.Errorf("got %+v", a)
	}
}

func TestProvider_GetChainStats(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getChainStats: func() (*indexerv1.ChainStats, error) {
			return &indexerv1.ChainStats{
				TotalBlocks:         10,
				TotalTransactions:   100,
				TotalAddresses:      20,
				AvgBlockTimeSeconds: 12.0,
			}, nil
		},
	})
	s, err := p.GetChainStats(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.TotalBlocks != 10 || s.TotalTransactions != 100 {
		t.Errorf("got %+v", s)
	}
}

func TestProvider_GetSyncStatus(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getSyncStatus: func() (*indexerv1.SyncStatus, error) {
			return &indexerv1.SyncStatus{LatestIndexedBlock: 500, IsSyncing: false}, nil
		},
	})
	s, err := p.GetSyncStatus(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.LastIndexedBlock != 500 || s.IsSyncing {
		t.Errorf("got %+v", s)
	}
}

func TestProvider_GetBlocks(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		listBlocks: func(req *indexerv1.ListBlocksRequest) (*indexerv1.ListBlocksResponse, error) {
			if req.GetPage().GetPageSize() != 10 {
				t.Errorf("page_size: %d", req.GetPage().GetPageSize())
			}
			return &indexerv1.ListBlocksResponse{
				Blocks: []*indexerv1.Block{
					{Number: 5}, {Number: 4}, {Number: 3},
				},
			}, nil
		},
	})
	blocks, err := p.GetBlocks(context.Background(), 10, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(blocks) != 3 || blocks[0].Number != 5 {
		t.Errorf("got %+v", blocks)
	}
}

func TestProvider_GetTransactionsByAddress(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		listTxs: func(req *indexerv1.ListTransactionsRequest) (*indexerv1.ListTransactionsResponse, error) {
			f := req.GetByAddress()
			if f == nil || f.GetAddress() != "0xa" {
				t.Errorf("address filter: %+v", f)
			}
			return &indexerv1.ListTransactionsResponse{
				Transactions: []*indexerv1.Transaction{{Hash: "0xtx1"}, {Hash: "0xtx2"}},
			}, nil
		},
	})
	txs, err := p.GetTransactionsByAddress(context.Background(), "0xa", 25, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("got %d, want 2", len(txs))
	}
}

func TestProvider_GetLogsByTransaction(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		listLogs: func(req *indexerv1.ListLogsRequest) (*indexerv1.ListLogsResponse, error) {
			if req.GetByTxHash() != "0xtx" {
				t.Errorf("by_tx_hash: %q", req.GetByTxHash())
			}
			return &indexerv1.ListLogsResponse{
				Logs: []*indexerv1.Log{{LogIndex: 0, Address: "0xc"}},
			}, nil
		},
	})
	logs, err := p.GetLogsByTransaction(context.Background(), "0xtx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(logs) != 1 || logs[0].Address != "0xc" {
		t.Errorf("got %+v", logs)
	}
}

func TestProvider_GetToken(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getToken: func(req *indexerv1.GetTokenRequest) (*indexerv1.Token, error) {
			return &indexerv1.Token{
				Address:   "0xt",
				Symbol:    "TOK",
				Decimals:  18,
				TokenType: indexerv1.TokenType_TOKEN_TYPE_ERC20,
			}, nil
		},
	})
	tok, err := p.GetToken(context.Background(), "0xt")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tok == nil || tok.Symbol != "TOK" || tok.TokenType != "ERC20" {
		t.Errorf("got %+v", tok)
	}
}

func TestProvider_GetContract(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getContract: func(req *indexerv1.GetContractRequest) (*indexerv1.Contract, error) {
			return &indexerv1.Contract{
				Address:          "0xc",
				Bytecode:         "0x60806040",
				Deployer:         "0xdeployer",
				DeploymentTxHash: "0xdeploytx",
				DeploymentBlock:  10,
			}, nil
		},
	})
	c, err := p.GetContract(context.Background(), "0xc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c == nil || c.Creator != "0xdeployer" || c.CreationTx != "0xdeploytx" {
		t.Errorf("got %+v", c)
	}
	// Chain-facts only — verification fields intentionally absent.
	if c.IsVerified {
		t.Error("IsVerified should default to false on chain-indexer response")
	}
}

func TestProvider_Search(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		search: func(req *indexerv1.SearchRequest) (*indexerv1.SearchResponse, error) {
			if req.GetQuery() != "pre" {
				t.Errorf("query: %q", req.GetQuery())
			}
			return &indexerv1.SearchResponse{
				Results: []*indexerv1.SearchResponse_SearchResult{
					{Kind: indexerv1.SearchResponse_SEARCH_RESULT_KIND_BLOCK, Item: &indexerv1.SearchResponse_SearchResult_Block{Block: &indexerv1.Block{Number: 42}}},
					{Kind: indexerv1.SearchResponse_SEARCH_RESULT_KIND_TOKEN, Item: &indexerv1.SearchResponse_SearchResult_Token{Token: &indexerv1.Token{Address: "0xtok", Symbol: "PRE"}}},
				},
			}, nil
		},
	})
	results, err := p.SearchSuggestions(context.Background(), "pre", 5)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2, got %d", len(results))
	}
	if results[0].Type != "block" || results[0].Value != "42" {
		t.Errorf("block result: %+v", results[0])
	}
	if results[1].Type != "token" || results[1].Label != "PRE" {
		t.Errorf("token result: %+v", results[1])
	}
}
