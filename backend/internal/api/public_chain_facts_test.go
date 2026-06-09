package api

// Path C (plan §2): publicChainFactsFromRPC drives chain-height facts in privacy
// mode through the node/proxy RPC forwarder. Contract (handlers.go:67-106):
//   - totalBlocks = int64(latest)+1 (genesis counted);
//   - latest<2 -> (totalBlocks, 0, true) (can't sample without touching genesis);
//   - genesis (block 0) deliberately excluded from the average sample;
//   - an implausible average (<=0 or >1h/block, e.g. dev-chain zero timestamps)
//     -> ok=true but avg 0 (leave upstream untouched);
//   - a denied/failed RPC -> ok=false (all-or-nothing).
//
// Tests build a real rpc.Client over an httptest JSON-RPC server keyed by method.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/internal/privacy"
	"explorer/internal/rpc"
	"explorer/internal/types"
)

// hpFakePrivacyClient returns a non-nil privacy.Client (handleGetStats only
// checks privacyClient != nil to set PrivacyEnabled and enable the RPC overlay).
func hpFakePrivacyClient(t *testing.T) *privacy.Client {
	t.Helper()
	return privacy.NewClient("http://privacy-proxy.invalid")
}

// hpRPCServer stands up a JSON-RPC 2.0 server. blockNumber is the eth_blockNumber
// hex result; tsByBlock maps a hex block number ("0x...") to its timestamp hex
// for eth_getBlockByNumber. A method with no canned answer returns a JSON-RPC
// error (simulating a denied/forwarded call).
func hpRPCClient(t *testing.T, blockNumber string, tsByBlock map[string]string, denyBlockNumber bool) *rpc.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env struct {
			ID     uint64        `json:"id"`
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&env)
		w.Header().Set("Content-Type", "application/json")
		id := itoaU(env.ID)
		switch env.Method {
		case "eth_blockNumber":
			if denyBlockNumber {
				_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+id+`,"error":{"code":-32000,"message":"denied"}}`)
				return
			}
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+id+`,"result":"`+blockNumber+`"}`)
		case "eth_getBlockByNumber":
			num, _ := env.Params[0].(string)
			ts, ok := tsByBlock[num]
			if !ok {
				_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+id+`,"error":{"code":-32000,"message":"denied"}}`)
				return
			}
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+id+`,"result":{"timestamp":"`+ts+`"}}`)
		default:
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+id+`,"error":{"code":-32601,"message":"method not found"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	c, err := rpc.New(srv.URL)
	if err != nil {
		t.Fatalf("rpc.New: %v", err)
	}
	return c
}

func itoaU(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestPublicChainFacts_NormalAverage(t *testing.T) {
	// latest=0x65 (101). Sample window: earliest=1 (101>100), so window=100.
	// ts(101)=1000s + (100*12)=1200 -> ts(101)=2200; ts(1)=1000 -> avg=12.0.
	c := hpRPCClient(t, "0x65", map[string]string{
		"0x65": "0x" + toHexU(2200), // latest ts
		"0x1":  "0x" + toHexU(1000), // earliest sample ts
	}, false)
	total, avg, ok := publicChainFactsFromRPC(context.Background(), c)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if total != 102 { // int64(101)+1
		t.Errorf("totalBlocks = %d, want 102 (latest+1)", total)
	}
	if avg < 11.9 || avg > 12.1 {
		t.Errorf("avgBlockTime = %v, want ~12.0", avg)
	}
}

func TestPublicChainFacts_BlockNumberDenied(t *testing.T) {
	// eth_blockNumber denied -> all-or-nothing: ok=false.
	c := hpRPCClient(t, "0x65", nil, true)
	total, avg, ok := publicChainFactsFromRPC(context.Background(), c)
	if ok || total != 0 || avg != 0 {
		t.Errorf("denied blockNumber -> (%d,%v,%v), want (0,0,false)", total, avg, ok)
	}
}

func TestPublicChainFacts_LatestBelow2(t *testing.T) {
	// latest=1 -> can't sample without genesis -> (totalBlocks=2, avg=0, ok=true).
	c := hpRPCClient(t, "0x1", nil, false)
	total, avg, ok := publicChainFactsFromRPC(context.Background(), c)
	if !ok || total != 2 || avg != 0 {
		t.Errorf("latest=1 -> (%d,%v,%v), want (2,0,true)", total, avg, ok)
	}
}

func TestPublicChainFacts_ImplausibleAverageSkipped(t *testing.T) {
	// Dev chain: early block timestamp 0 makes the average enormous -> guard
	// returns ok=true with avg 0 (don't override upstream with garbage).
	// latest=0x65 (101); ts(101)=very large, ts(1)=1 -> avg > 1h/block.
	c := hpRPCClient(t, "0x65", map[string]string{
		"0x65": "0x" + toHexU(100_000_000),
		"0x1":  "0x" + toHexU(1),
	}, false)
	total, avg, ok := publicChainFactsFromRPC(context.Background(), c)
	if !ok {
		t.Fatal("ok = false, want true (skip override, not hard fail)")
	}
	if total != 102 {
		t.Errorf("totalBlocks = %d, want 102", total)
	}
	if avg != 0 {
		t.Errorf("avgBlockTime = %v, want 0 (implausible average must be skipped)", avg)
	}
}

func TestPublicChainFacts_TimestampDenied(t *testing.T) {
	// latest sample fetch denied -> ok=false.
	c := hpRPCClient(t, "0x65", map[string]string{
		"0x1": "0x" + toHexU(1000), // only earliest present; latest denied
	}, false)
	_, _, ok := publicChainFactsFromRPC(context.Background(), c)
	if ok {
		t.Error("ok = true, want false when a sample timestamp fetch is denied")
	}
}

// TestPublicChainFacts_MaxUint64Overflow locks the int64(latest)+1 arithmetic
// at the uint64 ceiling. latest=MaxUint64 is not a realistic chain height, but
// the cast is a latent overflow (int64(MaxUint64)+1 wraps to 0). With the
// sample timestamps absent the function bails ok=false; this pins that an
// absurd height does not panic and is handled (no crash, deterministic result).
func TestPublicChainFacts_MaxUint64Overflow(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("publicChainFactsFromRPC panicked at MaxUint64 latest: %v", r)
		}
	}()
	c := hpRPCClient(t, "0xffffffffffffffff", nil, false) // sample fetches denied
	_, _, ok := publicChainFactsFromRPC(context.Background(), c)
	// Sample timestamps are absent -> ok=false (all-or-nothing). The point is it
	// returns deterministically without panicking on the overflowed total.
	if ok {
		t.Error("ok = true, want false (no sample data) — and must not panic on overflow")
	}
}

// TestHandleGetStats_PrivacyRPCOverrideReplacesBlocksOnly (§4.4): in privacy
// mode handleGetStats overlays public chain facts from RPC onto the proxy's
// ChainStats, but only TotalBlocks (and AvgBlockTime). TotalTransactions /
// TotalAddresses — which are RBAC-scoped per caller by the proxy — must be left
// exactly as the proxy returned them.
func TestHandleGetStats_PrivacyRPCOverrideReplacesBlocksOnly(t *testing.T) {
	rpcClient := hpRPCClient(t, "0x65", map[string]string{
		"0x65": "0x" + toHexU(2200),
		"0x1":  "0x" + toHexU(1000),
	}, false)
	s := &Server{
		provider:      &hpStub{chainStats: &types.ChainStats{TotalBlocks: 5, TotalTransactions: 100, TotalAddresses: 20}},
		privacyClient: hpFakePrivacyClient(t),
		rpc:           rpcClient,
	}
	w := httptest.NewRecorder()
	s.handleGetStats(w, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var stats types.ChainStats
	_ = json.Unmarshal(w.Body.Bytes(), &stats)
	if !stats.PrivacyEnabled {
		t.Error("PrivacyEnabled = false, want true (privacyClient set)")
	}
	if stats.TotalBlocks != 102 {
		t.Errorf("TotalBlocks = %d, want 102 (RPC override latest+1)", stats.TotalBlocks)
	}
	// The RBAC-scoped counters must NOT be touched by the RPC override.
	if stats.TotalTransactions != 100 {
		t.Errorf("TotalTransactions = %d, want 100 unchanged (proxy-scoped)", stats.TotalTransactions)
	}
	if stats.TotalAddresses != 20 {
		t.Errorf("TotalAddresses = %d, want 20 unchanged (proxy-scoped)", stats.TotalAddresses)
	}
}

func toHexU(n uint64) string {
	const hexdigits = "0123456789abcdef"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{hexdigits[n%16]}, b...)
		n /= 16
	}
	return string(b)
}
