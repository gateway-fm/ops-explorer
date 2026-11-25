package cache

import (
	"context"
	"testing"

	"explorer/internal/api"
	"explorer/internal/types"
)

type contractFake struct {
	*api.DirectDBProvider
	calls    int
	contract *types.Contract
}

func (f *contractFake) GetContract(_ context.Context, _ string) (*types.Contract, error) {
	f.calls++
	return f.contract, nil
}

func TestInvalidateContractDropsCachedNotVerified(t *testing.T) {
	inner := &contractFake{}
	p := NewProvider(inner)
	ctx := context.Background()
	const checksummed = "0x6B7FED77A857E39510D19dD96a553B1387120D3E"

	got, err := p.GetContract(ctx, checksummed)
	if err != nil || got != nil {
		t.Fatalf("first GetContract = %v, %v; want nil, nil", got, err)
	}

	inner.contract = &types.Contract{IsVerified: true}
	if got, _ := p.GetContract(ctx, checksummed); got != nil {
		t.Fatalf("second GetContract should be served from cache, got %v", got)
	}
	if inner.calls != 1 {
		t.Fatalf("inner calls = %d, want 1 before invalidation", inner.calls)
	}

	p.InvalidateContract(checksummed)

	got, err = p.GetContract(ctx, checksummed)
	if err != nil || got == nil || !got.IsVerified {
		t.Fatalf("GetContract after invalidation = %v, %v; want verified contract", got, err)
	}
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2 after invalidation", inner.calls)
	}
}
