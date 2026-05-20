package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"explorer/internal/api"
	"explorer/internal/types"
)

// SECURITY INVARIANT: cache keys are derived purely from method arguments —
// request context (auth identity, scope) is NOT part of the key. This is
// correct iff the wrapped provider returns the same value for the same
// arguments regardless of caller. Holds for indexerclient.Provider and
// DirectDBProvider. Does NOT hold for ProxyDataProvider (privacy mode
// applies per-caller RBAC redaction). cmd/api/provider_standalone.go
// enforces the wiring; do not wrap a caller-scoped provider here.
type CachingProvider struct {
	api.DataProvider

	chainStats *TTLCache[*types.ChainStats]
	blocks     *TTLCache[[]types.Block]
	txs        *TTLCache[[]types.Transaction]
	history    *TTLCache[[]types.TxHistoryPoint]
	contract   *TTLCache[*types.Contract]
	gas        *TTLCache[gasResult]
}

type gasResult struct {
	Slow, Normal, Fast, BaseFee *uint64
}

func NewProvider(inner api.DataProvider) *CachingProvider {
	return &CachingProvider{
		DataProvider: inner,
		chainStats:   New[*types.ChainStats](16, 2*time.Second),
		blocks:       New[[]types.Block](16, 1*time.Second),
		txs:          New[[]types.Transaction](16, 1*time.Second),
		history:      New[[]types.TxHistoryPoint](64, 15*time.Second),
		contract:     New[*types.Contract](10_000, 5*time.Minute),
		gas:          New[gasResult](8, 3*time.Second),
	}
}

func (p *CachingProvider) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	return p.chainStats.Get("stats", func() (*types.ChainStats, error) {
		return p.DataProvider.GetChainStats(ctx)
	})
}

func (p *CachingProvider) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	if beforeBlock != nil {
		return p.DataProvider.GetBlocks(ctx, limit, beforeBlock)
	}
	key := fmt.Sprintf("blocks:%d", limit)
	return p.blocks.Get(key, func() ([]types.Block, error) {
		return p.DataProvider.GetBlocks(ctx, limit, nil)
	})
}

func (p *CachingProvider) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	if beforeBlock != nil {
		return p.DataProvider.GetTransactionsWithCategories(ctx, limit, beforeBlock)
	}
	key := fmt.Sprintf("txs-cat:%d", limit)
	return p.txs.Get(key, func() ([]types.Transaction, error) {
		return p.DataProvider.GetTransactionsWithCategories(ctx, limit, nil)
	})
}

func (p *CachingProvider) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	key := fmt.Sprintf("history:%d:%d", intervalSeconds, limit)
	return p.history.Get(key, func() ([]types.TxHistoryPoint, error) {
		return p.DataProvider.GetTransactionHistory(ctx, intervalSeconds, limit)
	})
}

func (p *CachingProvider) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	key := strings.ToLower(address)
	return p.contract.Get(key, func() (*types.Contract, error) {
		return p.DataProvider.GetContract(ctx, address)
	})
}

func (p *CachingProvider) GetGasPrices(ctx context.Context, numBlocks int) (*uint64, *uint64, *uint64, *uint64, error) {
	key := fmt.Sprintf("gas:%d", numBlocks)
	r, err := p.gas.Get(key, func() (gasResult, error) {
		slow, normal, fast, baseFee, err := p.DataProvider.GetGasPrices(ctx, numBlocks)
		if err != nil {
			return gasResult{}, err
		}
		return gasResult{Slow: slow, Normal: normal, Fast: fast, BaseFee: baseFee}, nil
	})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return r.Slow, r.Normal, r.Fast, r.BaseFee, nil
}

var _ api.DataProvider = (*CachingProvider)(nil)
