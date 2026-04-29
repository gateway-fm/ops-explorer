package verifier

import (
	"context"
	"encoding/json"
	"explorer/internal/types"

	"explorer/pkg/eth/common"
)

// Database is the narrow slice of block-explorer's DB that verifier needs.
// Kept minimal because Phase 6 removed almost everything else from the DB
// package — this is the only block-explorer-local write path left.
type Database interface {
	VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error
}

// _ = types.Contract — placeholder retained so the types import stays live
// if the interface shape grows again.
var _ = types.Contract{}

type RPCClient interface {
	GetCode(ctx context.Context, address common.Address) ([]byte, error)
}
