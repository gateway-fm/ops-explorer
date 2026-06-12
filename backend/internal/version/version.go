// Package version holds the build identity of the block-explorer binaries.
//
// The three vars below are populated at build time via the linker:
//
//	go build -ldflags "\
//	  -X explorer/internal/version.Version=$(git describe --tags --always) \
//	  -X explorer/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X explorer/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// version.mk and the Dockerfiles wire these in (mirrors privacy-proxy's
// RD-1023). A plain `go build` with no ldflags leaves the "dev" defaults —
// that's intentional, so a developer build is obviously distinguishable from
// a released one and nothing breaks when the flags are absent.
//
// It is surfaced at GET /version (and /api/version) and logged once at
// startup. Unlike privacy-proxy this endpoint is unauthenticated: the
// explorer's own build version is shown publicly in the UI footer (as is
// conventional for block explorers), and it exposes no chain data.
package version

import "fmt"

// These are overwritten by -ldflags at build time. Keep them as plain string
// vars (not consts) so the linker's -X can reach them.
var (
	// Version is the release identity, normally `git describe --tags --always`
	// (e.g. "v0.8.2" or "v0.8.2-85-g3ebac71"). "dev" for an un-stamped local
	// build.
	Version = "dev"
	// Commit is the short git SHA the binary was built from.
	Commit = "none"
	// BuildTime is the UTC build timestamp in RFC3339 (e.g.
	// "2026-06-12T10:00:00Z").
	BuildTime = "unknown"
)

// String renders the build identity for logs and human-facing output, e.g.
// "v0.8.2 (commit 3ebac71, built 2026-06-12T10:00:00Z)".
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildTime)
}
