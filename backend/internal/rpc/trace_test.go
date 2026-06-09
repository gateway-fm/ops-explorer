package rpc

// Contract under test: FlattenCallFrame / flattenCallFrameRecursive (trace.go).
// The shape is derived from the geth debug_traceTransaction callTracer schema:
// the ROOT frame is the transaction's own top-level call, and the nested
// frame.Calls are the INTERNAL transactions an explorer surfaces (matching the
// Etherscan "Internal Txns" convention). So:
//   - a frame with no children flattens to 0 internal txs (root is skipped);
//   - children get 0-based, comma-joined TraceAddress paths ("0", "1", "0,0");
//   - callType is lower-cased and normalized to call/delegatecall/staticcall/
//     create/create2, unknown -> "call";
//   - the per-child path is a defensive COPY — sibling subtrees must not see
//     each other's indices (slice-aliasing guard).

import (
	"math/big"
	"testing"

	"explorer/pkg/eth/hexutil"
)

func bigHex(n int64) *hexutil.Big {
	b := hexutil.Big(*big.NewInt(n))
	return &b
}

// rootOnly is a single top-level call with no sub-calls.
func rootOnly() *CallFrame {
	return &CallFrame{Type: "CALL", From: "0xroot", To: "0xdest", Value: bigHex(5)}
}

func TestFlattenCallFrame_RootSkipped(t *testing.T) {
	// A transaction whose trace is just the top-level call has NO internal
	// transactions — the root is the tx itself, not an internal tx.
	got := FlattenCallFrame(rootOnly(), "0xtx", 10, nil)
	if len(got) != 0 {
		t.Fatalf("root-only frame -> %d internal txs, want 0 (root must be skipped)", len(got))
	}
}

func TestFlattenCallFrame_ExactlyOneChild(t *testing.T) {
	frame := &CallFrame{
		Type: "CALL", From: "0xroot", To: "0xa",
		Calls: []CallFrame{
			{Type: "STATICCALL", From: "0xa", To: "0xb", Value: bigHex(0)},
		},
	}
	got := FlattenCallFrame(frame, "0xtx", 10, nil)
	if len(got) != 1 {
		t.Fatalf("root + 1 child -> %d internal txs, want exactly 1 (off-by-one guard)", len(got))
	}
	if got[0].TraceAddress != "0" {
		t.Errorf("single child TraceAddress = %q, want \"0\"", got[0].TraceAddress)
	}
	if got[0].CallType != "staticcall" {
		t.Errorf("CallType = %q, want staticcall", got[0].CallType)
	}
	if got[0].From != "0xa" || got[0].To == nil || *got[0].To != "0xb" {
		t.Errorf("from/to = %q/%v", got[0].From, got[0].To)
	}
}

func TestFlattenCallFrame_TraceAddressPaths(t *testing.T) {
	// root
	//  ├─ 0  ── 0,0
	//  └─ 1
	frame := &CallFrame{
		Type: "CALL", From: "0xroot",
		Calls: []CallFrame{
			{
				Type: "CALL", From: "0x0",
				Calls: []CallFrame{
					{Type: "DELEGATECALL", From: "0x00"},
				},
			},
			{Type: "CALL", From: "0x1"},
		},
	}
	got := FlattenCallFrame(frame, "0xtx", 1, nil)
	gotPaths := make([]string, len(got))
	for i, tx := range got {
		gotPaths[i] = tx.TraceAddress
	}
	// Pre-order: child0, child0.0, child1.
	want := []string{"0", "0,0", "1"}
	if len(gotPaths) != len(want) {
		t.Fatalf("paths = %v, want %v", gotPaths, want)
	}
	for i := range want {
		if gotPaths[i] != want[i] {
			t.Errorf("path[%d] = %q, want %q (full set %v)", i, gotPaths[i], want[i], gotPaths)
		}
	}
}

// TestFlattenCallFrame_NoSiblingAliasing is the slice-aliasing guard. With two
// sibling subtrees that each have a deep child, an aliasing append() bug would
// let the second sibling's index bleed into the first's descendant path. Lock
// the defensive copy so a future append(path, i) "optimization" can't corrupt
// siblings.
func TestFlattenCallFrame_NoSiblingAliasing(t *testing.T) {
	frame := &CallFrame{
		Type: "CALL", From: "0xroot",
		Calls: []CallFrame{
			{Type: "CALL", From: "0x0", Calls: []CallFrame{{Type: "CALL", From: "0x00"}}},
			{Type: "CALL", From: "0x1", Calls: []CallFrame{{Type: "CALL", From: "0x10"}}},
		},
	}
	got := FlattenCallFrame(frame, "0xtx", 1, nil)
	paths := map[string]bool{}
	for _, tx := range got {
		if paths[tx.TraceAddress] {
			t.Errorf("duplicate TraceAddress %q — sibling subtrees aliased the path slice", tx.TraceAddress)
		}
		paths[tx.TraceAddress] = true
	}
	for _, want := range []string{"0", "0,0", "1", "1,0"} {
		if !paths[want] {
			t.Errorf("missing TraceAddress %q (have %v)", want, keys(paths))
		}
	}
}

func TestFlattenCallFrame_CallTypeNormalization(t *testing.T) {
	cases := map[string]string{
		"CALL":         "call",
		"DelegateCall": "delegatecall",
		"STATICCALL":   "staticcall",
		"CREATE":       "create",
		"CREATE2":      "create2",
		"WEIRDOP":      "call", // unknown -> call
		"":             "call",
	}
	for raw, want := range cases {
		frame := &CallFrame{Type: "CALL", From: "0xroot", Calls: []CallFrame{{Type: raw, From: "0xc"}}}
		got := FlattenCallFrame(frame, "0xtx", 1, nil)
		if len(got) != 1 {
			t.Fatalf("type %q: got %d txs", raw, len(got))
		}
		if got[0].CallType != want {
			t.Errorf("type %q normalized to %q, want %q", raw, got[0].CallType, want)
		}
	}
}

func TestFlattenCallFrame_ValueAndOptionalFields(t *testing.T) {
	frame := &CallFrame{
		Type: "CALL", From: "0xroot",
		Calls: []CallFrame{
			{
				Type: "CALL", From: "0xa", To: "0xb",
				Value:   bigHex(1234),
				Gas:     hexutil.Uint64(21000),
				GasUsed: hexutil.Uint64(20000),
				Input:   "0xabcd",
				Output:  "0x",  // "0x" -> nil
				Error:   "reverted",
			},
		},
	}
	got := FlattenCallFrame(frame, "0xtx", 1, nil)
	tx := got[0]
	if string(tx.Value) != "1234" {
		t.Errorf("Value = %q, want 1234", tx.Value)
	}
	if tx.Gas == nil || *tx.Gas != 21000 || tx.GasUsed == nil || *tx.GasUsed != 20000 {
		t.Errorf("gas/gasUsed = %v/%v", tx.Gas, tx.GasUsed)
	}
	if tx.Input == nil || *tx.Input != "0xabcd" {
		t.Errorf("Input = %v", tx.Input)
	}
	if tx.Output != nil {
		t.Errorf("Output = %v, want nil for \"0x\"", *tx.Output)
	}
	if tx.Error == nil || *tx.Error != "reverted" {
		t.Errorf("Error = %v", tx.Error)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
