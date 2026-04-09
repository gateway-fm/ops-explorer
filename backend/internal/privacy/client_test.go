package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveAddressID_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ResolveAddressResponse{
			RealAddress:     "0xabc123",
			DisclosureLevel: "full",
			GrantID:         "grant-1",
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	result, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RealAddress != "0xabc123" {
		t.Errorf("expected 0xabc123, got %s", result.RealAddress)
	}
}

func TestResolveAddressID_404_ReturnsErrNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveAddressID_403_ReturnsErrNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "grant has been revoked"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for 403, got %v", err)
	}
}

func TestResolveAddressID_401_ReturnsErrNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for 401, got %v", err)
	}
}

// ============================================================================
// Test: GetSharedLogs
// ============================================================================

func TestGetSharedLogs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/explorer/shared-logs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("limit") != "25" {
			t.Errorf("expected limit=25, got %s", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("offset") != "0" {
			t.Errorf("expected offset=0, got %s", r.URL.Query().Get("offset"))
		}
		topic0 := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
		topic1 := "0x0000000000000000000000001234567890abcdef1234567890abcdef12345678"
		topic2 := "0x000000000000000000000000abcdefabcdefabcdefabcdefabcdefabcdefabcd"
		json.NewEncoder(w).Encode(SharedLogsResponse{
			SharedLogs: []SharedLogEntry{
				{
					TxHash:          "0xabc",
					BlockNumber:     42,
					ContractAddress: "0xcontract",
					Logs: []SharedLog{
						{
							ID:          1,
							TxHash:      "0xabc",
							LogIndex:    0,
							Address:     "0xcontract",
							Topic0:      &topic0,
							Topic1:      &topic1,
							Topic2:      &topic2,
							Data:        "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000",
							BlockNumber: 42,
						},
					},
					SharedAt: "2026-04-03T00:00:00Z",
				},
			},
			Total:  1,
			Limit:  25,
			Offset: 0,
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	result, err := client.GetSharedLogs(context.Background(), "test-token", 25, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.SharedLogs) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.SharedLogs))
	}
	if result.SharedLogs[0].TxHash != "0xabc" {
		t.Errorf("expected tx_hash 0xabc, got %s", result.SharedLogs[0].TxHash)
	}
	if len(result.SharedLogs[0].Logs) != 1 {
		t.Fatalf("expected 1 log in entry, got %d", len(result.SharedLogs[0].Logs))
	}
	if result.SharedLogs[0].Logs[0].Address != "0xcontract" {
		t.Errorf("expected log address 0xcontract, got %s", result.SharedLogs[0].Logs[0].Address)
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
}

func TestGetSharedLogs_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	_, err := client.GetSharedLogs(context.Background(), "bad-token", 25, 0)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to mention 401, got: %v", err)
	}
}

func TestGetSharedLogs_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	_, err := client.GetSharedLogs(context.Background(), "test-token", 25, 0)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention 500, got: %v", err)
	}
}

func TestResolveAddressID_500_ReturnsGenericError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("database connection failed"))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1")
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("500 should not return ErrNotFound")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "status 500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
	if strings.Contains(errStr, "database connection failed") {
		t.Errorf("error must not contain proxy response body, got: %v", err)
	}
}
