//go:build !privacy

package indexerclient

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
	"explorer/internal/api"
)

// BUG-9: GetTransactionsPaginated set fetchLimit := int32(page * pageSize). The
// int multiplication is fine, but the int32 narrowing overflows to a NEGATIVE
// page_size for a large page, which then travels to the indexer as PageSize.
// A negative page size is nonsensical and (depending on the server) errors or
// returns garbage. The request's page_size must always be >= 0.
func TestGetTransactionsPaginated_PageSizeNeverNegative(t *testing.T) {
	var gotPageSize int32 = -999
	p := setupProvider(t, &fakeIndexer{
		listTxs: func(req *indexerv1.ListTransactionsRequest) (*indexerv1.ListTransactionsResponse, error) {
			gotPageSize = req.GetPage().GetPageSize()
			return &indexerv1.ListTransactionsResponse{}, nil
		},
	})

	// page * pageSize = MaxInt32 * 100, which overflows int32.
	_, _, err := p.GetTransactionsPaginated(context.Background(), math.MaxInt32, 100)
	if err != nil {
		t.Fatalf("GetTransactionsPaginated: %v", err)
	}
	if gotPageSize < 0 {
		t.Fatalf("page_size sent to indexer = %d, must never be negative (int32 overflow, BUG-9)", gotPageSize)
	}
}

// BUG-8 (documented cross-repo gap): the chain-indexer ListTransactions RPC
// uses CURSOR pagination (PageResponse carries only next_cursor) — there is NO
// total_items field, unlike ListAddresses/ListTokens (OffsetPageResponse). So
// GetTransactionsPaginated cannot surface a true chain-wide total; it currently
// returns len(fetched), which scales with the page and is misleading. Surfacing
// a correct total needs a chain-indexer change (offset-paginate ListTransactions
// or add a count). Skipped so CI stays green; tracked as a chain-indexer issue.
func TestGetTransactionsPaginated_TrueTotalIsUpstreamGap(t *testing.T) {
	t.Skip("TODO(chain-indexer): ListTransactions is cursor-only (no total_items) " +
		"— /transactions/paginated cannot report a real total. See report.")

	const realChainTotal = 10_000
	p := setupProvider(t, &fakeIndexer{
		listTxs: func(req *indexerv1.ListTransactionsRequest) (*indexerv1.ListTransactionsResponse, error) {
			// Even if the server only returns one page, the response should
			// carry the real total somewhere. It does not today.
			return &indexerv1.ListTransactionsResponse{
				Transactions: make([]*indexerv1.Transaction, 25),
			}, nil
		},
	})
	_, total, _ := p.GetTransactionsPaginated(context.Background(), 1, 25)
	if total != realChainTotal {
		t.Errorf("total = %d, want the real chain total %d (needs upstream total_items)", total, realChainTotal)
	}
}

// TestGetTransactionsPaginated_NoOverlapAcrossPages pins the behavior we CAN
// guarantee today: the cursor-fetch + slice path returns disjoint, in-order
// windows across pages (no dupes, no gaps within the fetched window). This is
// the reliability floor while the true-total gap (above) is upstream.
func TestGetTransactionsPaginated_NoOverlapAcrossPages(t *testing.T) {
	const pageSize = 10
	// Fake server returns the first req.PageSize rows of a deterministic 60-row
	// descending feed (mirrors the indexer's cursor semantics: each call yields
	// the head of the list up to page_size).
	makeRows := func(n int) []*indexerv1.Transaction {
		rows := make([]*indexerv1.Transaction, n)
		for i := 0; i < n; i++ {
			rows[i] = &indexerv1.Transaction{Hash: fmt.Sprintf("0x%04d", 1000-i), BlockNumber: uint64(1000 - i)}
		}
		return rows
	}
	p := setupProvider(t, &fakeIndexer{
		listTxs: func(req *indexerv1.ListTransactionsRequest) (*indexerv1.ListTransactionsResponse, error) {
			ps := int(req.GetPage().GetPageSize())
			if ps < 0 {
				t.Fatalf("negative page_size %d", ps)
			}
			all := makeRows(60)
			if ps > len(all) {
				ps = len(all)
			}
			return &indexerv1.ListTransactionsResponse{Transactions: all[:ps]}, nil
		},
	})

	seen := map[string]int{}
	for page := 1; page <= 3; page++ {
		txs, _, err := p.GetTransactionsPaginated(context.Background(), page, pageSize)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if len(txs) != pageSize {
			t.Fatalf("page %d: got %d rows, want %d", page, len(txs), pageSize)
		}
		for _, tx := range txs {
			seen[tx.Hash]++
			if seen[tx.Hash] > 1 {
				t.Errorf("tx %s appeared on more than one page (overlap)", tx.Hash)
			}
		}
	}
	if len(seen) != 3*pageSize {
		t.Errorf("union across 3 pages = %d unique, want %d (no dupes/gaps)", len(seen), 3*pageSize)
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

func TestDegraded_AllTransfers_StandaloneUnsupported(t *testing.T) {
	// /token-transfers (global unfiltered feed) is not a supported indexer use
	// case, so the indexerclient Provider does NOT override it and it falls
	// through to DirectDBProvider -> ErrChainDataNotAvailable. Handlers map that
	// to 500. Lock the error identity.
	p := setupProvider(t, &fakeIndexer{})
	_, _, err := p.GetAllTransfers(context.Background(), 25, 0)
	if !errors.Is(err, api.ErrChainDataNotAvailable) {
		t.Fatalf("GetAllTransfers err = %v, want ErrChainDataNotAvailable", err)
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
