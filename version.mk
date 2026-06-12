# --dirty flags an uncommitted working tree (e.g. "v0.8.2-85-g3ebac71-dirty")
# so a locally-built binary is never mistaken for a clean release build.
VERSION := $(shell git describe --tags --always --dirty)
GITREV := $(shell git rev-parse --short HEAD)
GITBRANCH := $(shell git rev-parse --abbrev-ref HEAD)
DATE := $(shell LANG=US date +"%a, %d %b %Y %X %z")

# Build identity injected into the Go binaries via -ldflags. BUILD_TIME is
# RFC3339 UTC so it parses cleanly downstream. GIT_COMMIT aliases GITREV under
# the name the Dockerfiles' ARG and the docker-compose `args:` blocks expect.
# The Dockerfiles inline the same -X flags (their build context is backend/,
# which excludes this file), so keep the package path and var names in sync
# with backend/internal/version.
GIT_COMMIT := $(GITREV)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := explorer/internal/version
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(GIT_COMMIT) -X $(VERSION_PKG).BuildTime=$(BUILD_TIME)

# Export so `make`-invoked docker-compose builds (run / dev / dev-stack /
# standalone / dev-multi) inherit the resolved build identity through their
# `args:` blocks. Without this, compose builds fall back to the Dockerfile
# defaults (dev/none/unknown) even when invoked via make.
export VERSION GIT_COMMIT BUILD_TIME
