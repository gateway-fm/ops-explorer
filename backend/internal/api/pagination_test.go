package api

// BUG-1: totalPages was computed as `int(total) / pageSize` at 9 handler sites.
// On a 32-bit platform `int` is 32 bits, so int(total) truncates an int64 total
// above math.MaxInt32, yielding a TotalPages inconsistent with the int64 Total
// surfaced in the same response (and potentially negative). The fix extracts a
// computeTotalPages helper that does the arithmetic in int64 and only narrows
// the final (small) quotient.
//
// BUG-3: extractChartDataPoints is a pure function over types.DailyStats. These
// table cases lock the success-rate, average-fee (wei->Gwei), avg-gas-used, and
// cumulative chart math against the spec, independent of upstream mapping.

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	"explorer/internal/types"
)

// BUG-7 (api side): paginate's cursor must be the last returned row's block
// number (for the ?before= round-trip), not the page length.
func TestPaginate_CursorIsLastBlockNumber(t *testing.T) {
	rows := []types.Transaction{
		{Hash: "0xa", BlockNumber: 200},
		{Hash: "0xb", BlockNumber: 150},
		{Hash: "0xc", BlockNumber: 120}, // limit+1 sentinel, trimmed
	}
	resp := paginate(rows, 2, func(tx types.Transaction) string {
		return strconv.FormatUint(tx.BlockNumber, 10)
	})
	if !resp.HasMore || len(resp.Data) != 2 {
		t.Fatalf("HasMore=%v len=%d, want true/2", resp.HasMore, len(resp.Data))
	}
	if resp.NextCursor == nil || *resp.NextCursor != "150" {
		t.Errorf("NextCursor = %v, want \"150\" (last returned row's block number)", resp.NextCursor)
	}
}

// RD-1149 (api side): cursorPage wraps a keyset page. The provider returns the
// authoritative opaque next-cursor, so HasMore == (nextCursor != "") and the
// emitted NextCursor is that opaque value verbatim (not a block number).
func TestCursorPage(t *testing.T) {
	rows := []types.Transaction{{Hash: "0xa", BlockNumber: 200}, {Hash: "0xb", BlockNumber: 150}}

	more := cursorPage(rows, "opaque-next")
	if !more.HasMore {
		t.Error("HasMore = false, want true when a next cursor is present")
	}
	if more.NextCursor == nil || *more.NextCursor != "opaque-next" {
		t.Errorf("NextCursor = %v, want \"opaque-next\" (opaque provider value verbatim)", more.NextCursor)
	}
	if len(more.Data) != 2 {
		t.Errorf("len(Data) = %d, want 2 (no over-fetch trimming)", len(more.Data))
	}

	last := cursorPage(rows, "")
	if last.HasMore {
		t.Error("HasMore = true, want false when the cursor is empty (exhausted)")
	}
	if last.NextCursor != nil {
		t.Errorf("NextCursor = %v, want nil when exhausted", *last.NextCursor)
	}

	// A nil page must serialize as an empty JSON array, not null, so clients
	// that treat Data as T[] don't break (Copilot review on #125).
	empty := cursorPage[types.Transaction](nil, "")
	if empty.Data == nil || len(empty.Data) != 0 {
		t.Errorf("Data = %v, want a non-nil empty slice", empty.Data)
	}
	b, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"data":[]`) {
		t.Errorf("marshaled = %s, want it to contain \"data\":[] (not null)", b)
	}
}

func ceilDiv(total int64, pageSize int) int {
	// Reference implementation in int64; only the final quotient is narrowed.
	ps := int64(pageSize)
	q := total / ps
	if total%ps != 0 {
		q++
	}
	return int(q)
}

func TestComputeTotalPages(t *testing.T) {
	cases := []struct {
		name     string
		total    int64
		pageSize int
		want     int
	}{
		{"empty", 0, 25, 0},
		{"exact", 50, 25, 2},
		{"remainder", 51, 25, 3},
		{"single-partial", 1, 25, 1},
		// The headline case: total well above math.MaxInt32. ceil(3e9/25).
		{"above-int32", 3_000_000_000, 25, 120_000_000},
		{"at-maxint32-plus-one", int64(math.MaxInt32) + 1, 100, ceilDiv(int64(math.MaxInt32)+1, 100)},
		{"huge-total-pagesize-1", int64(math.MaxInt32) + 7, 1, ceilDiv(int64(math.MaxInt32)+7, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeTotalPages(tc.total, tc.pageSize)
			if got != tc.want {
				t.Errorf("computeTotalPages(%d, %d) = %d, want %d", tc.total, tc.pageSize, got, tc.want)
			}
			if got < 0 {
				t.Errorf("computeTotalPages(%d, %d) = %d, must never be negative", tc.total, tc.pageSize, got)
			}
		})
	}
}

func TestComputeTotalPages_ZeroPageSizeNoPanic(t *testing.T) {
	// A defensive guard: pageSize 0 must not panic (divide-by-zero). Handlers
	// clamp pageSize before calling, but the helper must be safe regardless.
	if got := computeTotalPages(100, 0); got < 0 {
		t.Errorf("computeTotalPages(100,0) = %d, want a non-negative, non-panicking result", got)
	}
}

// --- BUG-3: chart-data math contract ---------------------------------------

func TestExtractChartDataPoints_SpecMath(t *testing.T) {
	stats := []types.DailyStats{
		{
			Date:                   "2026-06-01",
			TotalTransactions:      10,
			SuccessfulTxs:          9, // 9/10 => 90.0%
			AvgGasPrice:            1_000_000_000, // 1e9 wei => 1.0 Gwei
			TotalGasUsed:           200,
			TotalBlocks:            10, // avg_gas_used = 200/10 = 20
			ActiveAddresses:        3,
			NewAddresses:           2,
			CumulativeTransactions: 100,
			CumulativeAddresses:    50,
			CumulativeContracts:    7,
		},
		{
			Date:                   "2026-06-02",
			TotalTransactions:      20,
			SuccessfulTxs:          10, // 10/20 => 50.0%
			AvgGasPrice:            2_000_000_000, // 2.0 Gwei
			TotalGasUsed:           300,
			TotalBlocks:            0, // avg_gas_used guarded -> 0
			CumulativeTransactions: 120,
			CumulativeAddresses:    60,
			CumulativeContracts:    9,
		},
	}

	check := func(id string, idx int, want float64) {
		t.Helper()
		pts := ExtractChartDataPoints(id, stats)
		if len(pts) != len(stats) {
			t.Fatalf("%s: got %d points, want %d", id, len(pts), len(stats))
		}
		if pts[idx].Value != want {
			t.Errorf("%s[%d] = %v, want %v", id, idx, pts[idx].Value, want)
		}
	}

	check("txns_success_rate", 0, 90.0)
	check("txns_success_rate", 1, 50.0)
	check("avg_txn_fee", 0, 1.0)   // 1e9 wei / 1e9 = 1 Gwei
	check("avg_txn_fee", 1, 2.0)
	check("avg_gas_price", 0, 1.0)
	check("avg_gas_used", 0, 20.0) // 200/10
	check("avg_gas_used", 1, 0.0)  // TotalBlocks==0 guarded
	check("new_txns", 0, 10.0)
	check("active_accounts", 0, 3.0)
	check("new_accounts", 0, 2.0)
	check("txns_growth", 0, 100.0)
	check("accounts_growth", 1, 60.0)
	check("contracts_growth", 1, 9.0)
}

// TestExtractChartDataPoints_SuccessRateZeroTxns guards the success-rate
// divide-by-zero edge: a day with 0 transactions must yield 0%, not NaN.
func TestExtractChartDataPoints_SuccessRateZeroTxns(t *testing.T) {
	stats := []types.DailyStats{{Date: "2026-06-01", TotalTransactions: 0, SuccessfulTxs: 0}}
	pts := ExtractChartDataPoints("txns_success_rate", stats)
	if len(pts) != 1 || pts[0].Value != 0 {
		t.Fatalf("0-txn success rate = %+v, want value 0", pts)
	}
}

// TestExtractChartDataPoints_GasUsedGrowthMonotonic locks the cumulative
// gas-used-growth line: it is a running sum, so it must be non-decreasing.
func TestExtractChartDataPoints_GasUsedGrowthMonotonic(t *testing.T) {
	stats := []types.DailyStats{
		{Date: "d1", TotalGasUsed: 100},
		{Date: "d2", TotalGasUsed: 50},
		{Date: "d3", TotalGasUsed: 200},
	}
	pts := ExtractChartDataPoints("gas_used_growth", stats)
	want := []float64{100, 150, 350}
	for i, w := range want {
		if pts[i].Value != w {
			t.Errorf("gas_used_growth[%d] = %v, want %v", i, pts[i].Value, w)
		}
		if i > 0 && pts[i].Value < pts[i-1].Value {
			t.Errorf("gas_used_growth not monotonic at %d: %v < %v", i, pts[i].Value, pts[i-1].Value)
		}
	}
}
