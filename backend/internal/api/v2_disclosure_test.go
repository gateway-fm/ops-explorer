package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestV2DisclosureClosedConsumerMapping(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	const alias = "da1.render-test.3AVmeb8xFDUr-vAhQHJYIg"
	const hash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, test := range []struct{ path, resource, view, address, transaction string }{
		{"/grant/" + id + "/resolve/" + alias, "addresses", "resolve", alias, ""},
		{"/grant/" + id + "/" + alias + "/transactions?limit=7&cursor=opaque", "transactions", "list", alias, ""},
		{"/grant/" + id + "/" + alias + "/transactions/" + hash, "transactions", "detail", alias, hash},
		{"/grant/" + id + "/activity?limit=7&cursor=opaque", "activity_logs", "list", "", ""},
	} {
		t.Run(test.view+test.resource, func(t *testing.T) {
			method, path, raw, err := disclosureOperation(httptest.NewRequest("GET", "/api"+test.path, nil))
			if err != nil || method != "POST" || path != "/api/v1/disclosure/cases/"+id+"/read" {
				t.Fatal(method, path, err)
			}
			var body struct {
				V        int    `json:"v"`
				Resource string `json:"resource"`
				View     string `json:"view"`
				Address  string `json:"addressId"`
				Hash     string `json:"transactionHash"`
				Limit    int    `json:"limit"`
				Cursor   string `json:"cursor"`
			}
			if json.Unmarshal(raw, &body) != nil || body.V != 1 || body.Resource != test.resource || body.View != test.view || body.Address != test.address || body.Hash != test.transaction {
				t.Fatal(string(raw))
			}
			if test.view == "list" && (body.Limit != 7 || body.Cursor != "opaque") {
				t.Fatal("lost continuation", string(raw))
			}
		})
	}
	method, path, body, err := disclosureOperation(httptest.NewRequest("GET", "/api/v2/viewable-addresses?limit=50", nil))
	if err != nil || method != "GET" || path != "/api/v1/disclosure/cases?limit=50&role=recipient" || body != nil {
		t.Fatal(method, path, err)
	}
	request := httptest.NewRequest("POST", "/api/v1/disclosure/cases/"+id+"/read", strings.NewReader(`{"v":1,"resource":"compliance_report","view":"report"}`))
	request.Header.Set("Content-Type", "application/json")
	method, path, body, err = disclosureOperation(request)
	if err != nil || method != "POST" || path != "/api/v1/disclosure/cases/"+id+"/read" || string(body) != `{"v":1,"resource":"compliance_report","view":"report"}` {
		t.Fatal(method, path, err)
	}
}

func TestV2DisclosureRejectsLegacyAndAmbiguousInputs(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	const alias = "da1.render-test.3AVmeb8xFDUr-vAhQHJYIg"
	for _, path := range []string{
		"/api/disclosure/cases/1", "/api/grant/1/activity", "/api/grant/" + id + "/0x" + strings.Repeat("a", 40) + "/transactions",
		"/api/grant/" + id + "/" + alias + "/transactions?limit=1&limit=2", "/api/viewable-addresses?limit=01", "/api/viewable-addresses?limit=101",
		"/api/viewable-addresses?userId=1", "/api/grant/" + id + "/activity?cursor=" + strings.Repeat("x", 2049), "/api/grant/" + id + "/resolve/" + alias + "?limit=1",
		"/api/disclosure/cases/" + id + "/approve", "/api/disclosure/cases/" + id + "/read", "/api/disclosure/cases/trailing/extra/path",
	} {
		if _, _, _, err := disclosureOperation(httptest.NewRequest("GET", path, nil)); err == nil {
			t.Fatal("accepted", path)
		}
	}
	for _, body := range []string{"{}", strings.Repeat(" ", disclosureRequestLimit+1), "not json"} {
		request := httptest.NewRequest("POST", "/api/disclosure/cases", strings.NewReader(body))
		if body != "{}" {
			request.Header.Set("Content-Type", "application/json")
		}
		if _, _, _, err := disclosureOperation(request); err == nil {
			t.Fatal("accepted missing type/oversized/malformed body")
		}
	}
}
