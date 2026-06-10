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

# Download solc static binaries for contract verification, verifying each
# against its published sha256 (S-1). These hashes are the soliditylang
# list.json values, validated byte-for-byte against the GitHub-release
# solc-static-linux assets (confirmed for 0.8.26 and 0.8.28). The build FAILS
# (sha256sum -c) on any mismatch — no unverified compiler is baked in.
RUN mkdir -p /opt/solc && cd /opt/solc && \
    echo "0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782  solc-0.8.20" >  solc.sha256 && \
    echo "fb03a29a517452b9f12bcf459ef37d0a543765bb3bbc911e70a87d6a37c30d5f  solc-0.8.24" >> solc.sha256 && \
    echo "d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef  solc-0.8.26" >> solc.sha256 && \
    echo "9a0fb7e0db2c0641dbae1c5cc645dc686820c83af516226abb1c0a2f76636f25  solc-0.8.28" >> solc.sha256 && \
    for version in 0.8.20 0.8.24 0.8.26 0.8.28; do \
        wget -q -O solc-${version} \
            https://github.com/ethereum/solidity/releases/download/v${version}/solc-static-linux; \
    done && \
    sha256sum -c solc.sha256 && \
    chmod +x solc-0.8.20 solc-0.8.24 solc-0.8.26 solc-0.8.28 && \
    rm -f solc.sha256

# sol2uml renders contract UML class diagrams for the "View UML diagram"
# feature on the address contract tab. It's a Node CLI invoked from the Go
# backend (see internal/umlgen). S-1: pinned to an EXACT version for
# reproducibility/supply-chain integrity (was the floating @2 range).
RUN npm install -g sol2uml@2.5.26 && sol2uml --version

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /api /api

USER appuser:appgroup

ENTRYPOINT ["/api"]
