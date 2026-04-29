package api

import (
	"context"
	"encoding/json"
)

// APIDatabase is the slim subset of block-explorer DB methods still needed
// after RD-855 Phase 6. Chain data has been moved to
// gateway-fm/chain-indexer; the only block-explorer-local write path is
// contract verification, which the verifier package exercises through
// this interface.
type APIDatabase interface {
	VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error
}
