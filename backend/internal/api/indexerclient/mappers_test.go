//go:build !privacy

package indexerclient

import (
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	indexerv1 "explorer/gen/go/chain_indexer/v1"
	"explorer/internal/types"
)

// These tests exercise the proto -> explorer types mappers. They cover
// nil/empty handling, BigInt wrapping, timestamp conversion, topic-list
// splitting, and category / status / token-type enum translation.

func TestMapBlock(t *testing.T) {
	in := &indexerv1.Block{
		Number:           100,
		Hash:             "0xblock",
		ParentHash:       "0xparent",
		Timestamp:        timestamppb.New(timeOf(1_700_000_000)),
		Miner:            "0xminer",
		GasUsed:          21000,
		GasLimit:         30_000_000,
		BaseFeePerGas:    &indexerv1.BigInt{Value: "42"},
		Difficulty:       &indexerv1.BigInt{Value: "123"},
		TotalDifficulty:  &indexerv1.BigInt{Value: "456"},
		StateRoot:        "0xstate",
		TransactionsRoot: "0xtxroot",
		ReceiptsRoot:     "0xrcpt",
		ExtraData:        "0xextra",
		Nonce:            "0xnonce",
		Size:             500,
		TransactionCount: 7,
	}
	out := mapBlock(in)
	if out == nil || out.Number != 100 || out.Hash != "0xblock" || out.GasUsed != 21000 {
		t.Fatalf("scalar mismatch: %+v", out)
	}
	if out.BaseFeePerGas == nil || *out.BaseFeePerGas != 42 {
		t.Errorf("BaseFeePerGas: %v", out.BaseFeePerGas)
	}
	if out.Difficulty != "123" || out.TotalDifficulty != "456" {
		t.Errorf("difficulty: %q / %q", out.Difficulty, out.TotalDifficulty)
	}
	if out.TransactionCount != 7 {
		t.Errorf("tx count: %d", out.TransactionCount)
	}
}

func TestMapBlock_EmptyBaseFee(t *testing.T) {
	in := &indexerv1.Block{Number: 1, BaseFeePerGas: &indexerv1.BigInt{}}
	out := mapBlock(in)
	if out.BaseFeePerGas != nil {
		t.Errorf("empty BigInt should map to nil, got %v", out.BaseFeePerGas)
	}
}

func TestMapTransaction_ContractCreation(t *testing.T) {
	in := &indexerv1.Transaction{
		Hash:            "0xtx",
		BlockNumber:     1,
		TransactionIndex: 0,
		From:            "0xdeployer",
		To:              "", // empty = contract creation
		Value:           &indexerv1.BigInt{Value: "0"},
		GasUsed:         500000,
		GasPrice:        &indexerv1.BigInt{Value: "1000000000"},
		Status:          indexerv1.TransactionStatus_TRANSACTION_STATUS_SUCCESS,
		ContractAddress: "0xdeployed",
		Categories: []indexerv1.TransactionCategory{
			indexerv1.TransactionCategory_TRANSACTION_CATEGORY_CONTRACT_CREATION,
		},
	}
	out := mapTransaction(in)
	if out.To != nil {
		t.Errorf("To should be nil for contract creation, got %v", out.To)
	}
	if out.ContractAddress == nil || *out.ContractAddress != "0xdeployed" {
		t.Errorf("ContractAddress: %v", out.ContractAddress)
	}
	if out.Status != 1 {
		t.Errorf("Status: %d", out.Status)
	}
	if len(out.TxCategories) != 1 || out.TxCategories[0] != "contract_creation" {
		t.Errorf("categories: %v", out.TxCategories)
	}
}

func TestMapTransaction_TokenTransferCategory(t *testing.T) {
	in := &indexerv1.Transaction{
		Hash: "0xtx",
		Categories: []indexerv1.TransactionCategory{
			indexerv1.TransactionCategory_TRANSACTION_CATEGORY_COIN_TRANSFER,
			indexerv1.TransactionCategory_TRANSACTION_CATEGORY_TOKEN_TRANSFER,
		},
	}
	out := mapTransaction(in)
	got := out.TxCategories
	want := []string{"coin_transfer", "token_transfer"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("categories: got %v want %v", got, want)
	}
}

func TestMapLog_TopicsSplit(t *testing.T) {
	in := &indexerv1.Log{
		TransactionHash: "0xtx",
		LogIndex:        3,
		Address:         "0xcontract",
		Topics:          []string{"0xt0", "0xt1", "0xt2"},
		Data:            "0xdata",
		BlockNumber:     50,
		BlockTimestamp:  timestamppb.New(timeOf(1_700_000_000)),
	}
	out := mapLog(in)
	if out.Topic0 == nil || *out.Topic0 != "0xt0" {
		t.Errorf("topic0: %v", out.Topic0)
	}
	if out.Topic1 == nil || *out.Topic1 != "0xt1" {
		t.Errorf("topic1: %v", out.Topic1)
	}
	if out.Topic2 == nil || *out.Topic2 != "0xt2" {
		t.Errorf("topic2: %v", out.Topic2)
	}
	if out.Topic3 != nil {
		t.Errorf("topic3 should be nil, got %v", out.Topic3)
	}
	if out.Timestamp == nil || *out.Timestamp != 1_700_000_000 {
		t.Errorf("timestamp: %v", out.Timestamp)
	}
}

func TestMapToken_OptionalFields(t *testing.T) {
	in := &indexerv1.Token{
		Address:       "0xtok",
		Name:          "", // empty -> nil
		Symbol:        "TOK",
		Decimals:      18,
		TotalSupply:   &indexerv1.BigInt{Value: "1000"},
		TokenType:     indexerv1.TokenType_TOKEN_TYPE_ERC20,
		HolderCount:   50,
		TransferCount: 200,
		IconUrl:       "https://example.com/icon.png",
		PriceUsd:      "1.5",
	}
	out := mapToken(in)
	if out.Address != "0xtok" || out.Symbol != "TOK" || out.Decimals != 18 {
		t.Fatalf("scalar: %+v", out)
	}
	if out.Name != nil {
		t.Errorf("empty Name should map to nil, got %v", out.Name)
	}
	if out.TokenType != "ERC20" {
		t.Errorf("token type: %q", out.TokenType)
	}
	if out.TotalSupply == nil || *out.TotalSupply != "1000" {
		t.Errorf("total supply: %v", out.TotalSupply)
	}
	if out.USDPrice == nil || *out.USDPrice != 1.5 {
		t.Errorf("price: %v", out.USDPrice)
	}
}

func TestMapAddressStats(t *testing.T) {
	in := &indexerv1.Address{
		Address:        "0xaddr",
		IsContract:     true,
		TxCountIn:      3,
		TxCountOut:     7,
		TokenCount:     2,
		FirstSeenBlock: 100,
		LastSeenBlock:  200,
	}
	out := mapAddressStats(in)
	if out.Address != "0xaddr" || !out.IsContract || out.TxCount != 10 {
		t.Errorf("scalar: %+v", out)
	}
	if out.FirstSeen == nil || *out.FirstSeen != 100 {
		t.Errorf("FirstSeen: %v", out.FirstSeen)
	}
	if out.LastSeen == nil || *out.LastSeen != 200 {
		t.Errorf("LastSeen: %v", out.LastSeen)
	}
}

func TestMapChainStats(t *testing.T) {
	in := &indexerv1.ChainStats{
		TotalBlocks:         500,
		TotalTransactions:   1000,
		TotalAddresses:      50,
		AvgBlockTimeSeconds: 12.5,
	}
	out := mapChainStats(in)
	if out.TotalBlocks != 500 || out.TotalTransactions != 1000 || out.TotalAddresses != 50 {
		t.Errorf("counts: %+v", out)
	}
	if out.AvgBlockTime < 12.4 || out.AvgBlockTime > 12.6 {
		t.Errorf("AvgBlockTime: %v", out.AvgBlockTime)
	}
	if out.PrivacyEnabled {
		t.Error("PrivacyEnabled should be false for standalone block-explorer")
	}
}

func TestMapSyncStatus(t *testing.T) {
	in := &indexerv1.SyncStatus{LatestIndexedBlock: 999, IsSyncing: true}
	out := mapSyncStatus(in)
	if out.LastIndexedBlock != 999 || !out.IsSyncing {
		t.Errorf("%+v", out)
	}
}

func TestMapInternalTx_OptionalPointers(t *testing.T) {
	input := "0xinput"
	in := &indexerv1.InternalTransaction{
		TransactionHash: "0xtx",
		BlockNumber:     1,
		TraceAddress:    "0/1",
		CallType:        "CALL",
		From:            "0xfrom",
		To:              "0xto",
		Value:           &indexerv1.BigInt{Value: "100"},
		Gas:             21000,
		GasUsed:         20000,
		Input:           input,
	}
	out := mapInternalTx(in)
	if out.To == nil || *out.To != "0xto" {
		t.Errorf("To: %v", out.To)
	}
	if out.Gas == nil || *out.Gas != 21000 {
		t.Errorf("Gas: %v", out.Gas)
	}
	if out.GasUsed == nil || *out.GasUsed != 20000 {
		t.Errorf("GasUsed: %v", out.GasUsed)
	}
	if out.Input == nil || *out.Input != input {
		t.Errorf("Input: %v", out.Input)
	}
}

// TestMapDailyStatsList_CarriesPresentFields is the BUG-2 red->green test.
//
// The chain-indexer gRPC DailyStats message carries: Date, Transactions,
// NewAddresses, ActiveAddresses, Blocks, GasUsed, TotalFees. mapDailyStatsList
// previously dropped TotalFees entirely, so the per-day average transaction
// fee surfaced as 0 on /charts/avg_txn_fee and /charts/avg_gas_price for an
// otherwise-synced chain. TotalFees IS in the payload, so the correct fix is
// to map it into AvgGasPrice = TotalFees / Transactions (wei per tx). The
// consumer (extractChartDataPoints) divides AvgGasPrice by 1e9 to render Gwei.
func TestMapDailyStatsList_CarriesPresentFields(t *testing.T) {
	in := []*indexerv1.DailyStats{
		{
			Date:            "2026-06-01",
			Transactions:    10,
			NewAddresses:    4,
			ActiveAddresses: 7,
			Blocks:          5,
			GasUsed:         105000,
			// 10 txns, 10 Gwei average fee => total 100 Gwei == 100e9 wei.
			TotalFees: &indexerv1.BigInt{Value: "100000000000"},
		},
	}
	out := mapDailyStatsList(in)
	if len(out) != 1 {
		t.Fatalf("expected 1 row, got %d", len(out))
	}
	d := out[0]
	// Fields the gRPC payload provides and the mapper already carried.
	if d.Date != "2026-06-01" {
		t.Errorf("Date: %q", d.Date)
	}
	if d.TotalTransactions != 10 {
		t.Errorf("TotalTransactions: %d, want 10", d.TotalTransactions)
	}
	if d.TotalBlocks != 5 {
		t.Errorf("TotalBlocks: %d, want 5", d.TotalBlocks)
	}
	if d.TotalGasUsed != 105000 {
		t.Errorf("TotalGasUsed: %d, want 105000", d.TotalGasUsed)
	}
	if d.ActiveAddresses != 7 || d.NewAddresses != 4 {
		t.Errorf("addresses: active=%d new=%d", d.ActiveAddresses, d.NewAddresses)
	}
	// BUG-2: TotalFees is present in the payload but was dropped. Average fee
	// per tx = 100e9 wei / 10 txns = 10e9 wei (= 10 Gwei after the /1e9 in the
	// chart layer). This MUST be surfaced, not 0.
	wantAvg := int64(100000000000 / 10) // 10_000_000_000 wei
	if d.AvgGasPrice != wantAvg {
		t.Errorf("AvgGasPrice: %d, want %d (TotalFees/Transactions)", d.AvgGasPrice, wantAvg)
	}
}

// TestMapDailyStatsList_ZeroTxnsNoPanic guards the divide-by-zero edge: a day
// with fees recorded but zero transactions must not panic and must surface a
// zero average rather than NaN/Inf.
func TestMapDailyStatsList_ZeroTxnsNoPanic(t *testing.T) {
	in := []*indexerv1.DailyStats{
		{Date: "2026-06-02", Transactions: 0, TotalFees: &indexerv1.BigInt{Value: "5000"}},
	}
	out := mapDailyStatsList(in)
	if len(out) != 1 {
		t.Fatalf("expected 1 row, got %d", len(out))
	}
	if out[0].AvgGasPrice != 0 {
		t.Errorf("AvgGasPrice with 0 txns: %d, want 0", out[0].AvgGasPrice)
	}
}

// TestMapDailyStatsList_UpstreamGaps documents the BUG-2 cross-repo gap: these
// types.DailyStats fields have NO corresponding field in the gRPC DailyStats
// message, so the explorer cannot populate them without a chain-indexer change.
// The chart lines that read them (accounts_growth, txns_growth,
// txns_success_rate, avg_block_size, avg_block_time, new_contracts,
// contracts_growth, new_token_transfers) and the /charts/counters cumulative
// totals therefore render 0 on a synced chain. Skipped (not failed) so CI stays
// green; tracked as a chain-indexer issue (see final report).
func TestMapDailyStatsList_UpstreamGaps(t *testing.T) {
	t.Skip("TODO(chain-indexer): DailyStats lacks cumulative/success/contract/" +
		"token-transfer/block-time/block-size fields — see report. Cannot map " +
		"without an upstream gRPC change.")

	in := []*indexerv1.DailyStats{{Date: "2026-06-01", Transactions: 10}}
	out := mapDailyStatsList(in)
	d := out[0]
	// These assertions encode the CORRECT behavior once the upstream payload
	// carries the data; they intentionally fail against the current proto.
	if d.SuccessfulTxs == 0 {
		t.Error("SuccessfulTxs not surfaced (needs upstream success_count)")
	}
	if d.CumulativeTransactions == 0 {
		t.Error("CumulativeTransactions not surfaced (needs upstream cumulative_transactions)")
	}
	if d.CumulativeAddresses == 0 {
		t.Error("CumulativeAddresses not surfaced (needs upstream cumulative_addresses)")
	}
	if d.CumulativeContracts == 0 {
		t.Error("CumulativeContracts not surfaced (needs upstream cumulative_contracts)")
	}
	if d.NewContracts == 0 {
		t.Error("NewContracts not surfaced (needs upstream new_contracts)")
	}
	if d.TokenTransferCount == 0 {
		t.Error("TokenTransferCount not surfaced (needs upstream token_transfer_count)")
	}
	if d.AvgBlockTime == 0 {
		t.Error("AvgBlockTime not surfaced (needs upstream avg_block_time)")
	}
	if d.AvgBlockSize == 0 {
		t.Error("AvgBlockSize not surfaced (needs upstream avg_block_size)")
	}
}

func TestMapOPDeposit(t *testing.T) {
	ts := timestamppb.New(timeOf(1_700_000_000))
	in := &indexerv1.OPDeposit{
		L1TransactionHash:  "0xl1",
		L2TransactionHash:  "0xl2",
		L1BlockNumber:      100,
		L1BlockTimestamp:   ts,
		From:               "0xorigin",
	}
	out := mapOPDeposit(in)
	if out.L1TxHash != "0xl1" || out.L2TxHash != "0xl2" {
		t.Errorf("hashes: %+v", out)
	}
	if out.L1BlockTimestamp == nil || *out.L1BlockTimestamp != 1_700_000_000 {
		t.Errorf("timestamp: %v", out.L1BlockTimestamp)
	}
}

// Compile-time assurance the mappers' return types match the explorer
// types package.
var (
	_ *types.Block        = mapBlock(nil)
	_ *types.Transaction  = mapTransaction(nil)
	_ *types.Token        = mapToken(nil)
	_ *types.AddressStats = mapAddressStats(nil)
)

// bring mapBalances & mapAccounts under the type assertion umbrella too.
func TestMapBalancesEmpty(t *testing.T) {
	if got := mapBalances(nil); got != nil {
		t.Errorf("nil input should yield nil, got %v", got)
	}
	if got := mapAccounts(nil); got != nil {
		t.Errorf("nil input should yield nil, got %v", got)
	}
}
