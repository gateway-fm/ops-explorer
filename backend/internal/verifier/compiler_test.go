package verifier

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// findSolc looks for a solc binary in common locations.
// Returns the path to solc or an empty string if not found.
func findSolc() string {
	// Check /opt/solc/ for versioned binaries
	if entries, err := os.ReadDir("/opt/solc"); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				p := filepath.Join("/opt/solc", e.Name())
				if _, err := exec.LookPath(p); err == nil {
					return p
				}
			}
		}
	}

	// Check /usr/local/bin/solc
	if _, err := os.Stat("/usr/local/bin/solc"); err == nil {
		return "/usr/local/bin/solc"
	}

	// Check $PATH
	if p, err := exec.LookPath("solc"); err == nil {
		return p
	}

	return ""
}

// readContract reads a contract source file from the testdata directory.
func readContract(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read contract %s: %v", name, err)
	}
	return string(data)
}

func compileContract(t *testing.T, solcPath, filename, source string, optimized bool) *CompilerOutput {
	t.Helper()
	compiler := NewCompiler(solcPath, "test")
	sources := map[string]string{filename: source}
	output, err := compiler.CompileSource(context.Background(), sources, filename, "", optimized, 200, "", nil)
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}
	return output
}

func TestCompileCounter(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")
	output := compileContract(t, solcPath, "Counter.sol", source, false)

	if HasErrors(output) {
		t.Fatalf("compilation had errors: %v", GetErrors(output))
	}

	contract, err := GetContractOutput(output, "Counter.sol", "Counter")
	if err != nil {
		t.Fatalf("GetContractOutput failed: %v", err)
	}

	if contract.EVM.DeployedBytecode.Object == "" {
		t.Error("expected non-empty deployed bytecode")
	}
	if contract.EVM.Bytecode.Object == "" {
		t.Error("expected non-empty creation bytecode")
	}
	if len(contract.ABI) == 0 {
		t.Error("expected non-empty ABI")
	}
}

func TestCompileEventCounter(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "EventCounter.sol")
	output := compileContract(t, solcPath, "EventCounter.sol", source, false)

	if HasErrors(output) {
		t.Fatalf("compilation had errors: %v", GetErrors(output))
	}

	contract, err := GetContractOutput(output, "EventCounter.sol", "EventCounter")
	if err != nil {
		t.Fatalf("GetContractOutput failed: %v", err)
	}

	// Verify ABI contains event entries
	var abi []map[string]any
	if err := json.Unmarshal(contract.ABI, &abi); err != nil {
		t.Fatalf("failed to unmarshal ABI: %v", err)
	}

	eventCount := 0
	for _, entry := range abi {
		if entry["type"] == "event" {
			eventCount++
		}
	}
	if eventCount == 0 {
		t.Error("expected ABI to contain event entries")
	}
}

func TestCompileForwarder(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Forwarder.sol")
	output := compileContract(t, solcPath, "Forwarder.sol", source, false)

	if HasErrors(output) {
		t.Fatalf("compilation had errors: %v", GetErrors(output))
	}

	contract, err := GetContractOutput(output, "Forwarder.sol", "Forwarder")
	if err != nil {
		t.Fatalf("GetContractOutput failed: %v", err)
	}

	if contract.EVM.DeployedBytecode.Object == "" {
		t.Error("expected non-empty deployed bytecode")
	}
}

func TestCompileWithOptimization(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")

	outputNoOpt := compileContract(t, solcPath, "Counter.sol", source, false)
	outputWithOpt := compileContract(t, solcPath, "Counter.sol", source, true)

	if HasErrors(outputNoOpt) || HasErrors(outputWithOpt) {
		t.Fatal("compilation had errors")
	}

	contractNoOpt, err := GetContractOutput(outputNoOpt, "Counter.sol", "Counter")
	if err != nil {
		t.Fatalf("GetContractOutput (no opt) failed: %v", err)
	}
	contractWithOpt, err := GetContractOutput(outputWithOpt, "Counter.sol", "Counter")
	if err != nil {
		t.Fatalf("GetContractOutput (opt) failed: %v", err)
	}

	if contractNoOpt.EVM.DeployedBytecode.Object == contractWithOpt.EVM.DeployedBytecode.Object {
		t.Error("expected optimized and non-optimized bytecodes to differ")
	}
}

func TestCompileInvalidSource(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	invalidSource := `pragma solidity ^0.8.13; contract Broken { function foo( }`
	compiler := NewCompiler(solcPath, "test")
	sources := map[string]string{"Broken.sol": invalidSource}
	output, err := compiler.CompileSource(context.Background(), sources, "Broken.sol", "Broken", false, 200, "", nil)
	if err != nil {
		t.Fatalf("CompileSource returned unexpected error: %v", err)
	}

	if !HasErrors(output) {
		t.Error("expected HasErrors() to return true for invalid source")
	}

	errors := GetErrors(output)
	if len(errors) == 0 {
		t.Error("expected at least one error message")
	}
}

func TestGetContractOutput(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")
	output := compileContract(t, solcPath, "Counter.sol", source, false)

	contract, err := GetContractOutput(output, "Counter.sol", "Counter")
	if err != nil {
		t.Fatalf("GetContractOutput failed: %v", err)
	}

	if len(contract.ABI) == 0 {
		t.Error("expected ABI to be populated")
	}
	if contract.EVM.DeployedBytecode.Object == "" {
		t.Error("expected deployed bytecode to be populated")
	}
	if contract.EVM.Bytecode.Object == "" {
		t.Error("expected creation bytecode to be populated")
	}
}

func TestGetContractOutputNotFound(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")
	output := compileContract(t, solcPath, "Counter.sol", source, false)

	_, err := GetContractOutput(output, "Counter.sol", "NonExistentContract")
	if err == nil {
		t.Error("expected error for non-existent contract name")
	}
}

func TestCompileStandardJSON(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")
	stdInput := map[string]any{
		"language": "Solidity",
		"sources": map[string]any{
			"Counter.sol": map[string]any{
				"content": source,
			},
		},
		"settings": map[string]any{
			"optimizer": map[string]any{
				"enabled": false,
				"runs":    200,
			},
			"outputSelection": map[string]any{
				"*": map[string]any{
					"*": []string{"abi", "evm.bytecode", "evm.deployedBytecode"},
				},
			},
		},
	}

	inputJSON, err := json.Marshal(stdInput)
	if err != nil {
		t.Fatalf("failed to marshal standard JSON input: %v", err)
	}

	compiler := NewCompiler(solcPath, "test")
	output, err := compiler.CompileStandardJSON(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("CompileStandardJSON failed: %v", err)
	}

	if HasErrors(output) {
		t.Fatalf("compilation had errors: %v", GetErrors(output))
	}

	contract, err := GetContractOutput(output, "Counter.sol", "Counter")
	if err != nil {
		t.Fatalf("GetContractOutput failed: %v", err)
	}

	if contract.EVM.DeployedBytecode.Object == "" {
		t.Error("expected non-empty deployed bytecode")
	}
	if contract.EVM.Bytecode.Object == "" {
		t.Error("expected non-empty creation bytecode")
	}
	if len(contract.ABI) == 0 {
		t.Error("expected non-empty ABI")
	}
}

// TestCompileStandardJSONMatchesCompileSource verifies that CompileStandardJSON
// produces identical bytecode to CompileSource when given the same settings.
// This is the core invariant for standard JSON verification — if it breaks,
// contracts deployed via forge/hardhat won't match when verified with their JSON input.
func TestCompileStandardJSONMatchesCompileSource(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")

	tests := []struct {
		name      string
		optimized bool
		runs      int
	}{
		{"no optimization", false, 200},
		{"with optimization", true, 200},
		{"optimization 1000 runs", true, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := NewCompiler(solcPath, "test")

			// Compile via CompileSource
			sourceOutput, err := compiler.CompileSource(
				context.Background(),
				map[string]string{"Counter.sol": source},
				"Counter.sol", "Counter",
				tt.optimized, tt.runs, "", nil,
			)
			if err != nil {
				t.Fatalf("CompileSource failed: %v", err)
			}

			// Compile via CompileStandardJSON with identical settings
			stdInput := map[string]any{
				"language": "Solidity",
				"sources": map[string]any{
					"Counter.sol": map[string]any{
						"content": source,
					},
				},
				"settings": map[string]any{
					"optimizer": map[string]any{
						"enabled": tt.optimized,
						"runs":    tt.runs,
					},
					"outputSelection": map[string]any{
						"*": map[string]any{
							"*": []string{"abi", "evm.bytecode", "evm.deployedBytecode"},
						},
					},
				},
			}
			inputJSON, _ := json.Marshal(stdInput)

			jsonOutput, err := compiler.CompileStandardJSON(context.Background(), inputJSON)
			if err != nil {
				t.Fatalf("CompileStandardJSON failed: %v", err)
			}

			if HasErrors(sourceOutput) {
				t.Fatalf("CompileSource had errors: %v", GetErrors(sourceOutput))
			}
			if HasErrors(jsonOutput) {
				t.Fatalf("CompileStandardJSON had errors: %v", GetErrors(jsonOutput))
			}

			sourceContract, err := GetContractOutput(sourceOutput, "Counter.sol", "Counter")
			if err != nil {
				t.Fatalf("GetContractOutput (source) failed: %v", err)
			}
			jsonContract, err := GetContractOutput(jsonOutput, "Counter.sol", "Counter")
			if err != nil {
				t.Fatalf("GetContractOutput (json) failed: %v", err)
			}

			if sourceContract.EVM.DeployedBytecode.Object != jsonContract.EVM.DeployedBytecode.Object {
				t.Errorf("deployed bytecode mismatch:\n  CompileSource:       %d bytes\n  CompileStandardJSON: %d bytes",
					len(sourceContract.EVM.DeployedBytecode.Object)/2,
					len(jsonContract.EVM.DeployedBytecode.Object)/2,
				)
			}

			if sourceContract.EVM.Bytecode.Object != jsonContract.EVM.Bytecode.Object {
				t.Errorf("creation bytecode mismatch:\n  CompileSource:       %d bytes\n  CompileStandardJSON: %d bytes",
					len(sourceContract.EVM.Bytecode.Object)/2,
					len(jsonContract.EVM.Bytecode.Object)/2,
				)
			}
		})
	}
}

func TestCompileStandardJSONMissingOutputSelection(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")
	stdInput := map[string]any{
		"language": "Solidity",
		"sources": map[string]any{
			"Counter.sol": map[string]any{
				"content": source,
			},
		},
		"settings": map[string]any{
			"optimizer": map[string]any{
				"enabled": false,
				"runs":    200,
			},
		},
	}

	inputJSON, err := json.Marshal(stdInput)
	if err != nil {
		t.Fatalf("failed to marshal standard JSON input: %v", err)
	}

	compiler := NewCompiler(solcPath, "test")
	output, err := compiler.CompileStandardJSON(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("CompileStandardJSON failed: %v", err)
	}

	if HasErrors(output) {
		t.Fatalf("compilation had errors: %v", GetErrors(output))
	}

	contract, err := GetContractOutput(output, "Counter.sol", "Counter")
	if err != nil {
		t.Fatalf("GetContractOutput failed: %v", err)
	}

	if contract.EVM.DeployedBytecode.Object == "" {
		t.Error("expected non-empty deployed bytecode (ensureOutputSelection should have patched it)")
	}
	if len(contract.ABI) == 0 {
		t.Error("expected non-empty ABI (ensureOutputSelection should have patched it)")
	}
}

func TestCompileStandardJSONMissingLanguage(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	stdInput := map[string]any{
		"sources": map[string]any{
			"Counter.sol": map[string]any{"content": "pragma solidity ^0.8.13;"},
		},
	}
	inputJSON, _ := json.Marshal(stdInput)

	compiler := NewCompiler(solcPath, "test")
	_, err := compiler.CompileStandardJSON(context.Background(), inputJSON)
	if err == nil {
		t.Error("expected error for missing 'language' field")
	}
}

func TestCompileStandardJSONMissingSources(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	stdInput := map[string]any{
		"language": "Solidity",
	}
	inputJSON, _ := json.Marshal(stdInput)

	compiler := NewCompiler(solcPath, "test")
	_, err := compiler.CompileStandardJSON(context.Background(), inputJSON)
	if err == nil {
		t.Error("expected error for missing 'sources' field")
	}
}

func TestCompileStandardJSONInvalidJSON(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	compiler := NewCompiler(solcPath, "test")
	_, err := compiler.CompileStandardJSON(context.Background(), []byte(`{not valid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCompileStandardJSONNoSettings(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	source := readContract(t, "Counter.sol")
	// No settings at all — ensureOutputSelection should create them
	stdInput := map[string]any{
		"language": "Solidity",
		"sources": map[string]any{
			"Counter.sol": map[string]any{
				"content": source,
			},
		},
	}

	inputJSON, _ := json.Marshal(stdInput)

	compiler := NewCompiler(solcPath, "test")
	output, err := compiler.CompileStandardJSON(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("CompileStandardJSON failed: %v", err)
	}

	if HasErrors(output) {
		t.Fatalf("compilation had errors: %v", GetErrors(output))
	}

	contract, err := GetContractOutput(output, "Counter.sol", "Counter")
	if err != nil {
		t.Fatalf("GetContractOutput failed: %v", err)
	}

	if contract.EVM.DeployedBytecode.Object == "" {
		t.Error("expected non-empty deployed bytecode")
	}
}
