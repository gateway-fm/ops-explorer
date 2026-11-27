//go:build !privacy

package indexerclient

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
)

// pagedLogs serves total logs for one transaction in pages of the requested size,
// with a numeric cursor, the way the indexer pages a by_tx_hash read.
func pagedLogs(total int, calls *int) func(*indexerv1.ListLogsRequest) (*indexerv1.ListLogsResponse, error) {
	return func(req *indexerv1.ListLogsRequest) (*indexerv1.ListLogsResponse, error) {
		*calls++
		start, _ := strconv.Atoi(req.GetPage().GetCursor())
		size := int(req.GetPage().GetPageSize())
		if size <= 0 || size > 100 {
			size = 25
		}
		end := min(start+size, total)
		logs := make([]*indexerv1.Log, 0, end-start)
		for i := start; i < end; i++ {
			logs = append(logs, &indexerv1.Log{LogIndex: uint32(i), Address: "0xc"})
		}
		next := ""
		if end < total {
			next = strconv.Itoa(end)
		}
		return &indexerv1.ListLogsResponse{Logs: logs, Page: &indexerv1.PageResponse{NextCursor: next}}, nil
	}
}

func TestTxScopedReadsFollowTheCursor(t *testing.T) {
	for _, c := range []struct{ total, want, calls int }{
		{total: 0, want: 0, calls: 1},
		{total: 25, want: 25, calls: 1},
		{total: 260, want: 260, calls: 3},
		{total: maxTxScopedRows + 250, want: maxTxScopedRows, calls: maxTxScopedRows / txScopedPageSize},
	} {
		t.Run(fmt.Sprint(c.total), func(t *testing.T) {
			calls := 0
			p := setupProvider(t, &fakeIndexer{listLogs: pagedLogs(c.total, &calls)})
			logs, err := p.GetLogsByTransaction(context.Background(), "0xtx")
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(logs) != c.want || calls != c.calls {
				t.Errorf("got %d logs in %d calls, want %d in %d", len(logs), calls, c.want, c.calls)
			}
			for i, l := range logs {
				if l.LogIndex != i {
					t.Fatalf("log %d has index %d: pages out of order", i, l.LogIndex)
				}
			}
		})
	}
}

func TestTxScopedReadsStopOnARepeatedCursor(t *testing.T) {
	calls := 0
	p := setupProvider(t, &fakeIndexer{
		listTransfers: func(*indexerv1.ListTokenTransfersRequest) (*indexerv1.ListTokenTransfersResponse, error) {
			calls++
			return &indexerv1.ListTokenTransfersResponse{
				Transfers: []*indexerv1.TokenTransfer{{LogIndex: 1}},
				Page:      &indexerv1.PageResponse{NextCursor: "same"},
			}, nil
		},
	})
	rows, err := p.GetTransfersByTransaction(context.Background(), "0xtx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rows) != 2 || calls != 2 {
		t.Errorf("got %d rows in %d calls, want 2 in 2", len(rows), calls)
	}
}

func TestInternalTxsByTxPagesAndKeepsCreateTarget(t *testing.T) {
	var cursors []string
	p := setupProvider(t, &fakeIndexer{
		listInternal: func(req *indexerv1.ListInternalTransactionsRequest) (*indexerv1.ListInternalTransactionsResponse, error) {
			cursors = append(cursors, req.GetPage().GetCursor())
			if req.GetPage().GetCursor() == "" {
				return &indexerv1.ListInternalTransactionsResponse{
					InternalTransactions: []*indexerv1.InternalTransaction{{TraceAddress: "0", CallType: "call", To: "0xa"}},
					Page:                 &indexerv1.PageResponse{NextCursor: "p2"},
				}, nil
			}
			return &indexerv1.ListInternalTransactionsResponse{
				InternalTransactions: []*indexerv1.InternalTransaction{{TraceAddress: "1", CallType: "create2", ContractAddress: "0xnew"}},
			}, nil
		},
	})
	rows, err := p.GetInternalTransactionsByTx(context.Background(), "0xtx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rows) != 2 || fmt.Sprint(cursors) != "[ p2]" {
		t.Fatalf("got %d rows, cursors %q", len(rows), cursors)
	}
	if rows[1].To == nil || *rows[1].To != "0xnew" {
		t.Errorf("CREATE2 target = %v, want the new contract's address", rows[1].To)
	}
}
