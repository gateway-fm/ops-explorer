// Stage 2 handler overrides for block-explorer api. Ports the remaining
// DataProvider surface from SQL to chain-indexer gRPC. Methods not listed
// here still fall through to DirectDBProvider (e.g. ABI upload, indexer
// progress, node-RPC helpers like GetChainID/GetBalance/GetCode).
//
// Behavioral shifts vs SQL (same as privacy-proxy's indexerclient —
// documented inline; TODOs where a future indexer RPC would restore exact
// semantics):
//
//   - Offset totals on log / transfer / internal-tx list endpoints return
//     len(rows) instead of a DB-wide COUNT. UIs rendering "showing 50 of
//     12345" degrade to "showing 50 of 50". The indexer API doesn't expose
//     counts; adding a batch count RPC would fix it.
//   - GetTransactionHistory intervalSeconds quantizes to the indexer's
//     TimeBucket enum (HOUR / DAY / WEEK). Intermediate values round up.
//   - GetAllTransfers stays on SQL: the indexer requires a filter on
//     ListTokenTransfers by design; unfiltered global feed is not a
//     supported indexer use case.

package indexerclient

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
	"explorer/internal/types"
)

// isUnavailable returns true if err is a gRPC Unavailable — used to
// detect when the indexer reports a disabled feature (e.g. OP-Stack
// RPCs on a non-OP chain) and we should fall back to SQL.
func isUnavailable(err error) bool {
	return status.Code(err) == codes.Unavailable
}

// ----- Transactions: by-address cursor feed -----

func (p *Provider) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	req := &indexerv1.ListTransactionsRequest{
		Page: &indexerv1.PageRequest{PageSize: int32(limit)},
		Filter: &indexerv1.ListTransactionsRequest_ByAddress{
			ByAddress: &indexerv1.ListTransactionsRequest_AddressFilter{Address: address},
		},
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

// ----- Transactions: offset-paginated feeds -----
//
// The indexer doesn't expose offset pagination on ListTransactions; we
// translate by fetching page*pageSize rows cursor-style and slicing. For
// deep pages this is less efficient than the SQL OFFSET/LIMIT path; the
// first several pages (the common case in the UI) are fine.

func (p *Provider) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	if page < 1 {
		page = 1
	}
	fetchLimit := int32(page * pageSize)
	resp, err := p.client.ListTransactions(ctx, &indexerv1.ListTransactionsRequest{
		Page: &indexerv1.PageRequest{PageSize: fetchLimit},
	})
	if err != nil {
		return nil, 0, err
	}
	all := mapTransactions(resp.GetTransactions())
	start := (page - 1) * pageSize
	if start >= len(all) {
		return nil, int64(len(all)), nil
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], int64(len(all)), nil
}

func (p *Provider) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	// Same gRPC call — categories always present on proto Transaction.
	return p.GetTransactionsPaginated(ctx, page, pageSize)
}

// ----- Logs -----

func (p *Provider) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	resp, err := p.client.ListLogs(ctx, &indexerv1.ListLogsRequest{ByTxHash: txHash})
	if err != nil {
		return nil, err
	}
	return mapLogs(resp.GetLogs()), nil
}

func (p *Provider) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	// TODO: indexer ListLogs uses cursor pagination, not offset. For the
	// first page (offset=0) we return results normally; for deeper pages
	// consumers should switch to the cursor path on ListLogs if available.
	// Total count is page-local (len(rows)) — see file-level comment.
	_ = offset
	resp, err := p.client.ListLogs(ctx, &indexerv1.ListLogsRequest{
		ByAddress: address,
		Page:      &indexerv1.PageRequest{PageSize: int32(limit)},
	})
	if err != nil {
		return nil, 0, err
	}
	logs := mapLogs(resp.GetLogs())
	return logs, int64(len(logs)), nil
}

func (p *Provider) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	req := &indexerv1.ListLogsRequest{
		Page: &indexerv1.PageRequest{PageSize: int32(limit)},
	}
	if address != nil {
		req.ByAddress = *address
	}
	if topic0 != nil {
		req.Topic0 = *topic0
	}
	if fromBlock != nil || toBlock != nil {
		br := &indexerv1.BlockRange{}
		if fromBlock != nil {
			br.FromBlock = *fromBlock
		}
		if toBlock != nil {
			br.ToBlock = *toBlock
		}
		req.BlockRange = br
	}
	resp, err := p.client.ListLogs(ctx, req)
	if err != nil {
		return nil, err
	}
	return mapLogs(resp.GetLogs()), nil
}

// ----- Token transfers -----

func (p *Provider) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	resp, err := p.client.ListTokenTransfers(ctx, &indexerv1.ListTokenTransfersRequest{ByTxHash: txHash})
	if err != nil {
		return nil, err
	}
	return mapTokenTransfers(resp.GetTransfers()), nil
}

func (p *Provider) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	req := &indexerv1.ListTokenTransfersRequest{
		ByAddress: address,
		Page:      &indexerv1.PageRequest{PageSize: int32(limit)},
	}
	if beforeBlock != nil {
		req.BlockRange = &indexerv1.BlockRange{ToBlock: *beforeBlock}
	}
	resp, err := p.client.ListTokenTransfers(ctx, req)
	if err != nil {
		return nil, err
	}
	return mapTokenTransfers(resp.GetTransfers()), nil
}

func (p *Provider) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	_ = offset // page-local total; see file header
	resp, err := p.client.ListTokenTransfers(ctx, &indexerv1.ListTokenTransfersRequest{
		ByToken: tokenAddress,
		Page:    &indexerv1.PageRequest{PageSize: int32(limit)},
	})
	if err != nil {
		return nil, 0, err
	}
	transfers := mapTokenTransfers(resp.GetTransfers())
	return transfers, int64(len(transfers)), nil
}

// GetAllTransfers intentionally NOT overridden — falls through to
// *DirectDBProvider. The indexer's ListTokenTransfers requires at least
// one filter; unfiltered global transfer feed is not an indexer-supported
// use case. Admin-style path; OK to keep on SQL.

// ----- Internal transactions -----

func (p *Provider) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	resp, err := p.client.ListInternalTransactions(ctx, &indexerv1.ListInternalTransactionsRequest{
		ByTxHash: txHash,
	})
	if err != nil {
		return nil, err
	}
	return mapInternalTxs(resp.GetInternalTransactions()), nil
}

func (p *Provider) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	_ = offset
	resp, err := p.client.ListInternalTransactions(ctx, &indexerv1.ListInternalTransactionsRequest{
		ByAddress: address,
		Page:      &indexerv1.PageRequest{PageSize: int32(limit)},
	})
	if err != nil {
		return nil, 0, err
	}
	rows := mapInternalTxs(resp.GetInternalTransactions())
	return rows, int64(len(rows)), nil
}

func (p *Provider) GetInternalTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.InternalTransaction, error) {
	resp, err := p.client.ListInternalTransactions(ctx, &indexerv1.ListInternalTransactionsRequest{
		ByBlock: &indexerv1.ListInternalTransactionsRequest_BlockFilter{
			Selector: &indexerv1.ListInternalTransactionsRequest_BlockFilter_Number{Number: blockNumber},
		},
	})
	if err != nil {
		return nil, err
	}
	return mapInternalTxs(resp.GetInternalTransactions()), nil
}

// ----- Tokens -----

func (p *Provider) GetToken(ctx context.Context, address string) (*types.Token, error) {
	resp, err := p.client.GetToken(ctx, &indexerv1.GetTokenRequest{Address: address})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return mapToken(resp), nil
}

func (p *Provider) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	page := offset/limit + 1
	var tt indexerv1.TokenType
	switch tokenType {
	case "ERC20":
		tt = indexerv1.TokenType_TOKEN_TYPE_ERC20
	case "ERC721":
		tt = indexerv1.TokenType_TOKEN_TYPE_ERC721
	case "ERC1155":
		tt = indexerv1.TokenType_TOKEN_TYPE_ERC1155
	}
	resp, err := p.client.ListTokens(ctx, &indexerv1.ListTokensRequest{
		Page:      &indexerv1.OffsetPageRequest{Page: int32(page), PageSize: int32(limit)},
		TokenType: tt,
	})
	if err != nil {
		return nil, 0, err
	}
	return mapTokens(resp.GetTokens()), resp.GetPage().GetTotalItems(), nil
}

func (p *Provider) GetTokenHolders(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	page := offset/limit + 1
	resp, err := p.client.ListTokenHolders(ctx, &indexerv1.ListTokenHoldersRequest{
		TokenAddress: tokenAddress,
		Page:         &indexerv1.OffsetPageRequest{Page: int32(page), PageSize: int32(limit)},
	})
	if err != nil {
		return nil, 0, err
	}
	return mapTokenHolders(resp.GetHolders()), resp.GetPage().GetTotalItems(), nil
}

func (p *Provider) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	resp, err := p.client.ListTokenBalances(ctx, &indexerv1.ListTokenBalancesRequest{Address: address})
	if err != nil {
		return nil, err
	}
	return mapBalances(resp.GetBalances()), nil
}

// ----- Contracts -----

func (p *Provider) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	// Returns chain-facts only (address / bytecode / deployer / creation
	// tx / deployment block). If the caller needs ABI / verification
	// metadata it must merge from the SQL-backed GetContract —
	// DirectDBProvider. In standalone deployments this override can be
	// disabled (comment it out) to get the full SQL path.
	resp, err := p.client.GetContract(ctx, &indexerv1.GetContractRequest{Address: address})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return mapContract(resp), nil
}

func (p *Provider) IsContract(ctx context.Context, address string) (bool, error) {
	addr, err := p.client.GetAddress(ctx, &indexerv1.GetAddressRequest{Address: address})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return addr.GetIsContract(), nil
}

// ----- Accounts / search / history / daily -----

func (p *Provider) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	resp, err := p.client.ListAddresses(ctx, &indexerv1.ListAddressesRequest{
		Page: &indexerv1.OffsetPageRequest{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		return nil, 0, err
	}
	return mapAccounts(resp.GetAddresses()), resp.GetPage().GetTotalItems(), nil
}

func (p *Provider) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	resp, err := p.client.Search(ctx, &indexerv1.SearchRequest{
		Query:        query,
		LimitPerKind: uint32(limit),
	})
	if err != nil {
		return nil, err
	}
	return mapSearchResults(resp), nil
}

func (p *Provider) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	bucket := indexerv1.TimeBucket_TIME_BUCKET_HOUR
	switch {
	case intervalSeconds >= 86400*7:
		bucket = indexerv1.TimeBucket_TIME_BUCKET_WEEK
	case intervalSeconds >= 86400:
		bucket = indexerv1.TimeBucket_TIME_BUCKET_DAY
	}
	resp, err := p.client.GetTransactionHistory(ctx, &indexerv1.GetTransactionHistoryRequest{
		Bucket: bucket,
	})
	if err != nil {
		return nil, err
	}
	pts := mapTxHistory(resp)
	if limit > 0 && len(pts) > limit {
		pts = pts[:limit]
	}
	return pts, nil
}

func (p *Provider) GetDailyStats(ctx context.Context, from, to time.Time) ([]types.DailyStats, error) {
	resp, err := p.client.GetDailyStats(ctx, &indexerv1.GetDailyStatsRequest{
		FromDate: from.Format("2006-01-02"),
		ToDate:   to.Format("2006-01-02"),
	})
	if err != nil {
		return nil, err
	}
	return mapDailyStatsList(resp.GetDays()), nil
}

// ----- OP Deposit -----

func (p *Provider) GetOPDeposit(ctx context.Context, txHash string) (*types.OPDeposit, error) {
	// DataProvider takes a single hash; indexer accepts an L1 or L2 hash
	// via oneof. Try L2 first (block-explorer's original lookup key), fall
	// back to L1.
	resp, err := p.client.GetOPDeposit(ctx, &indexerv1.GetOPDepositRequest{
		Selector: &indexerv1.GetOPDepositRequest_L2TransactionHash{L2TransactionHash: txHash},
	})
	if err == nil {
		return mapOPDeposit(resp), nil
	}
	if !isNotFound(err) {
		// If the indexer isn't in OP-Stack mode it returns Unavailable —
		// fall through to SQL.
		if isUnavailable(err) {
			return p.DirectDBProvider.GetOPDeposit(ctx, txHash)
		}
		return nil, err
	}
	// L2 not found — try L1.
	resp, err = p.client.GetOPDeposit(ctx, &indexerv1.GetOPDepositRequest{
		Selector: &indexerv1.GetOPDepositRequest_L1TransactionHash{L1TransactionHash: txHash},
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		if isUnavailable(err) {
			return p.DirectDBProvider.GetOPDeposit(ctx, txHash)
		}
		return nil, err
	}
	return mapOPDeposit(resp), nil
}
