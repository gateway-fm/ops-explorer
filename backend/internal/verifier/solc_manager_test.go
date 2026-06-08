package verifier

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTestSolcManager builds a SolcManager wired to a stub release server,
// without starting the background refresh goroutines.
func newTestSolcManager(t *testing.T, handler http.HandlerFunc) (*SolcManager, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	sm := &SolcManager{
		basePath:           t.TempDir(),
		versions:           map[string]string{},
		stopRefresh:        make(chan struct{}),
		httpClient:         srv.Client(),
		releaseURLTemplate: srv.URL + "/v%s/solc-static-linux",
	}
	return sm, srv
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// S-1: downloadCompiler must verify the sha256 of the downloaded bytes against
// remoteBuild.SHA256, refuse on mismatch, and write NO file
// (PROD_READINESS_AUDIT §S-1).
func TestDownloadCompiler_RejectsHashMismatch(t *testing.T) {
	payload := []byte("not really a solc binary")
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	})

	build := &remoteBuild{
		Version: "0.8.26",
		SHA256:  "0x" + sha256hex([]byte("DIFFERENT CONTENT")), // deliberate mismatch
	}

	path, err := sm.downloadCompiler(build)
	if err == nil {
		t.Fatal("expected downloadCompiler to error on sha256 mismatch, got nil")
	}
	if path != "" {
		t.Errorf("expected no path on mismatch, got %q", path)
	}
	// No file must be written.
	dest := filepath.Join(sm.basePath, "solc-0.8.26")
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("expected NO file written on hash mismatch, but %s exists", dest)
	}
}

// S-1: a matching sha256 (with normalization — list.json prefixes 0x and the
// real release hash for v0.8.26 was validated to match the GitHub asset) must
// succeed and write the binary.
func TestDownloadCompiler_AcceptsMatchingHash(t *testing.T) {
	payload := []byte("pretend-solc-binary-bytes")
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	})

	build := &remoteBuild{
		Version: "0.8.26",
		SHA256:  "0X" + sha256hex(payload), // upper-0X + correct hash: normalization must accept
	}

	path, err := sm.downloadCompiler(build)
	if err != nil {
		t.Fatalf("expected matching hash to succeed, got error: %v", err)
	}
	if path == "" {
		t.Fatal("expected a non-empty path on success")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written binary: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("written bytes do not match downloaded payload")
	}
}

// S-1: an empty expected hash must fail closed (refuse) — never write/exec an
// unverifiable binary.
func TestDownloadCompiler_RefusesEmptyExpectedHash(t *testing.T) {
	payload := []byte("whatever")
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	})

	build := &remoteBuild{Version: "0.8.26", SHA256: ""} // no published hash

	path, err := sm.downloadCompiler(build)
	if err == nil {
		t.Fatal("expected downloadCompiler to fail closed on an empty expected hash, got nil")
	}
	if path != "" {
		t.Errorf("expected no path on empty-hash refusal, got %q", path)
	}
	dest := filepath.Join(sm.basePath, "solc-0.8.26")
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("expected NO file written on empty-hash refusal, but %s exists", dest)
	}
}
