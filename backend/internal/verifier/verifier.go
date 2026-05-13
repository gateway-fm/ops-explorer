package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"explorer/pkg/log"

	"explorer/pkg/eth/common"
)

type Verifier struct {
	db                  Database
	rpc                 RPCClient
	solcManager         *SolcManager
	useSourcifyFallback bool
}

type Config struct {
	SolcPath            string
	UseSourcifyFallback bool
}

func NewVerifier(database Database, rpcClient RPCClient, cfg *Config) *Verifier {
	return &Verifier{
		db:                  database,
		rpc:                 rpcClient,
		solcManager:         NewSolcManager(cfg.SolcPath),
		useSourcifyFallback: cfg.UseSourcifyFallback,
	}
}

func (v *Verifier) Verify(ctx context.Context, req *VerifyRequest) (*VerifyResponse, error) {
	response := &VerifyResponse{
		Address:      req.Address,
		ContractName: req.ContractName,
	}

	if !common.IsHexAddress(req.Address) {
		response.Error = "invalid address"
		return response, nil
	}
	address := common.HexToAddress(req.Address)

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

	compilerPath, err := v.solcManager.GetCompiler(req.CompilerVersion)
	if err != nil {
		response.Error = fmt.Sprintf("compiler version %s not available: %v", req.CompilerVersion, err)
		return response, nil
	}

	compiler := NewCompiler(compilerPath, req.CompilerVersion)

	runs := req.OptimizationRuns
	if runs == 0 {
		runs = 200
	}

	output, err := compiler.CompileSource(ctx, req.SourceFiles, req.MainContractFile, req.ContractName,
		req.OptimizationUsed, runs, req.EVMVersion, req.Libraries)
	if err != nil {
		response.Error = fmt.Sprintf("compilation failed: %v", err)
		return response, nil
	}

	if HasErrors(output) {
		errors := GetErrors(output)
		response.Error = fmt.Sprintf("compilation errors: %s", strings.Join(errors, "; "))
		return response, nil
	}

	contractOutput, err := GetContractOutput(output, req.MainContractFile, req.ContractName)
	if err != nil {
		response.Error = err.Error()
		return response, nil
	}

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

	response.Success = true
	response.ABI = contractOutput.ABI

	if err := v.storeVerification(ctx, req, contractOutput); err != nil {
		log.Warn("failed to store verification result", "error", err)
	}

	return response, nil
}

func (v *Verifier) storeVerification(ctx context.Context, req *VerifyRequest, output *ContractOutput) error {
	var sourceBuilder strings.Builder
	for filename, content := range req.SourceFiles {
		sourceBuilder.WriteString(fmt.Sprintf("// File: %s\n", filename))
		sourceBuilder.WriteString(content)
		sourceBuilder.WriteString("\n\n")
	}
	sourceCode := sourceBuilder.String()

	return v.db.VerifyContract(ctx, req.Address, req.ContractName, req.CompilerVersion,
		req.OptimizationUsed, sourceCode, output.ABI, req.EVMVersion,
		req.LicenseType, req.ConstructorArgs, req.OptimizationRuns)
}

func (v *Verifier) VerifyStandardJSON(ctx context.Context, data []byte) (*VerifyResponse, error) {
	var req StandardJSONVerifyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return &VerifyResponse{
			Error: fmt.Sprintf("invalid request: %v", err),
		}, nil
	}

	if req.Address == "" {
		return &VerifyResponse{Error: "address is required"}, nil
	}
	if req.CompilerVersion == "" {
		return &VerifyResponse{Error: "compilerVersion is required"}, nil
	}
	if req.ContractName == "" {
		return &VerifyResponse{Error: "contractName is required"}, nil
	}
	if len(req.StandardInput) == 0 {
		return &VerifyResponse{Error: "standardInput is required"}, nil
	}

	return v.verifyWithStandardInput(ctx, &req)
}

func (v *Verifier) verifyWithStandardInput(ctx context.Context, req *StandardJSONVerifyRequest) (*VerifyResponse, error) {
	response := &VerifyResponse{
		Address:      req.Address,
		ContractName: req.ContractName,
	}

	if !common.IsHexAddress(req.Address) {
		response.Error = "invalid address"
		return response, nil
	}
	address := common.HexToAddress(req.Address)

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

	compilerPath, err := v.solcManager.GetCompiler(req.CompilerVersion)
	if err != nil {
		response.Error = fmt.Sprintf("compiler version %s not available: %v", req.CompilerVersion, err)
		return response, nil
	}

	compiler := NewCompiler(compilerPath, req.CompilerVersion)

	output, err := compiler.CompileStandardJSON(ctx, req.StandardInput)
	if err != nil {
		response.Error = fmt.Sprintf("compilation failed: %v", err)
		return response, nil
	}

	if HasErrors(output) {
		errors := GetErrors(output)
		response.Error = fmt.Sprintf("compilation errors: %s", strings.Join(errors, "; "))
		return response, nil
	}

	contractOutput, err := GetContractOutput(output, req.ContractFile, req.ContractName)
	if err != nil {
		response.Error = err.Error()
		return response, nil
	}

	compiledBytecode := contractOutput.EVM.DeployedBytecode.Object
	comparison := CompareBytecode(onChainBytecode, compiledBytecode, req.ConstructorArgs)

	response.MatchType = comparison.MatchType
	response.BytecodeHash = comparison.OnChainHash
	response.CompiledHash = comparison.CompiledHash
	response.CompilerVersion = req.CompilerVersion

	if comparison.MatchType == MatchTypeNone {
		response.Error = fmt.Sprintf("bytecode does not match (on-chain: %d bytes, compiled: %d bytes)", comparison.OnChainLength, comparison.CompiledLength)
		log.Warn("bytecode mismatch (standard JSON)",
			"address", req.Address,
			"onChainLen", comparison.OnChainLength,
			"compiledLen", comparison.CompiledLength,
			"onChainHash", comparison.OnChainHash,
			"compiledHash", comparison.CompiledHash,
		)
		return response, nil
	}

	response.Success = true
	response.ABI = contractOutput.ABI

	if err := v.storeStandardJSONVerification(ctx, req, contractOutput); err != nil {
		log.Warn("failed to store standard JSON verification result", "error", err)
	}

	return response, nil
}

func (v *Verifier) storeStandardJSONVerification(ctx context.Context, req *StandardJSONVerifyRequest, output *ContractOutput) error {
	sourceCode := string(req.StandardInput)

	// Extract optimizer/EVM settings from the standard JSON input
	var input struct {
		Settings struct {
			Optimizer struct {
				Enabled bool `json:"enabled"`
				Runs    int  `json:"runs"`
			} `json:"optimizer"`
			EVMVersion string `json:"evmVersion"`
		} `json:"settings"`
	}
	json.Unmarshal(req.StandardInput, &input)

	optimizationRuns := input.Settings.Optimizer.Runs
	if optimizationRuns == 0 {
		optimizationRuns = 200
	}

	return v.db.VerifyContract(ctx, req.Address, req.ContractName, req.CompilerVersion,
		input.Settings.Optimizer.Enabled, sourceCode, output.ABI, input.Settings.EVMVersion,
		req.LicenseType, req.ConstructorArgs, optimizationRuns)
}

func (v *Verifier) ListCompilers() []CompilerVersion {
	return v.solcManager.ListVersions()
}

func (v *Verifier) HasCompiler(version string) bool {
	return v.solcManager.HasVersion(version)
}

func (v *Verifier) VerifyFromJSON(ctx context.Context, data []byte) (*VerifyResponse, error) {
	var req VerifyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return &VerifyResponse{
			Error: fmt.Sprintf("invalid request: %v", err),
		}, nil
	}

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

	if req.MainContractFile == "" {
		for filename := range req.SourceFiles {
			if strings.HasSuffix(filename, ".sol") {
				req.MainContractFile = filename
				break
			}
		}
	}

	return v.Verify(ctx, &req)
}

func (v *Verifier) RefreshCompilers() {
	v.solcManager.Refresh()
}
