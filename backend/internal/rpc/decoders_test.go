package rpc

// BUG-5: parseStringResult decodes an ABI-encoded string (offset, length, bytes)
// from UNTRUSTED contract return data (token name/symbol). offset+32+length is
// computed in uint64 and can WRAP past the length guard for a crafted return,
// then used to slice -> panic (a DoS via any token the explorer renders).
//
// BUG-6: ERC-20 decimals were decoded as int(big.Int.Int64()) from untrusted
// data — a value above MaxInt64 wraps to a negative/absurd int, corrupting every
// downstream value-formatting computation. Decimals must clamp to a sane range.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"explorer/pkg/eth/common"
)

// dcClient routes JSON-RPC by method to a canned raw result, or a JSON-RPC error
// when errFor[method] is set. Used for the multi-call decoder contracts.
func dcClient(t *testing.T, resultFor, errFor map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &env)
		w.Header().Set("Content-Type", "application/json")
		id := itoa(env.ID)
		if msg, ok := errFor[env.Method]; ok {
			emsg, _ := json.Marshal(msg)
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+id+`,"error":{"code":-32000,"message":`+string(emsg)+`}}`)
			return
		}
		res, ok := resultFor[env.Method]
		if !ok {
			res = "null"
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+id+`,"result":`+res+`}`)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("rpc.New: %v", err)
	}
	return c
}

// abiString encodes s as a single dynamic ABI string return (head offset 0x20,
// then length, then right-padded bytes) — the well-formed shape.
func abiString(s string) []byte {
	out := make([]byte, 0, 64+len(s))
	off := make([]byte, 32)
	off[31] = 0x20
	out = append(out, off...)
	length := make([]byte, 32)
	// length in the last 8 bytes is enough for test sizes.
	n := len(s)
	for i := 0; i < 8; i++ {
		length[31-i] = byte(n >> (8 * i))
	}
	out = append(out, length...)
	body := make([]byte, ((len(s)+31)/32)*32)
	copy(body, s)
	out = append(out, body...)
	return out
}

func TestParseStringResult_WellFormed(t *testing.T) {
	if got := parseStringResult(abiString("USD Coin")); got != "USD Coin" {
		t.Errorf("parseStringResult = %q, want %q", got, "USD Coin")
	}
	// Short (<64 bytes) fixed-bytes32 style string with NUL padding.
	short := make([]byte, 32)
	copy(short, "TOK")
	if got := parseStringResult(short); got != "TOK" {
		t.Errorf("short string = %q, want TOK", got)
	}
}

func TestParseStringResult_MaliciousNoPanicEmpty(t *testing.T) {
	mk := func(fill func([]byte)) []byte {
		b := make([]byte, 64)
		fill(b)
		return b
	}
	cases := map[string][]byte{
		// offset = 0xffff...ff (huge) -> caught by offset>=len guard.
		"huge-offset": mk(func(b []byte) {
			for i := 0; i < 32; i++ {
				b[i] = 0xff
			}
		}),
		// offset = 0x20 (valid), length = 0xffff...ff (huge). offset+32+length
		// overflows uint64 and would wrap past the guard -> slice panic.
		"huge-length": mk(func(b []byte) {
			b[31] = 0x20
			for i := 32; i < 64; i++ {
				b[i] = 0xff
			}
		}),
		// offset just below len, length crafted so offset+32+length wraps to a
		// small in-range value (the exact wraparound the guard must resist).
		"wraparound": func() []byte {
			b := make([]byte, 96)
			b[31] = 0x40 // offset = 64, points at the length word
			// length = 2^64 - 32 so that 64+32+length == 2^64 == 0 (wraps).
			for i := 0; i < 32; i++ {
				b[64+i] = 0xff
			}
			b[64+31] = 0xe0 // ...ffe0 = 2^64 - 32
			return b
		}(),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parseStringResult panicked on %s input: %v", name, r)
				}
			}()
			if got := parseStringResult(data); got != "" {
				t.Errorf("%s: parseStringResult = %q, want \"\" (reject malformed)", name, got)
			}
		})
	}
}

func FuzzParseStringResult(f *testing.F) {
	f.Add(abiString("hello"))
	f.Add(make([]byte, 64))
	f.Add([]byte{0x01, 0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		// Contract: never panics, regardless of input. Result is unconstrained.
		_ = parseStringResult(data)
	})
}

// --- BUG-6: decimals clamping ----------------------------------------------

func TestParseDecimals_Clamp(t *testing.T) {
	// 32-byte big-endian encodings.
	word := func(set func([]byte)) []byte {
		b := make([]byte, 32)
		set(b)
		return b
	}
	cases := []struct {
		name string
		data []byte
		want int
	}{
		{"normal-18", word(func(b []byte) { b[31] = 18 }), 18},
		{"zero", word(func(b []byte) {}), 0},
		{"six", word(func(b []byte) { b[31] = 6 }), 6},
		{"too-short-default-18", []byte{0x01}, 18},
		// All-0xff -> huge/negative via Int64(); must clamp to fallback 18, not
		// emit a negative or absurd value.
		{"all-ff-fallback", word(func(b []byte) {
			for i := range b {
				b[i] = 0xff
			}
		}), 18},
		// A value above 255 (256) is not a sane token decimals -> fallback 18.
		{"too-large-256", word(func(b []byte) { b[30] = 0x01 }), 18},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDecimals(tc.data)
			if got != tc.want {
				t.Errorf("parseDecimals(%x...) = %d, want %d", firstBytes(tc.data), got, tc.want)
			}
			if got < 0 || got > 255 {
				t.Errorf("parseDecimals = %d, must be a sane 0..255 (BUG-6)", got)
			}
		})
	}
}

func firstBytes(b []byte) []byte {
	if len(b) > 4 {
		return b[:4]
	}
	return b
}

// guard: confirm the well-formed helper itself is sane (defensive against a
// silently-broken abiString that would make the malicious cases vacuous).
func TestAbiStringHelperRoundTrips(t *testing.T) {
	if !strings.HasPrefix(string(abiString("x")[64:]), "x") {
		t.Fatal("abiString helper is broken; malicious-input tests would be vacuous")
	}
}

// --- GetTransactionByHash: receipt-nil optimistic fallback (plan §3) --------

func TestGetTransactionByHash_ReceiptNil_OptimisticStatus(t *testing.T) {
	// When the receipt fetch fails, GetTransactionByHash falls back optimistically
	// to status=1 and gasUsed=tx.Gas (it must NOT error or report status 0).
	hash := common.HexToHash("0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b")
	txObj := `{
		"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		"blockNumber":"0x10","from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
		"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb","value":"0x0",
		"gas":"0xc350","gasPrice":"0x4a817c800","input":"0x","nonce":"0x1","type":"0x0"
	}`
	c := dcClient(t,
		map[string]string{"eth_getTransactionByHash": txObj},
		// receipt call errors -> receipt nil path.
		map[string]string{"eth_getTransactionReceipt": "boom"},
	)
	tx, err := c.GetTransactionByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash: %v (receipt failure must not error the lookup)", err)
	}
	if tx == nil {
		t.Fatal("nil tx")
	}
	if tx.Status != 1 {
		t.Errorf("Status = %d, want 1 (optimistic fallback when receipt is nil)", tx.Status)
	}
	if tx.GasUsed != 0xc350 {
		t.Errorf("GasUsed = %d, want tx.Gas 50000 (receipt-nil fallback)", tx.GasUsed)
	}
}

func TestGetTransactionByHash_WithReceipt(t *testing.T) {
	hash := common.HexToHash("0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b")
	txObj := `{"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		"blockNumber":"0x10","from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
		"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb","value":"0x0","gas":"0xc350",
		"gasPrice":"0x1","input":"0x","nonce":"0x1","type":"0x0"}`
	rcpt := `{"transactionHash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		"blockNumber":"0x10","transactionIndex":"0x2","gasUsed":"0x5208","status":"0x0","logs":[]}`
	c := dcClient(t, map[string]string{
		"eth_getTransactionByHash": txObj,
		"eth_getTransactionReceipt": rcpt,
	}, nil)
	tx, err := c.GetTransactionByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Receipt present: status mirrors the receipt (0x0 -> 0), gasUsed from receipt.
	if tx.Status != 0 {
		t.Errorf("Status = %d, want 0 (from receipt status 0x0)", tx.Status)
	}
	if tx.GasUsed != 0x5208 {
		t.Errorf("GasUsed = %d, want 21000 (from receipt)", tx.GasUsed)
	}
}

// --- CheckTracingSupport: error classification (plan §3, lock the surprise) --

func TestCheckTracingSupport_Classification(t *testing.T) {
	cases := []struct {
		name   string
		errMsg string // "" => success
		want   bool
	}{
		{"supported", "", true},
		{"method-not-found", "the method debug_traceBlockByNumber does not exist/is not available", false},
		{"not-supported", "tracing is not supported by this node", false},
		{"does-not-exist", "method does not exist", false},
		// SURPRISING (locked): an UNKNOWN error returns true (assume supported).
		{"unknown-error-assumes-supported", "some transient upstream hiccup", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errFor map[string]string
			results := map[string]string{"debug_traceBlockByNumber": "[]"}
			if tc.errMsg != "" {
				errFor = map[string]string{"debug_traceBlockByNumber": tc.errMsg}
			}
			c := dcClient(t, results, errFor)
			got, err := c.CheckTracingSupport(context.Background())
			if err != nil {
				t.Fatalf("CheckTracingSupport returned err: %v (it classifies, never errors)", err)
			}
			if got != tc.want {
				t.Errorf("CheckTracingSupport(%q) = %v, want %v", tc.errMsg, got, tc.want)
			}
		})
	}
}
