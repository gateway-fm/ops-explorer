package verifier

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"explorer/pkg/log"
)

// normalizeHash strips an optional 0x/0X prefix and lowercases a hex hash so
// the published list.json value (0x-prefixed) compares equal to a computed
// sha256 hex digest.
func normalizeHash(h string) string {
	h = strings.TrimSpace(h)
	h = strings.TrimPrefix(h, "0x")
	h = strings.TrimPrefix(h, "0X")
	return strings.ToLower(h)
}

const (
	solcListURL          = "https://binaries.soliditylang.org/linux-amd64/list.json"
	solcGitHubReleaseURL = "https://github.com/ethereum/solidity/releases/download/v%s/solc-static-linux"
	refreshInterval      = 1 * time.Hour
)

// remoteBuild represents a single build entry from the remote list.json
type remoteBuild struct {
	Path        string `json:"path"`
	Version     string `json:"version"`
	LongVersion string `json:"longVersion"`
	Prerelease  string `json:"prerelease"`
	SHA256      string `json:"sha256"`
}

// remoteListResponse represents the response from list.json
type remoteListResponse struct {
	Builds []remoteBuild `json:"builds"`
}

type SolcManager struct {
	basePath     string
	versions     map[string]string // version -> local path
	remoteBuilds []remoteBuild     // cached remote builds (filtered, no prereleases)
	mu           sync.RWMutex
	stopRefresh  chan struct{}

	// httpClient and releaseURLTemplate are test seams (default to the real
	// client + GitHub release URL). releaseURLTemplate must contain a single
	// %s for the version.
	httpClient         *http.Client
	releaseURLTemplate string
}

func NewSolcManager(basePath string) *SolcManager {
	sm := &SolcManager{
		basePath:           basePath,
		versions:           make(map[string]string),
		stopRefresh:        make(chan struct{}),
		httpClient:         http.DefaultClient,
		releaseURLTemplate: solcGitHubReleaseURL,
	}
	sm.scanVersions()

	// Fetch remote list in background
	go func() {
		sm.RefreshRemoteList()
	}()

	// Start periodic refresh
	go sm.refreshLoop()

	return sm
}

func (sm *SolcManager) refreshLoop() {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.RefreshRemoteList()
		case <-sm.stopRefresh:
			return
		}
	}
}

func (sm *SolcManager) RefreshRemoteList() {
	resp, err := http.Get(solcListURL)
	if err != nil {
		log.Warn("failed to fetch remote solc list", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn("remote solc list returned non-200 status", "status", resp.StatusCode)
		return
	}

	var listResp remoteListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		log.Warn("failed to decode remote solc list", "error", err)
		return
	}

	// Filter out pre-release builds
	var filtered []remoteBuild
	for _, b := range listResp.Builds {
		if b.Prerelease == "" {
			filtered = append(filtered, b)
		}
	}

	sm.mu.Lock()
	sm.remoteBuilds = filtered
	sm.mu.Unlock()

	log.Info("refreshed remote solc compiler list", "count", len(filtered))
}

func (sm *SolcManager) scanVersions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, err := os.Stat(sm.basePath); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(sm.basePath)
	if err != nil {
		return
	}

	versionRegex := regexp.MustCompile(`^solc-(\d+\.\d+\.\d+)$`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		matches := versionRegex.FindStringSubmatch(name)
		if len(matches) == 2 {
			version := matches[1]
			sm.versions[version] = filepath.Join(sm.basePath, name)
		}
	}

	solcPath := filepath.Join(sm.basePath, "solc")
	if _, err := os.Stat(solcPath); err == nil {
		if version, err := sm.getVersionFromBinary(solcPath); err == nil {
			sm.versions[version] = solcPath
		}
	}
}

func (sm *SolcManager) getVersionFromBinary(path string) (string, error) {
	cmd := exec.Command(path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	versionRegex := regexp.MustCompile(`Version:\s*(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return "", fmt.Errorf("could not parse version from output")
	}

	return matches[1], nil
}

// findRemoteBuild looks up a remote build by version string (must hold at least RLock).
func (sm *SolcManager) findRemoteBuild(version string) *remoteBuild {
	for i := range sm.remoteBuilds {
		if sm.remoteBuilds[i].Version == version {
			return &sm.remoteBuilds[i]
		}
	}
	return nil
}

// downloadCompiler downloads a statically-linked solc binary from GitHub releases.
func (sm *SolcManager) downloadCompiler(build *remoteBuild) (string, error) {
	tmpl := sm.releaseURLTemplate
	if tmpl == "" {
		tmpl = solcGitHubReleaseURL
	}
	client := sm.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	url := fmt.Sprintf(tmpl, build.Version)
	log.Info("downloading solc compiler", "version", build.Version, "url", url)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download compiler: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d for solc %s", resp.StatusCode, build.Version)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read compiler binary: %w", err)
	}

	// S-1: verify the downloaded bytes against the published sha256 before
	// writing/exec'ing. Validated against v0.8.26: list.json's sha256
	// (0x-prefixed) byte-for-byte matches the GitHub-release solc-static-linux
	// asset this code fetches, so enforcement does not brick downloads. Fail
	// closed on an empty expected hash (an unverifiable binary must never be
	// written/exec'd) and on any mismatch. Normalize both sides: strip an
	// optional 0x/0X prefix and lowercase.
	expected := normalizeHash(build.SHA256)
	if expected == "" {
		return "", fmt.Errorf("refusing to install solc %s: no published sha256 to verify against", build.Version)
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if actual != expected {
		return "", fmt.Errorf("solc %s checksum mismatch: expected %s, got %s — refusing to install", build.Version, expected, actual)
	}

	// Ensure base path exists
	if err := os.MkdirAll(sm.basePath, 0750); err != nil {
		return "", fmt.Errorf("failed to create compiler directory: %w", err)
	}

	destPath := filepath.Join(sm.basePath, fmt.Sprintf("solc-%s", build.Version))
	if err := os.WriteFile(destPath, data, 0700); err != nil {
		return "", fmt.Errorf("failed to write compiler binary: %w", err)
	}

	log.Info("downloaded solc compiler", "version", build.Version, "path", destPath)

	return destPath, nil
}

func (sm *SolcManager) GetCompiler(version string) (string, error) {
	// Strip 'v' prefix and '+commit...' suffix for version matching
	version = strings.TrimPrefix(version, "v")
	if idx := strings.Index(version, "+"); idx != -1 {
		version = version[:idx]
	}

	// Check local first
	sm.mu.RLock()
	if path, ok := sm.versions[version]; ok {
		sm.mu.RUnlock()
		return path, nil
	}

	for v, path := range sm.versions {
		if strings.HasPrefix(v, version) {
			sm.mu.RUnlock()
			return path, nil
		}
	}

	// Check if available remotely
	build := sm.findRemoteBuild(version)
	sm.mu.RUnlock()

	if build == nil {
		return "", fmt.Errorf("compiler version %s not found", version)
	}

	// Download the compiler
	path, err := sm.downloadCompiler(build)
	if err != nil {
		return "", fmt.Errorf("failed to download compiler %s: %w", version, err)
	}

	// Register the newly downloaded compiler
	sm.mu.Lock()
	sm.versions[version] = path
	sm.mu.Unlock()

	return path, nil
}

func (sm *SolcManager) HasVersion(version string) bool {
	_, err := sm.GetCompiler(version)
	return err == nil
}

func (sm *SolcManager) ListVersions() []CompilerVersion {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// If we have remote builds, return the full remote list with local paths where available
	if len(sm.remoteBuilds) > 0 {
		versions := make([]CompilerVersion, 0, len(sm.remoteBuilds))
		for _, build := range sm.remoteBuilds {
			cv := CompilerVersion{
				Version:     build.Version,
				LongVersion: build.LongVersion,
			}
			if path, ok := sm.versions[build.Version]; ok {
				cv.Path = path
			}
			versions = append(versions, cv)
		}

		// Remote builds are typically oldest-first; reverse to newest-first
		sort.Slice(versions, func(i, j int) bool {
			return compareVersions(versions[i].Version, versions[j].Version) > 0
		})

		return versions
	}

	// Fallback to local-only list
	versions := make([]CompilerVersion, 0, len(sm.versions))
	for version, path := range sm.versions {
		versions = append(versions, CompilerVersion{
			Version: version,
			Path:    path,
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	return versions
}

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	for i := 0; i < 3; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 != n2 {
			return n1 - n2
		}
	}

	return 0
}

func (sm *SolcManager) Refresh() {
	sm.scanVersions()
	sm.RefreshRemoteList()
}

func (sm *SolcManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.remoteBuilds) > 0 {
		return len(sm.remoteBuilds)
	}
	return len(sm.versions)
}

func (sm *SolcManager) Stop() {
	close(sm.stopRefresh)
}
