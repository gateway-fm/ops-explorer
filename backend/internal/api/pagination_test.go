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
	"math"
	"testing"

	"explorer/internal/types"
)

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
