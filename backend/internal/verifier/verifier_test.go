package verifier

import (
	"context"
	"encoding/json"
	"testing"

	"explorer/internal/types"

	"explorer/pkg/eth/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabase is a mock implementation of Database
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) InsertContract(ctx context.Context, c *types.Contract) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *MockDatabase) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Contract), args.Error(1)
}

func (m *MockDatabase) IsContract(ctx context.Context, address string) (bool, error) {
	args := m.Called(ctx, address)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage, evmVersion string, licenseType string, constructorArgs string, optimizationRuns int) error {
	args := m.Called(ctx, address, name, compilerVersion, optimizationUsed, sourceCode, abi, evmVersion, licenseType, constructorArgs, optimizationRuns)
	return args.Error(0)
}

func (m *MockDatabase) SetContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	args := m.Called(ctx, address, abi)
	return args.Error(0)
}

// MockRPCClient is a mock implementation of RPCClient
type MockRPCClient struct {
	mock.Mock
}

func (m *MockRPCClient) GetCode(ctx context.Context, address common.Address) ([]byte, error) {
	args := m.Called(ctx, address)
	return args.Get(0).([]byte), args.Error(1)
}

func TestVerifyRequest(t *testing.T) {
	mockDB := new(MockDatabase)
	mockRPC := new(MockRPCClient)
	v := NewVerifier(mockDB, mockRPC, &Config{
		SolcPath: "/usr/local/bin/solc",
	})

	assert.NotNil(t, v)
}
