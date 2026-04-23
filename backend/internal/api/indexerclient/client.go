// Package indexerclient provides an api.DataProvider backed by the
// chain-indexer gRPC service. It embeds *api.DirectDBProvider so methods
// that are not yet ported to gRPC keep hitting the SQL path unchanged.
//
// This is the block-explorer equivalent of privacy-proxy's
// internal/explorer/indexerclient. Same pattern: gRPC-backed reads for
// hot paths, SQL fallback for everything else.
//
// Standalone block-explorer deployments set INDEXER_URL to point at a
// chain-indexer instance and share its postgres. Over time this lets the
// block-explorer's own indexer binary be retired — RD-855 end state.
package indexerclient

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	indexerv1 "explorer/gen/go/chain_indexer/v1"

	"explorer/internal/api"
)

// Provider wraps a gRPC chain-indexer client. It embeds *api.DirectDBProvider
// so unmigrated methods keep hitting SQL until they are ported.
type Provider struct {
	*api.DirectDBProvider

	client indexerv1.IndexerServiceClient
	conn   *grpc.ClientConn
}

// Config controls the gRPC client.
type Config struct {
	// IndexerURL is the gRPC dial target, e.g. "chain-indexer:50051".
	IndexerURL string

	// DialTimeout bounds how long New() will wait for the initial
	// connection. Default 5s.
	DialTimeout time.Duration
}

// New constructs a Provider. The provided fallback is used for methods
// not yet ported to gRPC.
func New(cfg Config, fallback *api.DirectDBProvider) (*Provider, error) {
	if cfg.IndexerURL == "" {
		return nil, errors.New("indexerclient: IndexerURL is required")
	}
	if fallback == nil {
		return nil, errors.New("indexerclient: fallback DirectDBProvider is required until all methods are ported")
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	conn, err := grpc.NewClient(
		cfg.IndexerURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("indexerclient: dial %s: %w", cfg.IndexerURL, err)
	}
	return &Provider{
		DirectDBProvider: fallback,
		client:           indexerv1.NewIndexerServiceClient(conn),
		conn:             conn,
	}, nil
}

// Close releases the gRPC connection. The embedded DirectDBProvider owns
// the DB pool; closing it is the caller's responsibility.
func (p *Provider) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// isNotFound returns true if err is a gRPC NotFound status.
func isNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}

// Compile-time assertion that *Provider satisfies api.DataProvider via
// the embedded DirectDBProvider plus its own overrides.
var _ api.DataProvider = (*Provider)(nil)
