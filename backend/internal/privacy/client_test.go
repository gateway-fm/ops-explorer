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
		if got := r.Header.Get("Authorization"); got != "Bearer viewer-token" {
			t.Fatalf("Authorization = %q, want viewer bearer token", got)
		}
		json.NewEncoder(w).Encode(ResolveAddressResponse{
			RealAddress:     "0xabc123",
			DisclosureLevel: "full",
			GrantID:         "grant-1",
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	result, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1", "viewer-token")
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
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1", "viewer-token")
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
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1", "viewer-token")
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
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1", "viewer-token")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for 401, got %v", err)
	}
}

func TestResolveAddressID_500_ReturnsGenericError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("database connection failed"))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	_, err := client.ResolveAddressID(context.Background(), "grant-1", "addr-1", "viewer-token")
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

func TestGetGrantTransactions_ForwardsViewerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer viewer-token" {
			t.Fatalf("Authorization = %q, want viewer bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactions":[]}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	body, status, err := client.GetGrantTransactions(
		context.Background(), "grant-1", "addr-1", "viewer-token", 25, "", nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK || string(body) != `{"transactions":[]}` {
		t.Fatalf("status/body = %d/%s", status, body)
	}
}
