package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Compiler struct {
	path    string
	version string
	timeout time.Duration
}

func NewCompiler(path, version string) *Compiler {
	return &Compiler{
		path:    path,
		version: version,
		timeout: 60 * time.Second,
	}
}

func (c *Compiler) Compile(ctx context.Context, input *CompilerInput) (*CompilerOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compiler input: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.path, "--standard-json")
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("compilation timed out")
		}
		return nil, fmt.Errorf("compilation failed: %s", stderr.String())
	}

	var output CompilerOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("failed to parse compiler output: %w", err)
	}

	return &output, nil
}

func (c *Compiler) CompileSource(ctx context.Context, sources map[string]string, mainFile, contractName string, optimized bool, runs int, evmVersion string, libraries map[string]string) (*CompilerOutput, error) {
	input := &CompilerInput{
		Language: "Solidity",
		Sources:  make(map[string]CompilerInputSource),
		Settings: CompilerSettings{
			Optimizer: OptimizerSettings{
				Enabled: optimized,
				Runs:    runs,
			},
			OutputSelection: map[string]map[string][]string{
				"*": {
					"*": []string{"abi", "evm.bytecode", "evm.deployedBytecode"},
				},
			},
		},
	}

	for filename, content := range sources {
		input.Sources[filename] = CompilerInputSource{Content: content}
	}

	if evmVersion != "" {
		input.Settings.EVMVersion = evmVersion
	}

	if len(libraries) > 0 {
		input.Settings.Libraries = make(map[string]map[string]string)
		for name, addr := range libraries {
			// Parse library name (could be "Contract:Library" or just "Library")
			parts := strings.Split(name, ":")
			var fileName, libName string
			if len(parts) == 2 {
				fileName = parts[0]
				libName = parts[1]
			} else {
				fileName = mainFile
				libName = name
			}

			if input.Settings.Libraries[fileName] == nil {
				input.Settings.Libraries[fileName] = make(map[string]string)
			}
			input.Settings.Libraries[fileName][libName] = addr
		}
	}

	return c.Compile(ctx, input)
}

func GetContractOutput(output *CompilerOutput, mainFile, contractName string) (*ContractOutput, error) {
	if contracts, ok := output.Contracts[mainFile]; ok {
		if contract, ok := contracts[contractName]; ok {
			return &contract, nil
		}
	}

	mainFileNoExt := strings.TrimSuffix(mainFile, ".sol")
	if contracts, ok := output.Contracts[mainFileNoExt]; ok {
		if contract, ok := contracts[contractName]; ok {
			return &contract, nil
		}
	}

	for _, contracts := range output.Contracts {
		if contract, ok := contracts[contractName]; ok {
			return &contract, nil
		}
	}

	return nil, fmt.Errorf("contract %s not found in compilation output", contractName)
}

func HasErrors(output *CompilerOutput) bool {
	for _, err := range output.Errors {
		if err.Severity == "error" {
			return true
		}
	}
	return false
}

func GetErrors(output *CompilerOutput) []string {
	var errors []string
	for _, err := range output.Errors {
		if err.Severity == "error" {
			errors = append(errors, err.FormattedMessage)
		}
	}
	return errors
}

func GetWarnings(output *CompilerOutput) []string {
	var warnings []string
	for _, err := range output.Errors {
		if err.Severity == "warning" {
			warnings = append(warnings, err.FormattedMessage)
		}
	}
	return warnings
}
