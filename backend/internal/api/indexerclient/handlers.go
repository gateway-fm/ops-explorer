//go:build !privacy

package indexerclient

import (
	"context"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
	"explorer/internal/types"
)

// Ported methods. Each overrides the corresponding embedded
// *api.DirectDBProvider method via Go method shadowing.
//
// Methods NOT listed here keep falling through to DirectDBProvider (SQL).
// Cutover is staged — add entries here over time.

// ----- Blocks -----

func (p *Provider) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	resp, err := p.client.GetBlock(ctx, &indexerv1.GetBlockRequest{
		Selector: &indexerv1.GetBlockRequest_Number{Number: number},
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return mapBlock(resp), nil
}

func (p *Provider) GetBlockByHash(ctx context.Context, hash string) (*types.Block, error) {
	resp, err := p.client.GetBlock(ctx, &indexerv1.GetBlockRequest{
		Selector: &indexerv1.GetBlockRequest_Hash{Hash: hash},
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return mapBlock(resp), nil
}

func (p *Provider) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	resp, err := p.client.GetLatestBlockNumber(ctx, &indexerv1.Empty{})
	if err != nil {
		return 0, err
	}
	return resp.GetNumber(), nil
}

func (p *Provider) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	req := &indexerv1.ListBlocksRequest{
		Page: &indexerv1.PageRequest{PageSize: int32(limit)},
	}
	if beforeBlock != nil {
		req.BeforeNumber = *beforeBlock
	}
	resp, err := p.client.ListBlocks(ctx, req)
	if err != nil {
		return nil, err
	}
	return mapBlocks(resp.GetBlocks()), nil
}

// ----- Transactions -----

func (p *Provider) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	resp, err := p.client.GetTransaction(ctx, &indexerv1.GetTransactionRequest{Hash: hash})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return mapTransaction(resp), nil
}

// GetTransactionWithCategories mirrors GetTransaction on the indexer —
// the proto always carries categories (materialized column from
// chain-indexer migration 003).
func (p *Provider) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	return p.GetTransaction(ctx, hash)
}

func (p *Provider) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	req := &indexerv1.ListTransactionsRequest{
		Page: &indexerv1.PageRequest{PageSize: int32(limit)},
	}
	if beforeBlock != nil {
		req.BlockRange = &indexerv1.BlockRange{ToBlock: *beforeBlock}
	}
	resp, err := p.client.ListTransactions(ctx, req)
	if err != nil {
		return nil, err
	}
	return mapTransactions(resp.GetTransactions()), nil
}

// GetTransactionsWithCategories shares the same gRPC call.
func (p *Provider) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	return p.GetTransactions(ctx, limit, beforeBlock)
}

func (p *Provider) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	req := &indexerv1.ListTransactionsRequest{
		Filter: &indexerv1.ListTransactionsRequest_ByBlock{
			ByBlock: &indexerv1.ListTransactionsRequest_BlockFilter{
				Selector: &indexerv1.ListTransactionsRequest_BlockFilter_Number{Number: blockNumber},
			},
		},
	}
	resp, err := p.client.ListTransactions(ctx, req)
	if err != nil {
		return nil, err
	}
	return mapTransactions(resp.GetTransactions()), nil
}

// ----- Address -----

func (p *Provider) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	resp, err := p.client.GetAddress(ctx, &indexerv1.GetAddressRequest{Address: address})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return mapAddressStats(resp), nil
}

// ----- Stats / sync -----

func (p *Provider) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	resp, err := p.client.GetChainStats(ctx, &indexerv1.Empty{})
	if err != nil {
		return nil, err
	}
	return mapChainStats(resp), nil
}

func (p *Provider) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	resp, err := p.client.GetSyncStatus(ctx, &indexerv1.Empty{})
	if err != nil {
		return nil, err
	}
	return mapSyncStatus(resp), nil
}

// Suppress unused-import warning when no handler references strPtrOrNil.
var _ = strPtrOrNil
