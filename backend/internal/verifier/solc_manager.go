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

type SolcManager struct {
	basePath string
	versions map[string]string // version -> path
	mu       sync.RWMutex
}

func NewSolcManager(basePath string) *SolcManager {
	sm := &SolcManager{
		basePath: basePath,
		versions: make(map[string]string),
	}
	sm.scanVersions()
	return sm
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

func (sm *SolcManager) GetCompiler(version string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Strip 'v' prefix and '+commit...' suffix for version matching
	version = strings.TrimPrefix(version, "v")
	if idx := strings.Index(version, "+"); idx != -1 {
		version = version[:idx]
	}

	if path, ok := sm.versions[version]; ok {
		return path, nil
	}

	for v, path := range sm.versions {
		if strings.HasPrefix(v, version) {
			return path, nil
		}
	}

	return "", fmt.Errorf("compiler version %s not found", version)
}

func (sm *SolcManager) HasVersion(version string) bool {
	_, err := sm.GetCompiler(version)
	return err == nil
}

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
}

func (sm *SolcManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.versions)
}
