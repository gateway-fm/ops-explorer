package publicapi

// Contract under test: the `/llm.md` Markdown pages. Each page type renders a
// golden file from fixed provider data (regenerate with
// `go test ./internal/publicapi -run TestLLMMD -update` and review the diff);
// the status codes, content type, escaping of chain-supplied text, link
// origin and rate limiting are asserted directly.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"explorer/internal/api"
	"explorer/internal/types"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

const (
	mdAddrA  = "0x1111111111111111111111111111111111111111"
	mdAddrB  = "0x2222222222222222222222222222222222222222"
	mdToken  = "0x3333333333333333333333333333333333333333"
	mdTxHash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	mdBlkHsh = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func sp(s string) *string { return &s }
func up(v uint64) *uint64 { return &v }

// mdProvider serves one small, fixed chain. Unknown keys return nil (not found).
type mdProvider struct {
	api.DataProvider
	fail        error // when set, every read fails with it
	statsErr    error // GetChainStats fails
	contractErr error // GetContract fails
	blockTxs    int   // block header transaction count; 0 means 1
	holderTotal int64 // token holder total; 0 means 1
}

func (p *mdProvider) block() *types.Block {
	return &types.Block{
		Number: 120, Hash: mdBlkHsh, ParentHash: "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Timestamp: 1764248400, GasUsed: 50000, GasLimit: 100000, BaseFeePerGas: up(100000000),
		TransactionCount: max(p.blockTxs, 1), Size: 812, Difficulty: "1", TotalDifficulty: "121", Nonce: "0x0000000000000000",
		Miner: mdAddrB, ExtraData: "0x", StateRoot: "0x01", TransactionsRoot: "0x02", ReceiptsRoot: "0x03",
	}
}

func (p *mdProvider) tx() *types.Transaction {
	return &types.Transaction{
		Hash: mdTxHash, BlockNumber: 120, BlockTimestamp: 1764248400, TxIndex: 0,
		From: mdAddrA, To: sp(mdToken), Value: "1500000000000000000",
		GasUsed: 21000, GasPrice: 2000000000, GasLimit: up(42000), MaxFeePerGas: up(3000000000),
		MaxPriorityFeePerGas: up(1000000000), Nonce: up(7), TxType: 2,
		InputData: "0xa9059cbb0000000000000000000000002222222222222222222222222222222222222222", Status: 1,
		TxCategories: []string{types.TxCategoryContractCall, types.TxCategoryTokenTransfer},
	}
}

func (p *mdProvider) token() *types.Token {
	return &types.Token{
		Address: mdToken, Symbol: "TST|X", Name: sp("Test [Token](http://evil.example) *bold*"), Decimals: 6,
		TokenType: "ERC20", TotalSupply: sp("10000000"), HolderCount: 2, TransferCount: 3,
		BlockNumber: 100, CreationTx: sp(mdTxHash),
	}
}

func (p *mdProvider) GetChainStats(context.Context) (*types.ChainStats, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	if p.statsErr != nil {
		return nil, p.statsErr
	}
	// Only the fields the indexer fills in (no TotalTokens).
	return &types.ChainStats{TotalBlocks: 121, TotalTransactions: 3000, TotalAddresses: 40, AvgBlockTime: 0.25}, nil
}
func (p *mdProvider) GetBlocks(context.Context, int, *uint64) ([]types.Block, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	return []types.Block{*p.block()}, nil
}
func (p *mdProvider) GetBlock(_ context.Context, n uint64) (*types.Block, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	if n != 120 {
		return nil, nil
	}
	return p.block(), nil
}
func (p *mdProvider) GetBlockByHash(_ context.Context, h string) (*types.Block, error) {
	if h != mdBlkHsh {
		return nil, nil
	}
	return p.block(), nil
}
func (p *mdProvider) GetTransactionsByBlock(context.Context, uint64) ([]types.Transaction, error) {
	return []types.Transaction{*p.tx()}, nil
}
func (p *mdProvider) GetTransactionsWithCategories(context.Context, int, *uint64) ([]types.Transaction, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	return []types.Transaction{*p.tx()}, nil
}
func (p *mdProvider) GetTransactionsPaginatedWithCategories(_ context.Context, page, size int) ([]types.Transaction, int64, error) {
	return []types.Transaction{*p.tx()}, 60, nil
}
func (p *mdProvider) GetTransactionWithCategories(_ context.Context, h string) (*types.Transaction, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	if h != mdTxHash {
		return nil, nil
	}
	return p.tx(), nil
}
func (p *mdProvider) GetTransfersByTransaction(context.Context, string) ([]types.TokenTransfer, error) {
	return []types.TokenTransfer{{TxHash: mdTxHash, TokenAddress: mdToken, From: mdAddrA, To: mdAddrB, Value: "2500000", BlockNumber: 120, TokenType: "ERC20"}}, nil
}
func (p *mdProvider) GetInternalTransactionsByTx(context.Context, string) ([]types.InternalTransaction, error) {
	return []types.InternalTransaction{{TxHash: mdTxHash, From: mdToken, To: sp(mdAddrB), Value: "0", CallType: "call"}}, nil
}
func (p *mdProvider) GetLogsByTransaction(context.Context, string) ([]types.Log, error) {
	return []types.Log{{TxHash: mdTxHash, LogIndex: 0, Address: mdToken,
		Topic0: sp("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"), Data: "0x2625a0"}}, nil
}
func (p *mdProvider) GetAddressStats(_ context.Context, a string) (*types.AddressStats, error) {
	return &types.AddressStats{Address: a, TxCount: 5, TokenTransferCount: 2, InternalTxCount: 1}, nil
}
func (p *mdProvider) GetBalance(context.Context, string) (*types.JSONString, error) {
	v := types.JSONString("2000000000000000000")
	return &v, nil
}
func (p *mdProvider) GetCode(_ context.Context, a string) ([]byte, error) {
	if a == mdToken {
		return []byte{0x60, 0x80}, nil
	}
	return nil, nil
}
func (p *mdProvider) GetTokenBalances(context.Context, string) ([]types.Balance, error) {
	return []types.Balance{
		{TokenAddress: mdToken, Balance: "2500000"},
		{TokenAddress: mdAddrB, Balance: "0"},
	}, nil
}
func (p *mdProvider) GetTransactionsByAddress(_ context.Context, _ string, _ int, cursor string, _ *uint64) ([]types.Transaction, string, error) {
	if cursor != "" {
		return nil, "", nil
	}
	return []types.Transaction{*p.tx()}, "c2", nil
}
func (p *mdProvider) GetToken(_ context.Context, a string) (*types.Token, error) {
	if a != mdToken {
		return nil, nil
	}
	return p.token(), nil
}
func (p *mdProvider) GetContract(_ context.Context, a string) (*types.Contract, error) {
	if p.contractErr != nil {
		return nil, p.contractErr
	}
	if a != mdToken {
		return nil, nil
	}
	return &types.Contract{
		Address: mdToken, Bytecode: "0x6080", Creator: mdAddrA, CreationTx: mdTxHash, BlockNumber: 100,
		IsVerified: true, ContractName: sp("TestToken"), CompilerVersion: sp("v0.8.30+commit.73712a01"),
		OptimizationUsed: func() *bool { b := true; return &b }(), OptimizationRuns: func() *int { n := 200; return &n }(),
		EVMVersion: sp("cancun"), LicenseType: sp("MIT"),
		SourceCode: sp("contract TestToken {\n    // a ``` fence inside source must not close the block\n}\n\n"),
		ABI:        json.RawMessage(`[{"type":"function","name":"transfer","inputs":[{"type":"address"},{"type":"uint256"}]}, {"type":"event","name":"Transfer","inputs":[{"type":"address"},{"type":"address"},{"type":"uint256"}]}]`),
	}, nil
}
func (p *mdProvider) GetTokens(context.Context, int, int, string, string) ([]types.Token, int64, error) {
	return []types.Token{*p.token()}, 1, nil
}
func (p *mdProvider) GetTokenHolders(context.Context, string, int, int) ([]types.TokenHolder, int64, error) {
	// Percentage stays 0, as the indexer leaves it.
	return []types.TokenHolder{{Address: mdAddrB, Balance: "2500000"}}, max(p.holderTotal, 1), nil
}
func (p *mdProvider) GetTransfersByToken(context.Context, string, int, int) ([]types.TokenTransfer, int64, error) {
	return []types.TokenTransfer{{TxHash: mdTxHash, TokenAddress: mdToken, From: mdAddrA, To: mdAddrB, Value: "2500000", BlockNumber: 120, TokenType: "ERC20"}}, 1, nil
}
func (p *mdProvider) GetDailyStats(context.Context, time.Time, time.Time) ([]types.DailyStats, error) {
	// Only the fields the indexer fills in.
	return []types.DailyStats{
		{Date: "2026-11-25", TotalTransactions: 900, TotalBlocks: 340000, TotalGasUsed: 18900000, AvgGasPrice: 42000000000000, ActiveAddresses: 12, NewAddresses: 2},
		{Date: "2026-11-26", TotalTransactions: 1000, TotalBlocks: 345600, TotalGasUsed: 21000000, AvgGasPrice: 42000000000000, ActiveAddresses: 15, NewAddresses: 3},
	}, nil
}
func (p *mdProvider) GetTransfersByAddress(_ context.Context, _ string, _ int, cursor string, _ *uint64) ([]types.TokenTransfer, string, error) {
	if cursor == "bad" {
		return nil, "", status.Error(codes.InvalidArgument, "invalid cursor")
	}
	return []types.TokenTransfer{{TxHash: mdTxHash, TokenAddress: mdToken, From: mdAddrA, To: mdAddrB, Value: "2500000", BlockNumber: 120, TokenType: "ERC20"}}, "t2", nil
}
func (p *mdProvider) GetInternalTransactionsByAddress(context.Context, string, int, int) ([]types.InternalTransaction, int64, error) {
	return []types.InternalTransaction{{TxHash: mdTxHash, BlockNumber: 120, From: mdToken, To: sp(mdAddrA), Value: "1000", CallType: "call"}}, 30, nil
}
func (p *mdProvider) GetLogsByAddress(context.Context, string, int, int) ([]types.Log, int64, error) {
	return []types.Log{{TxHash: mdTxHash, LogIndex: 0, Address: mdToken, BlockNumber: 120,
		Topic0: sp("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"), Data: "0x2625a0"}}, 1, nil
}
func (p *mdProvider) GetGasPrices(context.Context, int) (*uint64, *uint64, *uint64, *uint64, error) {
	return up(100000000), up(150000000), up(200000000), up(100000000), nil
}

func mdServer(p api.DataProvider, limit int) *Server {
	s := NewServer(p, nil, nil, 0, limit, time.Minute)
	s.SetSiteInfo(SiteInfo{NetworkName: "Example Testnet", CurrencySymbol: "tEX", PublicURL: "https://explorer.example/"})
	return s
}

func mdGet(s *Server, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "203.0.113.50:1"
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "llmmd", name+".md")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", path, err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden file:\n--- got ---\n%s", name, got)
	}
}

func TestLLMMD_Pages(t *testing.T) {
	s := mdServer(&mdProvider{}, 100000)
	pages := map[string]string{
		"home":           "/llm.md",
		"blocks":         "/blocks/llm.md",
		"block":          "/block/120/llm.md",
		"block_hash":     "/block/" + mdBlkHsh + "/llm.md",
		"transactions":   "/transactions/llm.md",
		"tx":             "/tx/" + mdTxHash + "/llm.md",
		"wallet":         "/address/" + mdAddrA + "/llm.md",
		"contract":       "/address/" + mdToken + "/llm.md",
		"contract_tab":   "/address/" + mdToken + "/llm.md?tab=contract",
		"addr_transfers": "/address/" + mdAddrA + "/llm.md?tab=transfers",
		"addr_internal":  "/address/" + mdAddrA + "/llm.md?tab=internal",
		"addr_logs":      "/address/" + mdAddrA + "/llm.md?tab=logs",
		"tokens":         "/tokens/llm.md",
		"token":          "/token/" + mdToken + "/llm.md",
		"stats":          "/stats/llm.md",
		"llms_txt":       "/llms.txt",
	}
	for name, path := range pages {
		t.Run(name, func(t *testing.T) {
			w := mdGet(s, path)
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s: status %d, body %s", path, w.Code, w.Body)
			}
			ct := w.Header().Get("Content-Type")
			if name == "llms_txt" {
				if !strings.HasPrefix(ct, "text/plain") {
					t.Errorf("content type %q", ct)
				}
			} else if ct != "text/markdown; charset=utf-8" {
				t.Errorf("content type %q", ct)
			}
			checkGolden(t, name, w.Body.String())
		})
	}
}

func TestLLMMD_StatusCodes(t *testing.T) {
	s := mdServer(&mdProvider{}, 100000)
	cases := []struct {
		path   string
		status int
		want   string
	}{
		{"/nope/llm.md", 404, "# Not found"},
		{"/tx/llm.md", 404, "# Not found"},
		{"/tx/0x" + strings.Repeat("e", 64) + "/llm.md", 404, "No transaction"},
		{"/tx/0x123/llm.md", 400, "# Bad request"},
		{"/tx/0x" + strings.Repeat("A", 64) + "/llm.md", 200, "# Transaction " + mdTxHash},
		{"/block/999/llm.md", 404, "No block 999"},
		{"/block/abc/llm.md", 400, "# Bad request"},
		{"/address/0xnothex/llm.md", 400, "# Bad request"},
		{"/token/" + mdAddrA + "/llm.md", 404, "No token at"},
		{"/address/" + mdAddrA + "/llm.md?tab=contract", 404, "No contract at"},
		{"/address/" + mdAddrA + "/llm.md?tab=coin-history", 400, "no Markdown version"},
		{"/address/" + mdAddrA + "/llm.md?tab=transfers&cursor=bad", 400, "A query parameter is not valid"},
		{"/nope/llms.txt", 404, "# Not found"},
	}
	for _, c := range cases {
		w := mdGet(s, c.path)
		if w.Code != c.status {
			t.Errorf("GET %s: status %d, want %d", c.path, w.Code, c.status)
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
			t.Errorf("GET %s: content type %q", c.path, ct)
		}
		if !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("GET %s: body lacks %q:\n%s", c.path, c.want, w.Body)
		}
	}
	// Non-Markdown unknown paths keep the plain 404, .md files included.
	for _, path := range []string{"/nope", "/api/v1/README.md"} {
		if w := mdGet(s, path); w.Code != 404 || strings.Contains(w.Header().Get("Content-Type"), "markdown") {
			t.Errorf("%s: status %d, type %q", path, w.Code, w.Header().Get("Content-Type"))
		}
	}
}

func TestLLMMD_SourceFailureIsNotEchoed(t *testing.T) {
	s := mdServer(&mdProvider{fail: errors.New("dial tcp 10.0.0.7:50051: connection refused")}, 100000)
	for _, path := range []string{"/llm.md", "/tx/" + mdTxHash + "/llm.md", "/blocks/llm.md"} {
		w := mdGet(s, path)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s: status %d, want 503", path, w.Code)
		}
		if strings.Contains(w.Body.String(), "10.0.0.7") {
			t.Errorf("GET %s leaks the data-source error:\n%s", path, w.Body)
		}
	}
}

func TestLLMMD_EscapesChainText(t *testing.T) {
	s := mdServer(&mdProvider{}, 100000)
	body := mdGet(s, "/token/"+mdToken+"/llm.md").Body.String()
	if strings.Contains(body, "[Token](http://evil.example)") {
		t.Errorf("token name rendered as a live link:\n%s", body)
	}
	if !strings.Contains(body, `Test \[Token\](http://evil.example) \*bold\* (TST\|X)`) {
		t.Errorf("token name not escaped as expected:\n%s", body)
	}
	src := mdGet(s, "/address/"+mdToken+"/llm.md?tab=contract").Body.String()
	if !strings.Contains(src, "````solidity\n") {
		t.Errorf("source fence not lengthened past the ``` inside the source:\n%s", src)
	}
}

func TestLLMMD_RateLimited(t *testing.T) {
	s := mdServer(&mdProvider{}, 1)
	if w := mdGet(s, "/llms.txt"); w.Code != 200 {
		t.Fatalf("first request: %d", w.Code)
	}
	if w := mdGet(s, "/tx/"+mdTxHash+"/llm.md"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: %d, want 429 (shared public-API limit)", w.Code)
	}
}

func TestLLMMD_BaseURL(t *testing.T) {
	s := NewServer(&mdProvider{}, nil, nil, 0, 100000, time.Minute)
	cases := []struct {
		host, proto, want string
	}{
		{"explorer.example", "https", "https://explorer.example"},
		{"explorer.example:8080", "", "http://explorer.example:8080"},
		{"explorer.example", "https, http", "https://explorer.example"},
		{"evil.example)[x](y", "https", ""},
		{"explorer.example", "javascript", "http://explorer.example"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/llm.md", nil)
		r.Host = c.host
		if c.proto != "" {
			r.Header.Set("X-Forwarded-Proto", c.proto)
		}
		if got := s.baseURL(r); got != c.want {
			t.Errorf("host %q proto %q: got %q, want %q", c.host, c.proto, got, c.want)
		}
	}
}

func TestFormatUnits(t *testing.T) {
	cases := []struct {
		raw  string
		dec  int
		want string
	}{
		{"1500000000000000000", 18, "1.5"},
		{"1", 18, "0.000000000000000001"},
		{"0", 18, "0"},
		{"1000000", 6, "1"},
		{"0x10", 0, "16"},
		{"-25", 1, "-2.5"},
		{"not a number", 18, "not a number"},
	}
	for _, c := range cases {
		if got := formatUnits(c.raw, c.dec); got != c.want {
			t.Errorf("formatUnits(%q, %d) = %q, want %q", c.raw, c.dec, got, c.want)
		}
	}
}

// Missing data is reported as missing, never as zeros or "not verified".
func TestLLMMD_UnavailableDataIsNotZero(t *testing.T) {
	s := mdServer(&mdProvider{statsErr: errors.New("indexer down")}, 100000)
	for _, path := range []string{"/llm.md", "/stats/llm.md"} {
		body := mdGet(s, path).Body.String()
		if strings.Contains(body, "Total blocks") || !strings.Contains(body, mdStatsUnavailable) {
			t.Errorf("%s: totals not reported as unavailable:\n%s", path, body)
		}
	}
	body := mdGet(mdServer(&mdProvider{contractErr: errors.New("timeout")}, 100000), "/address/"+mdToken+"/llm.md").Body.String()
	if strings.Contains(body, "**Verified:** No") || !strings.Contains(body, "**Verified:** Unknown") {
		t.Errorf("contract read failure shown as a verification result:\n%s", body)
	}
}

func TestLLMMD_BlockCountsFromHeader(t *testing.T) {
	body := mdGet(mdServer(&mdProvider{blockTxs: 300}, 100000), "/block/120/llm.md").Body.String()
	if !strings.Contains(body, "## Transactions (300)") || !strings.Contains(body, "Showing the first 1 of 300 transactions.") {
		t.Errorf("block page does not count from the header:\n%s", body)
	}
}

func TestLLMMD_TokenPaging(t *testing.T) {
	s := mdServer(&mdProvider{holderTotal: 60}, 100000)
	if body := mdGet(s, "/token/"+mdToken+"/llm.md").Body.String(); !strings.Contains(body, "/llm.md?page=2)") {
		t.Errorf("no next-page link with 60 holders:\n%s", body)
	}
	if body := mdGet(s, "/token/"+mdToken+"/llm.md?page=3").Body.String(); strings.Contains(body, "?page=4") {
		t.Errorf("next-page link past the last page:\n%s", body)
	}
}

func TestMDCode(t *testing.T) {
	// A huge backtick run picks its fence in one pass, not one allocation per length.
	run := strings.Repeat("`", 100000)
	start := time.Now()
	d := newMDDoc("")
	d.code("", "x"+run+"y")
	if time.Since(start) > time.Second {
		t.Fatalf("fence selection took %s", time.Since(start))
	}
	if !strings.HasPrefix(d.String(), run+"`\n") {
		t.Errorf("fence is not one longer than the longest run")
	}
	// The body is kept byte for byte, trailing blank lines included.
	d = newMDDoc("")
	d.code("", "a\n\n")
	if got := d.String(); got != "```\na\n\n```\n" {
		t.Errorf("code body changed: %q", got)
	}
}
