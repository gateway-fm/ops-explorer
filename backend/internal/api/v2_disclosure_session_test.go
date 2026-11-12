//go:build privacy

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// This qualifies the restricted app-session store, fixed TLS forwarding and
// final output capture. Protected Core source semantics are tested by OPS.
func TestV2DisclosureActualSessionCheckedWireAndChangedCapture(t *testing.T) {
	f := newWalletAppFixture(t)
	const id = "11111111-1111-4111-8111-111111111111"
	const alias = "da1.render-test.3AVmeb8xFDUr-vAhQHJYIg"
	const body = `{"v":1,"caseId":"11111111-1111-4111-8111-111111111111","resource":"transactions","result":{"items":[{"value":"9007199254740993123456789"}],"nextCursor":null}}`
	f.onWallet = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/disclosure/cases/"+id+"/read" || r.URL.RawQuery != "" {
			t.Error("wrong fixed protected upstream")
		}
		raw, _ := io.ReadAll(r.Body)
		var query struct {
			Address  string `json:"addressId"`
			Resource string `json:"resource"`
			View     string `json:"view"`
		}
		if json.Unmarshal(raw, &query) != nil || query.Address != alias || query.Resource != "transactions" || query.View != "list" {
			t.Error("wrong checked opaque selection")
		}
		w.Header().Set("Set-Cookie", "not-forwarded=1")
		_, _ = w.Write([]byte(body))
	}
	response := f.call("GET", "/api/grant/"+id+"/"+alias+"/transactions", "")
	if response.Code != 200 || response.Body.String() != body || response.Header().Get("Set-Cookie") != "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("checked bytes not preserved", response.Code)
	}
	f.onWallet = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":"disclosure context changed"}`))
	}
	if response := f.call("GET", "/api/grant/"+id+"/"+alias+"/transactions", ""); response.Code != 409 || !strings.Contains(response.Body.String(), "disclosure context changed") {
		t.Fatal("case conflict not preserved", response.Code)
	}
	if response := f.call("GET", "/api/auth/status", ""); response.Code != 200 || !strings.Contains(response.Body.String(), `"authenticated":true`) {
		t.Fatal("case conflict changed app identity")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	defer closeIfOpen(release)
	f.onWallet = func(w http.ResponseWriter, _ *http.Request) { close(entered); <-release; _, _ = w.Write([]byte(body)) }
	response = httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.server.Handler().ServeHTTP(response, f.request("GET", "/api/grant/"+id+"/"+alias+"/transactions", nil))
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("protected read did not reach TLS upstream")
	}
	if result := f.call("POST", "/api/auth/logout", "{}"); result.Code != 200 {
		t.Fatal("durable logout", result.Code)
	}
	close(release)
	<-done
	if response.Code != 409 || strings.Contains(response.Body.String(), "9007199254740993123456789") {
		t.Fatal("old protected bytes escaped final capture", response.Code)
	}
}
