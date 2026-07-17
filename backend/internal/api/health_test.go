package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// readyStub records whether the readiness probe touched the data provider.
// RD-1181: the readiness probe must NEVER call an upstream, so
// GetLatestBlockNumber must not be invoked and its error must not change the
// result. The embedded nil DataProvider supplies the rest of the method set;
// any method the probe wrongly calls (other than the one below) nil-panics,
// keeping the test honest.
type readyStub struct {
	DataProvider
	latestCalled bool
}

func (s *readyStub) GetLatestBlockNumber(context.Context) (uint64, error) {
	s.latestCalled = true
	return 0, errors.New("upstream down")
}

// The core RD-1181 regression guard: even when the upstream (chain-indexer /
// privacy-proxy) is down, readiness stays 200 and never probes it.
func TestReadiness_IgnoresUpstreamAndStaysReady(t *testing.T) {
	stub := &readyStub{}
	s := &Server{provider: stub}
	w := httptest.NewRecorder()

	s.handleReadinessCheck(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (readiness must not depend on upstream)", w.Code)
	}
	if stub.latestCalled {
		t.Error("readiness called GetLatestBlockNumber; it must not probe upstreams (RD-1181)")
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ready"] != true {
		t.Errorf("ready = %v, want true; body=%s", body["ready"], w.Body.String())
	}
}

// While draining (SIGTERM), readiness flips to 503 so k8s pulls the pod from
// the Service endpoints before in-flight requests are cut.
func TestReadiness_503WhileDraining(t *testing.T) {
	s := &Server{provider: &readyStub{}}
	s.draining.Store(true)
	w := httptest.NewRecorder()

	s.handleReadinessCheck(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 while draining", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ready"] != false {
		t.Errorf("ready = %v, want false", body["ready"])
	}
	if body["reason"] != "shutting down" {
		t.Errorf("reason = %v, want %q", body["reason"], "shutting down")
	}
}

// Liveness has no dependencies and never drains — it stays 200 regardless of
// upstream or shutdown state (a draining pod is not dead, so k8s must not
// restart it).
func TestLiveness_AlwaysAlive(t *testing.T) {
	s := &Server{provider: &readyStub{}}
	s.draining.Store(true)
	w := httptest.NewRecorder()

	s.handleLivenessCheck(w, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "alive" {
		t.Errorf("status = %v, want alive", body["status"])
	}
}
