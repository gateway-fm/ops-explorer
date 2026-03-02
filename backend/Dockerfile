FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /indexer ./cmd/indexer

FROM alpine:3.19

RUN apk --no-cache add ca-certificates libc6-compat

# Download solc static binaries for contract verification
RUN mkdir -p /opt/solc && \
    for version in 0.8.20 0.8.24 0.8.26 0.8.28; do \
        wget -q -O /opt/solc/solc-${version} \
            https://github.com/ethereum/solidity/releases/download/v${version}/solc-static-linux && \
        chmod +x /opt/solc/solc-${version}; \
    done

COPY --from=builder /api /api
COPY --from=builder /indexer /indexer

ENTRYPOINT ["/api"]
