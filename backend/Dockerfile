FROM golang:1.25-alpine AS builder

# Upgrade system packages
RUN apk update --no-cache && apk upgrade --no-cache

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /indexer ./cmd/indexer

FROM alpine:3.21

RUN apk update --no-cache && apk upgrade --no-cache
RUN apk --no-cache add ca-certificates libc6-compat

# Download solc static binaries for contract verification
RUN mkdir -p /opt/solc && \
    for version in 0.8.20 0.8.24 0.8.26 0.8.28; do \
        wget -q -O /opt/solc/solc-${version} \
            https://github.com/ethereum/solidity/releases/download/v${version}/solc-static-linux && \
        chmod +x /opt/solc/solc-${version}; \
    done

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /api /api
COPY --from=builder /indexer /indexer

USER appuser:appgroup

ENTRYPOINT ["/api"]
