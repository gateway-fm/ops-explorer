package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	if err.Error() != "privacy proxy request failed with status 500" {
		t.Errorf("expected generic error without response body, got: %v", err)
	}
}
