//go:build !privacy

package indexerclient

import (
	"strconv"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
	"explorer/internal/types"
)

// ----- Log -----

// mapLog converts one proto Log. Proto carries topics as a repeated list;
// types.Log splits them across Topic0..Topic3.
func mapLog(l *indexerv1.Log) types.Log {
	out := types.Log{
		TxHash:      l.GetTransactionHash(),
		LogIndex:    int(l.GetLogIndex()),
		Address:     l.GetAddress(),
		Data:        l.GetData(),
		BlockNumber: l.GetBlockNumber(),
		Removed:     l.GetRemoved(),
	}
	if ts := l.GetBlockTimestamp(); ts != nil {
		u := unixSec(ts)
		out.Timestamp = &u
	}
	topics := l.GetTopics()
	if len(topics) > 0 {
		t := topics[0]
		out.Topic0 = &t
	}
	if len(topics) > 1 {
		t := topics[1]
		out.Topic1 = &t
	}
	if len(topics) > 2 {
		t := topics[2]
		out.Topic2 = &t
	}
	if len(topics) > 3 {
		t := topics[3]
		out.Topic3 = &t
	}
	return out
}

func mapLogs(in []*indexerv1.Log) []types.Log {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.Log, 0, len(in))
	for _, l := range in {
		if l == nil {
			continue
		}
		out = append(out, mapLog(l))
	}
	return out
}

// ----- Token / TokenTransfer / TokenBalance / TokenHolder -----

func mapTokenType(t indexerv1.TokenType) string {
	switch t {
	case indexerv1.TokenType_TOKEN_TYPE_ERC20:
		return "ERC20"
	case indexerv1.TokenType_TOKEN_TYPE_ERC721:
		return "ERC721"
	case indexerv1.TokenType_TOKEN_TYPE_ERC1155:
		return "ERC1155"
	}
	return ""
}

func mapToken(t *indexerv1.Token) *types.Token {
	if t == nil {
		return nil
	}
	var name *string
	if n := t.GetName(); n != "" {
		name = &n
	}
	var totalSupply *string
	if s := big(t.GetTotalSupply()); s != "" {
		totalSupply = &s
	}
	var iconURL *string
	if u := t.GetIconUrl(); u != "" {
		iconURL = &u
	}
	var usdPrice *float64
	if p := t.GetPriceUsd(); p != "" {
		if v, err := strconv.ParseFloat(p, 64); err == nil {
			usdPrice = &v
		}
	}
	return &types.Token{
		Address:       t.GetAddress(),
		Symbol:        t.GetSymbol(),
		Name:          name,
		Decimals:      int(t.GetDecimals()),
		TokenType:     mapTokenType(t.GetTokenType()),
		TotalSupply:   totalSupply,
		HolderCount:   int(t.GetHolderCount()),
		TransferCount: int(t.GetTransferCount()),
		USDPrice:      usdPrice,
		IconURL:       iconURL,
	}
}

func mapTokens(in []*indexerv1.Token) []types.Token {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.Token, 0, len(in))
	for _, t := range in {
		if t == nil {
			continue
		}
		out = append(out, *mapToken(t))
	}
	return out
}

func mapTokenTransfer(t *indexerv1.TokenTransfer) types.TokenTransfer {
	out := types.TokenTransfer{
		TxHash:       t.GetTransactionHash(),
		LogIndex:     int(t.GetLogIndex()),
		TokenAddress: t.GetTokenAddress(),
		From:         t.GetFrom(),
		To:           t.GetTo(),
		Value:        types.JSONString(big(t.GetValue())),
		BlockNumber:  t.GetBlockNumber(),
		TokenType:    mapTokenType(t.GetTokenType()),
	}
	if ts := t.GetBlockTimestamp(); ts != nil {
		u := unixSec(ts)
		out.Timestamp = &u
	}
	if id := big(t.GetTokenId()); id != "" {
		out.TokenID = &id
	}
	return out
}

func mapTokenTransfers(in []*indexerv1.TokenTransfer) []types.TokenTransfer {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.TokenTransfer, 0, len(in))
	for _, t := range in {
		if t == nil {
			continue
		}
		out = append(out, mapTokenTransfer(t))
	}
	return out
}

func mapTokenHolders(in []*indexerv1.TokenBalance) []types.TokenHolder {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.TokenHolder, 0, len(in))
	for _, h := range in {
		if h == nil {
			continue
		}
		out = append(out, types.TokenHolder{
			Address: h.GetAddress(),
			Balance: types.JSONString(big(h.GetBalance())),
			// Percentage / IsContract not carried by the proto; leave zero.
		})
	}
	return out
}

func mapTokenInventoryItems(in []*indexerv1.TokenInventoryItem) []types.TokenInventoryItem {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.TokenInventoryItem, 0, len(in))
	for _, it := range in {
		if it == nil {
			continue
		}
		item := types.TokenInventoryItem{
			TokenID: big(it.GetTokenId()),
			Owner:   it.GetOwner(),
		}
		if uri := it.GetTokenUri(); uri != "" {
			item.TokenURI = &uri
		}
		// ERC-1155 fields: a non-empty quantity marks a multi-token id, where
		// the same id is shared across `holders` owners.
		if qty := it.GetQuantity(); qty != "" {
			item.Quantity = &qty
			holders := it.GetHolders()
			item.Holders = &holders
		}
		out = append(out, item)
	}
	return out
}

func mapBalances(in []*indexerv1.TokenBalance) []types.Balance {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.Balance, 0, len(in))
	for _, h := range in {
		if h == nil {
			continue
		}
		out = append(out, types.Balance{
			Address:      h.GetAddress(),
			TokenAddress: h.GetTokenAddress(),
			Balance:      types.JSONString(big(h.GetBalance())),
		})
	}
	return out
}

// ----- InternalTransaction -----

func mapInternalTx(it *indexerv1.InternalTransaction) types.InternalTransaction {
	out := types.InternalTransaction{
		TxHash:       it.GetTransactionHash(),
		BlockNumber:  it.GetBlockNumber(),
		TraceAddress: it.GetTraceAddress(),
		From:         it.GetFrom(),
		Value:        types.JSONString(big(it.GetValue())),
		CallType:     it.GetCallType(),
	}
	if to := it.GetTo(); to != "" {
		out.To = &to
	}
	if gas := it.GetGas(); gas != 0 {
		out.Gas = &gas
	}
	if g := it.GetGasUsed(); g != 0 {
		out.GasUsed = &g
	}
	if in := it.GetInput(); in != "" {
		out.Input = &in
	}
	if o := it.GetOutput(); o != "" {
		out.Output = &o
	}
	if e := it.GetError(); e != "" {
		out.Error = &e
	}
	if ts := it.GetBlockTimestamp(); ts != nil {
		u := unixSec(ts)
		out.Timestamp = &u
	}
	return out
}

func mapInternalTxs(in []*indexerv1.InternalTransaction) []types.InternalTransaction {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.InternalTransaction, 0, len(in))
	for _, it := range in {
		if it == nil {
			continue
		}
		out = append(out, mapInternalTx(it))
	}
	return out
}

// ----- Contract -----

// mapContract carries chain-facts only. ABI / source / verification
// metadata stay on SQL (they're a block-explorer-local concern).
func mapContract(c *indexerv1.Contract) *types.Contract {
	if c == nil {
		return nil
	}
	return &types.Contract{
		Address:     c.GetAddress(),
		Bytecode:    c.GetBytecode(),
		Creator:     c.GetDeployer(),
		CreationTx:  c.GetDeploymentTxHash(),
		BlockNumber: c.GetDeploymentBlock(),
	}
}

// ----- Accounts (AddressStats list) -----

func mapAccounts(in []*indexerv1.Address) []types.AddressStats {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.AddressStats, 0, len(in))
	for _, a := range in {
		if a == nil {
			continue
		}
		out = append(out, *mapAddressStats(a))
	}
	return out
}

// ----- Search -----

func mapSearchResults(in *indexerv1.SearchResponse) []types.SearchSuggestion {
	if in == nil {
		return nil
	}
	results := in.GetResults()
	out := make([]types.SearchSuggestion, 0, len(results))
	for _, r := range results {
		sug := types.SearchSuggestion{}
		switch r.GetKind() {
		case indexerv1.SearchResponse_SEARCH_RESULT_KIND_BLOCK:
			if b := r.GetBlock(); b != nil {
				sug.Type = "block"
				sug.Value = strconv.FormatUint(b.GetNumber(), 10)
				sug.Label = "Block #" + sug.Value
			}
		case indexerv1.SearchResponse_SEARCH_RESULT_KIND_TRANSACTION:
			if t := r.GetTransaction(); t != nil {
				sug.Type = "transaction"
				sug.Value = t.GetHash()
				sug.Label = truncateHash(sug.Value)
			}
		case indexerv1.SearchResponse_SEARCH_RESULT_KIND_ADDRESS:
			if a := r.GetAddress(); a != nil {
				sug.Type = "address"
				sug.Value = a.GetAddress()
				sug.Label = truncateHash(sug.Value)
			}
		case indexerv1.SearchResponse_SEARCH_RESULT_KIND_TOKEN:
			if t := r.GetToken(); t != nil {
				sug.Type = "token"
				sug.Value = t.GetAddress()
				sug.Label = t.GetSymbol()
			}
		default:
			continue
		}
		if sug.Type == "" {
			continue
		}
		out = append(out, sug)
	}
	return out
}

func truncateHash(h string) string {
	if len(h) <= 16 {
		return h
	}
	return h[:8] + "..." + h[len(h)-6:]
}

// ----- TxHistory / DailyStats / OPDeposit -----

func mapTxHistory(in *indexerv1.TransactionHistory) []types.TxHistoryPoint {
	if in == nil {
		return nil
	}
	buckets := in.GetBuckets()
	out := make([]types.TxHistoryPoint, 0, len(buckets))
	for _, b := range buckets {
		if b == nil {
			continue
		}
		out = append(out, types.TxHistoryPoint{
			Timestamp: unixSec(b.GetBucketStart()),
			Count:     int64(b.GetTransactionCount()),
		})
	}
	return out
}

func mapDailyStatsList(in []*indexerv1.DailyStats) []types.DailyStats {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.DailyStats, 0, len(in))
	for _, d := range in {
		if d == nil {
			continue
		}
		txns := d.GetTransactions()
		// AvgGasPrice carries the average fee per transaction in wei
		// (the chart layer divides by 1e9 to render Gwei). The gRPC payload
		// only gives us aggregate TotalFees + Transactions for the day, so
		// derive the per-tx average here. Guard against divide-by-zero on
		// days with no transactions.
		var avgFeeWei int64
		if txns > 0 {
			avgFeeWei = int64(bigToUint64(d.GetTotalFees()) / txns)
		}
		out = append(out, types.DailyStats{
			Date:              d.GetDate(),
			TotalBlocks:       int(d.GetBlocks()),
			TotalTransactions: int(txns),
			TotalGasUsed:      int64(d.GetGasUsed()),
			AvgGasPrice:       avgFeeWei,
			ActiveAddresses:   int(d.GetActiveAddresses()),
			NewAddresses:      int(d.GetNewAddresses()),
			// NOTE(chain-indexer): SuccessfulTxs, FailedTxs, NewContracts,
			// TokenTransferCount, AvgBlockTime, AvgBlockSize, and the three
			// Cumulative* fields have no source in the gRPC DailyStats message
			// and stay zero. See TestMapDailyStatsList_UpstreamGaps + report.
		})
	}
	return out
}

func mapOPDeposit(d *indexerv1.OPDeposit) *types.OPDeposit {
	if d == nil {
		return nil
	}
	out := &types.OPDeposit{
		L2TxHash:      d.GetL2TransactionHash(),
		L1BlockNumber: d.GetL1BlockNumber(),
		L1TxHash:      d.GetL1TransactionHash(),
		L1TxOrigin:    d.GetFrom(),
	}
	if ts := d.GetL1BlockTimestamp(); ts != nil {
		u := unixSec(ts)
		out.L1BlockTimestamp = &u
	}
	return out
}

// strPtrOrNil is referenced by the existing handlers.go; keep it here to
// centralise the helper.
var _ = strPtrOrNil
