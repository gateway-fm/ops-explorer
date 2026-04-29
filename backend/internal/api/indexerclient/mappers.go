//go:build !privacy

package indexerclient

import (
	"strconv"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
	"explorer/internal/types"
)

// Proto-to-explorer-types mappers. Kept narrow and field-by-field so
// breakage is explicit when the proto changes. Mirrors the privacy-proxy
// mappers with the same name; types.* on this side differs slightly in
// field naming (JSONString vs string, etc.).

func unixSec(t interface{ GetSeconds() int64 }) uint64 {
	if t == nil {
		return 0
	}
	s := t.GetSeconds()
	if s < 0 {
		return 0
	}
	return uint64(s)
}

func big(b *indexerv1.BigInt) string {
	if b == nil {
		return ""
	}
	return b.GetValue()
}

func bigToUint64Ptr(b *indexerv1.BigInt) *uint64 {
	if b == nil || b.GetValue() == "" {
		return nil
	}
	n, err := strconv.ParseUint(b.GetValue(), 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func bigToUint64(b *indexerv1.BigInt) uint64 {
	if b == nil || b.GetValue() == "" {
		return 0
	}
	n, err := strconv.ParseUint(b.GetValue(), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func mapBlock(b *indexerv1.Block) *types.Block {
	if b == nil {
		return nil
	}
	return &types.Block{
		Number:           b.GetNumber(),
		Hash:             b.GetHash(),
		ParentHash:       b.GetParentHash(),
		Timestamp:        unixSec(b.GetTimestamp()),
		GasUsed:          b.GetGasUsed(),
		GasLimit:         b.GetGasLimit(),
		BaseFeePerGas:    bigToUint64Ptr(b.GetBaseFeePerGas()),
		TransactionCount: int(b.GetTransactionCount()),
		Size:             b.GetSize(),
		Difficulty:       big(b.GetDifficulty()),
		TotalDifficulty:  big(b.GetTotalDifficulty()),
		Nonce:            b.GetNonce(),
		Miner:            b.GetMiner(),
		ExtraData:        b.GetExtraData(),
		StateRoot:        b.GetStateRoot(),
		TransactionsRoot: b.GetTransactionsRoot(),
		ReceiptsRoot:     b.GetReceiptsRoot(),
	}
}

func mapTxCategories(cs []indexerv1.TransactionCategory) []string {
	if len(cs) == 0 {
		return nil
	}
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		switch c {
		case indexerv1.TransactionCategory_TRANSACTION_CATEGORY_COIN_TRANSFER:
			out = append(out, "coin_transfer")
		case indexerv1.TransactionCategory_TRANSACTION_CATEGORY_CONTRACT_CREATION:
			out = append(out, "contract_creation")
		case indexerv1.TransactionCategory_TRANSACTION_CATEGORY_CONTRACT_CALL:
			out = append(out, "contract_call")
		case indexerv1.TransactionCategory_TRANSACTION_CATEGORY_TOKEN_TRANSFER:
			out = append(out, "token_transfer")
		}
	}
	return out
}

func mapTxStatus(s indexerv1.TransactionStatus) int {
	switch s {
	case indexerv1.TransactionStatus_TRANSACTION_STATUS_SUCCESS:
		return 1
	}
	return 0
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func mapTransaction(t *indexerv1.Transaction) *types.Transaction {
	if t == nil {
		return nil
	}
	var to *string
	if t.GetTo() != "" {
		s := t.GetTo()
		to = &s
	}
	var contractAddr *string
	if t.GetContractAddress() != "" {
		s := t.GetContractAddress()
		contractAddr = &s
	}
	gasLimit := t.GetGas()
	nonce := t.GetNonce()
	return &types.Transaction{
		Hash:                 t.GetHash(),
		BlockNumber:          t.GetBlockNumber(),
		BlockTimestamp:       unixSec(t.GetBlockTimestamp()),
		TxIndex:              int(t.GetTransactionIndex()),
		From:                 t.GetFrom(),
		To:                   to,
		ContractAddress:      contractAddr,
		Value:                types.JSONString(big(t.GetValue())),
		GasUsed:              t.GetGasUsed(),
		GasPrice:             bigToUint64(t.GetGasPrice()),
		GasLimit:             &gasLimit,
		MaxFeePerGas:         bigToUint64Ptr(t.GetMaxFeePerGas()),
		MaxPriorityFeePerGas: bigToUint64Ptr(t.GetMaxPriorityFeePerGas()),
		Nonce:                &nonce,
		TxType:               int(t.GetTxType()),
		InputData:            t.GetInput(),
		Status:               mapTxStatus(t.GetStatus()),
		TxCategories:         mapTxCategories(t.GetCategories()),
	}
}

func mapTransactions(in []*indexerv1.Transaction) []types.Transaction {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.Transaction, 0, len(in))
	for _, t := range in {
		if t == nil {
			continue
		}
		out = append(out, *mapTransaction(t))
	}
	return out
}

func mapBlocks(in []*indexerv1.Block) []types.Block {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.Block, 0, len(in))
	for _, b := range in {
		if b == nil {
			continue
		}
		out = append(out, *mapBlock(b))
	}
	return out
}

func mapAddressStats(a *indexerv1.Address) *types.AddressStats {
	if a == nil {
		return nil
	}
	first := a.GetFirstSeenBlock()
	last := a.GetLastSeenBlock()
	return &types.AddressStats{
		Address:            a.GetAddress(),
		TxCount:            int(a.GetTxCountIn() + a.GetTxCountOut()),
		TokenTransferCount: int(a.GetTokenCount()),
		FirstSeen:          &first,
		LastSeen:           &last,
		IsContract:         a.GetIsContract(),
	}
}

func mapChainStats(c *indexerv1.ChainStats) *types.ChainStats {
	if c == nil {
		return nil
	}
	return &types.ChainStats{
		TotalBlocks:       int64(c.GetTotalBlocks()),
		TotalTransactions: int64(c.GetTotalTransactions()),
		TotalAddresses:    int64(c.GetTotalAddresses()),
		AvgBlockTime:      float64(c.GetAvgBlockTimeSeconds()),
		PrivacyEnabled:    false, // block-explorer standalone has no privacy
	}
}

func mapSyncStatus(s *indexerv1.SyncStatus) *types.SyncStatus {
	if s == nil {
		return nil
	}
	return &types.SyncStatus{
		LastIndexedBlock: s.GetLatestIndexedBlock(),
		IsSyncing:        s.GetIsSyncing(),
	}
}
