//go:build !privacy

package indexerclient

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
)

// BUG-4: GetOPDeposit tries the L2 hash, then falls back to the L1 hash. An
// OP deposit is OPTIONAL enrichment on a transaction. When the L2 lookup is
// NotFound and the L1 retry fails with a non-NotFound, non-Unavailable error
// (e.g. Internal), the provider returned that raw gRPC error. Propagated, that
// turns "this tx simply has no deposit" into a hard failure. The correct
// contract: a failed deposit lookup yields (nil, nil) — no deposit, no error.
func TestGetOPDeposit_L2NotFound_L1Internal_NoError(t *testing.T) {
	calls := 0
	p := setupProvider(t, &fakeIndexer{
		getOPDeposit: func(req *indexerv1.GetOPDepositRequest) (*indexerv1.OPDeposit, error) {
			calls++
			if req.GetL2TransactionHash() != "" {
				return nil, status.Error(codes.NotFound, "no L2 deposit")
			}
			// L1 retry fails with an unrelated Internal error.
			return nil, status.Error(codes.Internal, "indexer hiccup")
		},
	})

	dep, err := p.GetOPDeposit(context.Background(), "0xabc")
	if err != nil {
		t.Fatalf("GetOPDeposit returned error %v; a failed optional-deposit lookup must be (nil,nil)", err)
	}
	if dep != nil {
		t.Errorf("deposit = %+v, want nil", dep)
	}
	if calls != 2 {
		t.Errorf("expected L2 then L1 lookup (2 calls), got %d", calls)
	}
}

// Sanity: a genuine L2 hit still maps and returns the deposit.
func TestGetOPDeposit_L2Hit(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getOPDeposit: func(req *indexerv1.GetOPDepositRequest) (*indexerv1.OPDeposit, error) {
			return &indexerv1.OPDeposit{L2TransactionHash: "0xl2", L1TransactionHash: "0xl1", L1BlockNumber: 7}, nil
		},
	})
	dep, err := p.GetOPDeposit(context.Background(), "0xl2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dep == nil || dep.L1TxHash != "0xl1" || dep.L1BlockNumber != 7 {
		t.Errorf("got %+v", dep)
	}
}

// Both L2 and L1 NotFound -> (nil, nil).
func TestGetOPDeposit_BothNotFound(t *testing.T) {
	p := setupProvider(t, &fakeIndexer{
		getOPDeposit: func(req *indexerv1.GetOPDepositRequest) (*indexerv1.OPDeposit, error) {
			return nil, status.Error(codes.NotFound, "nope")
		},
	})
	dep, err := p.GetOPDeposit(context.Background(), "0xabc")
	if err != nil || dep != nil {
		t.Errorf("both-not-found: got dep=%+v err=%v, want nil/nil", dep, err)
	}
}
