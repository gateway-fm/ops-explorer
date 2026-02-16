package verifier

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// SolcManager manages multiple solc compiler versions
type SolcManager struct {
	basePath string
	versions map[string]string // version -> path
	mu       sync.RWMutex
}

// NewSolcManager creates a new SolcManager
func NewSolcManager(basePath string) *SolcManager {
	sm := &SolcManager{
		basePath: basePath,
		versions: make(map[string]string),
	}
	sm.scanVersions()
	return sm
}

// scanVersions scans the base path for available solc versions
func (sm *SolcManager) scanVersions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if base path exists
	if _, err := os.Stat(sm.basePath); os.IsNotExist(err) {
		return
	}

	// Scan for solc binaries
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

	// Also check for "solc" binary without version suffix
	solcPath := filepath.Join(sm.basePath, "solc")
	if _, err := os.Stat(solcPath); err == nil {
		// Try to get version from the binary
		if version, err := sm.getVersionFromBinary(solcPath); err == nil {
			sm.versions[version] = solcPath
		}
	}
}

// getVersionFromBinary extracts the version from a solc binary
func (sm *SolcManager) getVersionFromBinary(path string) (string, error) {
	cmd := exec.Command(path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse version from output like "solc, the solidity compiler commandline interface\nVersion: 0.8.19+commit...."
	versionRegex := regexp.MustCompile(`Version:\s*(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return "", fmt.Errorf("could not parse version from output")
	}

	return matches[1], nil
}

// GetCompiler returns the path to a specific solc version
func (sm *SolcManager) GetCompiler(version string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Normalize version (remove 'v' prefix and '+commit...' suffix if present)
	version = strings.TrimPrefix(version, "v")
	if idx := strings.Index(version, "+"); idx != -1 {
		version = version[:idx]
	}

	// Check for exact match
	if path, ok := sm.versions[version]; ok {
		return path, nil
	}

	// Check for partial match (e.g., "0.8" matches "0.8.19")
	for v, path := range sm.versions {
		if strings.HasPrefix(v, version) {
			return path, nil
		}
	}

	return "", fmt.Errorf("compiler version %s not found", version)
}

// HasVersion checks if a specific version is available
func (sm *SolcManager) HasVersion(version string) bool {
	_, err := sm.GetCompiler(version)
	return err == nil
}

// ListVersions returns all available compiler versions
func (sm *SolcManager) ListVersions() []CompilerVersion {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	versions := make([]CompilerVersion, 0, len(sm.versions))
	for version, path := range sm.versions {
		versions = append(versions, CompilerVersion{
			Version: version,
			Path:    path,
		})
	}

	// Sort by version (descending)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	return versions
}

// compareVersions compares two semver versions
// Returns positive if v1 > v2, negative if v1 < v2, 0 if equal
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

// Refresh rescans for available compiler versions
func (sm *SolcManager) Refresh() {
	sm.scanVersions()
}

// Count returns the number of available compiler versions
func (sm *SolcManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.versions)
}
