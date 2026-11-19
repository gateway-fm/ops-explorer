//go:build privacy

package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
