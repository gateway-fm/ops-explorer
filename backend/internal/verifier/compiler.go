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
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "ld-linux") || strings.Contains(stderrStr, "Dynamic loader not found") || strings.Contains(stderrStr, "multiarch") {
			return nil, fmt.Errorf("compiler binary for solc %s is not compatible with this platform architecture", c.version)
		}
		return nil, fmt.Errorf("compilation failed: %s", stderrStr)
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

func (c *Compiler) CompileStandardJSON(ctx context.Context, standardInput json.RawMessage) (*CompilerOutput, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(standardInput, &raw); err != nil {
		return nil, fmt.Errorf("invalid standard JSON input: %w", err)
	}

	if _, ok := raw["language"]; !ok {
		return nil, fmt.Errorf("standard JSON input missing 'language' field")
	}
	if _, ok := raw["sources"]; !ok {
		return nil, fmt.Errorf("standard JSON input missing 'sources' field")
	}

	raw = ensureOutputSelection(raw)

	inputJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal standard JSON input: %w", err)
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
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "ld-linux") || strings.Contains(stderrStr, "Dynamic loader not found") || strings.Contains(stderrStr, "multiarch") {
			return nil, fmt.Errorf("compiler binary for solc %s is not compatible with this platform architecture", c.version)
		}
		return nil, fmt.Errorf("compilation failed: %s", stderrStr)
	}

	var output CompilerOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("failed to parse compiler output: %w", err)
	}

	return &output, nil
}

func ensureOutputSelection(input map[string]json.RawMessage) map[string]json.RawMessage {
	requiredOutputs := []string{"abi", "evm.bytecode", "evm.deployedBytecode"}

	settingsRaw, hasSettings := input["settings"]
	var settings map[string]json.RawMessage
	if hasSettings {
		if err := json.Unmarshal(settingsRaw, &settings); err != nil {
			settings = make(map[string]json.RawMessage)
		}
	} else {
		settings = make(map[string]json.RawMessage)
	}

	osRaw, hasOS := settings["outputSelection"]
	var outputSelection map[string]map[string][]string
	if hasOS {
		if err := json.Unmarshal(osRaw, &outputSelection); err != nil {
			outputSelection = make(map[string]map[string][]string)
		}
	} else {
		outputSelection = make(map[string]map[string][]string)
	}

	if _, ok := outputSelection["*"]; !ok {
		outputSelection["*"] = make(map[string][]string)
	}

	existing := outputSelection["*"]["*"]
	existingSet := make(map[string]bool)
	for _, v := range existing {
		existingSet[v] = true
	}
	for _, req := range requiredOutputs {
		if !existingSet[req] {
			existing = append(existing, req)
		}
	}
	outputSelection["*"]["*"] = existing

	osJSON, _ := json.Marshal(outputSelection)
	settings["outputSelection"] = osJSON

	settingsJSON, _ := json.Marshal(settings)
	input["settings"] = settingsJSON

	return input
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
