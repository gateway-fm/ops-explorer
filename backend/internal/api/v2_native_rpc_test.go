//go:build privacy

package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNativeRPCFiniteEnvelope(t *testing.T) {
	for _, raw := range []string{
		`[]`, `{"jsonrpc":"2.0","id":null,"method":"eth_call","params":[]}`,
		`{"jsonrpc":"2.0","id":1,"method":"eth_sendTransaction","params":[]}`,
		`{"jsonrpc":"2.0","id":1,"id":2,"method":"eth_call","params":[]}`,
		`{"jsonrpc":"2.0","id":1,"method":"eth_call","params":{},"url":"https://evil.test"}`,
		`{"jsonrpc":"2.0","id":9007199254740992,"method":"eth_call","params":[]}`,
		`{"jsonrpc":"2.0","id":1.0,"method":"eth_call","params":[]}`,
		`{"jsonrpc":"2.0","method":"eth_call","params":[]}`,
	} {
		if _, e := nativeRPCRequest([]byte(raw)); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, id := range []string{`1`, `"abi-1"`, `-1`} {
		raw := []byte(`{"jsonrpc":"2.0","id":` + id + `,"method":"eth_call","params":[{"to":"0xabc","data":"0x"},"latest"]}`)
		got, e := nativeRPCRequest(raw)
		if e != nil || got != id {
			t.Fatal("read envelope", e)
		}
		if !nativeRPCResponse([]byte(`{"jsonrpc":"2.0","id":`+id+`,"result":"0x"}`), got) {
			t.Fatal("result")
		}
		if !nativeRPCResponse([]byte(`{"jsonrpc":"2.0","id":`+id+`,"error":{"code":-32000,"message":"denied"}}`), got) {
			t.Fatal("policy error")
		}
		for _, response := range []string{`{"jsonrpc":"2.0","id":null,"result":"0x"}`, `{"jsonrpc":"2.0","id":` + id + `,"result":"0x","error":{}}`, `{"jsonrpc":"2.0","id":` + id + `,"id":` + id + `,"result":"0x"}`} {
			if nativeRPCResponse([]byte(response), got) {
				t.Fatal("mismatched response accepted")
			}
		}
	}
}

func TestNativeRPCActualSessionOrgAndFinalFence(t *testing.T) {
	f := newWalletAppFixture(t)
	// Read/RPC operations own the read deadline rather than the wallet5s one.
	f.server.v2Reads.client.Transport = http.DefaultTransport
	const org = "11111111-1111-4111-8111-111111111111"
	const request = `{"jsonrpc":"2.0","id":"abi-1","method":"eth_call","params":[{"to":"0x1111111111111111111111111111111111111111","data":"0x12345678"},"latest"]}`
	const result = `{"jsonrpc":"2.0","id":"abi-1","result":"0x9007199254740993123456789"}`
	self := nativeOwnSelf{ActorID: "ops:p1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), PrincipalKind: "human", SourceProfile: "azure", SourceProfileDigest: "sha256:" + strings.Repeat("a", 64), ExpiresAt: time.Now().Unix() + 300, Organizations: []nativeOwnOrganization{{ID: org, Name: "Bank A", Slug: "bank-a"}}}
	f.onWallet = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "forbidden=1")
		if r.URL.Path == "/api/v2/self" {
			_ = json.NewEncoder(w).Encode(self)
			return
		}
		if r.URL.Path != "/rpc/"+org || r.Method != "POST" {
			t.Error("unexpected upstream")
		}
		raw, _ := io.ReadAll(r.Body)
		if string(raw) != request {
			t.Error("request changed")
		}
		_, _ = io.WriteString(w, result)
	}
	response := f.call("POST", "/api/v2/rpc/"+org, request)
	if response.Code != 200 || response.Body.String() != result || response.Header().Get("Set-Cookie") != "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("native result/headers", response.Code, response.Body.String())
	}
	before := f.effects.Load()
	if got := f.call("POST", "/api/v2/rpc/22222222-2222-4222-8222-222222222222", request); got.Code != 403 || f.effects.Load() != before+1 {
		t.Fatal("nonmember org reached RPC", got.Code)
	}
	before = f.effects.Load()
	for _, path := range []string{"/api/v2/rpc/" + org + "?url=https://evil.test", "/api/v2/rpc/%31" + org[1:]} {
		if got := f.call("POST", path, request); got.Code < 400 {
			t.Fatal("bad target accepted")
		}
	}
	if f.effects.Load() != before {
		t.Fatal("invalid target reached OPS")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	defer closeIfOpen(release)
	f.onWallet = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/self" {
			_ = json.NewEncoder(w).Encode(self)
			return
		}
		close(entered)
		<-release
		_, _ = io.WriteString(w, result)
	}
	out := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.server.Handler().ServeHTTP(out, f.request("POST", "/api/v2/rpc/"+org, strings.NewReader(request)))
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("held RPC did not forward")
	}
	if got := f.call("POST", "/api/auth/logout", `{}`); got.Code != 200 {
		t.Fatal("logout", got.Code)
	}
	close(release)
	<-done
	if out.Code != 409 || strings.Contains(out.Body.String(), "9007199254740993") {
		t.Fatal("stale private response escaped", out.Code)
	}
}

func TestNativeOwnGroupDisplayTupleValidation(t *testing.T) {
	group := `{"id":"22222222-2222-2222-2222-222222222222","name":"\ufeffReader\ufeff","slug":"reader"}`
	for _, tc := range []struct {
		name, groups, count, truncated string
		valid                          bool
	}{
		{name: "legacy", valid: true},
		{"full", `[` + group + `]`, "1", "false", true},
		{"truncated", `[` + group + `]`, "9000", "true", true},
		{"zero displayed", `[]`, "9000", "true", true},
		{"empty", `[]`, "0", "false", true},
		{"missing count", `[` + group + `]`, "", "false", false},
		{"all null", "null", "null", "null", false},
		{"null groups", "null", "1", "true", false},
		{"null count", `[]`, "null", "false", false},
		{"null flag", `[]`, "0", "null", false},
		{"fractional count", `[]`, "1.5", "true", false},
		{"negative count", `[]`, "-1", "true", false},
		{"oversized count", `[]`, "4294967296", "true", false},
		{"inconsistent count", `[` + group + `]`, "0", "false", false},
		{"inconsistent flag", `[` + group + `]`, "1", "true", false},
		{"duplicate rows", `[` + group + `,` + group + `]`, "2", "false", false},
		{"noncanonical id", strings.Replace(`[`+group+`]`, "22222222-2222-2222-2222-222222222222", "00000000-0000-0000-0000-000000000000", 1), "1", "false", false},
		{"whitespace name", strings.Replace(`[`+group+`]`, `\ufeffReader\ufeff`, `\u0085Reader`, 1), "1", "false", false},
		{"oversized name", strings.Replace(`[`+group+`]`, `\ufeffReader\ufeff`, strings.Repeat("a", 257), 1), "1", "false", false},
		{"private field", strings.Replace(`[`+group+`]`, `"slug":"reader"`, `"slug":"reader","allowedMethods":["eth_call"]`, 1), "1", "false", false},
		{"duplicate field", strings.Replace(`[`+group+`]`, `"slug":"reader"`, `"slug":"reader","slug":"other"`, 1), "1", "false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			displayed := 0
			org := nativeOwnOrganization{Groups: json.RawMessage(tc.groups), GroupCount: json.RawMessage(tc.count), GroupsTruncated: json.RawMessage(tc.truncated)}
			if nativeOwnGroupsValid(org, &displayed) != tc.valid {
				t.Fatal("unexpected display validation", tc.valid)
			}
		})
	}
	rows := make([]nativeOwnGroup, 256)
	for i := range rows {
		rows[i] = nativeOwnGroup{ID: fmt.Sprintf("%08x-2222-2222-2222-222222222222", i+1), Name: fmt.Sprintf("Group %d", i), Slug: fmt.Sprintf("group-%d", i)}
	}
	raw, _ := json.Marshal(rows)
	org := nativeOwnOrganization{Groups: raw, GroupCount: json.RawMessage(`256`), GroupsTruncated: json.RawMessage(`false`)}
	displayed := 0
	if !nativeOwnGroupsValid(org, &displayed) || displayed != 256 {
		t.Fatal("valid maximum display rejected")
	}
	one, _ := json.Marshal(rows[:1])
	org.Groups = one
	org.GroupCount = json.RawMessage(`1`)
	if nativeOwnGroupsValid(org, &displayed) {
		t.Fatal("whole-response display budget exceeded")
	}
}

func TestNativeSelfOwnGroupsActualSessionInterop(t *testing.T) {
	f := newWalletAppFixture(t)
	f.server.v2Reads.client.Transport = http.DefaultTransport
	const org = "11111111-1111-4111-8111-111111111111"
	const rpcRequest = `{"jsonrpc":"2.0","id":"own-groups","method":"eth_call","params":[]}`
	const rpcResult = `{"jsonrpc":"2.0","id":"own-groups","result":"0x01"}`
	actor := "ops:p1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	var rpcCalls int
	payload := ""
	f.onWallet = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/self" {
			_, _ = io.WriteString(w, payload)
			return
		}
		if r.URL.Path != "/rpc/"+org {
			t.Error("unexpected upstream", r.URL.Path)
		}
		rpcCalls++
		_, _ = io.WriteString(w, rpcResult)
	}
	prefix := fmt.Sprintf(`{"actorId":%q,"principalKind":"human","sourceProfile":"azure","sourceProfileDigest":"sha256:%s","expiresAt":%d,"organizations":[`, actor, strings.Repeat("a", 64), time.Now().Unix()+300)
	legacy := `{"id":"` + org + `","name":"Bank","slug":"bank","isOrgAdmin":false,"isOrgReadonlyAdmin":false}`
	projected := strings.TrimSuffix(legacy, "}") + `,"groups":[{"id":"22222222-2222-2222-2222-222222222222","name":"\ufeffReader\ufeff","slug":"reader"}],"groupCount":9000,"groupsTruncated":true}`
	for _, row := range []string{legacy, projected, strings.Replace(projected, `"groupCount":9000,"groupsTruncated":true`, `"groupCount":1,"groupsTruncated":false`, 1)} {
		payload = prefix + row + `]}`
		got := f.call("GET", "/api/v2/self", "")
		if got.Code != 200 || got.Body.String() != payload {
			t.Fatal("actual Self failed compatible payload", got.Code, got.Body.String())
		}
		got = f.call("POST", "/api/v2/rpc/"+org, rpcRequest)
		if got.Code != 200 || got.Body.String() != rpcResult {
			t.Fatal("actual native RPC rejected own display metadata", got.Code, got.Body.String())
		}
		before := rpcCalls
		got = f.call("POST", "/api/v2/rpc/22222222-2222-2222-2222-222222222222", rpcRequest)
		if got.Code != 403 || rpcCalls != before {
			t.Fatal("display group expanded org authority", got.Code)
		}
	}
	for _, row := range []string{
		strings.Replace(projected, `,"groupCount":9000`, "", 1),
		strings.Replace(projected, `"groupsTruncated":true`, `"groupsTruncated":false`, 1),
		strings.Replace(projected, `"groupCount":9000`, `"groupCount":9000,"groupCount":1`, 1),
		strings.Replace(projected, `"slug":"reader"`, `"slug":"reader","allowedMethods":["eth_call"]`, 1),
	} {
		payload = prefix + row + `]}`
		before := rpcCalls
		for _, operation := range []struct{ method, path, body string }{{"GET", "/api/v2/self", ""}, {"POST", "/api/v2/rpc/" + org, rpcRequest}} {
			got := f.call(operation.method, operation.path, operation.body)
			if got.Code != 503 || strings.Contains(got.Body.String(), "Reader") || rpcCalls != before {
				t.Fatal("malformed display tuple escaped or reached RPC", got.Code)
			}
		}
	}
}
