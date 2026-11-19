package api

// The ABI reader uses the current native OPS session. This is a finite adapter
// to OPS's recorded policy evaluator, never a raw-node or signing proxy.
import (
	"bytes"
	"encoding/json"
	"errors"
	"explorer/internal/authv2"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const nativeRPCRequestLimit = 256 << 10
const nativeRPCResponseLimit = 2 << 20

type nativeOwnOrganization struct {
	ID                 string `json:"id"`
	Slug               string `json:"slug"`
	Name               string `json:"name"`
	IsOrgAdmin         bool   `json:"isOrgAdmin"`
	IsOrgReadonlyAdmin bool   `json:"isOrgReadonlyAdmin"`
}
type nativeOwnSelf struct {
	ActorID             string                  `json:"actorId"`
	PrincipalKind       string                  `json:"principalKind"`
	SourceProfile       string                  `json:"sourceProfile"`
	SourceProfileDigest string                  `json:"sourceProfileDigest"`
	ExpiresAt           int64                   `json:"expiresAt"`
	Organizations       []nativeOwnOrganization `json:"organizations"`
}

func nativeStrictObject(raw []byte, max int, allowed ...string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > max || !utf8.Valid(raw) {
		return nil, authv2.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return nil, authv2.ErrInvalid
	}
	out := map[string]json.RawMessage{}
	for d.More() {
		t, e = d.Token()
		key, ok := t.(string)
		if e != nil || !ok {
			return nil, authv2.ErrInvalid
		}
		valid := false
		for _, name := range allowed {
			valid = valid || key == name
		}
		if _, duplicate := out[key]; duplicate || !valid {
			return nil, authv2.ErrInvalid
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, authv2.ErrInvalid
		}
		out[key] = value
	}
	if t, e = d.Token(); e != nil || t != json.Delim('}') {
		return nil, authv2.ErrInvalid
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, authv2.ErrInvalid
	}
	return out, nil
}
func nativeRPCID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if raw[0] == '"' && json.Unmarshal(raw, &s) == nil {
		if len(s) > 128 {
			return "", false
		}
		for _, c := range s {
			if c < 32 || c == 127 {
				return "", false
			}
		}
		v, _ := json.Marshal(s)
		return string(v), true
	}
	n, e := strconv.ParseInt(string(raw), 10, 64)
	return string(raw), e == nil && n >= -9007199254740991 && n <= 9007199254740991 && strconv.FormatInt(n, 10) == string(raw)
}
func nativeRPCRequest(raw []byte) (string, error) {
	v, e := nativeStrictObject(raw, nativeRPCRequestLimit, "jsonrpc", "id", "method", "params")
	if e != nil || len(v) != 4 || string(v["jsonrpc"]) != `"2.0"` {
		return "", authv2.ErrInvalid
	}
	id, ok := nativeRPCID(v["id"])
	if !ok {
		return "", authv2.ErrInvalid
	}
	var method string
	if json.Unmarshal(v["method"], &method) != nil {
		return "", authv2.ErrInvalid
	}
	switch method {
	case "eth_call", "eth_estimateGas", "eth_chainId", "net_version", "eth_getTransactionCount", "eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_feeHistory", "eth_getTransactionReceipt", "eth_getTransactionByHash", "eth_getBlockByNumber", "eth_getBalance", "eth_getCode":
	default:
		return "", authv2.ErrInvalid
	}
	var params []json.RawMessage
	if len(v["params"]) == 0 || v["params"][0] != '[' || json.Unmarshal(v["params"], &params) != nil || len(params) > 16 {
		return "", authv2.ErrInvalid
	}
	return id, nil
}
func nativeRPCResponse(raw []byte, id string) bool {
	v, e := nativeStrictObject(raw, nativeRPCResponseLimit, "jsonrpc", "id", "result", "error")
	if e != nil || len(v) != 3 || string(v["jsonrpc"]) != `"2.0"` {
		return false
	}
	got, ok := nativeRPCID(v["id"])
	if !ok || got != id {
		return false
	}
	_, result := v["result"]
	_, rpcError := v["error"]
	return result != rpcError && (!rpcError || len(v["error"]) > 0 && v["error"][0] == '{')
}
func nativeSessionFailure(w http.ResponseWriter, e error) {
	status, code := 503, "temporarily_unavailable"
	if errors.Is(e, authv2.ErrChanged) {
		status, code = 409, "session_changed"
	}
	if errors.Is(e, authv2.ErrUnauthenticated) {
		status, code = 401, "authentication_required"
	}
	writeError(w, status, code)
}
func (s *Server) nativeUpstream(r *http.Request, method, path string, body []byte, max int) ([]byte, error) {
	upstream, e := http.NewRequestWithContext(r.Context(), method, s.v2Reads.origin+path, bytes.NewReader(body))
	if e != nil {
		return nil, authv2.ErrUnavailable
	}
	upstream.Header.Set("Authorization", "Bearer "+authv2.Token(r.Context()))
	upstream.Header.Set("Accept", "application/json")
	if method == "POST" {
		upstream.Header.Set("Content-Type", "application/json")
	}
	if e = s.v2.CheckCurrent(r.Context()); e != nil {
		return nil, e
	}
	response, e := s.v2Reads.client.Do(upstream)
	if e != nil {
		return nil, authv2.ErrUnavailable
	}
	defer response.Body.Close()
	media, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, int64(max)+1))
	if mediaErr != nil || media != "application/json" || response.StatusCode != 200 || readErr != nil || len(raw) > max || !utf8.Valid(raw) || !json.Valid(raw) {
		return nil, authv2.ErrUnavailable
	}
	if e = s.v2.CheckCurrent(r.Context()); e != nil {
		return nil, e
	}
	return raw, nil
}
func (s *Server) nativeSelf(r *http.Request) (nativeOwnSelf, []byte, error) {
	var self nativeOwnSelf
	raw, e := s.nativeUpstream(r, "GET", "/api/v2/self", nil, 512<<10)
	if e != nil {
		return self, nil, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&self) != nil || d.Decode(&struct{}{}) != io.EOF || self.ActorID != authv2.Actor(r.Context()) || self.ExpiresAt <= time.Now().Unix() || self.Organizations == nil || len(self.Organizations) > 256 {
		return self, nil, authv2.ErrUnavailable
	}
	return self, raw, nil
}
func (s *Server) handleV2NativeSelf(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" || r.URL.RawQuery != "" || r.URL.EscapedPath() != "/api/v2/self" || r.ContentLength > 0 {
		writeError(w, 400, "invalid_request")
		return
	}
	if r.Body != nil {
		b, e := io.ReadAll(io.LimitReader(r.Body, 1))
		if e != nil || len(b) != 0 {
			writeError(w, 400, "invalid_request")
			return
		}
	}
	_, raw, e := s.nativeSelf(r)
	if e != nil {
		nativeSessionFailure(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}
func (s *Server) handleV2NativeRPC(w http.ResponseWriter, r *http.Request) {
	org := chi.URLParam(r, "org")
	parsed, e := uuid.Parse(org)
	media, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || parsed == uuid.Nil || parsed.String() != org || r.URL.EscapedPath() != "/api/v2/rpc/"+org || r.URL.RawQuery != "" || r.Method != "POST" || r.Body == nil || r.ContentLength > nativeRPCRequestLimit || len(r.Header.Values("Content-Encoding")) != 0 || len(r.Header.Values("Content-Type")) != 1 || mediaErr != nil || media != "application/json" {
		writeError(w, 400, "invalid_request")
		return
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, nativeRPCRequestLimit+1))
	if e != nil {
		writeError(w, 400, "invalid_request")
		return
	}
	id, e := nativeRPCRequest(raw)
	if e != nil {
		writeError(w, 400, "invalid_request")
		return
	}
	self, _, e := s.nativeSelf(r)
	if e != nil {
		nativeSessionFailure(w, e)
		return
	}
	permitted := false
	for _, v := range self.Organizations {
		permitted = permitted || v.ID == org
	}
	if !permitted {
		writeError(w, 403, "denied")
		return
	}
	response, e := s.nativeUpstream(r, "POST", "/rpc/"+org, raw, nativeRPCResponseLimit)
	if e != nil {
		nativeSessionFailure(w, e)
		return
	}
	if !nativeRPCResponse(response, id) {
		nativeSessionFailure(w, authv2.ErrUnavailable)
		return
	}
	if e = s.v2.CheckCurrent(r.Context()); e != nil {
		nativeSessionFailure(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(response)
}
