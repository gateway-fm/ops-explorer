package verifier

import (
	"context"
	"encoding/json"
	"explorer/internal/types"

	"explorer/pkg/eth/common"
)

type Database interface {
	InsertContract(ctx context.Context, c *types.Contract) error
	GetContract(ctx context.Context, address string) (*types.Contract, error)
	IsContract(ctx context.Context, address string) (bool, error)
	VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error
	SetContractABI(ctx context.Context, address string, abi json.RawMessage) error
}

type RPCClient interface {
	GetCode(ctx context.Context, address common.Address) ([]byte, error)
}
