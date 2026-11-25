package api

// Sourcify handler tests (plan §2 + §5 seam). The handlers call package-level
// http against the Sourcify API; the new sourcifyHTTP + sourcifyBaseURL seam on
// Server (mirroring impersonationHTTP) lets us point them at an httptest server.
// Contract:
//   - chainId is ParseUint-validated to prevent SSRF (non-numeric -> 400);
//   - isLocalChain short-circuits BEFORE any network call (uses the provider);
//   - upstream 404 -> 404; non-200 -> that status; garbage JSON -> 500.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"explorer/internal/types"
)

// scStub is a provider for the Sourcify handlers.
type scStub struct {
	DataProvider
	chainID     uint64
	chainIDErr  error
	contract    *types.Contract
	contractErr error
	verifyErr   error

	verifiedSource string
}

func (s *scStub) GetChainID(context.Context) (uint64, error) { return s.chainID, s.chainIDErr }
func (s *scStub) GetContract(context.Context, string) (*types.Contract, error) {
	return s.contract, s.contractErr
}
func (s *scStub) VerifyContract(_ context.Context, _, _, _ string, _ bool, source string, _ json.RawMessage, _, _, _ string, _ int) error {
	s.verifiedSource = source
	return s.verifyErr
}

// scServer wires a Server with the Sourcify seam pointed at a mock upstream.
func scServer(t *testing.T, prov DataProvider, upstream http.HandlerFunc) *Server {
	t.Helper()
	var base string
	if upstream != nil {
		srv := httptest.NewServer(upstream)
		t.Cleanup(srv.Close)
		base = srv.URL
	} else {
		base = "http://sourcify.invalid" // must never be reached
	}
	return &Server{
		provider:        prov,
		sourcifyHTTP:    &http.Client{},
		sourcifyBaseURL: base,
	}
}

const scAddr = "0x52908400098527886E0F7030069857D2E4169EE7"

// --- chainId SSRF guard -----------------------------------------------------

func TestHandleCheckSourcify_InvalidChainId400(t *testing.T) {
	// A non-numeric chainId must be rejected (SSRF guard) BEFORE any network
	// call — upstream nil asserts no request is made.
	s := scServer(t, &scStub{}, nil)
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=evilhost", map[string]string{"address": scAddr})
	s.handleCheckSourcify(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 for non-numeric chainId", w.Code)
	}
}

func TestHandleFetchSourcify_InvalidChainId400(t *testing.T) {
	s := scServer(t, &scStub{}, nil)
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=notanumber", map[string]string{"address": scAddr})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestHandleFetchSourcify_InvalidAddress400(t *testing.T) {
	s := scServer(t, &scStub{}, nil)
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=1", map[string]string{"address": "nothex"})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 for bad address", w.Code)
	}
}

// --- isLocalChain short-circuit (no network) --------------------------------

func TestHandleCheckSourcify_LocalChainNoNetwork(t *testing.T) {
	// chainId 31337 (Hardhat) -> served from the provider, NO upstream call.
	prov := &scStub{contract: &types.Contract{IsVerified: true}}
	s := scServer(t, prov, nil) // upstream nil: a network call would dial .invalid and fail
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=31337", map[string]string{"address": scAddr})
	s.handleCheckSourcify(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var resp struct {
		IsVerified bool   `json:"isVerified"`
		Status     string `json:"status"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.IsVerified || resp.Status != "local" {
		t.Errorf("local verified -> %+v, want isVerified=true status=local", resp)
	}
}

// --- upstream status mapping ------------------------------------------------

func TestHandleFetchSourcify_Upstream404(t *testing.T) {
	s := scServer(t, &scStub{}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=1", map[string]string{"address": scAddr})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("upstream 404 -> handler %d, want 404", w.Code)
	}
}

func TestHandleFetchSourcify_GarbageJSON500(t *testing.T) {
	s := scServer(t, &scStub{}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not json"))
	})
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=1", map[string]string{"address": scAddr})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("garbage JSON -> handler %d, want 500", w.Code)
	}
}

func TestHandleFetchSourcify_NoABI404(t *testing.T) {
	// Valid JSON file list but no metadata.json/ABI -> 404 "no ABI found".
	s := scServer(t, &scStub{}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"match":"match","sources":{"README.md":{"content":"hi"}},"metadata":{}}`))
	})
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=1", map[string]string{"address": scAddr})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("no ABI -> handler %d, want 404", w.Code)
	}
}

func TestHandleFetchSourcify_HappyPathPersists(t *testing.T) {
	prov := &scStub{}
	var gotPath, gotFields string
	body := `{
		"match": "exact_match",
		"sources": {
			"src/A.sol": {"content": "contract A {}"},
			"src/C.sol": {"content": "contract C {}"}
		},
		"metadata": {
			"output": {"abi": [{"type": "function", "name": "foo"}]},
			"settings": {"compilationTarget": {"src/C.sol": "C"}},
			"compiler": {"version": "0.8.20+commit.a1b79de6"}
		}
	}`
	s := scServer(t, prov, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotFields = r.URL.Path, r.URL.Query().Get("fields")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=1", map[string]string{"address": scAddr})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("happy path -> %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if gotPath != "/v2/contract/1/"+scAddr || gotFields != "sources,metadata" {
		t.Errorf("upstream request = %s?fields=%s, want /v2/contract/1/%s?fields=sources,metadata", gotPath, gotFields, scAddr)
	}
	var resp struct {
		Success         bool   `json:"success"`
		ContractName    string `json:"contractName"`
		CompilerVersion string `json:"compilerVersion"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success || resp.ContractName != "C" || resp.CompilerVersion != "0.8.20+commit.a1b79de6" {
		t.Errorf("summary = %+v, want success/C/0.8.20+commit.a1b79de6", resp)
	}
	if prov.verifiedSource != "contract C {}" {
		t.Errorf("stored source = %q, want the compilation target's source", prov.verifiedSource)
	}
}

func TestHandleFetchSourcify_UnsupportedChain404(t *testing.T) {
	s := scServer(t, &scStub{}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"customCode":"unsupported_chain","message":"Chain 99999 not found"}`))
	})
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=99999", map[string]string{"address": scAddr})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unsupported chain -> handler %d, want 404", w.Code)
	}
}

func TestHandleFetchSourcify_Upstream400RelaysBody(t *testing.T) {
	s := scServer(t, &scStub{}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"customCode":"invalid_parameter","message":"bad address"}`))
	})
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId=1", map[string]string{"address": scAddr})
	s.handleFetchSourcify(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_parameter") {
		t.Fatalf("upstream 400 -> %d %q, want 400 carrying the upstream body", w.Code, w.Body.String())
	}
}

func checkSourcify(t *testing.T, chainID string, upstream http.HandlerFunc) (int, bool, string) {
	t.Helper()
	s := scServer(t, &scStub{}, upstream)
	w := httptest.NewRecorder()
	r := hpChiReq(http.MethodGet, "/x?chainId="+chainID, map[string]string{"address": scAddr})
	s.handleCheckSourcify(w, r)
	var resp struct {
		IsVerified bool   `json:"isVerified"`
		Status     string `json:"status"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp.IsVerified, resp.Status
}

func TestHandleCheckSourcify_V2Matches(t *testing.T) {
	cases := []struct {
		match      string
		wantStatus string
	}{
		{`"exact_match"`, "perfect"},
		{`"match"`, "partial"},
		{`null`, "not verified"},
	}
	for _, tc := range cases {
		var gotPath string
		code, verified, status := checkSourcify(t, "1", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"match":` + tc.match + `,"chainId":"1"}`))
		})
		if gotPath != "/v2/contract/1/"+scAddr {
			t.Errorf("upstream path = %s, want /v2/contract/1/%s", gotPath, scAddr)
		}
		if code != http.StatusOK || status != tc.wantStatus || verified != (tc.wantStatus != "not verified") {
			t.Errorf("match %s -> code=%d verified=%v status=%q, want 200/%q", tc.match, code, verified, status, tc.wantStatus)
		}
	}
}

func TestHandleCheckSourcify_NotFoundIsNotVerified(t *testing.T) {
	code, verified, status := checkSourcify(t, "1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"match":null,"creationMatch":null,"runtimeMatch":null,"chainId":"1"}`))
	})
	if code != http.StatusOK || verified || status != "not verified" {
		t.Errorf("404 -> code=%d verified=%v status=%q, want 200/false/not verified", code, verified, status)
	}
}

func TestHandleCheckSourcify_UnsupportedChainIsNotVerified(t *testing.T) {
	code, verified, status := checkSourcify(t, "99999", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"customCode":"unsupported_chain","message":"Chain 99999 not found"}`))
	})
	if code != http.StatusOK || verified || status != "not verified" {
		t.Errorf("unsupported chain -> code=%d verified=%v status=%q, want 200/false/not verified", code, verified, status)
	}
}

func TestHandleCheckSourcify_UpstreamErrorIsBadGateway(t *testing.T) {
	for _, tc := range []struct {
		code int
		body string
	}{
		{http.StatusInternalServerError, `{"customCode":"internal_error"}`},
		{http.StatusBadRequest, `{"customCode":"invalid_parameter"}`},
	} {
		code, _, _ := checkSourcify(t, "1", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
			_, _ = w.Write([]byte(tc.body))
		})
		if code != http.StatusBadGateway {
			t.Errorf("upstream %d %s -> handler %d, want 502", tc.code, tc.body, code)
		}
	}
}
