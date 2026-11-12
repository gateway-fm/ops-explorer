package api

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"github.com/google/uuid"
)

const disclosureRequestLimit = 128 << 10
const disclosureResponseLimit = 2 << 20

// The five Explorer consumers adapt to the existing case/read API. They add
// no independent grants or customer approval protocol to Core.
func (s *Server) setupV2DisclosureRoutes(router chi.Router) {
	for _, path := range []string{"/viewable-addresses", "/grant/{grant_id}/resolve/{address_id}", "/grant/{grant_id}/{address_id}/transactions", "/grant/{grant_id}/{address_id}/transactions/{tx_hash}", "/grant/{grant_id}/activity"} {
		router.Get(path, s.handleV2Disclosure)
	}
	router.Get("/disclosure/cases", s.handleV2Disclosure)
	router.Post("/disclosure/cases", s.handleV2Disclosure)
	router.Get("/disclosure/cases/{case_id}", s.handleV2Disclosure)
	router.Get("/disclosure/cases/{case_id}/audit", s.handleV2Disclosure)
	router.Post("/disclosure/cases/{case_id}/revoke", s.handleV2Disclosure)
	router.Post("/disclosure/cases/{case_id}/read", s.handleV2Disclosure)
}

func disclosureUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
func disclosureAlias(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != "da1" || len(parts[1]) < 1 || len(parts[1]) > 64 {
		return false
	}
	for _, c := range parts[1] {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	return err == nil && len(raw) == 16 && base64.RawURLEncoding.EncodeToString(raw) == parts[2]
}
func disclosureQuery(raw string, allowed ...string) (url.Values, error) {
	if len(raw) > 16<<10 {
		return nil, authv2.ErrInvalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, authv2.ErrInvalid
	}
	for name, entries := range values {
		known := false
		for _, key := range allowed {
			known = known || name == key
		}
		if !known || len(entries) != 1 || entries[0] == "" {
			return nil, authv2.ErrInvalid
		}
	}
	if value := values.Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 100 || strconv.Itoa(n) != value {
			return nil, authv2.ErrInvalid
		}
	}
	if len(values.Get("cursor")) > 8192 {
		return nil, authv2.ErrInvalid
	}
	return values, nil
}
func disclosureBody(request *http.Request) ([]byte, error) {
	if request.Body == nil || request.ContentLength > disclosureRequestLimit || len(request.Header.Values("Content-Encoding")) != 0 || len(request.Header.Values("Content-Type")) != 1 {
		return nil, authv2.ErrInvalid
	}
	media, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, authv2.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, disclosureRequestLimit+1))
	if err != nil || len(raw) > disclosureRequestLimit || !utf8.Valid(raw) || !json.Valid(raw) {
		return nil, authv2.ErrInvalid
	}
	return raw, nil
}

func disclosureOperation(request *http.Request) (string, string, []byte, error) {
	method := request.Method
	if method != "GET" && method != "POST" {
		return "", "", nil, authv2.ErrInvalid
	}
	path := ""
	for _, prefix := range []string{"/api/v2", "/api/v1", "/api"} {
		if strings.HasPrefix(request.URL.Path, prefix+"/") {
			path = strings.TrimPrefix(request.URL.Path, prefix)
			break
		}
	}
	if path == "" || request.URL.RawPath != "" {
		return "", "", nil, authv2.ErrInvalid
	}
	if method == "GET" && request.Body != nil {
		body, err := io.ReadAll(io.LimitReader(request.Body, 1))
		if err != nil || len(body) > 0 || request.ContentLength > 0 {
			return "", "", nil, authv2.ErrInvalid
		}
	}
	if path == "/viewable-addresses" && method == "GET" {
		q, err := disclosureQuery(request.URL.RawQuery, "limit", "cursor")
		if err != nil {
			return "", "", nil, err
		}
		q.Set("role", "recipient")
		return "GET", "/api/v1/disclosure/cases?" + q.Encode(), nil, nil
	}
	if strings.HasPrefix(path, "/grant/") {
		if method != "GET" {
			return "", "", nil, authv2.ErrInvalid
		}
		parts := strings.Split(strings.TrimPrefix(path, "/grant/"), "/")
		if len(parts) < 2 || !disclosureUUID(parts[0]) {
			return "", "", nil, authv2.ErrInvalid
		}
		query := map[string]any{"v": 1}
		list := false
		switch {
		case len(parts) == 2 && parts[1] == "activity":
			query["resource"], query["view"], list = "activity_logs", "list", true
		case len(parts) == 3 && parts[1] == "resolve" && disclosureAlias(parts[2]):
			query["resource"], query["view"], query["addressId"] = "addresses", "resolve", parts[2]
		case len(parts) == 3 && disclosureAlias(parts[1]) && parts[2] == "transactions":
			query["resource"], query["view"], query["addressId"], list = "transactions", "list", parts[1], true
		case len(parts) == 4 && disclosureAlias(parts[1]) && parts[2] == "transactions" && strings.HasPrefix(parts[3], "0x") && len(parts[3]) == 66 && strings.ToLower(parts[3]) == parts[3] && walletHex(parts[3][2:], 32):
			query["resource"], query["view"], query["addressId"], query["transactionHash"] = "transactions", "detail", parts[1], parts[3]
		default:
			return "", "", nil, authv2.ErrInvalid
		}
		allowed := []string{}
		if list {
			allowed = []string{"limit", "cursor"}
		}
		values, err := disclosureQuery(request.URL.RawQuery, allowed...)
		if err != nil {
			return "", "", nil, err
		}
		if value := values.Get("limit"); value != "" {
			n, _ := strconv.Atoi(value)
			query["limit"] = n
		}
		if value := values.Get("cursor"); value != "" {
			if len(value) > 2048 {
				return "", "", nil, authv2.ErrInvalid
			}
			query["cursor"] = value
		}
		body, err := json.Marshal(query)
		return "POST", "/api/v1/disclosure/cases/" + parts[0] + "/read", body, err
	}
	if !strings.HasPrefix(path, "/disclosure/cases") {
		return "", "", nil, authv2.ErrInvalid
	}
	parts := strings.Split(strings.TrimPrefix(path, "/disclosure/cases"), "/")
	if len(parts) > 1 && (parts[0] != "" || !disclosureUUID(parts[1])) {
		return "", "", nil, authv2.ErrInvalid
	}
	allowed := []string{}
	valid := false
	switch {
	case path == "/disclosure/cases" && method == "GET":
		allowed = []string{"role", "organizationId", "cursor", "limit"}
		valid = true
	case path == "/disclosure/cases" && method == "POST":
		valid = true
	case len(parts) == 2 && method == "GET":
		valid = true
	case len(parts) == 3 && parts[2] == "audit" && method == "GET":
		allowed = []string{"cursor", "limit"}
		valid = true
	case len(parts) == 3 && (parts[2] == "read" || parts[2] == "revoke") && method == "POST":
		valid = true
	}
	if !valid {
		return "", "", nil, authv2.ErrInvalid
	}
	values, err := disclosureQuery(request.URL.RawQuery, allowed...)
	if err != nil {
		return "", "", nil, err
	}
	var body []byte
	if method == "POST" {
		body, err = disclosureBody(request)
		if err != nil {
			return "", "", nil, err
		}
	}
	upstream := "/api/v1" + path
	if len(values) > 0 {
		upstream += "?" + values.Encode()
	}
	return method, upstream, body, nil
}

func (s *Server) handleV2Disclosure(w http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	request = request.WithContext(ctx)
	method, path, body, err := disclosureOperation(request)
	if err != nil {
		writeError(w, 400, "invalid disclosure request")
		return
	}
	upstream, err := http.NewRequestWithContext(ctx, method, s.v2Reads.origin+path, bytes.NewReader(body))
	if err != nil {
		writeError(w, 503, "disclosure service unavailable")
		return
	}
	upstream.Header.Set("Authorization", "Bearer "+authv2.Token(ctx))
	upstream.Header.Set("Accept", "application/json")
	if method == "POST" {
		upstream.Header.Set("Content-Type", "application/json")
	}
	if err = s.v2.CheckCurrent(ctx); err != nil {
		status, message := 503, "temporarily_unavailable"
		if errors.Is(err, authv2.ErrChanged) {
			status, message = 409, "session_changed"
		} else if errors.Is(err, authv2.ErrUnauthenticated) {
			status, message = 401, "authentication_required"
		}
		writeError(w, status, message)
		return
	}
	response, err := s.v2Reads.client.Do(upstream)
	if err != nil {
		writeError(w, 503, "disclosure service unavailable")
		return
	}
	defer response.Body.Close()
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, disclosureResponseLimit+1))
	if err != nil || media != "application/json" || readErr != nil || len(raw) > disclosureResponseLimit || !utf8.Valid(raw) || !json.Valid(raw) || !disclosureStatus(response.StatusCode, raw) {
		writeError(w, 503, "disclosure service unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(raw)
}
func disclosureStatus(status int, raw []byte) bool {
	if status == 200 || status == 201 {
		return true
	}
	expected := map[int]string{400: "invalid disclosure request", 401: "invalid or expired access", 404: "disclosure case not found", 409: "disclosure context changed", 422: "protected data scope unavailable", 503: "disclosure service unavailable"}[status]
	if expected == "" || len(raw) > 1024 {
		return false
	}
	var value struct {
		Error string `json:"error"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || value.Error != expected {
		return false
	}
	return true
}
