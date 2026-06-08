# D-1: pin the builder to go1.25.11 in lockstep with go.mod's `toolchain
# go1.25.11` so the runtime binary is built against the patched stdlib that
# clears GO-2026-5039 (net/textproto, privacy path) and GO-2026-5037
# (crypto/x509). Do not float below 1.25.11.
FROM golang:1.25.11-alpine AS builder

# Upgrade system packages
RUN apk update --no-cache && apk upgrade --no-cache

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api
# cmd/indexer was retired in RD-855 Phase 6 — chain data now comes
# from the gateway-fm/chain-indexer service over gRPC, so the api
# image no longer ships an embedded indexer binary.

FROM alpine:3.22

RUN apk update --no-cache && apk upgrade --no-cache
RUN apk --no-cache add ca-certificates libc6-compat nodejs npm

# Download solc static binaries for contract verification
RUN mkdir -p /opt/solc && \
    for version in 0.8.20 0.8.24 0.8.26 0.8.28; do \
        wget -q -O /opt/solc/solc-${version} \
            https://github.com/ethereum/solidity/releases/download/v${version}/solc-static-linux && \
        chmod +x /opt/solc/solc-${version}; \
    done

# sol2uml renders contract UML class diagrams for the "View UML diagram"
# feature on the address contract tab. It's a Node CLI invoked from the Go
# backend (see internal/umlgen). Pinned to a major version for reproducibility.
RUN npm install -g sol2uml@2 && sol2uml --version

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /api /api

USER appuser:appgroup

ENTRYPOINT ["/api"]
