//go:build !privacy

package indexerclient

import (
	"context"
	"fmt"
	"math"
	"testing"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
)

// BUG-9 (historical): the old cursor-fetch path computed fetchLimit :=
// int32(page * pageSize), which overflowed int32 to a NEGATIVE page_size for a
// large page. The offset RPC sends page and pageSize as independent int32s
// (no multiplication), so the overflow is structurally impossible — this pins
// that the forwarded page_size is never negative even at the int boundary.
func TestGetTransactionsPaginated_PageSizeNeverNegative(t *testing.T) {
	var gotPageSize int32 = -999
	p := setupProvider(t, &fakeIndexer{
		listTxsPaginated: func(req *indexerv1.ListTransactionsPaginatedRequest) (*indexerv1.ListTransactionsPaginatedResponse, error) {
			gotPageSize = req.GetPage().GetPageSize()
			return &indexerv1.ListTransactionsPaginatedResponse{
				Page: &indexerv1.OffsetPageResponse{TotalItems: 0},
			}, nil
		},
	})

	_, _, err := p.GetTransactionsPaginated(context.Background(), math.MaxInt32, 100)
	if err != nil {
		t.Fatalf("GetTransactionsPaginated: %v", err)
	}
	if gotPageSize < 0 {
		t.Fatalf("page_size sent to indexer = %d, must never be negative", gotPageSize)
	}
}

// BUG-8 (FIXED by chain-indexer RD-1061): the indexer now exposes an
// offset-paginated ListTransactionsPaginated whose OffsetPageResponse carries a
// true chain-wide total_items — unlike the cursor-based ListTransactions, which
// has no count. GetTransactionsPaginated surfaces total_items directly, so the
// /transactions page total is the real chain total, stable across pages, not
// len(fetched). This is the previously-skipped gap, now closed.
func TestGetTransactionsPaginated_TrueTotalIsSurfaced(t *testing.T) {
	const realChainTotal = 10_000
	p := setupProvider(t, &fakeIndexer{
		listTxsPaginated: func(req *indexerv1.ListTransactionsPaginatedRequest) (*indexerv1.ListTransactionsPaginatedResponse, error) {
			// Server returns just one page of rows but the real total in total_items.
			return &indexerv1.ListTransactionsPaginatedResponse{
				Transactions: make([]*indexerv1.Transaction, 25),
				Page:         &indexerv1.OffsetPageResponse{Page: 1, PageSize: 25, TotalItems: realChainTotal},
			}, nil
		},
	})
	_, total, err := p.GetTransactionsPaginated(context.Background(), 1, 25)
	if err != nil {
		t.Fatalf("GetTransactionsPaginated: %v", err)
	}
	if total != realChainTotal {
		t.Errorf("total = %d, want the real chain total %d (total_items)", total, realChainTotal)
	}
}

// TestGetTransactionsPaginated_MultiPage reconciles the offset-paginated tx
// feed end-to-end: total_items is surfaced unchanged on EVERY page (stable, not
// page-local — the bug this fixed), windows are disjoint and in-order, the row
// union has no dupes/gaps, and the number of non-empty pages == ceil(N/pageSize).
// Mirrors TestReconcile_AccountsMultiPage now that the indexer offset-paginates.
func TestGetTransactionsPaginated_MultiPage(t *testing.T) {
	const (
		N        = 57
		pageSize = 10
	)
	p := setupProvider(t, &fakeIndexer{
		listTxsPaginated: func(req *indexerv1.ListTransactionsPaginatedRequest) (*indexerv1.ListTransactionsPaginatedResponse, error) {
			page := int(req.GetPage().GetPage())
			ps := int(req.GetPage().GetPageSize())
			if ps != pageSize {
				t.Errorf("page_size = %d, want %d", ps, pageSize)
			}
			start, end := offsetWindow(N, page, ps)
			rows := make([]*indexerv1.Transaction, 0, end-start)
			for i := start; i < end; i++ {
				rows = append(rows, &indexerv1.Transaction{Hash: fmt.Sprintf("0x%05d", i), BlockNumber: uint64(N - i)})
			}
			return &indexerv1.ListTransactionsPaginatedResponse{
				Transactions: rows,
				Page:         &indexerv1.OffsetPageResponse{Page: int32(page), PageSize: int32(ps), TotalItems: N},
			}, nil
		},
	})

	seen := map[string]bool{}
	sum := 0
	nonEmptyPages := 0
	wantPages := (N + pageSize - 1) / pageSize
	for page := 1; page <= wantPages+2; page++ { // overshoot to prove empties past the end
		rows, total, err := p.GetTransactionsPaginated(context.Background(), page, pageSize)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if total != N {
			t.Errorf("page %d: total = %d, want %d (must surface total_items unchanged, not page-local)", page, total, N)
		}
		if len(rows) > 0 {
			nonEmptyPages++
		}
		for _, tx := range rows {
			if seen[tx.Hash] {
				t.Errorf("tx %s seen on more than one page (overlap)", tx.Hash)
			}
			seen[tx.Hash] = true
			sum++
		}
	}
	if sum != N {
		t.Errorf("Σ len(pages) = %d, want %d", sum, N)
	}
	if len(seen) != N {
		t.Errorf("union(data) = %d unique, want %d (no gaps)", len(seen), N)
	}
	if nonEmptyPages != wantPages {
		t.Errorf("non-empty pages = %d, want ceil(%d/%d) = %d", nonEmptyPages, N, pageSize, wantPages)
	}
}

// =============================================================================
// §4.1 — Standalone multi-page reconciliation against a real total_items.
//
// ListAddresses (GetAccountsPaginated) and ListTokens (GetTokens) ride the
// OffsetPageResponse which DOES carry total_items. This is the strongest
// "counters match reality" test the repo can host: serve N deterministic rows
// with total_items=N, page through, and assert:
//   - total == N on every page (faithfully surfaced, not corrupted)
//   - union(data) == N unique (no dupes, no gaps)
//   - Σ len(page_i) == N
//   - number of non-empty pages == ceil(N/pageSize)
// =============================================================================

// offsetWindow serves rows[(page-1)*pageSize : ...] honoring the OffsetPageRequest
// and always reports total_items=len(rows). page<=0 is treated as page 1 (proto
// contract). Returns the page slice and the OffsetPageResponse.
func offsetWindow(total, page, pageSize int) (start, end int) {
	if page <= 0 {
		page = 1
	}
	start = (page - 1) * pageSize
	if start > total {
		start = total
	}
	end = start + pageSize
	if end > total {
		end = total
	}
	return start, end
}

func TestReconcile_AccountsMultiPage(t *testing.T) {
	const (
		N        = 57
		pageSize = 10
	)
	p := setupProvider(t, &fakeIndexer{
		listAddresses: func(req *indexerv1.ListAddressesRequest) (*indexerv1.ListAddressesResponse, error) {
			page := int(req.GetPage().GetPage())
			ps := int(req.GetPage().GetPageSize())
			if ps != pageSize {
				t.Errorf("page_size = %d, want %d", ps, pageSize)
			}
			start, end := offsetWindow(N, page, ps)
			rows := make([]*indexerv1.Address, 0, end-start)
			for i := start; i < end; i++ {
				rows = append(rows, &indexerv1.Address{Address: fmt.Sprintf("0xacc%05d", i)})
			}
			return &indexerv1.ListAddressesResponse{
				Addresses: rows,
				Page:      &indexerv1.OffsetPageResponse{Page: int32(page), PageSize: int32(ps), TotalItems: N},
			}, nil
		},
	})

	seen := map[string]bool{}
	sum := 0
	nonEmptyPages := 0
	wantPages := (N + pageSize - 1) / pageSize
	for page := 1; page <= wantPages+2; page++ { // overshoot to prove empties past the end
		rows, total, err := p.GetAccountsPaginated(context.Background(), page, pageSize)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if total != N {
			t.Errorf("page %d: total = %d, want %d (must surface total_items unchanged)", page, total, N)
		}
		if len(rows) > 0 {
			nonEmptyPages++
		}
		for _, r := range rows {
			if seen[r.Address] {
				t.Errorf("address %s seen on more than one page (dup)", r.Address)
			}
			seen[r.Address] = true
			sum++
		}
	}
	if sum != N {
		t.Errorf("Σ len(pages) = %d, want %d", sum, N)
	}
	if len(seen) != N {
		t.Errorf("union(data) = %d unique, want %d (no gaps)", len(seen), N)
	}
	if nonEmptyPages != wantPages {
		t.Errorf("non-empty pages = %d, want ceil(%d/%d) = %d", nonEmptyPages, N, pageSize, wantPages)
	}
}

// =============================================================================
// §4.3 — degraded endpoints: total == len(rows) (page-local), locked + marked
// as a KNOWN degradation so it can't silently worsen; and /token-transfers
// standalone -> ErrChainDataNotAvailable (admin global feed unsupported).
// =============================================================================

func TestDegraded_LogsByAddress_TotalIsPageLocal(t *testing.T) {
	const limit = 5
	p := setupProvider(t, &fakeIndexer{
		listLogs: func(req *indexerv1.ListLogsRequest) (*indexerv1.ListLogsResponse, error) {
			logs := make([]*indexerv1.Log, limit) // server returns exactly `limit`
			for i := range logs {
				logs[i] = &indexerv1.Log{LogIndex: uint32(i), Address: "0xc"}
			}
			return &indexerv1.ListLogsResponse{Logs: logs}, nil
		},
	})
	rows, total, err := p.GetLogsByAddress(context.Background(), "0xc", limit, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// KNOWN DEGRADATION (handlers_extended.go file comment): the indexer ListLogs
	// has no count, so total degrades to len(rows). Lock it so it can't silently
	// change to a wrong non-page-local value.
	if total != int64(len(rows)) {
		t.Errorf("total = %d, want len(rows) = %d (known page-local degradation)", total, len(rows))
	}
}

func TestDegraded_TransfersByToken_TotalIsPageLocal(t *testing.T) {
	const limit = 4
	p := setupProvider(t, &fakeIndexer{
		listTransfers: func(req *indexerv1.ListTokenTransfersRequest) (*indexerv1.ListTokenTransfersResponse, error) {
			tr := make([]*indexerv1.TokenTransfer, limit)
			for i := range tr {
				tr[i] = &indexerv1.TokenTransfer{LogIndex: uint32(i)}
			}
			return &indexerv1.ListTokenTransfersResponse{Transfers: tr}, nil
		},
	})
	rows, total, err := p.GetTransfersByToken(context.Background(), "0xtok", limit, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if total != int64(len(rows)) {
		t.Errorf("total = %d, want len(rows) = %d (known page-local degradation)", total, len(rows))
	}
}

func TestProvider_GetAllTransfers_GlobalFeed(t *testing.T) {
	// The global token-transfers feed IS a supported indexer call as of the
	// "global token-transfers feed" feature (chain-indexer ListAllTokenTransfers).
	// The Provider forwards offset/limit as an OffsetPageRequest, maps the
	// response transfers, and surfaces total_items; an empty tokenType means
	// "all types" (no TokenType filter). Lock that contract.
	var gotReq *indexerv1.ListAllTokenTransfersRequest
	p := setupProvider(t, &fakeIndexer{
		listAllTransfers: func(req *indexerv1.ListAllTokenTransfersRequest) (*indexerv1.ListAllTokenTransfersResponse, error) {
			gotReq = req
			return &indexerv1.ListAllTokenTransfersResponse{
				Transfers: []*indexerv1.TokenTransfer{{LogIndex: 0}, {LogIndex: 1}},
				Page:      &indexerv1.OffsetPageResponse{Page: 1, PageSize: 10, TotalItems: 2},
			}, nil
		},
	})

	rows, total, err := p.GetAllTransfers(context.Background(), "", 10, 0)
	if err != nil {
		t.Fatalf("GetAllTransfers err = %v, want nil", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(rows) != 2 {
		t.Errorf("len(rows) = %d, want 2", len(rows))
	}
	if gotReq.GetPage().GetPageSize() != 10 {
		t.Errorf("forwarded page_size = %d, want 10", gotReq.GetPage().GetPageSize())
	}
	if gotReq.GetTokenType() != indexerv1.TokenType_TOKEN_TYPE_UNSPECIFIED {
		t.Errorf("tokenType = %v, want UNSPECIFIED for empty filter", gotReq.GetTokenType())
	}
}

func TestReconcile_TokensMultiPage(t *testing.T) {
	const (
		N        = 25
		pageSize = 10
	)
	p := setupProvider(t, &fakeIndexer{
		listTokens: func(req *indexerv1.ListTokensRequest) (*indexerv1.ListTokensResponse, error) {
			page := int(req.GetPage().GetPage())
			ps := int(req.GetPage().GetPageSize())
			start, end := offsetWindow(N, page, ps)
			rows := make([]*indexerv1.Token, 0, end-start)
			for i := start; i < end; i++ {
				rows = append(rows, &indexerv1.Token{Address: fmt.Sprintf("0xtok%05d", i), TokenType: indexerv1.TokenType_TOKEN_TYPE_ERC20})
			}
			return &indexerv1.ListTokensResponse{
				Tokens: rows,
				Page:   &indexerv1.OffsetPageResponse{Page: int32(page), PageSize: int32(ps), TotalItems: N},
			}, nil
		},
	})

	seen := map[string]bool{}
	sum := 0
	wantPages := (N + pageSize - 1) / pageSize
	for page := 1; page <= wantPages; page++ {
		rows, total, err := p.GetTokens(context.Background(), pageSize, (page-1)*pageSize, "", "")
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if total != N {
			t.Errorf("page %d: total = %d, want %d", page, total, N)
		}
		for _, r := range rows {
			if seen[r.Address] {
				t.Errorf("token %s seen twice (dup)", r.Address)
			}
			seen[r.Address] = true
			sum++
		}
	}
	if sum != N || len(seen) != N {
		t.Errorf("reconcile: sum=%d unique=%d, want %d both", sum, len(seen), N)
	}
}
