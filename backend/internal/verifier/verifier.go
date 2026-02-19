package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"explorer/internal/db"
	"explorer/internal/log"
	"explorer/internal/rpc"

	"github.com/ethereum/go-ethereum/common"
)

// Verifier handles contract verification
type Verifier struct {
	db              *db.DB
	rpc             *rpc.Client
	solcManager     *SolcManager
	useSourcifyFallback bool
}

// Config holds verifier configuration
type Config struct {
	SolcPath            string
	UseSourcifyFallback bool
}

// NewVerifier creates a new Verifier
func NewVerifier(database *db.DB, rpcClient *rpc.Client, cfg *Config) *Verifier {
	return &Verifier{
		db:              database,
		rpc:             rpcClient,
		solcManager:     NewSolcManager(cfg.SolcPath),
		useSourcifyFallback: cfg.UseSourcifyFallback,
	}
}

// Verify verifies a contract by compiling source and comparing bytecode
func (v *Verifier) Verify(ctx context.Context, req *VerifyRequest) (*VerifyResponse, error) {
	response := &VerifyResponse{
		Address:      req.Address,
		ContractName: req.ContractName,
	}

	// Normalize address
	if !common.IsHexAddress(req.Address) {
		response.Error = "invalid address"
		return response, nil
	}
	address := common.HexToAddress(req.Address)

	// Get on-chain bytecode
	onChainCode, err := v.rpc.GetCode(ctx, address)
	if err != nil {
		response.Error = fmt.Sprintf("failed to fetch on-chain bytecode: %v", err)
		return response, nil
	}

	if len(onChainCode) == 0 {
		response.Error = "no bytecode at address (not a contract)"
		return response, nil
	}

	onChainBytecode := common.Bytes2Hex(onChainCode)

	// Check if we have the compiler version
	compilerPath, err := v.solcManager.GetCompiler(req.CompilerVersion)
	if err != nil {
		response.Error = fmt.Sprintf("compiler version %s not available: %v", req.CompilerVersion, err)
		return response, nil
	}

	// Create compiler
	compiler := NewCompiler(compilerPath, req.CompilerVersion)

	// Default optimization runs if not specified
	runs := req.OptimizationRuns
	if runs == 0 {
		runs = 200
	}

	// Compile the source
	output, err := compiler.CompileSource(ctx, req.SourceFiles, req.MainContractFile, req.ContractName,
		req.OptimizationUsed, runs, req.EVMVersion, req.Libraries)
	if err != nil {
		response.Error = fmt.Sprintf("compilation failed: %v", err)
		return response, nil
	}

	// Check for compilation errors
	if HasErrors(output) {
		errors := GetErrors(output)
		response.Error = fmt.Sprintf("compilation errors: %s", strings.Join(errors, "; "))
		return response, nil
	}

	// Get the contract output
	contractOutput, err := GetContractOutput(output, req.MainContractFile, req.ContractName)
	if err != nil {
		response.Error = err.Error()
		return response, nil
	}

	// Compare bytecodes
	compiledBytecode := contractOutput.EVM.DeployedBytecode.Object
	comparison := CompareBytecode(onChainBytecode, compiledBytecode, req.ConstructorArgs)

	response.MatchType = comparison.MatchType
	response.BytecodeHash = comparison.OnChainHash
	response.CompiledHash = comparison.CompiledHash
	response.CompilerVersion = req.CompilerVersion

	if comparison.MatchType == MatchTypeNone {
		response.Error = fmt.Sprintf("bytecode does not match (on-chain: %d bytes, compiled: %d bytes)", comparison.OnChainLength, comparison.CompiledLength)
		log.Warn("bytecode mismatch",
			"address", req.Address,
			"onChainLen", comparison.OnChainLength,
			"compiledLen", comparison.CompiledLength,
			"onChainHash", comparison.OnChainHash,
			"compiledHash", comparison.CompiledHash,
			"optimizer", req.OptimizationUsed,
			"runs", runs,
			"evmVersion", req.EVMVersion,
		)
		return response, nil
	}

	// Verification successful
	response.Success = true
	response.ABI = contractOutput.ABI

	// Store verification result in database
	if err := v.storeVerification(ctx, req, contractOutput); err != nil {
		log.Warn("failed to store verification result", "error", err)
	}

	return response, nil
}

// storeVerification stores the verification result in the database
func (v *Verifier) storeVerification(ctx context.Context, req *VerifyRequest, output *ContractOutput) error {
	// Build source code (concatenate all files for storage)
	var sourceBuilder strings.Builder
	for filename, content := range req.SourceFiles {
		sourceBuilder.WriteString(fmt.Sprintf("// File: %s\n", filename))
		sourceBuilder.WriteString(content)
		sourceBuilder.WriteString("\n\n")
	}
	sourceCode := sourceBuilder.String()

	return v.db.VerifyContract(ctx, req.Address, req.ContractName, req.CompilerVersion,
		req.OptimizationUsed, sourceCode, output.ABI, req.EVMVersion)
}

// ListCompilers returns the list of available compiler versions
func (v *Verifier) ListCompilers() []CompilerVersion {
	return v.solcManager.ListVersions()
}

// HasCompiler checks if a specific compiler version is available
func (v *Verifier) HasCompiler(version string) bool {
	return v.solcManager.HasVersion(version)
}

// VerifyFromJSON verifies a contract from a JSON request body
func (v *Verifier) VerifyFromJSON(ctx context.Context, data []byte) (*VerifyResponse, error) {
	var req VerifyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return &VerifyResponse{
			Error: fmt.Sprintf("invalid request: %v", err),
		}, nil
	}

	// Validate required fields
	if req.Address == "" {
		return &VerifyResponse{Error: "address is required"}, nil
	}

	// Convert legacy sourceCode to sourceFiles if needed
	if len(req.SourceFiles) == 0 && req.SourceCode != "" {
		filename := req.ContractName + ".sol"
		req.SourceFiles = map[string]string{filename: req.SourceCode}
		req.MainContractFile = filename
	}

	if len(req.SourceFiles) == 0 {
		return &VerifyResponse{Error: "sourceFiles or sourceCode is required"}, nil
	}
	if req.ContractName == "" {
		return &VerifyResponse{Error: "contractName is required"}, nil
	}
	if req.CompilerVersion == "" {
		return &VerifyResponse{Error: "compilerVersion is required"}, nil
	}

	// Set default main contract file if not specified
	if req.MainContractFile == "" {
		// Use the first .sol file
		for filename := range req.SourceFiles {
			if strings.HasSuffix(filename, ".sol") {
				req.MainContractFile = filename
				break
			}
		}
	}

	return v.Verify(ctx, &req)
}

// RefreshCompilers rescans for available compiler versions
func (v *Verifier) RefreshCompilers() {
	v.solcManager.Refresh()
}
