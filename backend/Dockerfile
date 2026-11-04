# D-1: pin the builder to go1.26.10 in lockstep with go.mod's `toolchain
# go1.26.10` so the runtime binary is built against the patched stdlib that
# clears AIKIDO-2026-934590 (fix = stdlib 1.26.10), CVE-2026-56857 and
# CVE-2026-78663 (Aikido criticals, fix = stdlib 1.26.9) on top of the
# earlier stdlib DoS (Aikido #10140394, 1.26.6), GO-2026-5039
# (net/textproto, privacy path) and GO-2026-5037 (crypto/x509).
# Do not float below 1.26.10.
FROM golang:1.26.10-alpine AS builder

# Upgrade system packages
RUN apk update --no-cache && apk upgrade --no-cache

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build identity stamped into the binary via -ldflags (mirrors privacy-proxy
# RD-1023). docker-compose `args:` / `make` forward these; a plain `docker build`
# falls back to the safe defaults so it never fails without git metadata.
ARG VERSION=dev
ARG GIT_COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
	-ldflags "-X explorer/internal/version.Version=${VERSION} -X explorer/internal/version.Commit=${GIT_COMMIT} -X explorer/internal/version.BuildTime=${BUILD_TIME}" \
	-o /api ./cmd/api
# cmd/indexer was retired in RD-855 Phase 6 — chain data now comes
# from the gateway-fm/chain-indexer service over gRPC, so the api
# image no longer ships an embedded indexer binary.

# Node runtime for sol2uml, pinned to a patched release.
FROM node:24.22.1-alpine

RUN apk update --no-cache && apk upgrade --no-cache
RUN apk --no-cache add ca-certificates libc6-compat

# sol2uml renders contract UML class diagrams for the "View UML diagram"
# feature on the address contract tab. It's a Node CLI invoked from the Go
# backend (see internal/umlgen). S-1: pinned to an EXACT version for
# reproducibility/supply-chain integrity (was the floating @2 range).
RUN npm install -g sol2uml@2.5.26 && sol2uml --version

RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    mkdir -p /opt/solc && chown appuser:appgroup /opt/solc

COPY --from=builder /api /api

USER appuser:appgroup

ENTRYPOINT ["/api"]
