package publicapi

// BUG-1 / BUG-7 / BUG-pub: public-api pagination contract.
//
//   - BUG-1: offsetPaginated computed totalPages as int(total)/pageSize,
//     truncating the int64 total to platform int before dividing (32-bit
//     overflow -> TotalPages inconsistent with Total). Fixed to int64 math.
//   - BUG-7: paginate emitted nextCursor = strconv.Itoa(len(items)) — the page
//     length, not a real cursor. The API documents ?before=<block>, so the
//     cursor MUST be the last returned row's block number for the round-trip to
//     page forward. Locked here.
//   - BUG-pub: handleGetAddressTransfers computed offset then `_ = offset`, so
//     ?page=2 returned the same rows as ?page=1. Locked + fixed to apply offset.

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"testing"

	"explorer/internal/types"
)

// --- BUG-1: offsetPaginated int64 safety ------------------------------------

func TestOffsetPaginated_TotalPagesInt64(t *testing.T) {
	cases := []struct {
		name     string
		total    int64
		pageSize int
		want     int
	}{
		{"exact", 50, 25, 2},
		{"remainder", 51, 25, 3},
		{"empty", 0, 25, 0},
		{"above-int32", 3_000_000_000, 25, 120_000_000},
		{"maxint32-plus-one", int64(math.MaxInt32) + 1, 100, int((int64(math.MaxInt32)+1+99)/100)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := offsetPaginated([]int{}, tc.total, 1, tc.pageSize)
			if resp.TotalPages != tc.want {
				t.Errorf("TotalPages = %d, want %d (total=%d pageSize=%d)", resp.TotalPages, tc.want, tc.total, tc.pageSize)
			}
			if resp.TotalPages < 0 {
				t.Errorf("TotalPages = %d, must never be negative", resp.TotalPages)
			}
			if resp.Total != tc.total {
				t.Errorf("Total = %d, want %d (must surface the int64 total unchanged)", resp.Total, tc.total)
			}
		})
	}
}

func TestOffsetPaginated_NilDataBecomesEmptySlice(t *testing.T) {
	resp := offsetPaginated[int](nil, 0, 1, 25)
	if resp.Data == nil {
		t.Error("Data is nil, want [] (must marshal as [] not null)")
	}
}

// --- BUG-7: paginate cursor is the last row's block number ------------------

func txCursor(tx types.Transaction) string {
	return strconv.FormatUint(tx.BlockNumber, 10)
}

func TestPaginate_CursorIsLastBlockNumber(t *testing.T) {
	// limit=2, fetched 3 (limit+1) -> hasMore true, page trimmed to 2 rows.
	// The cursor MUST be the last returned row's block number (100), not the
	// page length (2), so the next request's ?before=100 pages forward.
	rows := []types.Transaction{
		{Hash: "0xa", BlockNumber: 102},
		{Hash: "0xb", BlockNumber: 100},
		{Hash: "0xc", BlockNumber: 98}, // the +1 sentinel, trimmed
	}
	resp := paginate(rows, 2, txCursor)
	if !resp.HasMore {
		t.Fatal("HasMore = false, want true")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(resp.Data))
	}
	if resp.NextCursor == nil {
		t.Fatal("NextCursor = nil, want the last row's block number")
	}
	if *resp.NextCursor != "100" {
		t.Errorf("NextCursor = %q, want %q (last returned row's block number, not page length)", *resp.NextCursor, "100")
	}
}

func TestPaginate_NoCursorWhenNoMore(t *testing.T) {
	rows := []types.Transaction{{Hash: "0xa", BlockNumber: 102}}
	resp := paginate(rows, 25, txCursor)
	if resp.HasMore {
		t.Error("HasMore = true, want false")
	}
	if resp.NextCursor != nil {
		t.Errorf("NextCursor = %v, want nil when there is no next page", *resp.NextCursor)
	}
}

// --- BUG-pub: handleGetAddressTransfers honors ?page= -----------------------

// tcTransfersProvider returns a deterministic, descending-block-number window
// of transfers. It implements offset semantics so a correctly-fixed handler can
// page; the cursor (beforeBlock) is honored when set.
type tcTransfersProvider struct {
	tcProvider
	rows []types.TokenTransfer
}

func (p *tcTransfersProvider) GetTransfersByAddress(_ context.Context, _ string, limit int, _ string, beforeBlock *uint64) ([]types.TokenTransfer, string, error) {
	start := 0
	if beforeBlock != nil {
		for i, r := range p.rows {
			if r.BlockNumber < *beforeBlock {
				start = i
				break
			}
			start = len(p.rows)
		}
	}
	end := start + limit
	if end > len(p.rows) {
		end = len(p.rows)
	}
	if start > len(p.rows) {
		start = len(p.rows)
	}
	return p.rows[start:end], "", nil
}

func makeTransfers(n int) []types.TokenTransfer {
	out := make([]types.TokenTransfer, n)
	for i := 0; i < n; i++ {
		out[i] = types.TokenTransfer{
			TxHash:      "0x" + string(rune('a'+i%26)),
			LogIndex:    i,
			BlockNumber: uint64(1000 - i), // strictly descending, unique
		}
	}
	return out
}

func TestHandleGetAddressTransfers_PageNotIgnored(t *testing.T) {
	const addr = "0x52908400098527886E0F7030069857D2E4169EE7" // checksum-valid
	prov := &tcTransfersProvider{rows: makeTransfers(60)}
	s := tcServer(prov)

	decode := func(path string) []types.TokenTransfer {
		w := tcReq(s, path, "203.0.113.50:1")
		if w.Code != http.StatusOK {
			t.Fatalf("%s -> %d, want 200 (body=%s)", path, w.Code, w.Body.String())
		}
		var resp types.PaginatedResponse[types.TokenTransfer]
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s decode: %v", path, err)
		}
		return resp.Data
	}

	page1 := decode("/api/v1/addresses/" + addr + "/transfers?page=1&pageSize=10")
	page2 := decode("/api/v1/addresses/" + addr + "/transfers?page=2&pageSize=10")

	if len(page1) == 0 || len(page2) == 0 {
		t.Fatalf("empty pages: p1=%d p2=%d", len(page1), len(page2))
	}
	// BUG-pub: with the bug, page2 == page1 (offset discarded). They must differ.
	if page1[0].BlockNumber == page2[0].BlockNumber {
		t.Fatalf("page2 first block %d == page1 first block %d — ?page= is ignored (BUG-pub)",
			page2[0].BlockNumber, page1[0].BlockNumber)
	}
	// No overlap between the two pages (no dupes).
	seen := map[uint64]bool{}
	for _, r := range page1 {
		seen[r.BlockNumber] = true
	}
	for _, r := range page2 {
		if seen[r.BlockNumber] {
			t.Errorf("block %d appears on both page1 and page2 (overlap)", r.BlockNumber)
		}
	}
}
