package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGrantAddressDoesNotReadRegularProvider(t *testing.T) {
	// A grant is only authority for grant endpoints. Regular address stats can
	// be unavailable, or disclose information outside the grant's date scope.
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"real_address":"0x1111111111111111111111111111111111111111","disclosure_level":"full","grant_id":"g1"}`))
	}, nil)
	r := tcChiReq(tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), "did:viewer"), map[string]string{"grantId": "g1", "addressId": "a1"})
	w := httptest.NewRecorder()
	s.handleGetGrantedAddress(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	for _, field := range []string{"balance", "tx_count", "is_contract"} {
		if _, ok := got[field]; ok {
			t.Errorf("unavailable grant-scoped %s must be omitted: %s", field, w.Body.String())
		}
	}
}

func TestGrantTransactionDetailProxy(t *testing.T) {
	for _, status := range []int{200, 401, 403, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			const body = `{"result":"grant only"}`
			s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.EscapedPath() != "/api/v1/explorer/grant/g%2F1/a%3F2/transactions/0x123" {
					t.Errorf("path = %q", r.URL.EscapedPath())
				}
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					t.Error("viewer credential was not forwarded")
				}
				w.WriteHeader(status)
				w.Write([]byte(body))
			}, nil)
			r := tcChiReq(tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), "did:viewer"), map[string]string{"grantId": "g/1", "addressId": "a?2", "hash": "0x123"})
			w := httptest.NewRecorder()
			s.handleGetGrantedTransaction(w, r)
			if w.Code != status || w.Body.String() != body {
				t.Errorf("got %d %s, want %d %s", w.Code, w.Body.String(), status, body)
			}
		})
	}
}

func TestGrantTransactionDetailRequiresViewer(t *testing.T) {
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) { t.Error("upstream called without authentication") }, nil)
	r := tcChiReq(httptest.NewRequest(http.MethodGet, "/x", nil), map[string]string{"grantId": "g1", "addressId": "a1", "hash": "0x123"})
	w := httptest.NewRecorder()
	s.handleGetGrantedTransaction(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", w.Code)
	}
}

func TestGrantReadsRejectViewAs(t *testing.T) {
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) { t.Error("grant service called during View-as") }, nil)
	for name, handler := range map[string]http.HandlerFunc{
		"list addresses": s.handleGetViewableAddresses,
		"resolve":        s.handleGetGrantedAddress,
		"transactions":   s.handleGetGrantedAddressTransactions,
		"transaction":    s.handleGetGrantedTransaction,
		"activity":       s.handleGetGrantActivityLogs,
	} {
		t.Run(name, func(t *testing.T) {
			r := tcChiReq(tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), "did:admin"), map[string]string{"grantId": "g1", "addressId": "a1", "hash": "0x123"})
			r.Header.Set(ImpersonateHeader, "view-as-token")
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", w.Code)
			}
		})
	}
}
