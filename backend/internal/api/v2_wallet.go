package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"explorer/internal/authv2"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

const walletRequestLimit = 8 << 10
const walletResponseLimit = 1 << 20

func v2WalletDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func walletHex(value string, size int) bool {
	if len(value) != size*2 {
		return false
	}
	_, e := hex.DecodeString(value)
	return e == nil
}
func walletAddress(value string) bool {
	return strings.HasPrefix(value, "0x") && walletHex(strings.TrimPrefix(value, "0x"), 20) && strings.ToLower(value) == value
}

// The two write DTOs contain only strings. Reading tokens explicitly rejects
// duplicate/unknown fields without a new general JSON parser or V1 DTOs.
func walletBody(r *http.Request, verify bool) ([]byte, error) {
	if r.Body == nil || r.ContentLength > walletRequestLimit || len(r.Header.Values("Content-Encoding")) != 0 || len(r.Header.Values("Content-Type")) != 1 {
		return nil, authv2.ErrInvalid
	}
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return nil, authv2.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, walletRequestLimit+1))
	if e != nil || len(raw) > walletRequestLimit || !utf8.Valid(raw) {
		return nil, authv2.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return nil, authv2.ErrInvalid
	}
	fields := map[string]string{}
	for d.More() {
		key, e := d.Token()
		name, ok := key.(string)
		if e != nil || !ok || name != "address" && (!verify || name != "nonce" && name != "signature") {
			return nil, authv2.ErrInvalid
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, authv2.ErrInvalid
		}
		var value string
		if d.Decode(&value) != nil {
			return nil, authv2.ErrInvalid
		}
		fields[name] = value
	}
	if token, e = d.Token(); e != nil || token != json.Delim('}') {
		return nil, authv2.ErrInvalid
	}
	if _, e = d.Token(); e != io.EOF || !walletAddress(fields["address"]) {
		return nil, authv2.ErrInvalid
	}
	if !verify && len(fields) != 1 || verify && (len(fields) != 3 || !walletHex(fields["nonce"], 32) || fields["nonce"] != strings.ToLower(fields["nonce"]) || !strings.HasPrefix(fields["signature"], "0x") || !walletHex(strings.TrimPrefix(fields["signature"], "0x"), 65)) {
		return nil, authv2.ErrInvalid
	}
	return raw, nil
}

func (s *Server) handleV2Wallet(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if r.URL.RawQuery != "" {
		writeError(w, 400, "invalid wallet request")
		return
	}
	path := ""
	var body []byte
	var e error
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/eth/addresses":
		path = "/api/v2/eth/addresses"
	case r.Method == "DELETE" && walletAddress(chi.URLParam(r, "address")) && r.URL.Path == "/api/eth/addresses/"+chi.URLParam(r, "address"):
		path = "/api/v2/eth/addresses/" + chi.URLParam(r, "address")
	case r.Method == "POST" && (r.URL.Path == "/api/eth/link/challenge" || r.URL.Path == "/api/eth/link/verify"):
		body, e = walletBody(r, strings.HasSuffix(r.URL.Path, "/verify"))
		path = "/api/v2" + strings.TrimPrefix(r.URL.Path, "/api")
	default:
		e = authv2.ErrInvalid
	}
	if r.Method != "POST" {
		var raw []byte
		raw, e = io.ReadAll(io.LimitReader(r.Body, 1))
		if len(raw) != 0 || r.ContentLength > 0 {
			e = authv2.ErrInvalid
		}
	}
	if e != nil || path == "" {
		writeError(w, 400, "invalid wallet request")
		return
	}
	upstream, e := http.NewRequestWithContext(ctx, r.Method, s.v2Reads.origin+path, bytes.NewReader(body))
	if e != nil {
		writeError(w, 503, "wallet service unavailable")
		return
	}
	upstream.Header.Set("Authorization", "Bearer "+authv2.Token(ctx))
	upstream.Header.Set("Accept", "application/json")
	if r.Method == "POST" {
		upstream.Header.Set("Content-Type", "application/json")
	}
	// No network/SQL work is inserted between this exact-capture read COMMIT
	// and forwarding. Already forwarded original-Actor effects may still commit.
	if e = s.v2.CheckCurrent(ctx); e != nil {
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
		writeError(w, 503, "wallet service unavailable")
		return
	}
	defer response.Body.Close()
	media, _, e := mime.ParseMediaType(response.Header.Get("Content-Type"))
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, walletResponseLimit+1))
	if e != nil || media != "application/json" || readErr != nil || len(raw) > walletResponseLimit || !utf8.Valid(raw) || !json.Valid(raw) || !walletStatus(response.StatusCode, raw) {
		writeError(w, 503, "wallet service unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(raw)
}

func walletStatus(status int, raw []byte) bool {
	if status == 200 {
		return true // Native wire remains bytes; V2 UI validates its exact DTO.
	}
	expected := map[int]string{400: "invalid wallet request", 401: "invalid or expired access", 404: "wallet link not found", 409: "wallet state conflict", 503: "wallet service unavailable"}[status]
	if expected == "" {
		return false
	}
	if len(raw) > 1024 {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	open, e1 := d.Token()
	name, e2 := d.Token()
	var value string
	e3 := d.Decode(&value)
	end, e4 := d.Token()
	_, e5 := d.Token()
	return e1 == nil && e2 == nil && e3 == nil && e4 == nil && e5 == io.EOF && open == json.Delim('{') && name == "error" && value == expected && end == json.Delim('}')
}
