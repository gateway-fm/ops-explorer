package txflow

import (
	"context"
	"errors"
	"strings"
	"sync"

	"explorer/internal/types"
	"explorer/pkg/log"

	"golang.org/x/sync/errgroup"
)

// Source is the read-only data Load needs. api.DataProvider satisfies it.
type Source interface {
	GetTransaction(ctx context.Context, hash string) (*types.Transaction, error)
	GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error)
	GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error)
	GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error)
	GetContract(ctx context.Context, address string) (*types.Contract, error)
	GetToken(ctx context.Context, address string) (*types.Token, error)
}

// ErrNotFound means the transaction is not indexed. Load never indexes on demand.
var ErrNotFound = errors.New("transaction not found")

// maxLookups bounds the contract and token lookups for one flow, so a trace that
// touches hundreds of contracts costs a bounded number of reads. Addresses past the
// bound are shown by their short form.
const (
	maxLookups       = 64
	lookupConcurrent = 8
)

// Load reads a transaction's stored data and builds its flow. A missing transaction
// returns an error from the source or ErrNotFound. Internal calls that cannot be read
// degrade the flow to events and transfers instead of failing it.
func Load(ctx context.Context, src Source, hash string, opts Options) (*Flow, error) {
	tx, err := src.GetTransaction(ctx, hash)
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, ErrNotFound
	}

	in := Input{Tx: *tx}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var e error
		in.Internal, e = src.GetInternalTransactionsByTx(gctx, tx.Hash)
		if e != nil && gctx.Err() == nil {
			// Degrade to events and transfers, but leave the operator a trace of why.
			log.Warn("transaction flow: internal calls could not be read", "tx", tx.Hash, "error", e)
			in.InternalUnavailable, in.Internal = true, nil
			return nil
		}
		return e
	})
	g.Go(func() error {
		var e error
		in.Logs, e = src.GetLogsByTransaction(gctx, tx.Hash)
		return e
	})
	g.Go(func() error {
		var e error
		in.Transfers, e = src.GetTransfersByTransaction(gctx, tx.Hash)
		return e
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	in.Contracts, in.Tokens = lookupMetadata(ctx, src, in)
	return Build(in, opts), nil
}

// lookupMetadata fetches contract metadata for the addresses in the flow, and token
// metadata for token contracts. Lookup failures leave the address unnamed.
func lookupMetadata(ctx context.Context, src Source, in Input) (map[string]*types.Contract, map[string]*types.Token) {
	var addrs []string
	tokenAddrs := map[string]bool{}
	seen := map[string]bool{}
	add := func(a string, token bool) {
		k := strings.ToLower(a)
		if !isHexAddress(k) || k == zeroAddress {
			return
		}
		if token {
			tokenAddrs[k] = true
		}
		if !seen[k] {
			seen[k] = true
			addrs = append(addrs, k)
		}
	}
	// Token contracts first: their symbols name the most important steps.
	for _, t := range in.Transfers {
		add(t.TokenAddress, true)
	}
	if in.Tx.To != nil {
		add(*in.Tx.To, false)
	}
	for _, it := range in.Internal {
		if it.To != nil {
			add(*it.To, false)
		}
	}
	for _, l := range in.Logs {
		add(l.Address, false)
	}
	if len(addrs) > maxLookups {
		addrs = addrs[:maxLookups]
	}

	var mu sync.Mutex
	contracts := map[string]*types.Contract{}
	tokens := map[string]*types.Token{}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(lookupConcurrent)
	for _, a := range addrs {
		g.Go(func() error {
			c, err := src.GetContract(gctx, a)
			if err == nil && c != nil {
				mu.Lock()
				contracts[a] = c
				mu.Unlock()
			}
			if !tokenAddrs[a] {
				return nil
			}
			t, err := src.GetToken(gctx, a)
			if err == nil && t != nil {
				mu.Lock()
				tokens[a] = t
				mu.Unlock()
			}
			return nil
		})
	}
	_ = g.Wait()
	return contracts, tokens
}
