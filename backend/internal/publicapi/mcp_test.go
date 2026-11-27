package publicapi

// Contract under test: the MCP server at /mcp. A real MCP client (the SDK's
// Streamable HTTP client) connects over HTTP, lists the tools and calls each
// one; every tool must return exactly the Markdown of the matching `/llm.md`
// page. The key check, the 404 while no key is configured, error results and
// the shared rate limit are asserted directly.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"explorer/internal/api"
	"explorer/internal/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpTestKey = "test-key-for-unit-tests-only"

func mcpKeyHash(k string) string {
	sum := sha256.Sum256([]byte(k))
	return hex.EncodeToString(sum[:])
}

func (p *mdProvider) SearchSuggestions(context.Context, string, int) ([]types.SearchSuggestion, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	return []types.SearchSuggestion{
		{Type: "block", Value: "120", Label: "Block #120"},
		{Type: "transaction", Value: mdTxHash, Label: "0xaaaa…aaaa"},
		{Type: "address", Value: mdAddrA, Label: "0x1111…1111"},
		{Type: "token", Value: mdToken, Label: "TST|X [x](http://evil.example)"},
		{Type: "token", Value: "javascript:alert(1)", Label: "bad"}, // not an address: dropped
		{Type: "unknown", Value: mdAddrB, Label: "?"},               // unknown kind: dropped
	}, nil
}

// bearer adds the Authorization header to every request the client sends.
type bearer struct {
	key  string
	next http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.key)
	return b.next.RoundTrip(r)
}

func mcpTestServer(t *testing.T, p api.DataProvider, limit int, publicURL string) (*Server, *httptest.Server) {
	t.Helper()
	s := NewServer(p, nil, nil, 0, limit, time.Minute)
	s.SetSiteInfo(SiteInfo{NetworkName: "Example Testnet", CurrencySymbol: "tEX", PublicURL: publicURL})
	if err := s.SetMCPKeyHashes(mcpKeyHash("other-key") + ", " + strings.ToUpper(mcpKeyHash(mcpTestKey))); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)
	return s, ts
}

func mcpConnect(t *testing.T, endpoint, key string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             endpoint + "/mcp",
		HTTPClient:           &http.Client{Transport: bearer{key: key, next: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func mcpCall(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("%s: %d content items, want 1", name, len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s: content is %T, want text", name, res.Content[0])
	}
	return tc.Text, res.IsError
}

func TestMCP_ToolsMatchMarkdownPages(t *testing.T) {
	s, ts := mcpTestServer(t, &mdProvider{}, 1000, "https://explorer.example")
	cs := mcpConnect(t, ts.URL, mcpTestKey)

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %s is not marked read-only", tool.Name)
		}
	}
	sort.Strings(names)
	want := []string{"get_address", "get_block", "get_chain_stats", "get_contract", "get_token", "get_transaction", "list_address_transactions", "search"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("tools = %v, want %v", names, want)
	}

	cases := []struct {
		tool string
		args map[string]any
		page string
	}{
		{"get_transaction", map[string]any{"hash": mdTxHash}, "/tx/" + mdTxHash + "/llm.md"},
		{"get_block", map[string]any{"block": "120"}, "/block/120/llm.md"},
		{"get_block", map[string]any{"block": mdBlkHsh}, "/block/" + mdBlkHsh + "/llm.md"},
		{"get_address", map[string]any{"address": mdAddrA}, "/address/" + mdAddrA + "/llm.md"},
		{"get_address", map[string]any{"address": mdAddrA, "cursor": "c1"}, "/address/" + mdAddrA + "/llm.md?cursor=c1"},
		{"list_address_transactions", map[string]any{"address": mdAddrA}, "/address/" + mdAddrA + "/llm.md"},
		{"list_address_transactions", map[string]any{"address": mdAddrA, "kind": "token_transfers"}, "/address/" + mdAddrA + "/llm.md?tab=transfers"},
		{"list_address_transactions", map[string]any{"address": mdAddrA, "kind": "internal"}, "/address/" + mdAddrA + "/llm.md?tab=internal"},
		{"list_address_transactions", map[string]any{"address": mdAddrA, "kind": "logs"}, "/address/" + mdAddrA + "/llm.md?tab=logs"},
		{"get_contract", map[string]any{"address": mdToken}, "/address/" + mdToken + "/llm.md?tab=contract"},
		{"get_token", map[string]any{"address": mdToken}, "/token/" + mdToken + "/llm.md"},
		{"get_token", map[string]any{"address": mdToken, "page": 2}, "/token/" + mdToken + "/llm.md?page=2"},
		{"get_chain_stats", nil, "/stats/llm.md"},
	}
	for _, c := range cases {
		got, isErr := mcpCall(t, cs, c.tool, c.args)
		page := mdGet(s, c.page)
		if page.Code != http.StatusOK {
			t.Fatalf("%s: page %s returned %d", c.tool, c.page, page.Code)
		}
		if isErr {
			t.Errorf("%s %v: unexpected tool error:\n%s", c.tool, c.args, got)
		}
		if got != page.Body.String() {
			t.Errorf("%s %v differs from %s:\n--- tool ---\n%s\n--- page ---\n%s", c.tool, c.args, c.page, got, page.Body.String())
		}
	}
}

func TestMCP_Search(t *testing.T) {
	_, ts := mcpTestServer(t, &mdProvider{}, 1000, "https://explorer.example")
	cs := mcpConnect(t, ts.URL, mcpTestKey)
	got, isErr := mcpCall(t, cs, "search", map[string]any{"query": "TST"})
	if isErr {
		t.Fatalf("search returned a tool error:\n%s", got)
	}
	checkGolden(t, "mcp_search", got)
	// The bad match is dropped; the label's link syntax stays escaped text.
	if strings.Contains(got, "javascript:") || !strings.Contains(got, `\[x\](http://evil.example)`) {
		t.Errorf("search output carries an unchecked link:\n%s", got)
	}

	got, isErr = mcpCall(t, cs, "search", map[string]any{"query": "  "})
	if !isErr || !strings.Contains(got, "Bad request") {
		t.Errorf("empty query: isError=%v, text:\n%s", isErr, got)
	}
}

func TestMCP_ToolErrors(t *testing.T) {
	_, ts := mcpTestServer(t, &mdProvider{}, 1000, "https://explorer.example")
	cs := mcpConnect(t, ts.URL, mcpTestKey)
	for _, c := range []struct {
		tool string
		args map[string]any
		want string
	}{
		{"get_transaction", map[string]any{"hash": "0x12"}, "Bad request"},
		{"get_transaction", map[string]any{"hash": "0x" + strings.Repeat("d", 64)}, "Not found"},
		{"get_address", map[string]any{"address": "nope"}, "Bad request"},
		{"list_address_transactions", map[string]any{"address": mdAddrA, "kind": "everything"}, "Bad request"},
		{"get_block", map[string]any{"block": "-1"}, "Bad request"},
	} {
		got, isErr := mcpCall(t, cs, c.tool, c.args)
		if !isErr || !strings.HasPrefix(got, "# "+c.want) {
			t.Errorf("%s %v: isError=%v, want %q, got:\n%s", c.tool, c.args, isErr, c.want, got)
		}
	}

	// A data-source failure is a tool error without the internal details.
	_, ts2 := mcpTestServer(t, &mdProvider{fail: errors.New("dial tcp 10.0.0.1:9090: refused")}, 1000, "https://explorer.example")
	cs2 := mcpConnect(t, ts2.URL, mcpTestKey)
	got, isErr := mcpCall(t, cs2, "get_transaction", map[string]any{"hash": mdTxHash})
	if !isErr || !strings.Contains(got, "Temporarily unavailable") || strings.Contains(got, "10.0.0.1") {
		t.Errorf("source failure: isError=%v, text:\n%s", isErr, got)
	}
}

func TestMCP_LinksUseRequestOrigin(t *testing.T) {
	// Without a configured public URL, links use the origin the client
	// called, as on the /llm.md pages.
	_, ts := mcpTestServer(t, &mdProvider{}, 1000, "")
	cs := mcpConnect(t, ts.URL, mcpTestKey)
	got, _ := mcpCall(t, cs, "get_transaction", map[string]any{"hash": mdTxHash})
	if !strings.Contains(got, "]("+ts.URL+"/block/120/llm.md)") {
		t.Errorf("links do not use %s:\n%s", ts.URL, got)
	}
}

const mcpInitBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}`

func mcpPost(t *testing.T, url, auth string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url+"/mcp", strings.NewReader(mcpInitBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestMCP_RequiresKey(t *testing.T) {
	_, ts := mcpTestServer(t, &mdProvider{}, 1000, "")
	for _, auth := range []string{"", "Bearer", "Bearer ", "Bearer wrong-key", "Basic " + mcpTestKey, mcpTestKey, "Bearer " + mcpKeyHash(mcpTestKey)} {
		resp := mcpPost(t, ts.URL, auth)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Authorization %q: status %d, want 401", auth, resp.StatusCode)
		}
		if !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("Authorization %q: WWW-Authenticate = %q", auth, resp.Header.Get("WWW-Authenticate"))
		}
	}
	if resp := mcpPost(t, ts.URL, "Bearer "+mcpTestKey); resp.StatusCode != http.StatusOK {
		t.Errorf("valid key: status %d, want 200", resp.StatusCode)
	}
	if resp := mcpPost(t, ts.URL, "bearer "+mcpKeyHash("x")); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unlisted key: status %d, want 401", resp.StatusCode)
	}
}

func TestMCP_OffWithoutKeys(t *testing.T) {
	s := NewServer(&mdProvider{}, nil, nil, 0, 1000, time.Minute)
	if s.MCPEnabled() {
		t.Fatal("MCP enabled with no keys")
	}
	ts := httptest.NewServer(s.router)
	defer ts.Close()
	if resp := mcpPost(t, ts.URL, "Bearer "+mcpTestKey); resp.StatusCode != http.StatusNotFound {
		t.Errorf("no keys configured: status %d, want 404", resp.StatusCode)
	}
}

func TestMCP_KeyHashParsing(t *testing.T) {
	s := NewServer(&mdProvider{}, nil, nil, 0, 1000, time.Minute)
	if err := s.SetMCPKeyHashes(" , "); err != nil || s.MCPEnabled() {
		t.Errorf("blank list: err=%v enabled=%v", err, s.MCPEnabled())
	}
	secretish := "not-a-hash-value-1234"
	err := s.SetMCPKeyHashes(mcpKeyHash("a") + "," + secretish)
	if err == nil {
		t.Fatal("accepted a value that is not a SHA-256 digest")
	}
	if strings.Contains(err.Error(), secretish) {
		t.Errorf("error echoes the configured value: %v", err)
	}
	if err := s.SetMCPKeyHashes(mcpKeyHash("a")[:62]); err == nil {
		t.Error("accepted a short digest")
	}
}

func TestMCP_RateLimited(t *testing.T) {
	// /mcp shares the public API's per-client limit, and requests without a
	// valid key count too.
	_, ts := mcpTestServer(t, &mdProvider{}, 2, "")
	if resp := mcpPost(t, ts.URL, ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("first request: %d", resp.StatusCode)
	}
	resp := mcpPost(t, ts.URL, "Bearer "+mcpTestKey)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("second request: %d, remaining %q", resp.StatusCode, resp.Header.Get("X-RateLimit-Remaining"))
	}
	if resp := mcpPost(t, ts.URL, "Bearer "+mcpTestKey); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("third request: %d, want 429", resp.StatusCode)
	}
}

func TestMCP_PlainToolsListWithoutSession(t *testing.T) {
	// The documented curl check: one tools/list POST, no initialize first.
	_, ts := mcpTestServer(t, &mdProvider{}, 1000, "")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+mcpTestKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") || !strings.Contains(string(body), `"get_block"`) {
		t.Errorf("tools/list: %d %s\n%s", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
}
