//go:build !privacy

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

// offsetPage converts an (offset, limit) pair into the 1-based page number the
// indexer's offset-paginated RPCs expect. It guards a zero/negative limit (which
// would panic on the offset/limit division) and clamps the page so the
// subsequent int32(page) cast can't overflow on a pathologically large offset.
// Returns the page and the normalised limit (also used for PageSize).
func offsetPage(offset, limit int) (page, normLimit int) {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	page = offset/limit + 1
	const maxPage = 1 << 30 // keep int32(page) safely in range
	if page > maxPage {
		page = maxPage
	}
	return page, limit
}

// ----- Transactions: by-address cursor feed -----

func (p *Provider) GetTransactionsByAddress(ctx context.Context, address string, limit int, cursor string, beforeBlock *uint64) ([]types.Transaction, string, error) {
	// cursor (opaque keyset) takes precedence over the legacy block-exclusive
	// beforeBlock bound; the indexer applies whichever is set (RD-1149).
	req := &indexerv1.ListTransactionsRequest{
		Page: &indexerv1.PageRequest{PageSize: int32(limit), Cursor: cursor},
		Filter: &indexerv1.ListTransactionsRequest_ByAddress{
			ByAddress: &indexerv1.ListTransactionsRequest_AddressFilter{Address: address},
		},
	}
	if cursor == "" && beforeBlock != nil {
		req.BlockRange = &indexerv1.BlockRange{ToBlock: *beforeBlock}
	}
	resp, err := p.client.ListTransactions(ctx, req)
	if err != nil {
		return nil, "", err
	}
	return mapTransactions(resp.GetTransactions()), resp.GetPage().GetNextCursor(), nil
}

// ----- Transactions: offset-paginated feeds -----
//
// Uses the indexer's offset-paginated ListTransactionsPaginated RPC, which
// returns a true chain-wide total in OffsetPageResponse.total_items (RD-1061).
// Previously this fell back to the cursor-based ListTransactions and returned
// len(fetched) as the total — a page-local value that grew as the client
// paginated (e.g. 47 on page 1, 70 on page 2), inflating the computed page
// count. Mirrors GetAccountsPaginated / GetTokens, which already ride
// OffsetPageResponse.

func (p *Provider) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	resp, err := p.client.ListTransactionsPaginated(ctx, &indexerv1.ListTransactionsPaginatedRequest{
		Page: &indexerv1.OffsetPageRequest{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		return nil, 0, err
	}
	return mapTransactions(resp.GetTransactions()), resp.GetPage().GetTotalItems(), nil
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

func (p *Provider) GetTransfersByAddress(ctx context.Context, address string, limit int, cursor string, beforeBlock *uint64) ([]types.TokenTransfer, string, error) {
	// cursor (opaque keyset) takes precedence over the legacy block-exclusive
	// beforeBlock bound; the indexer applies whichever is set (RD-1149).
	req := &indexerv1.ListTokenTransfersRequest{
		ByAddress: address,
		Page:      &indexerv1.PageRequest{PageSize: int32(limit), Cursor: cursor},
	}
	if cursor == "" && beforeBlock != nil {
		req.BlockRange = &indexerv1.BlockRange{ToBlock: *beforeBlock}
	}
	resp, err := p.client.ListTokenTransfers(ctx, req)
	if err != nil {
		return nil, "", err
	}
	return mapTokenTransfers(resp.GetTransfers()), resp.GetPage().GetNextCursor(), nil
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

// GetAllTransfers serves the global "Token Transfers" page via the indexer's
// ListAllTokenTransfers RPC (offset pagination + accurate total + optional
// token-standard filter). tokenType is "ERC20"/"ERC721"/"ERC1155" or "" for all.
func (p *Provider) GetAllTransfers(ctx context.Context, tokenType string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	page, limit := offsetPage(offset, limit)
	var tt indexerv1.TokenType
	switch tokenType {
	case "ERC20":
		tt = indexerv1.TokenType_TOKEN_TYPE_ERC20
	case "ERC721":
		tt = indexerv1.TokenType_TOKEN_TYPE_ERC721
	case "ERC1155":
		tt = indexerv1.TokenType_TOKEN_TYPE_ERC1155
	}
	resp, err := p.client.ListAllTokenTransfers(ctx, &indexerv1.ListAllTokenTransfersRequest{
		Page:      &indexerv1.OffsetPageRequest{Page: int32(page), PageSize: int32(limit)},
		TokenType: tt,
	})
	if err != nil {
		return nil, 0, err
	}
	return mapTokenTransfers(resp.GetTransfers()), resp.GetPage().GetTotalItems(), nil
}

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

func (p *Provider) GetTokens(ctx context.Context, limit int, offset int, tokenType, search string) ([]types.Token, int64, error) {
	page, limit := offsetPage(offset, limit)
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
		Search:    search,
	})
	if err != nil {
		return nil, 0, err
	}
	return mapTokens(resp.GetTokens()), resp.GetPage().GetTotalItems(), nil
}

func (p *Provider) GetTokenHolders(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	page, limit := offsetPage(offset, limit)
	resp, err := p.client.ListTokenHolders(ctx, &indexerv1.ListTokenHoldersRequest{
		TokenAddress: tokenAddress,
		Page:         &indexerv1.OffsetPageRequest{Page: int32(page), PageSize: int32(limit)},
	})
	if err != nil {
		return nil, 0, err
	}
	return mapTokenHolders(resp.GetHolders()), resp.GetPage().GetTotalItems(), nil
}

func (p *Provider) GetTokenInventory(ctx context.Context, tokenAddress string, tokenID string, limit int, offset int) ([]types.TokenInventoryItem, int64, error) {
	page, limit := offsetPage(offset, limit)
	resp, err := p.client.ListTokenInventory(ctx, &indexerv1.ListTokenInventoryRequest{
		TokenAddress: tokenAddress,
		TokenId:      tokenID,
		Page:         &indexerv1.OffsetPageRequest{Page: int32(page), PageSize: int32(limit)},
	})
	if err != nil {
		return nil, 0, err
	}
	return mapTokenInventoryItems(resp.GetItems()), resp.GetPage().GetTotalItems(), nil
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
	// Chain facts (address / bytecode / deployer / creation tx / deployment
	// block) come from the indexer. ABI / verification metadata (name,
	// compiler, source code, ABI) live in the explorer's own SQL store, so we
	// merge them in from the SQL-backed DirectDBProvider — without this the
	// contract tab can never show verified source, and the "View UML diagram"
	// feature (which renders that source via sol2uml) would have nothing to
	// work with.
	resp, err := p.client.GetContract(ctx, &indexerv1.GetContractRequest{Address: address})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	contract := mapContract(resp)

	verification, vErr := p.DirectDBProvider.GetContract(ctx, address)
	if vErr == nil && verification != nil && verification.IsVerified {
		mergeVerification(contract, verification)
	}
	return contract, nil
}

// mergeVerification overlays SQL-sourced verification metadata onto a
// chain-facts contract, leaving the on-chain fields (bytecode, creator,
// creation tx, deployment block) intact.
func mergeVerification(dst, v *types.Contract) {
	dst.IsVerified = true
	dst.ContractName = v.ContractName
	dst.CompilerVersion = v.CompilerVersion
	dst.OptimizationUsed = v.OptimizationUsed
	dst.OptimizationRuns = v.OptimizationRuns
	dst.EVMVersion = v.EVMVersion
	dst.LicenseType = v.LicenseType
	dst.SourceCode = v.SourceCode
	dst.ConstructorArgs = v.ConstructorArgs
	if len(v.ABI) > 0 {
		dst.ABI = v.ABI
	}
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
		// Unavailable still means "not OP-Stack mode" -> fall through to SQL.
		if isUnavailable(err) {
			return p.DirectDBProvider.GetOPDeposit(ctx, txHash)
		}
		// BUG-4: an OP deposit is optional enrichment on a tx. The L2 lookup
		// already came back NotFound; if the L1 retry fails for ANY other
		// reason (NotFound, Internal, ...), that means "no deposit for this
		// tx" — surfacing the raw gRPC error would turn an absent deposit into
		// a hard failure for any consumer that propagates it. Return no deposit.
		return nil, nil
	}
	return mapOPDeposit(resp), nil
}

// ----- Gas prices -----

func (p *Provider) GetGasPrices(ctx context.Context, numBlocks int) (slow, normal, fast, baseFee *uint64, err error) {
	resp, rpcErr := p.client.GetGasPrices(ctx, &indexerv1.GetGasPricesRequest{
		SampleBlockCount: uint32(numBlocks),
	})
	if rpcErr != nil {
		return nil, nil, nil, nil, rpcErr
	}
	return bigToUint64Ptr(resp.GetSlow()),
		bigToUint64Ptr(resp.GetNormal()),
		bigToUint64Ptr(resp.GetFast()),
		bigToUint64Ptr(resp.GetBaseFee()),
		nil
}
