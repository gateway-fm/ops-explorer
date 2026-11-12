package api

import (
	"context"
	"encoding/json"
	"errors"
	"explorer/internal/authv2"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

const v2ExplorerMount = "/api/v2/explorer"
const v2ExplorerResponseLimit = 8 << 20

type v2ReadRoute struct{ path, query string }

var v2ReadRoutes = []v2ReadRoute{
	{"/chain-id", ""}, {"/stats", ""}, {"/stats/tx-history", "chart"},
	{"/blocks", "page"}, {"/blocks/latest/number", ""}, {"/blocks/hash/{hash}", ""}, {"/blocks/{number}", ""}, {"/blocks/{number}/transactions", "transactions"}, {"/blocks/{number}/internal", "page"},
	{"/transactions/paginated", "transactions"}, {"/transactions", "transactions"}, {"/transactions/{hash}", ""}, {"/transactions/{hash}/internal", "page"}, {"/transactions/{hash}/transfers", "page"}, {"/transactions/{hash}/logs", "logs"},
	{"/addresses/{address}/stats", ""}, {"/addresses/{address}/transactions", "transactions"}, {"/addresses/{address}/balance", ""}, {"/addresses/{address}/code", ""}, {"/addresses/{address}/balances", "page"}, {"/addresses/{address}/transfers", "page"}, {"/addresses/{address}/internal", "page"}, {"/addresses/{address}/logs", "scopedLogs"}, {"/addresses/{address}/contract", ""}, {"/addresses/{address}/is-contract", ""},
	{"/logs", "logs"}, {"/tokens", "tokens"}, {"/tokens/{address}", ""}, {"/tokens/{address}/holders", "page"}, {"/tokens/{address}/transfers", "page"}, {"/transfers", "page"}, {"/accounts", "page"}, {"/search/suggestions", "search"}, {"/sync/status", ""}, {"/sync/indexer-progress", ""}, {"/sync/catchup", ""},
}

func v2ReadDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (s *Server) setupV2ReadRoutes(r chi.Router) {
	for _, route := range v2ReadRoutes {
		route := route
		r.Get(v2ExplorerMount+route.path, func(w http.ResponseWriter, r *http.Request) { s.handleV2Read(w, r, route) })
	}
}
func decimalBound(v string, max uint64) bool {
	if v == "" || len(v) > 20 || len(v) > 1 && v[0] == '0' {
		return false
	}
	for _, c := range v {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, e := strconv.ParseUint(v, 10, 64)
	return e == nil && n <= max
}
func v2ReadPath(r *http.Request, route v2ReadRoute) (string, error) {
	path := route.path
	for _, name := range []string{"address", "hash", "number"} {
		marker := "{" + name + "}"
		if !strings.Contains(path, marker) {
			continue
		}
		v := chi.URLParam(r, name)
		valid := walletAddress(v)
		if name == "hash" {
			valid = strings.HasPrefix(v, "0x") && walletHex(strings.TrimPrefix(v, "0x"), 32) && strings.ToLower(v) == v
		}
		if name == "number" {
			valid = decimalBound(v, 1<<63-1)
		}
		if !valid {
			return "", authv2.ErrInvalid
		}
		path = strings.ReplaceAll(path, marker, url.PathEscape(v))
	}
	// No percent aliases, encoded separators, traversal or alternate route family.
	if r.URL.EscapedPath() != v2ExplorerMount+path {
		return "", authv2.ErrInvalid
	}
	return v2ExplorerMount + path, nil
}
func v2ReadQuery(raw string, kind string) (string, error) {
	if len(raw) > 16<<10 {
		return "", authv2.ErrInvalid
	}
	q, e := url.ParseQuery(raw)
	if e != nil {
		return "", authv2.ErrInvalid
	}
	allowed := map[string]bool{}
	maxLimit := uint64(1000)
	if kind != "" {
		allowed["limit"], allowed["cursor"] = true, true
	}
	switch kind {
	case "transactions":
		allowed["category"] = true
	case "logs", "scopedLogs":
		for _, k := range []string{"address", "topic0", "from", "to"} {
			if kind != "scopedLogs" || k != "address" {
				allowed[k] = true
			}
		}
	case "tokens":
		allowed["type"] = true
	case "search":
		allowed["q"] = true
		maxLimit = 20
	case "chart":
		allowed["interval"] = true
		maxLimit = 365
	}
	for k, values := range q {
		if !allowed[k] || len(values) != 1 || values[0] == "" {
			return "", authv2.ErrInvalid
		}
		v := values[0]
		switch k {
		case "limit":
			if !decimalBound(v, maxLimit) || v == "0" {
				return "", authv2.ErrInvalid
			}
		case "cursor":
			if len(v) > 8192 {
				return "", authv2.ErrInvalid
			}
			for _, c := range v {
				if c < 33 || c > 126 {
					return "", authv2.ErrInvalid
				}
			}
		case "category":
			if v != "coin_transfer" && v != "contract_creation" && v != "contract_call" && v != "token_transfer" {
				return "", authv2.ErrInvalid
			}
		case "address":
			if !walletAddress(v) {
				return "", authv2.ErrInvalid
			}
		case "topic0":
			if !strings.HasPrefix(v, "0x") || !walletHex(strings.TrimPrefix(v, "0x"), 32) || strings.ToLower(v) != v {
				return "", authv2.ErrInvalid
			}
		case "from", "to":
			if !decimalBound(v, 1<<63-1) {
				return "", authv2.ErrInvalid
			}
		case "type":
			if v != "ERC20" && v != "ERC721" && v != "ERC1155" {
				return "", authv2.ErrInvalid
			}
		case "interval":
			if v != "hour" && v != "day" && v != "week" {
				return "", authv2.ErrInvalid
			}
		case "q":
			if !utf8.ValidString(v) || len(v) > 128 || strings.TrimSpace(v) != v {
				return "", authv2.ErrInvalid
			}
			for _, c := range v {
				if c < 32 || c == 127 {
					return "", authv2.ErrInvalid
				}
			}
		}
	}
	if kind == "search" && q.Get("q") == "" {
		return "", authv2.ErrInvalid
	}
	if q.Get("from") != "" && q.Get("to") != "" {
		a, _ := strconv.ParseUint(q.Get("from"), 10, 64)
		b, _ := strconv.ParseUint(q.Get("to"), 10, 64)
		if a > b {
			return "", authv2.ErrInvalid
		}
	}
	return q.Encode(), nil
}
func v2ReadError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}{Error: struct {
		Code string `json:"code"`
	}{Code: code}})
}
func v2ReadStatus(status int, raw []byte) bool {
	if status == 200 {
		return true
	}
	code := map[int]string{400: "invalid_request", 401: "unauthenticated", 403: "denied", 404: "not_found", 409: "cursor_stale", 422: "too_broad", 503: "unavailable"}[status]
	if code == "" || len(raw) > 1024 {
		return false
	}
	var v struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil || v.Error.Code != code {
		return false
	}
	return d.Decode(&struct{}{}) == io.EOF
}
func (s *Server) handleV2Read(w http.ResponseWriter, r *http.Request, route v2ReadRoute) {
	path, e := v2ReadPath(r, route)
	if e != nil {
		v2ReadError(w, 400, "invalid_request")
		return
	}
	query, e := v2ReadQuery(r.URL.RawQuery, route.query)
	if e != nil || r.Method != "GET" || r.ContentLength > 0 || len(r.Header.Values("Content-Encoding")) > 0 {
		v2ReadError(w, 400, "invalid_request")
		return
	}
	if r.Body != nil {
		raw, e := io.ReadAll(io.LimitReader(r.Body, 1))
		if e != nil || len(raw) > 0 {
			v2ReadError(w, 400, "invalid_request")
			return
		}
	}
	target := s.v2Reads.origin + path
	if query != "" {
		target += "?" + query
	}
	upstream, e := http.NewRequestWithContext(r.Context(), "GET", target, nil)
	if e != nil {
		v2ReadError(w, 503, "unavailable")
		return
	}
	upstream.Header.Set("Authorization", "Bearer "+authv2.Token(r.Context()))
	upstream.Header.Set("Accept", "application/json")
	if e = s.v2.CheckCurrent(r.Context()); e != nil {
		status, code := 503, "temporarily_unavailable"
		if errors.Is(e, authv2.ErrChanged) {
			status, code = 409, "session_changed"
		} else if errors.Is(e, authv2.ErrUnauthenticated) {
			status, code = 401, "authentication_required"
		}
		writeError(w, status, code)
		return
	}
	response, e := s.v2Reads.client.Do(upstream)
	if e != nil {
		v2ReadError(w, 503, "unavailable")
		return
	}
	defer response.Body.Close()
	media, _, e := mime.ParseMediaType(response.Header.Get("Content-Type"))
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, v2ExplorerResponseLimit+1))
	if e != nil || media != "application/json" || readErr != nil || len(raw) > v2ExplorerResponseLimit || !utf8.Valid(raw) || !json.Valid(raw) || !v2ReadStatus(response.StatusCode, raw) {
		v2ReadError(w, 503, "unavailable")
		return
	}
	// Keep the source-checked lossless DTO verbatim. The original private writer
	// rechecks the exact active capture at the real first response byte.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(raw)
}
