FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /explorer ./cmd/explorer
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /indexer ./cmd/indexer

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

# Copy all binaries
COPY --from=builder /explorer /explorer
COPY --from=builder /api /api
COPY --from=builder /indexer /indexer

# Default to the combined explorer (for backwards compatibility)
ENTRYPOINT ["/explorer"]
