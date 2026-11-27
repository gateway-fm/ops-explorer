package txflow

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"testing"

	"explorer/internal/types"
	ethcommon "explorer/pkg/eth/common"
	"explorer/pkg/eth/crypto"
	"explorer/pkg/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sender = "0x1111111111111111111111111111111111111111"
	alice  = "0x2222222222222222222222222222222222222222"
	vault  = "0x3333333333333333333333333333333333333333"
	usdc   = "0x4444444444444444444444444444444444444444"
	impl   = "0x5555555555555555555555555555555555555555"
	oracle = "0x6666666666666666666666666666666666666666"
)

func ptr[T any](v T) *T { return &v }

func word(v any) string {
	switch x := v.(type) {
	case string: // address
		return strings.Repeat("0", 24) + strings.TrimPrefix(strings.ToLower(x), "0x")
	case int:
		return fmt.Sprintf("%064x", x)
	case *big.Int:
		return fmt.Sprintf("%064x", x)
	}
	panic("unsupported")
}

func topicOf(sig string) string { return "0x" + hex.EncodeToString(crypto.Keccak256([]byte(sig))) }
func selectorOf(sig string) string {
	return hex.EncodeToString(crypto.Keccak256([]byte(sig))[:4])
}

func transferLog(idx int, token, from, to string, amount int) types.Log {
	return types.Log{
		LogIndex: idx, Address: token,
		Topic0: ptr(topicOf("Transfer(address,address,uint256)")),
		Topic1: ptr("0x" + word(from)), Topic2: ptr("0x" + word(to)),
		Data: word(amount),
	}
}

func TestSimpleValueTransfer(t *testing.T) {
	f := Build(Input{Tx: types.Transaction{
		Hash: "0xabc", From: sender, To: ptr(alice), Value: "1500000000000000000", Status: 1,
	}}, Options{Currency: "tTEST"})

	require.Len(t, f.Steps, 1)
	s := f.Steps[0]
	assert.Equal(t, KindValueTransfer, s.Kind)
	assert.Equal(t, "1.5", s.Amount)
	assert.Equal(t, "The sender 0x1111…1111 sent 1.5 tTEST to 0x2222…2222", s.Text)
	assert.Equal(t, "success", f.Status)
	assert.False(t, f.HasCallData)
	assert.Empty(t, f.Notes, "a plain transfer needs no note about missing call data")
	require.Len(t, f.Participants, 2)
	assert.Equal(t, "account", f.Participants[0].Kind)
	assert.Equal(t, "The sender 0x1111…1111 sent 1.5 tTEST to 0x2222…2222.", f.Summary)
}

func TestTokenTransferUnverified(t *testing.T) {
	input := selectorOf("transfer(address,uint256)") + word(alice) + word(100_000_000)
	f := Build(Input{
		Tx:        types.Transaction{Hash: "0xabc", From: sender, To: ptr(usdc), Value: "0", InputData: input, Status: 1},
		Logs:      []types.Log{transferLog(0, usdc, sender, alice, 100_000_000)},
		Transfers: []types.TokenTransfer{{LogIndex: 0, TokenAddress: usdc, From: sender, To: alice, Value: "100000000", TokenType: "ERC20"}},
		Contracts: map[string]*types.Contract{usdc: {Address: usdc}},
		Tokens:    map[string]*types.Token{usdc: {Address: usdc, Symbol: "USDC", Decimals: 6, TokenType: "ERC20"}},
	}, Options{})

	require.Len(t, f.Steps, 2)
	call := f.Steps[0]
	assert.Equal(t, "transfer", call.Method)
	assert.Equal(t, DecodedCommon, call.Decoded)
	assert.Equal(t, []Arg{{Name: "to", Type: "address", Value: alice}, {Name: "amount", Type: "uint256", Value: "100000000"}}, call.Args)
	assert.Equal(t, "The sender 0x1111…1111 called transfer on the USDC token", call.Text)

	tr := f.Steps[1]
	assert.Equal(t, KindTokenTransfer, tr.Kind)
	assert.Equal(t, "100", tr.Token.Amount)
	assert.Equal(t, "The sender 0x1111…1111 sent 100 USDC to 0x2222…2222", tr.Text)
	assert.Equal(t, "token", f.Participants[1].Kind)
	assert.Equal(t, "USDC", f.Participants[1].Label)
	assert.Contains(t, f.Notes[0], "No internal calls are recorded")
}

func TestUnknownSelectorAndEventFallBack(t *testing.T) {
	f := Build(Input{
		Tx:   types.Transaction{Hash: "0xabc", From: sender, To: ptr(vault), Value: "0", InputData: "aaa51e42" + word(1), Status: 1},
		Logs: []types.Log{{LogIndex: 0, Address: vault, Topic0: ptr(topicOf("Mystery(uint256)")), Data: word(7)}},
	}, Options{})
	require.Len(t, f.Steps, 2)
	assert.Equal(t, "0xaaa51e42", f.Steps[0].Method)
	assert.Equal(t, "The sender 0x1111…1111 called function 0xaaa51e42 on 0x3333…3333", f.Steps[0].Text)
	assert.Equal(t, KindEvent, f.Steps[1].Kind)
	assert.Contains(t, f.Steps[1].Text, "reported event 0x")
}

// vaultABI is a verified contract with a function and an event.
var vaultABI = json.RawMessage(`[
 {"type":"function","name":"deposit","inputs":[{"name":"token","type":"address"},{"name":"amount","type":"uint256"}]},
 {"type":"event","name":"Deposited","inputs":[{"name":"user","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false}]}
]`)

// complexInput is a tx that calls a verified vault through a proxy: the vault pulls
// tokens, reads an oracle many times, makes a failing call, then emits Deposited.
func complexInput() Input {
	deposit := selectorOf("deposit(address,uint256)") + word(usdc) + word(5_000_000)
	pull := "0x" + selectorOf("transferFrom(address,address,uint256)") + word(sender) + word(vault) + word(5_000_000)
	price := "0x" + selectorOf("latestAnswer()")
	internal := []types.InternalTransaction{
		{TraceAddress: "0", From: vault, To: ptr(impl), CallType: "delegatecall", Value: "0", Input: ptr("0x" + deposit)},
		{TraceAddress: "0,0", From: vault, To: ptr(usdc), CallType: "call", Value: "0", Input: ptr(pull)},
	}
	for i := 1; i <= 30; i++ {
		internal = append(internal, types.InternalTransaction{TraceAddress: fmt.Sprintf("0,%d", i), From: vault, To: ptr(oracle), CallType: "staticcall", Value: "0", Input: ptr(price)})
	}
	internal = append(internal,
		types.InternalTransaction{TraceAddress: "0,31", From: vault, To: ptr(alice), CallType: "call", Value: "1000", Error: ptr("out of gas")},
		types.InternalTransaction{TraceAddress: "0,32", From: vault, To: ptr(alice), CallType: "call", Value: "2000000000000000000"},
	)
	// Stored order is not guaranteed; Build must sort numerically ("0,10" after "0,9").
	internal[3], internal[20] = internal[20], internal[3]
	return Input{
		Tx:       types.Transaction{Hash: "0xabc", From: sender, To: ptr(vault), Value: "0", InputData: deposit, Status: 1},
		Internal: internal,
		Logs: []types.Log{
			transferLog(0, usdc, sender, vault, 5_000_000),
			{LogIndex: 1, Address: vault, Topic0: ptr(topicOf("Deposited(address,uint256)")), Topic1: ptr("0x" + word(sender)), Data: word(5_000_000)},
		},
		Transfers: []types.TokenTransfer{{LogIndex: 0, TokenAddress: usdc, From: sender, To: vault, Value: "5000000", TokenType: "ERC20"}},
		Contracts: map[string]*types.Contract{
			vault:  {Address: vault, IsVerified: true, ContractName: ptr("Vault"), ABI: vaultABI},
			impl:   {Address: impl},
			usdc:   {Address: usdc},
			oracle: {Address: oracle},
		},
		Tokens: map[string]*types.Token{usdc: {Address: usdc, Symbol: "USDC", Decimals: 6}},
	}
}

func TestManyCallTrace(t *testing.T) {
	f := Build(complexInput(), Options{})
	require.True(t, f.HasCallData)

	var kinds []string
	for _, s := range f.Steps {
		kinds = append(kinds, fmt.Sprintf("%s:%s:%d", s.Kind, s.Method, s.Repeat))
	}
	assert.Equal(t, []string{
		"call:deposit:0",      // the sender calls the verified Vault
		"call:deposit:0",      // Vault delegates to its implementation (decoded with Vault's ABI)
		"call:transferFrom:0", // Vault pulls USDC
		"token_transfer:Transfer:0",
	}, kinds[:4])

	assert.Equal(t, "Vault ran deposit using the code of contract 0x5555…5555", f.Steps[1].Text)
	assert.Equal(t, "The sender 0x1111…1111 sent 5 USDC to Vault", f.Steps[3].Text)

	// 30 identical oracle reads merge into one step.
	reads := f.Steps[4]
	assert.Equal(t, 30, reads.Repeat)
	assert.True(t, reads.ReadOnly)
	assert.Equal(t, "Vault read function "+"0x"+selectorOf("latestAnswer()")+" from contract 0x6666…6666 (30 times in a row)", reads.Text)

	failed := f.Steps[5]
	assert.Equal(t, "out of gas", failed.Error)
	assert.Contains(t, failed.Text, "This call failed: out of gas")

	sent := f.Steps[6]
	assert.Equal(t, KindValueTransfer, sent.Kind)
	assert.Equal(t, "Vault sent 2 ETH to 0x2222…2222", sent.Text)

	// Deposited is emitted by Vault (via its implementation): shown at the end of its call.
	ev := f.Steps[7]
	assert.Equal(t, "Deposited", ev.Method)
	assert.Equal(t, DecodedVerified, ev.Decoded)
	assert.Equal(t, "Vault reported Deposited (user: the sender 0x1111…1111, amount: 5000000)", ev.Text)
	assert.Equal(t, 1, ev.Depth, "emitted by Vault's code running in the delegatecall")
	assert.Equal(t, 2, f.Steps[3].Depth, "the USDC transfer is emitted inside transferFrom")
	assert.Len(t, f.Steps, 8)
	for i, s := range f.Steps {
		assert.Equal(t, i, s.Index)
	}
	assert.Contains(t, strings.Join(f.Notes, " "), "exact position")
	assert.Equal(t, "The sender 0x1111…1111 called deposit on Vault. It led to 34 internal calls, 1 token transfer and 2 events.", f.Summary)
}

func TestRolledBackDescendants(t *testing.T) {
	f := Build(Input{
		Tx: types.Transaction{Hash: "0xabc", From: sender, To: ptr(vault), Value: "0", InputData: "deadbeef", Status: 0, RevertReason: ptr("not allowed")},
		Internal: []types.InternalTransaction{
			{TraceAddress: "0", From: vault, To: ptr(alice), CallType: "call", Value: "5"},
		},
	}, Options{})
	assert.Equal(t, "failed", f.Status)
	assert.Equal(t, "not allowed", f.Steps[0].Error)
	assert.True(t, f.Steps[1].RolledBack)
	assert.Contains(t, f.Steps[1].Text, "Undone, because a call above it failed")
}

func TestLogPlacementAfterSubcall(t *testing.T) {
	// A pair contract sends tokens out, then emits Swap: Swap must follow the transfer.
	pair := vault
	in := Input{
		Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(pair), Value: "0", InputData: "12345678", Status: 1},
		Internal: []types.InternalTransaction{
			{TraceAddress: "0", From: pair, To: ptr(usdc), CallType: "call", Value: "0", Input: ptr("0x" + selectorOf("transfer(address,uint256)") + word(alice) + word(1))},
			{TraceAddress: "1", From: pair, To: ptr(usdc), CallType: "staticcall", Value: "0", Input: ptr("0x" + selectorOf("balanceOf(address)") + word(pair))},
		},
		Logs: []types.Log{
			transferLog(0, usdc, pair, alice, 1),
			{LogIndex: 1, Address: pair, Topic0: ptr(topicOf("Swap(uint256)")), Data: word(1)},
		},
	}
	f := Build(in, Options{})
	var order []string
	for _, s := range f.Steps {
		order = append(order, s.Method)
	}
	assert.Equal(t, []string{"0x12345678", "transfer", "Transfer", "balanceOf", "0x" + strings.TrimPrefix(topicOf("Swap(uint256)"), "0x")}, order)
}

func TestCapAndCollapse(t *testing.T) {
	var internal []types.InternalTransaction
	for i := 0; i < 2000; i++ {
		// Alternate targets so nothing merges: every call is its own step.
		to := alice
		if i%2 == 1 {
			to = oracle
		}
		internal = append(internal, types.InternalTransaction{TraceAddress: fmt.Sprint(i), From: vault, To: ptr(to), CallType: "staticcall", Value: "0", Input: ptr("0x12345678")})
	}
	f := Build(Input{Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "aabbccdd", Status: 1}, Internal: internal}, Options{})
	assert.Len(t, f.Steps, DefaultMaxSteps)
	assert.Equal(t, 2001, f.TotalSteps)
	assert.True(t, f.Truncated)
	assert.Contains(t, f.Notes[0], "Showing the first 250 steps; the transaction has 2001 calls and events in all.")
	assert.LessOrEqual(t, len(f.Participants), 4)

	same := make([]types.InternalTransaction, 5000)
	for i := range same {
		same[i] = types.InternalTransaction{TraceAddress: fmt.Sprint(i), From: vault, To: ptr(oracle), CallType: "staticcall", Value: "0", Input: ptr("0x12345678")}
	}
	f = Build(Input{Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "aabbccdd", Status: 1}, Internal: same}, Options{})
	require.Len(t, f.Steps, 2)
	assert.Equal(t, 5000, f.Steps[1].Repeat)
	assert.False(t, f.Truncated)
}

func TestContractCreation(t *testing.T) {
	f := Build(Input{Tx: types.Transaction{Hash: "0x1", From: sender, ContractAddress: ptr(vault), Value: "0", InputData: "6080", Status: 1}}, Options{})
	require.Len(t, f.Steps, 1)
	assert.Equal(t, KindCreate, f.Steps[0].Kind)
	assert.Equal(t, "The sender 0x1111…1111 created a new contract at 0x3333…3333", f.Steps[0].Text)
}

func TestMintBurnAndNFT(t *testing.T) {
	n := newNamer(Input{Tokens: map[string]*types.Token{usdc: {Symbol: "USDC", Decimals: 6}}})
	mint := Step{Kind: KindTokenTransfer, From: zeroAddress, To: alice, Token: &TokenAmount{Address: usdc, Symbol: "USDC", Amount: "3", Raw: "3000000"}}
	assert.Equal(t, "3 USDC were created (minted) for 0x2222…2222", stepText(mint, n, ""))
	burn := Step{Kind: KindTokenTransfer, From: alice, To: zeroAddress, Token: &TokenAmount{Address: usdc, Symbol: "USDC", Amount: "3", Raw: "3000000"}}
	assert.Equal(t, "0x2222…2222 destroyed (burned) 3 USDC", stepText(burn, n, ""))
	nft := Step{Kind: KindTokenTransfer, From: alice, To: sender, Token: &TokenAmount{Address: impl, Standard: "ERC721", Raw: "1", TokenID: ptr("42")}}
	assert.Equal(t, "0x2222…2222 sent tokens of 0x5555…5555 #42 to 0x1111…1111", stepText(nft, n, ""))
}

func TestDecodeInputs(t *testing.T) {
	str := "hello"
	params := []abiParam{
		{Name: "a", Type: "int256"},
		{Name: "pt", Type: "tuple", Components: []abiParam{{Type: "uint8"}, {Type: "uint8"}}},
		{Name: "s", Type: "string"},
		{Name: "ok", Type: "bool"},
		{Name: "b4", Type: "bytes4"},
	}
	minus2 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(2))
	data := word(minus2) + word(1) + word(2) + word(6*32) + word(1) + "deadbeef" + strings.Repeat("0", 56) +
		word(len(str)) + hex.EncodeToString([]byte(str)) + strings.Repeat("0", 64-2*len(str))
	raw := ethcommon.FromHex(data)
	args := decodeInputs(params, raw)
	require.Len(t, args, 5)
	assert.Equal(t, "-2", args[0].Value)
	assert.Equal(t, "(uint8,uint8)", args[1].Type)
	assert.Equal(t, "", args[1].Value)
	assert.Equal(t, "hello", args[2].Value)
	assert.Equal(t, "true", args[3].Value)
	assert.Equal(t, "0xdeadbeef", args[4].Value)

	// Truncated data yields fewer args, not a panic.
	assert.Len(t, decodeInputs(params, raw[:40]), 1)
	// A bogus offset for a string leaves its value empty.
	bad := ethcommon.FromHex(word(1 << 40))
	assert.Equal(t, []Arg{{Name: "s", Type: "string", Value: ""}}, decodeInputs([]abiParam{{Name: "s", Type: "string"}}, bad))
}

func TestParseABIStringForm(t *testing.T) {
	quoted, _ := json.Marshal(string(vaultABI))
	idx := parseABI(quoted)
	_, ok := idx.function("0x" + selectorOf("deposit(address,uint256)"))
	assert.True(t, ok)
	assert.Empty(t, parseABI(json.RawMessage(`{not json`)).functions)
}

func TestFormatUnits(t *testing.T) {
	for _, c := range []struct {
		v    string
		d    int
		want string
	}{{"1500000", 6, "1.5"}, {"1", 18, "0.000000000000000001"}, {"1000", 0, "1000"}, {"0", 6, "0"}, {"-25", 1, "-2.5"}} {
		v, _ := new(big.Int).SetString(c.v, 10)
		assert.Equal(t, c.want, formatUnits(v, c.d), c.v)
	}
}

func TestMarkdown(t *testing.T) {
	in := complexInput()
	in.Contracts[vault].ContractName = ptr("Vault_[v2]*")
	md := Build(in, Options{}).Markdown()
	assert.Contains(t, md, "## Flow\n\n")
	assert.Contains(t, md, "\n1. The sender 0x1111…1111 called deposit on Vault\\_\\[v2\\]\\*\n")
	assert.Contains(t, md, "\n  2. Vault\\_\\[v2\\]\\* ran deposit")
	assert.Contains(t, md, "> Events are shown")
}

// fakeSource implements Source over fixed data.
type fakeSource struct {
	in          Input
	internalErr error
	logsErr     error
	mu          sync.Mutex
	lookups     int
}

func (f *fakeSource) GetTransaction(_ context.Context, hash string) (*types.Transaction, error) {
	if hash != f.in.Tx.Hash {
		return nil, nil
	}
	return &f.in.Tx, nil
}
func (f *fakeSource) GetInternalTransactionsByTx(context.Context, string) ([]types.InternalTransaction, error) {
	return f.in.Internal, f.internalErr
}
func (f *fakeSource) GetLogsByTransaction(context.Context, string) ([]types.Log, error) {
	return f.in.Logs, f.logsErr
}
func (f *fakeSource) GetTransfersByTransaction(context.Context, string) ([]types.TokenTransfer, error) {
	return f.in.Transfers, nil
}
func (f *fakeSource) GetContract(_ context.Context, a string) (*types.Contract, error) {
	f.mu.Lock()
	f.lookups++
	f.mu.Unlock()
	return f.in.Contracts[a], nil
}
func (f *fakeSource) GetToken(_ context.Context, a string) (*types.Token, error) {
	if t := f.in.Tokens[a]; t != nil {
		return t, nil
	}
	return nil, errors.New("not a token")
}

func TestLoad(t *testing.T) {
	src := &fakeSource{in: complexInput()}
	f, err := Load(context.Background(), src, "0xabc", Options{})
	require.NoError(t, err)
	assert.Equal(t, "Vault reported Deposited (user: the sender 0x1111…1111, amount: 5000000)", f.Steps[len(f.Steps)-1].Text)
	assert.Equal(t, "USDC", f.Steps[3].Token.Symbol)

	_, err = Load(context.Background(), src, "0xother", Options{})
	assert.ErrorIs(t, err, ErrNotFound)

	src.internalErr = errors.New("indexer down")
	f, err = Load(context.Background(), src, "0xabc", Options{})
	require.NoError(t, err)
	assert.False(t, f.HasCallData)
	assert.Contains(t, strings.Join(f.Notes, " "), "Internal calls could not be read")
	assert.NotContains(t, strings.Join(f.Notes, " "), "No internal calls are recorded", "the two explanations exclude each other")

	src.internalErr, src.logsErr = nil, errors.New("boom")
	_, err = Load(context.Background(), src, "0xabc", Options{})
	assert.Error(t, err)
}

func TestLoadBoundsMetadataLookups(t *testing.T) {
	var internal []types.InternalTransaction
	for i := 0; i < 300; i++ {
		to := fmt.Sprintf("0x%040x", 0x1000+i)
		internal = append(internal, types.InternalTransaction{TraceAddress: fmt.Sprint(i), From: vault, To: ptr(to), CallType: "call", Value: "0"})
	}
	src := &fakeSource{in: Input{Tx: types.Transaction{Hash: "0xabc", From: sender, To: ptr(vault), Value: "0", Status: 1}, Internal: internal}}
	_, err := Load(context.Background(), src, "0xabc", Options{})
	require.NoError(t, err)
	assert.Equal(t, maxLookups, src.lookups)
}

func TestDynamicBytesHostileOffsets(t *testing.T) {
	str := []abiParam{{Name: "s", Type: "string"}}
	maxInt := new(big.Int).SetUint64(1<<63 - 1)
	maxWord := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	cases := map[string]string{
		"offset near MaxInt64":   word(maxInt),
		"offset 2^256-1":         word(maxWord),
		"length near MaxInt64":   word(32) + word(maxInt),
		"length past the data":   word(32) + word(64) + strings.Repeat("00", 32),
		"offset leaves <32 left": word(40) + strings.Repeat("00", 20),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			var args []Arg
			assert.NotPanics(t, func() { args = decodeInputs(str, ethcommon.FromHex(data)) })
			require.Len(t, args, 1)
			assert.Equal(t, "", args[0].Value)
		})
	}
}

func TestArraysAndHugeStaticParams(t *testing.T) {
	params := []abiParam{
		{Name: "ids", Type: "uint256[]"},
		{Name: "pair", Type: "int256[2]"},
		{Name: "after", Type: "address"},
	}
	data := ethcommon.FromHex(word(4*32) + word(7) + word(8) + word(alice) + word(1) + word(99))
	args := decodeInputs(params, data)
	require.Len(t, args, 3)
	assert.Equal(t, "", args[0].Value, "a dynamic array's head word is an offset, not its value")
	assert.Equal(t, "", args[1].Value, "a fixed array is not its first element")
	assert.Equal(t, alice, args[2].Value, "a fixed array still advances the head by its size")

	// Nested fixed arrays whose head size would overflow are not decoded at all.
	huge := []abiParam{{Name: "m", Type: "uint256[1024][1024][1024][1024][1024][1024][1024]"}, {Name: "x", Type: "uint256"}}
	assert.NotPanics(t, func() { assert.Empty(t, decodeInputs(huge, data)) })
}

func TestRepeatsKeepDifferentValuesApart(t *testing.T) {
	ev := func(idx, amount int) types.Log {
		return types.Log{LogIndex: idx, Address: vault, Topic0: ptr(topicOf("Deposited(address,uint256)")), Topic1: ptr("0x" + word(sender)), Data: word(amount)}
	}
	in := Input{
		Tx:        types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "aabbccdd", Status: 1},
		Logs:      []types.Log{ev(0, 1), ev(1, 2), ev(2, 2)},
		Contracts: map[string]*types.Contract{vault: {Address: vault, IsVerified: true, ContractName: ptr("Vault"), ABI: vaultABI}},
	}
	f := Build(in, Options{})
	require.Len(t, f.Steps, 3)
	assert.Equal(t, "Vault reported Deposited (user: the sender 0x1111…1111, amount: 1)", f.Steps[1].Text)
	assert.Equal(t, "Vault reported Deposited (user: the sender 0x1111…1111, amount: 2) (2 times in a row)", f.Steps[2].Text)
	assert.Equal(t, "2", f.Steps[2].Args[1].Value, "a merged step keeps its arguments")
}

func TestDelegatecallValueIsNotAPayment(t *testing.T) {
	f := Build(Input{
		Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "1000000000000000000", Status: 1},
		Internal: []types.InternalTransaction{
			{TraceAddress: "0", From: vault, To: ptr(impl), CallType: "delegatecall", Value: "1000000000000000000"},
		},
	}, Options{})
	require.Len(t, f.Steps, 2)
	assert.Equal(t, KindCall, f.Steps[1].Kind)
	assert.Empty(t, f.Steps[1].Value)
	assert.NotContains(t, f.Steps[1].Text, "ETH")
}

func TestLogAfterReturnedCallMovesOn(t *testing.T) {
	// The router pulls tokens (log 0), reports its own event (log 1), then sends tokens
	// on (log 2). Log 2 belongs to the second token call, not the returned first one,
	// and log 1 comes between the two calls.
	in := Input{
		Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "12345678", Status: 1},
		Internal: []types.InternalTransaction{
			{TraceAddress: "0", From: vault, To: ptr(usdc), CallType: "call", Value: "0", Input: ptr("0x" + selectorOf("transferFrom(address,address,uint256)") + word(sender) + word(vault) + word(1))},
			{TraceAddress: "1", From: vault, To: ptr(usdc), CallType: "call", Value: "0", Input: ptr("0x" + selectorOf("transfer(address,uint256)") + word(alice) + word(1))},
		},
		Logs: []types.Log{
			transferLog(0, usdc, sender, vault, 1),
			{LogIndex: 1, Address: vault, Topic0: ptr(topicOf("Routed(uint256)")), Data: word(1)},
			transferLog(2, usdc, vault, alice, 1),
		},
	}
	f := Build(in, Options{})
	var order []string
	for _, s := range f.Steps {
		order = append(order, fmt.Sprintf("%s@%d", s.Method, s.Depth))
	}
	assert.Equal(t, []string{"0x12345678@0", "transferFrom@1", "Transfer@1", topicOf("Routed(uint256)") + "@0", "transfer@1", "Transfer@1"}, order)
}

func TestMarkdownEscapesSummary(t *testing.T) {
	in := complexInput()
	in.Contracts[vault].ContractName = ptr("V*ult")
	md := Build(in, Options{}).Markdown()
	assert.Contains(t, md, "called deposit on V\\*ult. It led to")
}

// order renders each step as method@depth, for placement tests.
func order(f *Flow) []string {
	var out []string
	for _, s := range f.Steps {
		out = append(out, fmt.Sprintf("%s@%d", s.Method, s.Depth))
	}
	return out
}

func call(trace, from, to, input string) types.InternalTransaction {
	return types.InternalTransaction{TraceAddress: trace, From: from, To: ptr(to), CallType: "call", Value: "0", Input: ptr("0x" + input)}
}

func eventLog(idx int, addr, sig string) types.Log {
	return types.Log{LogIndex: idx, Address: addr, Topic0: ptr(topicOf(sig)), Data: word(1)}
}

func TestOuterEventComesBeforeNestedCall(t *testing.T) {
	// The vault reports Withdrawing, then calls a strategy that calls the token. The
	// vault does not run while the strategy does, so its event comes before that call.
	strategy := impl
	in := Input{
		Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "12345678", Status: 1},
		Internal: []types.InternalTransaction{
			call("0", vault, strategy, "aabbccdd"),
			call("0,0", strategy, usdc, selectorOf("transfer(address,uint256)")+word(alice)+word(1)),
		},
		Logs: []types.Log{eventLog(0, vault, "Withdrawing(uint256)"), transferLog(1, usdc, strategy, alice, 1)},
	}
	assert.Equal(t, []string{"0x12345678@0", topicOf("Withdrawing(uint256)") + "@0", "0xaabbccdd@1", "transfer@2", "Transfer@2"}, order(Build(in, Options{})))
}

func TestPullThenPayKeepsEachTransferInItsCall(t *testing.T) {
	// Two calls to the same token, a Transfer each, nothing in between.
	in := Input{
		Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "12345678", Status: 1},
		Internal: []types.InternalTransaction{
			call("0", vault, usdc, selectorOf("transferFrom(address,address,uint256)")+word(sender)+word(vault)+word(1)),
			call("1", vault, usdc, selectorOf("transfer(address,uint256)")+word(alice)+word(1)),
		},
		Logs: []types.Log{transferLog(0, usdc, sender, vault, 1), transferLog(1, usdc, vault, alice, 1)},
	}
	assert.Equal(t, []string{"0x12345678@0", "transferFrom@1", "Transfer@1", "transfer@1", "Transfer@1"}, order(Build(in, Options{})))
}

func TestReentrantCallbackEvents(t *testing.T) {
	// vault -> a -> b -> a again (a callback). b reports Before, the callback reports
	// Callback, then b reports After once the callback returned.
	a, b := impl, oracle
	in := Input{
		Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "12345678", Status: 1},
		Internal: []types.InternalTransaction{
			call("0", vault, a, "aaaaaaaa"),
			call("0,0", a, b, "bbbbbbbb"),
			call("0,0,0", b, a, "cccccccc"),
		},
		Logs: []types.Log{eventLog(0, b, "Before()"), eventLog(1, a, "Callback()"), eventLog(2, b, "After()")},
	}
	assert.Equal(t, []string{
		"0x12345678@0", "0xaaaaaaaa@1", "0xbbbbbbbb@2", topicOf("Before()") + "@2",
		"0xcccccccc@3", topicOf("Callback()") + "@3", topicOf("After()") + "@2",
	}, order(Build(in, Options{})))
}

func TestReadOnlyCallsEmitNothing(t *testing.T) {
	// A staticcall to the token cannot have emitted its Transfer; the later call did.
	in := Input{
		Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "12345678", Status: 1},
		Internal: []types.InternalTransaction{
			{TraceAddress: "0", From: vault, To: ptr(usdc), CallType: "staticcall", Value: "0", Input: ptr("0x" + selectorOf("balanceOf(address)") + word(vault))},
			call("1", vault, usdc, selectorOf("transfer(address,uint256)")+word(alice)+word(1)),
		},
		Logs: []types.Log{transferLog(0, usdc, vault, alice, 1)},
	}
	assert.Equal(t, []string{"0x12345678@0", "balanceOf@1", "transfer@1", "Transfer@1"}, order(Build(in, Options{})))
}

func TestRepeatedCallsWithDifferentArgsStayApart(t *testing.T) {
	transfer := func(trace string, amount int) types.InternalTransaction {
		return call(trace, vault, usdc, selectorOf("transfer(address,uint256)")+word(alice)+word(amount))
	}
	in := Input{
		Tx:       types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "12345678", Status: 1},
		Internal: []types.InternalTransaction{transfer("0", 1), transfer("1", 2), call("2", vault, oracle, "aaaaaaaa"+word(1)), call("3", vault, oracle, "aaaaaaaa"+word(2))},
	}
	f := Build(in, Options{})
	require.Len(t, f.Steps, 5, "same function and text, different calldata: nothing merges")
	assert.Equal(t, f.Steps[1].Text, f.Steps[2].Text)
	assert.Equal(t, "1", f.Steps[1].Args[1].Value)
	assert.Equal(t, "2", f.Steps[2].Args[1].Value)
	for _, s := range f.Steps {
		assert.Zero(t, s.Repeat)
	}
}

func TestRawItemCeiling(t *testing.T) {
	same := make([]types.InternalTransaction, 2*maxRawItems)
	for i := range same {
		same[i] = types.InternalTransaction{TraceAddress: fmt.Sprint(i), From: vault, To: ptr(oracle), CallType: "staticcall", Value: "0", Input: ptr("0x12345678")}
	}
	f := Build(Input{Tx: types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "aabbccdd", Status: 1}, Internal: same}, Options{})
	require.Len(t, f.Steps, 2)
	assert.Equal(t, maxRawItems-1, f.Steps[1].Repeat)
	assert.True(t, f.Truncated)
	assert.Equal(t, 2*maxRawItems+1, f.TotalSteps)
	assert.Contains(t, f.Notes[0], "Showing the first 2 steps")
}

func TestTransferBatchRespectsTheCap(t *testing.T) {
	batch := make([]types.TokenTransfer, 1000)
	for i := range batch {
		batch[i] = types.TokenTransfer{LogIndex: 0, TokenAddress: usdc, From: vault, To: alice, Value: "1", TokenType: "ERC1155", TokenID: ptr(fmt.Sprint(i))}
	}
	f := Build(Input{
		Tx:        types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", InputData: "aabbccdd", Status: 1},
		Logs:      []types.Log{eventLog(0, usdc, "TransferBatch(address,address,address,uint256[],uint256[])")},
		Transfers: batch,
	}, Options{})
	assert.Len(t, f.Steps, DefaultMaxSteps)
	assert.True(t, f.Truncated)
	assert.Equal(t, 1001, f.TotalSteps)
}

func TestDelegatecallWithoutInput(t *testing.T) {
	f := Build(Input{
		Tx:       types.Transaction{Hash: "0x1", From: sender, To: ptr(vault), Value: "0", Status: 1},
		Internal: []types.InternalTransaction{{TraceAddress: "0", From: vault, To: ptr(impl), CallType: "delegatecall", Value: "0"}},
	}, Options{})
	require.Len(t, f.Steps, 2)
	assert.Equal(t, "0x3333…3333 ran the code of 0x5555…5555", f.Steps[1].Text)
}

func TestLongStringsAreMarkedCut(t *testing.T) {
	long := strings.Repeat("a", maxDecodedBytes+10)
	data := word(32) + word(len(long)) + hex.EncodeToString([]byte(long)) + strings.Repeat("0", 64-2*(len(long)%32))
	args := decodeInputs([]abiParam{{Name: "s", Type: "string"}}, ethcommon.FromHex(data))
	require.Len(t, args, 1)
	assert.Equal(t, strings.Repeat("a", maxDecodedBytes)+"…", args[0].Value)

	short := word(32) + word(2) + "6869" + strings.Repeat("0", 60)
	assert.Equal(t, "hi", decodeInputs([]abiParam{{Name: "s", Type: "string"}}, ethcommon.FromHex(short))[0].Value)
}

func TestUnsupportedDecimalsLeaveAmountUnknown(t *testing.T) {
	assert.Equal(t, "", formatUnits(big.NewInt(1), 78))
	assert.Equal(t, "", formatUnits(big.NewInt(1), -1))
	f := Build(Input{
		Tx:        types.Transaction{Hash: "0x1", From: sender, To: ptr(usdc), Value: "0", Status: 1},
		Logs:      []types.Log{transferLog(0, usdc, sender, alice, 1)},
		Transfers: []types.TokenTransfer{{LogIndex: 0, TokenAddress: usdc, From: sender, To: alice, Value: "1", TokenType: "ERC20"}},
		Tokens:    map[string]*types.Token{usdc: {Address: usdc, Symbol: "ODD", Decimals: 78, TokenType: "ERC20"}},
	}, Options{})
	assert.Equal(t, "The sender 0x1111…1111 sent 1 units ODD to 0x2222…2222", f.Steps[len(f.Steps)-1].Text)
}

func TestLoadWarningCarriesNoHash(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Default()
	log.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer log.SetDefault(prev)

	hash := "0x" + strings.Repeat("ab", 32)
	in := complexInput()
	in.Tx.Hash = hash
	src := &fakeSource{in: in, internalErr: errors.New(`Get "http://proxy/api/v1/explorer/transactions/` + hash + `/internal": dial tcp: refused`)}
	_, err := Load(context.Background(), src, hash, Options{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "internal calls could not be read")
	assert.NotContains(t, strings.ToLower(buf.String()), strings.Repeat("ab", 32))
}
