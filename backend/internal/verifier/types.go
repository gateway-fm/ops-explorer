package verifier

import "encoding/json"

// VerifyRequest represents a contract verification request
type VerifyRequest struct {
	Address          string            `json:"address"`
	SourceFiles      map[string]string `json:"sourceFiles"`      // filename -> content (preferred)
	SourceCode       string            `json:"sourceCode"`       // single file source (legacy, converted to sourceFiles)
	MainContractFile string            `json:"mainContractFile"` // e.g., "Token.sol"
	ContractName     string            `json:"contractName"`     // e.g., "Token"
	CompilerVersion  string            `json:"compilerVersion"`  // e.g., "0.8.19"
	OptimizationUsed bool              `json:"optimizationUsed"`
	OptimizationRuns int               `json:"optimizationRuns"` // e.g., 200
	ConstructorArgs  string            `json:"constructorArgs,omitempty"` // hex-encoded
	EVMVersion       string            `json:"evmVersion,omitempty"`      // e.g., "london"
	Libraries        map[string]string `json:"libraries,omitempty"`       // library name -> address
}

// VerifyResponse represents the result of a verification attempt
type VerifyResponse struct {
	Success          bool            `json:"success"`
	Address          string          `json:"address"`
	ContractName     string          `json:"contractName,omitempty"`
	CompilerVersion  string          `json:"compilerVersion,omitempty"`
	MatchType        MatchType       `json:"matchType,omitempty"` // exact, partial, none
	ABI              json.RawMessage `json:"abi,omitempty"`
	Error            string          `json:"error,omitempty"`
	BytecodeHash     string          `json:"bytecodeHash,omitempty"`
	CompiledHash     string          `json:"compiledHash,omitempty"`
}

// MatchType represents the type of bytecode match
type MatchType string

const (
	MatchTypeExact   MatchType = "exact"   // Exact bytecode match
	MatchTypePartial MatchType = "partial" // Match except metadata hash
	MatchTypeNone    MatchType = "none"    // No match
)

// CompilerInput represents the standard-json input for solc
type CompilerInput struct {
	Language string                         `json:"language"`
	Sources  map[string]CompilerInputSource `json:"sources"`
	Settings CompilerSettings               `json:"settings"`
}

// CompilerInputSource represents a source file in the compiler input
type CompilerInputSource struct {
	Content string `json:"content"`
}

// CompilerSettings represents the compiler settings
type CompilerSettings struct {
	Optimizer      OptimizerSettings `json:"optimizer"`
	EVMVersion     string            `json:"evmVersion,omitempty"`
	Libraries      map[string]map[string]string `json:"libraries,omitempty"`
	OutputSelection map[string]map[string][]string `json:"outputSelection"`
}

// OptimizerSettings represents the optimizer settings
type OptimizerSettings struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
}

// CompilerOutput represents the output from solc compilation
type CompilerOutput struct {
	Contracts map[string]map[string]ContractOutput `json:"contracts"`
	Errors    []CompilerError                       `json:"errors,omitempty"`
}

// ContractOutput represents the output for a single contract
type ContractOutput struct {
	ABI json.RawMessage `json:"abi"`
	EVM EVMOutput       `json:"evm"`
}

// EVMOutput represents the EVM-related output
type EVMOutput struct {
	Bytecode         BytecodeOutput `json:"bytecode"`
	DeployedBytecode BytecodeOutput `json:"deployedBytecode"`
}

// BytecodeOutput represents bytecode output
type BytecodeOutput struct {
	Object         string `json:"object"`
	SourceMap      string `json:"sourceMap,omitempty"`
	LinkReferences map[string]map[string][]LinkReference `json:"linkReferences,omitempty"`
}

// LinkReference represents a library link reference
type LinkReference struct {
	Start  int `json:"start"`
	Length int `json:"length"`
}

// CompilerError represents a compilation error or warning
type CompilerError struct {
	Component        string `json:"component"`
	ErrorCode        string `json:"errorCode"`
	FormattedMessage string `json:"formattedMessage"`
	Message          string `json:"message"`
	Severity         string `json:"severity"` // error, warning
	Type             string `json:"type"`
}

// CompilerVersion represents a solc version
type CompilerVersion struct {
	Version  string `json:"version"`  // e.g., "0.8.19"
	Path     string `json:"path"`     // Path to the binary
	LongVersion string `json:"longVersion,omitempty"` // Full version string
}

// CompilerListResponse represents the response for listing available compilers
type CompilerListResponse struct {
	Versions []CompilerVersion `json:"versions"`
}
