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
		basePath:    t.TempDir(),
		versions:    map[string]string{},
		stopRefresh: make(chan struct{}),
		httpClient:  srv.Client(),
		binariesURL: srv.URL,
		platform:    "linux-arm64",
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
		Path:    "solc-linux-arm64-v0.8.26+commit.8a97fa7a",
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

// S-1: a matching sha256 (with normalization — list.json prefixes 0x) must
// succeed and write the binary.
func TestDownloadCompiler_AcceptsMatchingHash(t *testing.T) {
	payload := []byte("pretend-solc-binary-bytes")
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	})

	build := &remoteBuild{
		Version: "0.8.26",
		Path:    "solc-linux-arm64-v0.8.26+commit.8a97fa7a",
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

	build := &remoteBuild{Version: "0.8.26", Path: "solc-linux-arm64-v0.8.26+commit.8a97fa7a", SHA256: ""} // no published hash

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

func TestSolcPlatform(t *testing.T) {
	cases := map[string]string{"amd64": "linux-amd64", "arm64": "linux-arm64", "riscv64": ""}
	for goarch, want := range cases {
		if got := solcPlatform(goarch); got != want {
			t.Errorf("solcPlatform(%q) = %q, want %q", goarch, got, want)
		}
	}
}

func TestDownloadCompiler_FetchesPlatformBinary(t *testing.T) {
	payload := []byte("arm64-solc")
	var gotPath string
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write(payload)
	})

	build := &remoteBuild{
		Version: "0.8.38",
		Path:    "solc-linux-arm64-v0.8.38+commit.1835d247",
		SHA256:  "0x" + sha256hex(payload),
	}
	path, err := sm.downloadCompiler(build)
	if err != nil {
		t.Fatalf("downloadCompiler: %v", err)
	}
	if want := "/linux-arm64/solc-linux-arm64-v0.8.38+commit.1835d247"; gotPath != want {
		t.Errorf("requested %q, want %q", gotPath, want)
	}
	if want := filepath.Join(sm.basePath, "solc-0.8.38"); path != want {
		t.Errorf("installed at %q, want %q", path, want)
	}
}

func TestDownloadCompiler_RejectsUnsafeBuildPath(t *testing.T) {
	requested := false
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, _ *http.Request) {
		requested = true
	})

	for _, p := range []string{"", "../linux-amd64/solc", "sub/solc", `sub\solc`, ".."} {
		if _, err := sm.downloadCompiler(&remoteBuild{Version: "0.8.38", Path: p, SHA256: "0xab"}); err == nil {
			t.Errorf("expected error for build path %q", p)
		}
	}
	if requested {
		t.Error("expected no HTTP request for an unsafe build path")
	}
}

func TestDownloadCompiler_RejectsUnsafeVersion(t *testing.T) {
	requested := false
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, _ *http.Request) {
		requested = true
	})

	for _, v := range []string{"", "0.8.1/../../../tmp/x", "../0.8.38", "0.8.38+commit.1835d247", "0.8"} {
		if _, err := sm.downloadCompiler(&remoteBuild{Version: v, Path: "solc-linux-arm64-v0.8.38+commit.1835d247", SHA256: "0xab"}); err == nil {
			t.Errorf("expected error for version %q", v)
		}
	}
	if requested {
		t.Error("expected no HTTP request for an unsafe version")
	}
}

func TestRefreshRemoteList_UsesPlatformListAndSkipsPrereleases(t *testing.T) {
	var gotPath string
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"builds":[
			{"path":"solc-linux-arm64-v0.8.38+commit.1835d247","version":"0.8.38","longVersion":"0.8.38+commit.1835d247","sha256":"0x01"},
			{"path":"solc-linux-arm64-v0.8.39-nightly","version":"0.8.39","prerelease":"nightly","sha256":"0x02"}
		]}`))
	})

	sm.RefreshRemoteList()

	if gotPath != "/linux-arm64/list.json" {
		t.Errorf("fetched %q, want /linux-arm64/list.json", gotPath)
	}
	versions := sm.ListVersions()
	if len(versions) != 1 || versions[0].Version != "0.8.38" {
		t.Errorf("ListVersions() = %+v, want only 0.8.38", versions)
	}
}

func TestRefreshRemoteList_UnsupportedPlatformDoesNotFetch(t *testing.T) {
	requested := false
	sm, _ := newTestSolcManager(t, func(w http.ResponseWriter, _ *http.Request) {
		requested = true
	})
	sm.platform = ""

	sm.RefreshRemoteList()

	if requested {
		t.Error("expected no list fetch on an unsupported platform")
	}
	if sm.Count() != 0 {
		t.Errorf("Count() = %d, want 0", sm.Count())
	}
}
